package chainevent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

const (
	reorgMaterializationAlgorithm         = "joincare-reorg-materialization-prepare-v1"
	reorgMaterializationStatePreparing    = "preparing"
	reorgMaterializationStateCompleted    = "completed"
	reorgMaterializationStateInvalidated  = "invalidated"
	reorgMaterializationBranchOld         = "old_committed"
	reorgMaterializationBranchReplacement = "replacement"
	reorgMaterializationBranchDone        = "done"
	reorgMaterializationEffectSemantic    = "semantic_event"
	reorgMaterializationEffectIgnored     = "ignored_abi_log"
	reorgV2PaymentTokenConfigured         = "PaymentTokenConfigured"
	reorgV2ProjectTokenConfigured         = "ProjectTokenConfigured"
	reorgV2PlatformFeesUpdated            = "PlatformFeesUpdated"
	reorgMaterializationLeaseTTL          = 2 * time.Minute
	reorgMaterializationPageSize          = 100
	reorgMaterializationMaxSourceRows     = 50_000
	reorgMaterializationMaxSourcePayload  = 12 * 1024 * 1024
)

type ReorgMaterializationJob struct{}

func NewReorgMaterializationJob() model.Add {
	var job cron.Job = new(ReorgMaterializationJob)
	return job.(model.Add)
}

func (j *ReorgMaterializationJob) Add(crontab *cron.Cron) {
	crontab.AddJob("7/10 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

func (j *ReorgMaterializationJob) Run() {
	if !chain.V2Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	outerToken, err := newRunLockToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg materialization outer token err: %s", err)
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken, reorgMaterializationLeaseTTL)
	if err != nil || !locked {
		return
	}
	defer func() {
		_ = db.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken)
	}()

	run, job, err := ensureMaterializationRun()
	if errors.Is(err, errMaterializationAlreadyComplete) || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("prepare reorg materialization run err: %s", err)
		return
	}
	leaseToken, err := newMaterializationLeaseToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg materialization lease err: %s", err)
		return
	}
	job, err = claimMaterializationLease(run.ID, leaseToken)
	if err != nil {
		return
	}
	if err := processMaterializationPage(ctx, run, job, leaseToken); err != nil {
		_ = recordMaterializationError(run.ID, leaseToken, job.FenceEpoch, "MATERIALIZATION_PREPARE_FAILED", err)
		log.GetLogger().Errorf("prepare reorg materialization page run=%d err: %s", run.ID, err)
	}
}

var errMaterializationAlreadyComplete = errors.New("reorg materialization preparation is complete")

func ensureMaterializationRun() (*entity.ChainEventReorgMaterializationRun, *entity.ChainEventReorgMaterializationJob, error) {
	incident, err := activeRecoveryIncident()
	if err != nil {
		return nil, nil, err
	}
	recovery := entity.ChainEventReorgRecoveryRun{}
	if err := db.Mysql().Joins("JOIN chain_event_reorg_recovery_job j ON j.run_id=chain_event_reorg_recovery_run.id AND j.state=?", reorgRecoveryStateDone).
		Joins("JOIN chain_event_reorg_recovery_manifest m ON m.run_id=chain_event_reorg_recovery_run.id").
		Where("chain_event_reorg_recovery_run.incident_id = ?", incident.ID).Take(&recovery).Error; err != nil {
		return nil, nil, err
	}
	manifest := entity.ChainEventReorgRecoveryManifest{}
	if err := db.Mysql().Where("run_id = ? AND incident_id = ?", recovery.ID, incident.ID).Take(&manifest).Error; err != nil {
		return nil, nil, err
	}
	existing := entity.ChainEventReorgMaterializationRun{}
	err = db.Mysql().Where("recovery_run_id = ?", recovery.ID).Take(&existing).Error
	if err == nil {
		job := entity.ChainEventReorgMaterializationJob{}
		if err := db.Mysql().Where("run_id = ?", existing.ID).Take(&job).Error; err != nil {
			return nil, nil, err
		}
		if job.State != reorgMaterializationStatePreparing {
			return nil, nil, errMaterializationAlreadyComplete
		}
		return &existing, &job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	run := &entity.ChainEventReorgMaterializationRun{
		RunUUID: strings.ToLower(uuid.NewString()), RecoveryRunID: recovery.ID, IncidentID: incident.ID,
		AlgorithmVersion: reorgMaterializationAlgorithm, RecoveryManifestDigest: manifest.ManifestDigest,
		ChainID: recovery.ChainID, ContractAddress: recovery.ContractAddress, LegacyContractAddress: recovery.LegacyContractAddress,
		LegacySourceRegistered: recovery.LegacySourceRegistered,
		BoundaryBlock:          recovery.BoundaryBlock, BoundaryBlockHash: recovery.BoundaryBlockHash,
		OldCursorBlock: incident.CursorBlock, OldCursorBlockHash: incident.ExpectedBlockHash,
		TargetSafeBlock: recovery.TargetSafeBlock, TargetSafeBlockHash: recovery.TargetSafeBlockHash,
		ExpectedReplacementLogCount: manifest.LogCount, ReplacementLogSourceDigest: manifest.LogEvidenceDigest,
	}
	job := &entity.ChainEventReorgMaterializationJob{}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requireMaterializationInputsTx(tx, *incident, recovery, manifest); err != nil {
			return err
		}
		oldCount, oldBytes, err := oldMaterializationSourceSizeTx(tx, recovery.BoundaryBlock, incident.CursorBlock, recovery.LegacyContractAddress, recovery.ContractAddress)
		if err != nil {
			return err
		}
		if oldCount > reorgMaterializationMaxSourceRows || manifest.LogCount > reorgMaterializationMaxSourceRows {
			return errors.New("reorg materialization source row limit exceeded")
		}
		var replacementPayloadBytes uint64
		if err := tx.Model(&entity.ChainEventReorgRecoveryLog{}).Where("run_id = ?", recovery.ID).Select("COALESCE(SUM(OCTET_LENGTH(raw_log)),0)").Scan(&replacementPayloadBytes).Error; err != nil {
			return err
		}
		if replacementPayloadBytes > reorgMaterializationMaxSourcePayload {
			return errors.New("reorg materialization replacement source payload limit exceeded")
		}
		if oldBytes > reorgMaterializationMaxSourcePayload {
			return errors.New("reorg materialization old source payload limit exceeded")
		}
		sourceDigest, err := calculateOldMaterializationSourceDigestTx(tx, recovery.BoundaryBlock, incident.CursorBlock, recovery.LegacyContractAddress, recovery.ContractAddress)
		if err != nil {
			return err
		}
		run.ExpectedOldRecordCount = oldCount
		run.OldRecordSourceDigest = sourceDigest
		run.CreatedAt = time.Now()
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		if err := copyOldMaterializationSourceTx(tx, run); err != nil {
			return err
		}
		active := uint8(1)
		*job = entity.ChainEventReorgMaterializationJob{
			RunID: run.ID, IncidentID: incident.ID, State: reorgMaterializationStatePreparing, ActiveSlot: &active,
			NextBranch: reorgMaterializationBranchOld, NextOldBlock: recovery.BoundaryBlock + 1,
			NextReplacementOrdinal: 1, NextEffectOrdinal: 1, NextBranchOrdinal: 1,
			OldEffectDigest: reorgZeroDigest, ReplacementEffectDigest: reorgZeroDigest, UpdatedAt: time.Now(),
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return appendMaterializationEventTx(tx, run.ID, "CREATED", "old_branch", "", "pinned completed recovery manifest and old committed event range")
	})
	return run, job, err
}

