package chainevent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

const (
	reorgRecoveryAlgorithm    = "joincare-reorg-recovery-scan-v1"
	reorgRecoveryStateScan    = "scanning"
	reorgRecoveryStateDone    = "completed"
	reorgRecoveryInvalidated  = "invalidated"
	reorgRecoveryBoundaryLast = "last_canonical_checkpoint"
	reorgRecoveryBatchSize    = uint64(100)
	reorgRecoveryLeaseTTL     = 2 * time.Minute
	reorgRecoveryMaxBatchLogs = 50_000
	reorgRecoveryMaxBatchRaw  = 12 * 1024 * 1024
)

type ReorgRecoveryScanJob struct{}

func NewReorgRecoveryScanJob() model.Add {
	var job cron.Job = new(ReorgRecoveryScanJob)
	return job.(model.Add)
}

func (j *ReorgRecoveryScanJob) Add(crontab *cron.Cron) {
	crontab.AddJob("5/10 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

func (j *ReorgRecoveryScanJob) Run() {
	if !chain.V2Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	outerToken, err := newRunLockToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg recovery lock token err: %s", err)
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken, reorgRecoveryLeaseTTL)
	if err != nil || !locked {
		return
	}
	defer func() {
		_ = db.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken)
	}()

	incident, err := activeRecoveryIncident()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("load reorg recovery incident err: %s", err)
		return
	}
	job, err := ensureRecoveryAttempt(ctx, incident)
	if errors.Is(err, errRecoveryAlreadyComplete) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("prepare reorg recovery attempt incident=%d err: %s", incident.ID, err)
		return
	}
	leaseToken, err := newRecoveryLeaseToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg recovery DB lease err: %s", err)
		return
	}
	job, err = claimRecoveryLease(job.RunID, leaseToken)
	if err != nil {
		log.GetLogger().Errorf("claim reorg recovery DB lease run=%d err: %s", job.RunID, err)
		return
	}
	if err := processRecoveryBatch(ctx, incident, job, leaseToken); err != nil {
		_ = recordRecoveryError(job.RunID, leaseToken, job.FenceEpoch, "RECOVERY_SCAN_FAILED", err)
		log.GetLogger().Errorf("scan reorg recovery batch run=%d err: %s", job.RunID, err)
	}
}

var errRecoveryAlreadyComplete = errors.New("reorg recovery scan is already complete")

func activeRecoveryIncident() (*entity.ChainEventReorgIncident, error) {
	incident := entity.ChainEventReorgIncident{}
	err := db.Mysql().Where("active_slot = 1 AND status = ? AND chain_id = ? AND contract_address = ?", reorgStatusDetected, chain.GetClient().ChainID.Uint64(), strings.ToLower(chain.V2ContractAddress().Hex())).Take(&incident).Error
	return &incident, err
}