func requireMaterializationInputsTx(tx *gorm.DB, incident entity.ChainEventReorgIncident, recovery entity.ChainEventReorgRecoveryRun, manifest entity.ChainEventReorgRecoveryManifest) error {
	active := entity.ChainEventReorgIncident{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND active_slot = 1 AND status = ?", incident.ID, reorgStatusDetected).Take(&active).Error; err != nil {
		return err
	}
	state := entity.ChainEventSyncState{}
	if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", incident.CursorBlock, incident.ExpectedBlockHash).Take(&state).Error; err != nil {
		return errors.New("chain event cursor changed before materialization preparation")
	}
	if recovery.IncidentID != incident.ID || recovery.BoundaryBlock >= incident.CursorBlock || recovery.TargetSafeBlock < incident.CursorBlock || manifest.RunID != recovery.ID || manifest.IncidentID != incident.ID || manifest.ManifestDigest != recoveryManifestDigest(recovery.ID, incident.ID, uint64(manifest.BatchCount), manifest.LogCount, manifest.BatchEvidenceDigest, manifest.LogEvidenceDigest) {
		return errors.New("completed reorg recovery manifest identity is invalid")
	}
	return nil
}

func oldRecordSourceDigest(rows []entity.ChainEventRecord) string {
	digest := reorgZeroDigest
	for index, row := range rows {
		digest = oldRecordSourceRowDigest(uint64(index+1), row, digest)
	}
	return digest
}

func oldRecordSourceRowDigest(ordinal uint64, row entity.ChainEventRecord, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-old-record-source.v1", ordinal, row.ID, row.Event, row.BlockNumber, strings.ToLower(row.BlockHash), strings.ToLower(row.TxHash), row.LogIndex, strings.ToLower(row.ContractAddress), row.ContractVersion, sha256Hex([]byte(row.Data)), previous)
}

func oldMaterializationSourceQuery(tx *gorm.DB, boundary, cursor uint64, legacyAddress, v2Address string) *gorm.DB {
	query := tx.Model(&entity.ChainEventRecord{}).Where("block_number > ? AND block_number <= ? AND contract_version = ? AND LOWER(contract_address) = ?", boundary, cursor, constant.ContractVersionV2Stablecoin, strings.ToLower(v2Address))
	if strings.TrimSpace(legacyAddress) != "" {
		query = tx.Model(&entity.ChainEventRecord{}).Where("block_number > ? AND block_number <= ? AND ((contract_version = ? AND LOWER(contract_address) = ?) OR (contract_version = ? AND LOWER(contract_address) = ?))", boundary, cursor, constant.ContractVersionV1Legacy, strings.ToLower(legacyAddress), constant.ContractVersionV2Stablecoin, strings.ToLower(v2Address))
	}
	return query
}

func oldMaterializationSourceSizeTx(tx *gorm.DB, boundary, cursor uint64, legacyAddress, v2Address string) (uint64, uint64, error) {
	var size struct {
		Count uint64 `gorm:"column:row_count"`
		Bytes uint64 `gorm:"column:payload_bytes"`
	}
	err := oldMaterializationSourceQuery(tx, boundary, cursor, legacyAddress, v2Address).
		Select("COUNT(*) AS row_count,COALESCE(SUM(OCTET_LENGTH(data)),0) AS payload_bytes").Scan(&size).Error
	return size.Count, size.Bytes, err
}

func eachOldMaterializationSourcePageTx(tx *gorm.DB, boundary, cursor uint64, legacyAddress, v2Address string, handle func([]entity.ChainEventRecord) error) error {
	lastBlock, lastID := boundary, uint64(0)
	for {
		rows := make([]entity.ChainEventRecord, 0, reorgMaterializationPageSize)
		query := oldMaterializationSourceQuery(tx, boundary, cursor, legacyAddress, v2Address).
			Clauses(clause.Locking{Strength: "UPDATE"})
		if lastID != 0 {
			query = query.Where("block_number > ? OR (block_number = ? AND id > ?)", lastBlock, lastBlock, lastID)
		}
		if err := query.Order("block_number ASC,id ASC").Limit(reorgMaterializationPageSize).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := handle(rows); err != nil {
			return err
		}
		last := rows[len(rows)-1]
		lastBlock, lastID = uint64(last.BlockNumber), last.ID
	}
}

func calculateOldMaterializationSourceDigestTx(tx *gorm.DB, boundary, cursor uint64, legacyAddress, v2Address string) (string, error) {
	digest, ordinal := reorgZeroDigest, uint64(1)
	err := eachOldMaterializationSourcePageTx(tx, boundary, cursor, legacyAddress, v2Address, func(rows []entity.ChainEventRecord) error {
		for _, row := range rows {
			digest = oldRecordSourceRowDigest(ordinal, row, digest)
			ordinal++
		}
		return nil
	})
	return digest, err
}

func copyOldMaterializationSourceTx(tx *gorm.DB, run *entity.ChainEventReorgMaterializationRun) error {
	previous, ordinal := reorgZeroDigest, uint64(1)
	return eachOldMaterializationSourcePageTx(tx, run.BoundaryBlock, run.OldCursorBlock, run.LegacyContractAddress, run.ContractAddress, func(rows []entity.ChainEventRecord) error {
		for _, row := range rows {
			rowDigest := oldRecordSourceRowDigest(ordinal, row, previous)
			snapshot := entity.ChainEventReorgMaterializationOldSource{RunID: run.ID, Ordinal: ordinal, ChainEventRecordID: row.ID, Event: row.Event, BlockNumber: uint64(row.BlockNumber), BlockHash: strings.ToLower(row.BlockHash), TxHash: strings.ToLower(row.TxHash), LogIndex: row.LogIndex, ContractAddress: strings.ToLower(row.ContractAddress), ContractVersion: row.ContractVersion, EventData: []byte(row.Data), EventDataDigest: sha256Hex([]byte(row.Data)), PreviousRowDigest: previous, RowDigest: rowDigest, CreatedAt: time.Now()}
			if err := tx.Create(&snapshot).Error; err != nil {
				return err
			}
			previous, ordinal = rowDigest, ordinal+1
		}
		return nil
	})
}

func newMaterializationLeaseToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func claimMaterializationLease(runID uint64, token string) (*entity.ChainEventReorgMaterializationJob, error) {
	job := entity.ChainEventReorgMaterializationJob{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where("run_id = ? AND state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP(6))", runID, reorgMaterializationStatePreparing).Updates(map[string]interface{}{
			"lease_token": token, "lease_expires_at": gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL 120 SECOND)"),
			"fence_epoch": gorm.Expr("fence_epoch + 1"), "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg materialization lease is held by another worker")
		}
		return tx.Where("run_id = ? AND lease_token = ?", runID, token).Take(&job).Error
	})
	return &job, err
}

func processMaterializationPage(ctx context.Context, run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token string) error {
	_ = ctx
	switch job.NextBranch {
	case reorgMaterializationBranchOld:
		return processOldMaterializationPage(run, job, token)
	case reorgMaterializationBranchReplacement:
		return processReplacementMaterializationPage(run, job, token)
	case reorgMaterializationBranchDone:
		return finalizeMaterializationRun(run, job, token)
	default:
		return errors.New("unknown reorg materialization branch checkpoint")
	}
}

func processOldMaterializationPage(run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token string) error {
	rows := make([]entity.ChainEventReorgMaterializationOldSource, 0, reorgMaterializationPageSize)
	query := db.Mysql().Where("run_id = ?", run.ID)
	if job.NextOldRecordID != 0 {
		query = query.Where("block_number > ? OR (block_number = ? AND chain_event_record_id > ?)", job.NextOldBlock, job.NextOldBlock, job.NextOldRecordID)
	}
	if err := query.Order("ordinal ASC").Limit(reorgMaterializationPageSize).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return completeMaterializationBranch(run, job, token, reorgMaterializationBranchOld)
	}
	effects := make([]materializationEffectInput, 0, len(rows))
	previous := job.OldEffectDigest
	if job.OldEffectCount == 0 {
		previous = reorgZeroDigest
	} else {
		prior := entity.ChainEventReorgMaterializationOldSource{}
		if err := db.Mysql().Where("run_id = ? AND ordinal = ?", run.ID, job.OldEffectCount).Take(&prior).Error; err != nil {
			return err
		}
		previous = prior.RowDigest
	}
	for _, row := range rows {
		effect, err := oldSourceEffect(row, previous)
		if err != nil {
			return invalidateMaterializationRun(run.ID, token, job.FenceEpoch, "OLD_RECORD_INVALID", err.Error())
		}
		effects = append(effects, effect)
		previous = row.RowDigest
	}
	last := rows[len(rows)-1]
	return commitMaterializationEffects(run, job, token, effects, last.BlockNumber, last.ChainEventRecordID, job.NextReplacementOrdinal)
}