func ensureRecoveryAttempt(ctx context.Context, incident *entity.ChainEventReorgIncident) (*entity.ChainEventReorgRecoveryJob, error) {
	job := entity.ChainEventReorgRecoveryJob{}
	err := db.Mysql().Where("incident_id = ? AND active_slot = 1 AND state = ?", incident.ID, reorgRecoveryStateScan).Take(&job).Error
	if err == nil {
		return &job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var completed int64
	if err := db.Mysql().Model(&entity.ChainEventReorgRecoveryManifest{}).Where("incident_id = ?", incident.ID).Count(&completed).Error; err != nil {
		return nil, err
	}
	if completed != 0 {
		return nil, errRecoveryAlreadyComplete
	}
	var priorAttempts int64
	if err := db.Mysql().Model(&entity.ChainEventReorgRecoveryRun{}).Where("incident_id = ?", incident.ID).Count(&priorAttempts).Error; err != nil {
		return nil, err
	}
	if priorAttempts != 0 {
		return nil, errors.New("prior reorg recovery attempt is terminal; audited retry authorization is required")
	}
	run, err := buildRecoveryRun(ctx, incident)
	if err != nil {
		return nil, err
	}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		active := entity.ChainEventReorgIncident{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND active_slot = 1 AND status = ?", incident.ID, reorgStatusDetected).Take(&active).Error; err != nil {
			return err
		}
		state := entity.ChainEventSyncState{}
		if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", incident.CursorBlock, incident.ExpectedBlockHash).Take(&state).Error; err != nil {
			return errors.New("chain event cursor changed after reorg detection")
		}
		var maxAttempt uint
		if err := tx.Model(&entity.ChainEventReorgRecoveryRun{}).Where("incident_id = ?", incident.ID).Select("COALESCE(MAX(attempt_no),0)").Scan(&maxAttempt).Error; err != nil {
			return err
		}
		run.AttemptNo = maxAttempt + 1
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		activeSlot := uint8(1)
		job = entity.ChainEventReorgRecoveryJob{
			RunID: run.ID, IncidentID: incident.ID, State: reorgRecoveryStateScan, ActiveSlot: &activeSlot,
			NextBlock: run.BoundaryBlock + 1, NextBatchOrdinal: 1, NextLogOrdinal: 1,
			BatchEvidenceDigest: reorgZeroDigest, LogEvidenceDigest: reorgZeroDigest, UpdatedAt: time.Now(),
		}
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		return appendRecoveryEventTx(tx, run.ID, "CREATED", "boundary", "", fmt.Sprintf("pinned %s boundary %d and safe target %d", run.BoundaryKind, run.BoundaryBlock, run.TargetSafeBlock))
	})
	return &job, err
}

var errNoCanonicalCheckpoint = errors.New("no committed chain-event checkpoint matches the canonical branch")

func buildRecoveryRun(ctx context.Context, incident *entity.ChainEventReorgIncident) (*entity.ChainEventReorgRecoveryRun, error) {
	if incident == nil || incident.V2StartBlock == 0 {
		return nil, errors.New("reorg incident has no V2 start boundary")
	}
	checkpoints := make([]entity.ChainEventBlockCheckpoint, 0)
	if err := db.Mysql().Where("chain_id = ? AND block_number <= ?", incident.ChainID, incident.CursorBlock).Order("id ASC").Find(&checkpoints).Error; err != nil {
		return nil, err
	}
	if err := validateCheckpointEvidence(checkpoints, incident); err != nil {
		return nil, err
	}
	boundaryKind := reorgRecoveryBoundaryLast
	boundaryBlock := uint64(0)
	boundaryHash := ""
	var boundaryID *uint64
	for index := len(checkpoints) - 1; index >= 0; index-- {
		checkpoint := checkpoints[index]
		header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(checkpoint.BlockNumber))
		if err != nil {
			return nil, fmt.Errorf("read checkpoint block %d: %w", checkpoint.BlockNumber, err)
		}
		if strings.EqualFold(header.Hash().Hex(), checkpoint.BlockHash) {
			id := checkpoint.ID
			boundaryID = &id
			boundaryBlock = checkpoint.BlockNumber
			boundaryHash = strings.ToLower(checkpoint.BlockHash)
			break
		}
	}
	if boundaryID == nil {
		return nil, errNoCanonicalCheckpoint
	}
	_, safeBlock, err := chain.GetClient().LatestAndSafeBlock(ctx)
	if err != nil {
		return nil, err
	}
	if safeBlock < incident.CursorBlock || safeBlock <= boundaryBlock {
		return nil, fmt.Errorf("safe target %d does not cover detected cursor %d and recovery boundary %d", safeBlock, incident.CursorBlock, boundaryBlock)
	}
	targetHeader, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(safeBlock))
	if err != nil {
		return nil, fmt.Errorf("read recovery safe target %d: %w", safeBlock, err)
	}
	scanned := safeBlock - boundaryBlock
	legacyAddress := ""
	if chain.V1Enabled() {
		legacyAddress = strings.ToLower(chain.GetClient().ContractAddress.Hex())
	}
	return &entity.ChainEventReorgRecoveryRun{
		RunUUID: strings.ToLower(uuid.NewString()), IncidentID: incident.ID,
		ChainID: incident.ChainID, ContractAddress: strings.ToLower(incident.ContractAddress), LegacyContractAddress: legacyAddress,
		LegacySourceRegistered: chain.V1Enabled(),
		ContractVersion:        constant.ContractVersionV2Stablecoin,
		AlgorithmVersion:       reorgRecoveryAlgorithm, BoundaryKind: boundaryKind, BoundaryCheckpointID: boundaryID,
		BoundaryBlock: boundaryBlock, BoundaryBlockHash: boundaryHash, TargetSafeBlock: safeBlock,
		TargetSafeBlockHash: strings.ToLower(targetHeader.Hash().Hex()), ConfirmationDepth: chain.GetClient().Confirmations,
		ExpectedBatchCount: uint((scanned + reorgRecoveryBatchSize - 1) / reorgRecoveryBatchSize), ExpectedScannedBlocks: scanned, RequestedAt: time.Now(),
	}, nil
}

func validateCheckpointEvidence(checkpoints []entity.ChainEventBlockCheckpoint, incident *entity.ChainEventReorgIncident) error {
	previousDigest := reorgZeroDigest
	previousBlock := uint64(0)
	foundIncident := false
	for _, checkpoint := range checkpoints {
		if checkpoint.ChainID != incident.ChainID || checkpoint.BlockNumber <= previousBlock || checkpoint.PreviousCheckpointHash != previousDigest || checkpoint.CheckpointHash != checkpointDigest(checkpoint.ChainID, checkpoint.BlockNumber, checkpoint.BlockHash, checkpoint.PreviousCheckpointHash) {
			return errors.New("committed chain event checkpoint evidence is invalid")
		}
		if checkpoint.ID == incident.CursorCheckpointID && checkpoint.BlockNumber == incident.CursorBlock && strings.EqualFold(checkpoint.BlockHash, incident.ExpectedBlockHash) {
			foundIncident = true
		}
		previousDigest = checkpoint.CheckpointHash
		previousBlock = checkpoint.BlockNumber
	}
	if !foundIncident {
		return errors.New("reorg incident cursor checkpoint is absent from committed evidence")
	}
	return nil
}

func newRecoveryLeaseToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func claimRecoveryLease(runID uint64, token string) (*entity.ChainEventReorgRecoveryJob, error) {
	job := entity.ChainEventReorgRecoveryJob{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ? AND state = ?", runID, reorgRecoveryStateScan).Take(&job).Error; err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgRecoveryJob{}).Where("run_id = ? AND state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP(6))", runID, reorgRecoveryStateScan).Updates(map[string]interface{}{
			"lease_token": token, "lease_expires_at": gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL 120 SECOND)"),
			"fence_epoch": gorm.Expr("fence_epoch + 1"), "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg recovery DB lease is held by another worker")
		}
		return tx.Where("run_id = ? AND lease_token = ?", runID, token).Take(&job).Error
	})
	return &job, err
}