func processReplacementMaterializationPage(run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token string) error {
	rows := make([]entity.ChainEventReorgRecoveryLog, 0, reorgMaterializationPageSize)
	if err := db.Mysql().Where("run_id = ? AND ordinal >= ?", run.RecoveryRunID, job.NextReplacementOrdinal).Order("ordinal ASC").Limit(reorgMaterializationPageSize).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return completeMaterializationBranch(run, job, token, reorgMaterializationBranchReplacement)
	}
	effects := make([]materializationEffectInput, 0, len(rows))
	for _, row := range rows {
		effect, err := replacementLogEffect(row)
		if err != nil {
			return invalidateMaterializationRun(run.ID, token, job.FenceEpoch, "REPLACEMENT_LOG_INVALID", err.Error())
		}
		effects = append(effects, effect)
	}
	return commitMaterializationEffects(run, job, token, effects, job.NextOldBlock, job.NextOldRecordID, rows[len(rows)-1].Ordinal+1)
}

type materializationEffectInput struct {
	Branch, SourceTable                         string
	SourceID, BlockNumber                       uint64
	BlockHash, TxHash                           string
	LogIndex                                    uint
	ContractAddress, ContractVersion, EventType string
	ProjectID                                   uint64
	Classification                              string
	SemanticPayload                             []byte
}

type materializationV2GlobalEvent struct {
	Event           string          `json:"event"`
	ContractAddress common.Address  `json:"contractAddress"`
	BlockNumber     uint64          `json:"blockNumber"`
	BlockHash       string          `json:"blockHash"`
	TxHash          string          `json:"txHash"`
	LogIndex        uint            `json:"logIndex"`
	Token           *common.Address `json:"token,omitempty"`
	Allowed         *bool           `json:"allowed,omitempty"`
	Decimals        *uint8          `json:"decimals,omitempty"`
	FeeRate         *uint16         `json:"successFeeRate,omitempty"`
	FeeReceiver     *common.Address `json:"feeReceiver,omitempty"`
}

func oldRecordEffect(row entity.ChainEventRecord) (materializationEffectInput, error) {
	if row.ID == 0 || row.BlockNumber <= 0 || row.BlockHash == "" || row.TxHash == "" || row.ContractVersion == "" {
		return materializationEffectInput{}, errors.New("old chain event record identity is incomplete")
	}
	payload := []byte(row.Data)
	if !supportedMaterializationEvent(row.ContractVersion, row.Event) {
		return materializationEffectInput{}, fmt.Errorf("old chain event type %q is unsupported for %s", row.Event, row.ContractVersion)
	}
	projectID, err := projectIDFromOldSemanticPayload(row, payload)
	if err != nil {
		return materializationEffectInput{}, err
	}
	return materializationEffectInput{Branch: reorgMaterializationBranchOld, SourceTable: "chain_event_record", SourceID: row.ID, BlockNumber: uint64(row.BlockNumber), BlockHash: strings.ToLower(row.BlockHash), TxHash: strings.ToLower(row.TxHash), LogIndex: row.LogIndex, ContractAddress: strings.ToLower(row.ContractAddress), ContractVersion: row.ContractVersion, EventType: row.Event, ProjectID: projectID, Classification: reorgMaterializationEffectSemantic, SemanticPayload: payload}, nil
}

func oldSourceEffect(row entity.ChainEventReorgMaterializationOldSource, expectedPrevious string) (materializationEffectInput, error) {
	if row.ID == 0 || row.RunID == 0 || row.ChainEventRecordID == 0 || row.RowDigest == "" || row.EventDataDigest != sha256Hex(row.EventData) {
		return materializationEffectInput{}, errors.New("old materialization snapshot identity is invalid")
	}
	record := entity.ChainEventRecord{ID: row.ChainEventRecordID, Event: row.Event, BlockNumber: int(row.BlockNumber), BlockHash: row.BlockHash, TxHash: row.TxHash, LogIndex: row.LogIndex, ContractAddress: row.ContractAddress, ContractVersion: row.ContractVersion, Data: string(row.EventData)}
	if row.PreviousRowDigest != expectedPrevious || row.RowDigest != oldRecordSourceRowDigest(row.Ordinal, record, expectedPrevious) {
		return materializationEffectInput{}, errors.New("old materialization snapshot digest chain is invalid")
	}
	effect, err := oldRecordEffect(record)
	if err != nil {
		return materializationEffectInput{}, err
	}
	effect.SourceTable, effect.SourceID = "chain_event_reorg_materialization_old_source", row.ID
	return effect, nil
}