func processRecoveryBatch(ctx context.Context, incident *entity.ChainEventReorgIncident, job *entity.ChainEventReorgRecoveryJob, leaseToken string) error {
	run := entity.ChainEventReorgRecoveryRun{}
	if err := db.Mysql().Where("id = ? AND incident_id = ?", job.RunID, incident.ID).Take(&run).Error; err != nil {
		return err
	}
	legacyAddress := ""
	if chain.V1Enabled() {
		legacyAddress = chain.GetClient().ContractAddress.Hex()
	}
	if !strings.EqualFold(run.ContractAddress, chain.V2ContractAddress().Hex()) || run.LegacySourceRegistered != chain.V1Enabled() || !strings.EqualFold(run.LegacyContractAddress, legacyAddress) {
		return invalidateRecoveryRun(job.RunID, leaseToken, job.FenceEpoch, "SOURCE_IDENTITY_CHANGED", "registered event source addresses changed during recovery scan")
	}
	if err := verifyRecoveryPinnedBoundary(ctx, run); err != nil {
		return invalidateRecoveryRun(job.RunID, leaseToken, job.FenceEpoch, "BOUNDARY_HASH_CHANGED", err.Error())
	}
	targetHeader, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(run.TargetSafeBlock))
	if err != nil {
		return err
	}
	if !strings.EqualFold(targetHeader.Hash().Hex(), run.TargetSafeBlockHash) {
		return invalidateRecoveryRun(job.RunID, leaseToken, job.FenceEpoch, "TARGET_HASH_CHANGED", "pinned recovery safe target is no longer canonical")
	}
	fromBlock := job.NextBlock
	if fromBlock > run.TargetSafeBlock {
		return finalizeRecoveryRun(ctx, incident, run, job, leaseToken)
	}
	toBlock := fromBlock + reorgRecoveryBatchSize - 1
	if toBlock > run.TargetSafeBlock {
		toBlock = run.TargetSafeBlock
	}
	headers, err := readRecoveryHeaders(ctx, run, fromBlock, toBlock)
	if err != nil {
		return err
	}
	logs, err := chain.GetClient().Client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock), ToBlock: new(big.Int).SetUint64(toBlock),
		Addresses: recoveryEventAddresses(run),
	})
	if err != nil {
		return fmt.Errorf("filter replacement-branch V2 logs: %w", err)
	}
	sortRecoveryLogs(logs)
	if len(logs) > reorgRecoveryMaxBatchLogs {
		return fmt.Errorf("replacement-branch log count %d exceeds per-batch limit %d", len(logs), reorgRecoveryMaxBatchLogs)
	}
	rawBytes := 0
	for _, entry := range logs {
		header := headers[entry.BlockNumber]
		if entry.Removed || header == nil || entry.BlockHash != header.Hash() || recoveryContractVersion(run, entry.Address) == "" {
			return errors.New("replacement-branch log does not match fixed canonical headers")
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		rawBytes += len(raw)
		if rawBytes > reorgRecoveryMaxBatchRaw {
			return fmt.Errorf("replacement-branch raw log evidence exceeds per-batch limit %d", reorgRecoveryMaxBatchRaw)
		}
	}
	// Re-read both range ends after FilterLogs. Conflicting fixed-height reads
	// invalidate this attempt instead of producing partial canonical evidence.
	for _, block := range []uint64{fromBlock, toBlock, run.TargetSafeBlock} {
		header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
		if err != nil || header.Hash() != headersForBlock(headers, targetHeader, block).Hash() {
			return invalidateRecoveryRun(job.RunID, leaseToken, job.FenceEpoch, "FIXED_HEIGHT_CHANGED", fmt.Sprintf("canonical block %d changed during recovery scan", block))
		}
	}
	return commitRecoveryBatch(incident, run, job, leaseToken, headers, logs, fromBlock, toBlock)
}

func recoveryEventAddresses(run entity.ChainEventReorgRecoveryRun) []common.Address {
	addresses := []common.Address{common.HexToAddress(run.ContractAddress)}
	if run.LegacySourceRegistered {
		addresses = append([]common.Address{common.HexToAddress(run.LegacyContractAddress)}, addresses...)
	}
	return addresses
}

func recoveryContractVersion(run entity.ChainEventReorgRecoveryRun, address common.Address) string {
	if strings.EqualFold(address.Hex(), run.ContractAddress) {
		return constant.ContractVersionV2Stablecoin
	}
	if run.LegacySourceRegistered && strings.EqualFold(address.Hex(), run.LegacyContractAddress) {
		return constant.ContractVersionV1Legacy
	}
	return ""
}