func supportedMaterializationEvent(version, event string) bool {
	if version == constant.ContractVersionV2Stablecoin {
		switch event {
		case commonChain.V2ProjectCreated, commonChain.V2ProjectTokensDeposited, commonChain.V2ProjectActivated,
			commonChain.V2Purchased, commonChain.V2ProjectEnded, commonChain.V2ProjectCancelled,
			commonChain.V2Refunded, commonChain.V2Claimed, commonChain.V2ProjectProceedsWithdrawn,
			commonChain.V2PlatformFeeWithdrawn, commonChain.V2ProjectTokensWithdrawn:
			return true
		}
	}
	if version == constant.ContractVersionV1Legacy {
		switch event {
		case commonChain.ProjectCreated, commonChain.ProjectTokensDeposited, commonChain.ProjectActivated,
			commonChain.Purchase, commonChain.ProjectEnded, commonChain.Claimed, commonChain.Refunded,
			commonChain.RaisedFundsWithdrawn, commonChain.ProjectTokensWithdrawn,
			commonChain.PlatformFeesUpdated, commonChain.ProjectCancelled:
			return true
		}
	}
	return false
}

func replacementLogEffect(row entity.ChainEventReorgRecoveryLog) (materializationEffectInput, error) {
	if row.ID == 0 || sha256Hex(row.RawLog) != row.RawLogDigest {
		return materializationEffectInput{}, errors.New("replacement raw log digest mismatch")
	}
	entry := types.Log{}
	if err := json.Unmarshal(row.RawLog, &entry); err != nil {
		return materializationEffectInput{}, fmt.Errorf("decode replacement raw log JSON: %w", err)
	}
	if entry.BlockNumber != row.BlockNumber || !strings.EqualFold(entry.BlockHash.Hex(), row.BlockHash) || !strings.EqualFold(entry.TxHash.Hex(), row.TxHash) || entry.Index != row.LogIndex || !strings.EqualFold(entry.Address.Hex(), row.ContractAddress) {
		return materializationEffectInput{}, errors.New("replacement raw log identity mismatch")
	}
	input := materializationEffectInput{Branch: reorgMaterializationBranchReplacement, SourceTable: "chain_event_reorg_recovery_log", SourceID: row.ID, BlockNumber: row.BlockNumber, BlockHash: strings.ToLower(row.BlockHash), TxHash: strings.ToLower(row.TxHash), LogIndex: row.LogIndex, ContractAddress: strings.ToLower(row.ContractAddress), ContractVersion: row.ContractVersion}
	var payload []byte
	if row.ContractVersion == constant.ContractVersionV2Stablecoin {
		global, err := decodeMaterializationV2GlobalLog(entry)
		if err != nil {
			return input, err
		}
		if global != nil {
			payload, err = jsonv2.Marshal(global)
			if err != nil {
				return input, err
			}
			input.EventType, input.Classification, input.SemanticPayload = global.Event, reorgMaterializationEffectSemantic, payload
			return input, nil
		}
		decoded, err := commonChain.DecodeV2Log(entry)
		if err != nil {
			return input, err
		}
		if decoded == nil {
			if !allowedIgnoredMaterializationLog(entry, row.ContractVersion) {
				return input, errors.New("unknown replacement V2 event topic is not approved as non-material")
			}
			input.EventType, input.Classification, input.SemanticPayload = "ignored", reorgMaterializationEffectIgnored, []byte("{}")
			return input, nil
		}
		payload, err = jsonv2.Marshal(decoded)
		if err != nil {
			return input, err
		}
		input.EventType, input.ProjectID = decoded.Event, decoded.ProjectID
	} else if row.ContractVersion == constant.ContractVersionV1Legacy {
		decoded, err := decodeChainLog(entry)
		if err != nil {
			return input, err
		}
		if decoded == nil {
			if !allowedIgnoredMaterializationLog(entry, row.ContractVersion) {
				return input, errors.New("unknown replacement V1 event topic is not approved as non-material")
			}
			input.EventType, input.Classification, input.SemanticPayload = "ignored", reorgMaterializationEffectIgnored, []byte("{}")
			return input, nil
		}
		decoded = (&GetChainEventJob{}).ProcessData(decoded)
		payload, err = json.Marshal(decoded)
		if err != nil {
			return input, err
		}
		input.EventType, input.ProjectID = decoded.Event, uint64(decoded.ProjectID)
	} else {
		return input, errors.New("replacement contract version is unsupported")
	}
	input.Classification, input.SemanticPayload = reorgMaterializationEffectSemantic, payload
	return input, nil
}

func decodeMaterializationV2GlobalLog(entry types.Log) (*materializationV2GlobalEvent, error) {
	if len(entry.Topics) == 0 {
		return nil, nil
	}
	event := &materializationV2GlobalEvent{ContractAddress: entry.Address, BlockNumber: entry.BlockNumber, BlockHash: entry.BlockHash.Hex(), TxHash: entry.TxHash.Hex(), LogIndex: entry.Index}
	switch entry.Topics[0] {
	case crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)")):
		event.Event = reorgV2PaymentTokenConfigured
	case crypto.Keccak256Hash([]byte("ProjectTokenConfigured(address,bool,uint8)")):
		event.Event = reorgV2ProjectTokenConfigured
	case crypto.Keccak256Hash([]byte("PlatformFeesUpdated(uint16,address)")):
		event.Event = reorgV2PlatformFeesUpdated
	default:
		return nil, nil
	}
	if len(entry.Topics) != 2 || entry.Address == (common.Address{}) {
		return nil, fmt.Errorf("decode V2 global %s: invalid topic or contract identity", event.Event)
	}
	topicBytes := entry.Topics[1].Bytes()
	for _, value := range topicBytes[:12] {
		if value != 0 {
			return nil, fmt.Errorf("decode V2 global %s: indexed address padding is invalid", event.Event)
		}
	}
	indexedAddress := common.BytesToAddress(topicBytes[12:])
	if indexedAddress == (common.Address{}) {
		return nil, fmt.Errorf("decode V2 global %s: indexed address is zero", event.Event)
	}
	word := func(offset int) []byte { return entry.Data[offset*32 : (offset+1)*32] }
	if event.Event == reorgV2PlatformFeesUpdated {
		if len(entry.Data) != 32 {
			return nil, errors.New("decode V2 global PlatformFeesUpdated: invalid data length")
		}
		fee := new(big.Int).SetBytes(word(0))
		if !fee.IsUint64() || fee.Uint64() > 1000 {
			return nil, errors.New("decode V2 global PlatformFeesUpdated: fee rate is invalid")
		}
		feeRate := uint16(fee.Uint64())
		event.FeeRate, event.FeeReceiver = &feeRate, &indexedAddress
		return event, nil
	}
	if len(entry.Data) != 64 {
		return nil, fmt.Errorf("decode V2 global %s: invalid data length", event.Event)
	}
	allowedRaw, decimalsRaw := new(big.Int).SetBytes(word(0)), new(big.Int).SetBytes(word(1))
	if !allowedRaw.IsUint64() || allowedRaw.Uint64() > 1 || !decimalsRaw.IsUint64() || decimalsRaw.Uint64() == 0 || decimalsRaw.Uint64() > 18 {
		return nil, fmt.Errorf("decode V2 global %s: token configuration is invalid", event.Event)
	}
	allowed := allowedRaw.Uint64() == 1
	decimals := uint8(decimalsRaw.Uint64())
	event.Token, event.Allowed, event.Decimals = &indexedAddress, &allowed, &decimals
	return event, nil
}

func allowedIgnoredMaterializationLog(entry types.Log, version string) bool {
	if len(entry.Topics) == 0 {
		return false
	}
	allowed := map[common.Hash]struct{}{
		crypto.Keccak256Hash([]byte("OwnershipTransferred(address,address)")): {},
		crypto.Keccak256Hash([]byte("Initialized(uint64)")):                   {},
		crypto.Keccak256Hash([]byte("Paused(address)")):                       {},
		crypto.Keccak256Hash([]byte("Unpaused(address)")):                     {},
	}
	if version == constant.ContractVersionV1Legacy {
		allowed[commonChain.EventTopic(commonChain.UserCheckedIn)] = struct{}{}
	}
	_, ok := allowed[entry.Topics[0]]
	return ok
}

func projectIDFromOldSemanticPayload(row entity.ChainEventRecord, payload []byte) (uint64, error) {
	if row.ContractVersion == constant.ContractVersionV2Stablecoin {
		var event commonChain.V2Event
		if err := jsonv2.Unmarshal(payload, &event); err != nil || event.ProjectID == 0 {
			return 0, errors.New("old V2 event payload is invalid")
		}
		if event.Event != row.Event || event.BlockNumber != uint64(row.BlockNumber) || !strings.EqualFold(event.BlockHash, row.BlockHash) || !strings.EqualFold(event.TxHash, row.TxHash) || event.LogIndex != row.LogIndex || !strings.EqualFold(event.ContractAddress.Hex(), row.ContractAddress) {
			return 0, errors.New("old V2 event payload identity mismatch")
		}
		return event.ProjectID, nil
	}
	if row.ContractVersion == constant.ContractVersionV1Legacy {
		var event commonChain.Data
		if err := json.Unmarshal(payload, &event); err != nil {
			return 0, errors.New("old V1 event payload is invalid")
		}
		if row.Event == commonChain.PlatformFeesUpdated {
			if event.ProjectID != 0 {
				return 0, errors.New("old V1 global event project identity is invalid")
			}
			return 0, nil
		}
		if event.ProjectID <= 0 {
			return 0, errors.New("old V1 event project identity is invalid")
		}
		return uint64(event.ProjectID), nil
	}
	return 0, errors.New("old event contract version is unsupported")
}