func readRecoveryHeaders(ctx context.Context, run entity.ChainEventReorgRecoveryRun, fromBlock, toBlock uint64) (map[uint64]*types.Header, error) {
	headers := make(map[uint64]*types.Header, toBlock-fromBlock+1)
	previousHash := common.HexToHash(run.BoundaryBlockHash)
	if fromBlock == 1 && run.BoundaryBlock == 0 {
		previousHash = common.Hash{}
	}
	if fromBlock > run.BoundaryBlock+1 {
		prior, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(fromBlock-1))
		if err != nil {
			return nil, err
		}
		if err := requireCommittedRecoveryBoundary(run.ID, fromBlock-1, prior.Hash().Hex()); err != nil {
			return nil, err
		}
		previousHash = prior.Hash()
	}
	for block := fromBlock; block <= toBlock; block++ {
		header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
		if err != nil {
			return nil, fmt.Errorf("read recovery block %d: %w", block, err)
		}
		if block > 1 && header.ParentHash != previousHash {
			return nil, fmt.Errorf("recovery header continuity failed at block %d", block)
		}
		headers[block] = header
		previousHash = header.Hash()
	}
	return headers, nil
}

func requireCommittedRecoveryBoundary(runID, block uint64, blockHash string) error {
	var count int64
	err := db.Mysql().Model(&entity.ChainEventReorgRecoveryBatch{}).
		Where("run_id = ? AND to_block = ? AND to_block_hash = ?", runID, block, strings.ToLower(blockHash)).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("reorg recovery resume boundary does not match committed batch evidence")
	}
	return nil
}

func headersForBlock(headers map[uint64]*types.Header, target *types.Header, block uint64) *types.Header {
	if header := headers[block]; header != nil {
		return header
	}
	return target
}

func sortRecoveryLogs(logs []types.Log) {
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].BlockNumber != logs[j].BlockNumber {
			return logs[i].BlockNumber < logs[j].BlockNumber
		}
		if logs[i].TxIndex != logs[j].TxIndex {
			return logs[i].TxIndex < logs[j].TxIndex
		}
		return logs[i].Index < logs[j].Index
	})
}

func commitRecoveryBatch(incident *entity.ChainEventReorgIncident, run entity.ChainEventReorgRecoveryRun, job *entity.ChainEventReorgRecoveryJob, leaseToken string, headers map[uint64]*types.Header, logs []types.Log, fromBlock, toBlock uint64) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockRecoveryJobTx(tx, job.RunID, leaseToken, job.FenceEpoch)
		if err != nil {
			return err
		}
		if locked.NextBlock != fromBlock || locked.NextBatchOrdinal != job.NextBatchOrdinal || locked.NextLogOrdinal != job.NextLogOrdinal {
			return errors.New("reorg recovery checkpoint changed before batch commit")
		}
		if active, err := hasActiveReorgIncidentTx(tx, incident.ChainID, incident.ContractAddress); err != nil || !active {
			if err != nil {
				return err
			}
			return errors.New("active reorg incident disappeared during recovery scan")
		}
		previousLog := locked.LogEvidenceDigest
		firstLogOrdinal := locked.NextLogOrdinal
		for index, entry := range logs {
			raw, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			rawDigest := sha256Hex(raw)
			ordinal := firstLogOrdinal + uint64(index)
			rowDigest := recoveryLogDigest(run, ordinal, entry, rawDigest, previousLog)
			row := entity.ChainEventReorgRecoveryLog{
				RunID: run.ID, Ordinal: ordinal, BlockNumber: entry.BlockNumber, BlockHash: strings.ToLower(entry.BlockHash.Hex()),
				TxHash: strings.ToLower(entry.TxHash.Hex()), TxIndex: entry.TxIndex, LogIndex: entry.Index,
				ContractAddress: strings.ToLower(entry.Address.Hex()), ContractVersion: recoveryContractVersion(run, entry.Address),
				RawLog: raw, RawLogDigest: rawDigest, PreviousRowDigest: previousLog, RowDigest: rowDigest, ObservedAt: time.Now(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			previousLog = rowDigest
		}
		batchLogDigest := reorgZeroDigest
		if len(logs) != 0 {
			batchLogDigest = previousLog
		}
		batchDigest := recoveryBatchDigest(run.ID, uint64(locked.NextBatchOrdinal), fromBlock, toBlock, headers[fromBlock].Hash().Hex(), headers[toBlock].Hash().Hex(), firstLogOrdinal, uint64(len(logs)), batchLogDigest, locked.BatchEvidenceDigest)
		batch := entity.ChainEventReorgRecoveryBatch{
			RunID: run.ID, Ordinal: locked.NextBatchOrdinal, FromBlock: fromBlock, ToBlock: toBlock,
			FromBlockHash: strings.ToLower(headers[fromBlock].Hash().Hex()), ToBlockHash: strings.ToLower(headers[toBlock].Hash().Hex()),
			FirstLogOrdinal: firstLogOrdinal, LogCount: uint64(len(logs)), BatchLogDigest: batchLogDigest,
			PreviousBatchDigest: locked.BatchEvidenceDigest, BatchDigest: batchDigest, ObservedAt: time.Now(),
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		if err := appendRecoveryEventTx(tx, run.ID, "BATCH_COMMITTED", "scan", "", fmt.Sprintf("committed recovery batch %d covering blocks %d-%d", batch.Ordinal, fromBlock, toBlock)); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgRecoveryJob{}).Where(activeRecoveryLeaseWhere, run.ID, leaseToken, job.FenceEpoch).Updates(map[string]interface{}{
			"next_block": toBlock + 1, "next_batch_ordinal": locked.NextBatchOrdinal + 1,
			"next_log_ordinal": locked.NextLogOrdinal + uint64(len(logs)), "observed_log_count": locked.ObservedLogCount + uint64(len(logs)),
			"batch_evidence_digest": batchDigest, "log_evidence_digest": previousLog,
			"lease_token": "", "lease_expires_at": nil, "last_error_code": "", "last_error_message": "", "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg recovery lease lost before batch checkpoint")
		}
		return nil
	})
}