func commitMaterializationEffects(run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token string, inputs []materializationEffectInput, nextOldBlock, nextOldID, nextReplacement uint64) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockMaterializationJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if err := requireActiveMaterializationIncidentTx(tx, run.IncidentID); err != nil {
			return err
		}
		previous := locked.OldEffectDigest
		if locked.NextBranch == reorgMaterializationBranchReplacement {
			previous = locked.ReplacementEffectDigest
		}
		branchOrdinal := locked.NextBranchOrdinal
		effectOrdinal := locked.NextEffectOrdinal
		ignored := locked.IgnoredLogCount
		for _, input := range inputs {
			semanticDigest := sha256Hex(input.SemanticPayload)
			effectDigest := materializationEffectDigest(run.ID, effectOrdinal, input, branchOrdinal, semanticDigest, previous)
			row := entity.ChainEventReorgMaterializationEffect{RunID: run.ID, Ordinal: effectOrdinal, Branch: input.Branch, BranchOrdinal: branchOrdinal, SourceTable: input.SourceTable, SourceID: input.SourceID, BlockNumber: input.BlockNumber, BlockHash: input.BlockHash, TxHash: input.TxHash, LogIndex: input.LogIndex, ContractAddress: input.ContractAddress, ContractVersion: input.ContractVersion, EventType: input.EventType, ProjectID: input.ProjectID, Classification: input.Classification, SemanticPayload: input.SemanticPayload, SemanticDigest: semanticDigest, PreviousEffectDigest: previous, EffectDigest: effectDigest, CreatedAt: time.Now()}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if input.Classification == reorgMaterializationEffectIgnored {
				ignored++
			}
			previous, branchOrdinal, effectOrdinal = effectDigest, branchOrdinal+1, effectOrdinal+1
		}
		updates := map[string]interface{}{"next_old_block": nextOldBlock, "next_old_record_id": nextOldID, "next_replacement_ordinal": nextReplacement, "next_effect_ordinal": effectOrdinal, "next_branch_ordinal": branchOrdinal, "ignored_log_count": ignored, "lease_token": "", "lease_expires_at": nil, "last_error_code": "", "last_error_message": "", "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)")}
		if locked.NextBranch == reorgMaterializationBranchOld {
			updates["old_effect_count"], updates["old_effect_digest"] = locked.OldEffectCount+uint64(len(inputs)), previous
		} else {
			updates["replacement_effect_count"], updates["replacement_effect_digest"] = locked.ReplacementEffectCount+uint64(len(inputs)), previous
		}
		result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where(activeMaterializationLeaseWhere, run.ID, token, job.FenceEpoch).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg materialization lease lost before effect checkpoint")
		}
		return nil
	})
}

func completeMaterializationBranch(run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token, branch string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockMaterializationJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if err := requireActiveMaterializationIncidentTx(tx, run.IncidentID); err != nil {
			return err
		}
		if locked.NextBranch != branch {
			return errors.New("materialization branch checkpoint changed")
		}
		count, digest := locked.OldEffectCount, locked.OldEffectDigest
		nextBranch := reorgMaterializationBranchReplacement
		if branch == reorgMaterializationBranchReplacement {
			count, digest, nextBranch = locked.ReplacementEffectCount, locked.ReplacementEffectDigest, reorgMaterializationBranchDone
		}
		summary := entity.ChainEventReorgMaterializationBranch{RunID: run.ID, Branch: branch, EffectCount: count, IgnoredCount: locked.IgnoredLogCount, EffectDigest: digest, SummaryDigest: materializationBranchDigest(run.ID, branch, count, locked.IgnoredLogCount, digest), CompletedAt: time.Now()}
		if err := tx.Create(&summary).Error; err != nil {
			return err
		}
		if err := appendMaterializationEventTx(tx, run.ID, "BRANCH_COMPLETED", branch, "", fmt.Sprintf("completed %s semantic effect chain", branch)); err != nil {
			return err
		}
		updates := map[string]interface{}{"next_branch": nextBranch, "next_branch_ordinal": 1, "lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)")}
		result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where(activeMaterializationLeaseWhere, run.ID, token, job.FenceEpoch).Updates(updates)
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg materialization lease lost before branch finalization")
		}
		return nil
	})
}

func finalizeMaterializationRun(run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockMaterializationJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if locked.NextBranch != reorgMaterializationBranchDone {
			return errors.New("materialization branches are incomplete")
		}
		if err := verifyMaterializationSourcesTx(tx, run, locked); err != nil {
			return invalidateMaterializationRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		manifest := entity.ChainEventReorgMaterializationManifest{RunID: run.ID, IncidentID: run.IncidentID, RecoveryManifestDigest: run.RecoveryManifestDigest, OldEffectCount: locked.OldEffectCount, ReplacementEffectCount: locked.ReplacementEffectCount, IgnoredLogCount: locked.IgnoredLogCount, OldEffectDigest: locked.OldEffectDigest, ReplacementEffectDigest: locked.ReplacementEffectDigest, ManifestDigest: materializationManifestDigest(run.ID, run.IncidentID, run.RecoveryManifestDigest, locked.OldEffectCount, locked.ReplacementEffectCount, locked.IgnoredLogCount, locked.OldEffectDigest, locked.ReplacementEffectDigest), CompletedAt: time.Now()}
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		if err := appendMaterializationEventTx(tx, run.ID, "COMPLETED", "finalize", "", "completed old/replacement semantic effect preparation"); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where(activeMaterializationLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{"state": reorgMaterializationStateCompleted, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg materialization lease lost before manifest finalization")
		}
		return nil
	})
}