const activeRecoveryLeaseWhere = "run_id = ? AND lease_token = ? AND fence_epoch = ? AND state = 'scanning' AND lease_expires_at > CURRENT_TIMESTAMP(6)"

func lockRecoveryJobTx(tx *gorm.DB, runID uint64, token string, epoch uint64) (*entity.ChainEventReorgRecoveryJob, error) {
	job := entity.ChainEventReorgRecoveryJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activeRecoveryLeaseWhere, runID, token, epoch).Take(&job).Error
	return &job, err
}

func finalizeRecoveryRun(ctx context.Context, incident *entity.ChainEventReorgIncident, run entity.ChainEventReorgRecoveryRun, job *entity.ChainEventReorgRecoveryJob, leaseToken string) error {
	if err := verifyRecoveryPinnedBoundary(ctx, run); err != nil {
		return invalidateRecoveryRun(run.ID, leaseToken, job.FenceEpoch, "BOUNDARY_HASH_CHANGED", err.Error())
	}
	target, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(run.TargetSafeBlock))
	if err != nil {
		return err
	}
	if !strings.EqualFold(target.Hash().Hex(), run.TargetSafeBlockHash) {
		return invalidateRecoveryRun(run.ID, leaseToken, job.FenceEpoch, "TARGET_HASH_CHANGED", "pinned recovery target changed before finalization")
	}
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockRecoveryJobTx(tx, run.ID, leaseToken, job.FenceEpoch)
		if err != nil {
			return err
		}
		if locked.NextBlock != run.TargetSafeBlock+1 || locked.NextBatchOrdinal != run.ExpectedBatchCount+1 {
			return errors.New("reorg recovery range is incomplete")
		}
		manifest := entity.ChainEventReorgRecoveryManifest{
			RunID: run.ID, IncidentID: incident.ID, BatchCount: run.ExpectedBatchCount, LogCount: locked.ObservedLogCount,
			BatchEvidenceDigest: locked.BatchEvidenceDigest, LogEvidenceDigest: locked.LogEvidenceDigest,
			ManifestDigest: recoveryManifestDigest(run.ID, incident.ID, uint64(run.ExpectedBatchCount), locked.ObservedLogCount, locked.BatchEvidenceDigest, locked.LogEvidenceDigest), CompletedAt: time.Now(),
		}
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		if err := appendRecoveryEventTx(tx, run.ID, "COMPLETED", "finalize", "", fmt.Sprintf("completed fixed-safe recovery scan through block %d", run.TargetSafeBlock)); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgRecoveryJob{}).Where(activeRecoveryLeaseWhere, run.ID, leaseToken, job.FenceEpoch).Updates(map[string]interface{}{
			"state": reorgRecoveryStateDone, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg recovery lease lost before manifest finalization")
		}
		return nil
	})
}