func verifyMaterializationSourcesTx(tx *gorm.DB, run *entity.ChainEventReorgMaterializationRun, job *entity.ChainEventReorgMaterializationJob) error {
	incident := entity.ChainEventReorgIncident{}
	if err := tx.Where("id = ? AND active_slot = 1 AND status = ?", run.IncidentID, reorgStatusDetected).Take(&incident).Error; err != nil {
		return err
	}
	recoveryManifest := entity.ChainEventReorgRecoveryManifest{}
	if err := tx.Where("run_id = ? AND incident_id = ?", run.RecoveryRunID, run.IncidentID).Take(&recoveryManifest).Error; err != nil {
		return err
	}
	if recoveryManifest.ManifestDigest != run.RecoveryManifestDigest || recoveryManifest.LogCount != run.ExpectedReplacementLogCount || recoveryManifest.LogEvidenceDigest != run.ReplacementLogSourceDigest || job.ReplacementEffectCount != run.ExpectedReplacementLogCount || job.IgnoredLogCount > job.ReplacementEffectCount {
		return errors.New("replacement recovery source changed or is incomplete")
	}
	var snapshot struct {
		Count uint64 `gorm:"column:row_count"`
		Root  string `gorm:"column:root_digest"`
	}
	if err := tx.Model(&entity.ChainEventReorgMaterializationOldSource{}).Where("run_id = ?", run.ID).
		Select("COUNT(*) AS row_count,COALESCE((SELECT row_digest FROM chain_event_reorg_materialization_old_source s2 WHERE s2.run_id=? ORDER BY ordinal DESC LIMIT 1),?) AS root_digest", run.ID, reorgZeroDigest).Scan(&snapshot).Error; err != nil {
		return err
	}
	if snapshot.Count != run.ExpectedOldRecordCount || snapshot.Root != run.OldRecordSourceDigest || job.OldEffectCount != run.ExpectedOldRecordCount {
		return errors.New("old committed event source changed or is incomplete")
	}
	return nil
}

const activeMaterializationLeaseWhere = "run_id = ? AND lease_token = ? AND fence_epoch = ? AND state = 'preparing' AND lease_expires_at > CURRENT_TIMESTAMP(6)"

func lockMaterializationJobTx(tx *gorm.DB, runID uint64, token string, epoch uint64) (*entity.ChainEventReorgMaterializationJob, error) {
	job := entity.ChainEventReorgMaterializationJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activeMaterializationLeaseWhere, runID, token, epoch).Take(&job).Error
	return &job, err
}

func requireActiveMaterializationIncidentTx(tx *gorm.DB, incidentID uint64) error {
	var count int64
	if err := tx.Model(&entity.ChainEventReorgIncident{}).Where("id = ? AND active_slot = 1 AND status = ?", incidentID, reorgStatusDetected).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return errors.New("active reorg incident disappeared during materialization preparation")
	}
	return nil
}

func invalidateMaterializationRun(runID uint64, token string, epoch uint64, code, message string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		return invalidateMaterializationRunTx(tx, runID, token, epoch, code, message)
	})
}

func invalidateMaterializationRunTx(tx *gorm.DB, runID uint64, token string, epoch uint64, code, message string) error {
	if _, err := lockMaterializationJobTx(tx, runID, token, epoch); err != nil {
		return err
	}
	if err := appendMaterializationEventTx(tx, runID, "INVALIDATED", "prepare", code, truncateFailureError(message, 500)); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where(activeMaterializationLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{"state": reorgMaterializationStateInvalidated, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "last_error_code": code, "last_error_message": truncateFailureError(message, 500), "completed_at": now, "updated_at": now})
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("reorg materialization lease lost before invalidation")
	}
	return nil
}

func recordMaterializationError(runID uint64, token string, epoch uint64, code string, processErr error) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgMaterializationJob{}).Where(activeMaterializationLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{"last_error_code": code, "last_error_message": truncateFailureError(processErr.Error(), 500), "lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg materialization lease lost before error checkpoint")
		}
		return nil
	})
}

func appendMaterializationEventTx(tx *gorm.DB, runID uint64, eventType, stage, code, message string) error {
	previous := entity.ChainEventReorgMaterializationEvent{}
	previousHash := reorgZeroDigest
	sequence := uint(1)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ?", runID).Order("sequence_no DESC").Take(&previous).Error
	if err == nil {
		previousHash, sequence = previous.EventHash, previous.SequenceNo+1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := entity.ChainEventReorgMaterializationEvent{RunID: runID, SequenceNo: sequence, EventType: eventType, Stage: stage, ErrorCode: code, Message: message, PreviousEventHash: previousHash, EventHash: evidenceDigest("joincare.chain-event.reorg-materialization-event.v1", runID, uint64(sequence), eventType, stage, code, message, previousHash), CreatedAt: time.Now()}
	return tx.Create(&event).Error
}

func materializationEffectDigest(runID, ordinal uint64, input materializationEffectInput, branchOrdinal uint64, semanticDigest, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-materialization-effect.v1", runID, ordinal, input.Branch, branchOrdinal, input.SourceTable, input.SourceID, input.BlockNumber, input.BlockHash, input.TxHash, input.LogIndex, input.ContractAddress, input.ContractVersion, input.EventType, input.ProjectID, input.Classification, semanticDigest, previous)
}
func materializationBranchDigest(runID uint64, branch string, count, ignored uint64, digest string) string {
	return evidenceDigest("joincare.chain-event.reorg-materialization-branch.v1", runID, branch, count, ignored, digest)
}
func materializationManifestDigest(runID, incidentID uint64, recoveryDigest string, oldCount, replacementCount, ignored uint64, oldDigest, replacementDigest string) string {
	return evidenceDigest("joincare.chain-event.reorg-materialization-manifest.v1", runID, incidentID, recoveryDigest, oldCount, replacementCount, ignored, oldDigest, replacementDigest)
}