func verifyRecoveryPinnedBoundary(ctx context.Context, run entity.ChainEventReorgRecoveryRun) error {
	header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(run.BoundaryBlock))
	if err != nil {
		return fmt.Errorf("read pinned recovery boundary %d: %w", run.BoundaryBlock, err)
	}
	if !strings.EqualFold(header.Hash().Hex(), run.BoundaryBlockHash) {
		return fmt.Errorf("pinned recovery boundary %d changed during scan", run.BoundaryBlock)
	}
	return nil
}

func invalidateRecoveryRun(runID uint64, token string, epoch uint64, code, message string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if _, err := lockRecoveryJobTx(tx, runID, token, epoch); err != nil {
			return err
		}
		if err := appendRecoveryEventTx(tx, runID, "INVALIDATED", "scan", code, message); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgRecoveryJob{}).Where(activeRecoveryLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
			"state": reorgRecoveryInvalidated, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
			"last_error_code": code, "last_error_message": message, "completed_at": now, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg recovery lease lost before invalidation")
		}
		return nil
	})
}

func recordRecoveryError(runID uint64, token string, epoch uint64, code string, processErr error) error {
	message := truncateFailureError(processErr.Error(), 500)
	result := db.Mysql().Model(&entity.ChainEventReorgRecoveryJob{}).Where(activeRecoveryLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
		"last_error_code": code, "last_error_message": message, "lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
	})
	return result.Error
}

func appendRecoveryEventTx(tx *gorm.DB, runID uint64, eventType, stage, code, message string) error {
	previous := entity.ChainEventReorgRecoveryEvent{}
	previousHash := reorgZeroDigest
	sequence := uint(1)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ?", runID).Order("sequence_no DESC").Take(&previous).Error
	if err == nil {
		previousHash = previous.EventHash
		sequence = previous.SequenceNo + 1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := entity.ChainEventReorgRecoveryEvent{
		RunID: runID, SequenceNo: sequence, EventType: eventType, Stage: stage, ErrorCode: code, Message: message,
		PreviousEventHash: previousHash, EventHash: recoveryEventDigest(runID, uint64(sequence), eventType, stage, code, message, previousHash), CreatedAt: time.Now(),
	}
	return tx.Create(&event).Error
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
func recoveryEventDigest(runID, sequence uint64, eventType, stage, code, message, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-recovery-event.v1", runID, sequence, eventType, stage, code, message, previous)
}
func recoveryLogDigest(run entity.ChainEventReorgRecoveryRun, ordinal uint64, entry types.Log, rawDigest, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-recovery-log.v1", run.ID, ordinal, entry.BlockNumber, strings.ToLower(entry.BlockHash.Hex()), strings.ToLower(entry.TxHash.Hex()), entry.TxIndex, entry.Index, strings.ToLower(entry.Address.Hex()), recoveryContractVersion(run, entry.Address), rawDigest, previous)
}
func recoveryBatchDigest(runID, ordinal, from, to uint64, fromHash, toHash string, firstLog, logCount uint64, logDigest, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-recovery-batch.v1", runID, ordinal, from, to, strings.ToLower(fromHash), strings.ToLower(toHash), firstLog, logCount, logDigest, previous)
}
func recoveryManifestDigest(runID, incidentID, batchCount, logCount uint64, batchDigest, logDigest string) string {
	return evidenceDigest("joincare.chain-event.reorg-recovery-manifest.v1", runID, incidentID, batchCount, logCount, batchDigest, logDigest)
}
