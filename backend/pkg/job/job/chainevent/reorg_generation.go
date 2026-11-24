package chainevent

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

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
	reorgGenerationAlgorithm         = "joincare-reorg-generation-foundation-v1"
	reorgGenerationSchema            = "reorg-generation-v1"
	reorgGenerationKindCandidate     = "candidate"
	reorgGenerationStatePreparing    = "preparing"
	reorgGenerationStateCompleted    = "completed"
	reorgGenerationStateInvalidated  = "invalidated"
	reorgGenerationStageSnapshot     = "snapshot"
	reorgGenerationStageRequirements = "requirements"
	reorgGenerationStageFinalize     = "finalize"
	reorgGenerationStageDone         = "done"
	reorgGenerationOutcomeBlocked    = "BLOCKED"
	reorgGenerationLeaseTTL          = 2 * time.Minute
	reorgGenerationMaxRows           = 50_000
	reorgGenerationMaxPayload        = 12 * 1024 * 1024
	reorgGenerationMaxRowPayload     = 1024 * 1024
	reorgGenerationSnapshotPageSize  = 500
)

type generationRequirement struct{ Code, Status, Detail string }

var generationRequirements = []generationRequirement{
	{"BOUNDARY_STATE_SNAPSHOT_REQUIRED", "unmet", "no trusted complete state snapshot is bound to the common-ancestor boundary"},
	{"V1_SOURCE_ORIGIN_UNATTESTED", "unmet", "no signed V1 deployment origin and full-history source boundary is installed"},
	{"GLOBAL_CONFIG_BASELINE_REQUIRED", "unmet", "no fixed-boundary V1 and V2 governed configuration baseline is installed"},
	{"OUTBOX_GENERATION_BINDING_REQUIRED", "unmet", "runtime outbox and nonce rows are snapshotted but remain generation-unattributed"},
	{"GENERATION_CHECKPOINT_SCHEMA_REQUIRED", "verified", "generation-local descriptor, candidate interfaces, and checkpoint schema are installed"},
}

type ReorgGenerationJob struct{}

func NewReorgGenerationJob() model.Add {
	var job cron.Job = new(ReorgGenerationJob)
	return job.(model.Add)
}

func (j *ReorgGenerationJob) Add(crontab *cron.Cron) {
	crontab.AddJob("29/10 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

func (j *ReorgGenerationJob) Run() {
	if !chain.V2Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	outerToken, err := newRunLockToken()
	if err != nil {
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken, reorgGenerationLeaseTTL)
	if err != nil || !locked {
		return
	}
	defer func() {
		_ = db.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken)
	}()

	run, _, err := ensureGenerationRun()
	if errors.Is(err, errGenerationAlreadyComplete) || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("prepare reorg generation foundation err: %s", err)
		return
	}
	token, err := newAssessmentLeaseToken()
	if err != nil {
		return
	}
	job, err := claimGenerationLease(run.ID, token)
	if err != nil {
		return
	}
	if err := processGenerationStage(run, job, token); err != nil {
		_ = recordGenerationError(run.ID, token, job.FenceEpoch, "GENERATION_FOUNDATION_FAILED", err)
		log.GetLogger().Errorf("process reorg generation foundation run=%d err: %s", run.ID, err)
	}
}

var errGenerationAlreadyComplete = errors.New("reorg generation foundation is complete")

var generationLeaseExpiryExpr = func() clause.Expr {
	return gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6),INTERVAL 120 SECOND)")
}

func ensureGenerationRun() (*entity.ChainEventReorgGeneration, *entity.ChainEventReorgGenerationJob, error) {
	incident, err := activeRecoveryIncident()
	if err != nil {
		return nil, nil, err
	}
	return ensureGenerationRunForIncident(incident)
}

func ensureGenerationRunForIncident(incident *entity.ChainEventReorgIncident) (*entity.ChainEventReorgGeneration, *entity.ChainEventReorgGenerationJob, error) {
	if incident == nil {
		return nil, nil, errors.New("active reorg incident is required")
	}
	latest := entity.ChainEventReorgGeneration{}
	err := db.Mysql().Where("incident_id = ?", incident.ID).Order("attempt_no DESC").Take(&latest).Error
	if err == nil {
		job := entity.ChainEventReorgGenerationJob{}
		if err := db.Mysql().Where("run_id = ?", latest.ID).Take(&job).Error; err != nil {
			return nil, nil, err
		}
		if job.State == reorgGenerationStatePreparing {
			return &latest, &job, nil
		}
		if job.State == reorgGenerationStateCompleted {
			return nil, nil, errGenerationAlreadyComplete
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	prerequisite := entity.ChainEventReorgPrerequisiteRun{}
	if err := db.Mysql().Joins("JOIN chain_event_reorg_prerequisite_job j ON j.run_id=chain_event_reorg_prerequisite_run.id AND j.state=?", reorgPrerequisiteStateCompleted).
		Joins("JOIN chain_event_reorg_prerequisite_manifest m ON m.run_id=chain_event_reorg_prerequisite_run.id AND m.outcome=? AND m.candidate_ready=0", reorgPrerequisiteOutcomeBlocked).
		Where("chain_event_reorg_prerequisite_run.incident_id = ?", incident.ID).Take(&prerequisite).Error; err != nil {
		return nil, nil, err
	}
	pm := entity.ChainEventReorgPrerequisiteManifest{}
	assessment := entity.ChainEventReorgAssessmentRun{}
	am := entity.ChainEventReorgAssessmentManifest{}
	materialization := entity.ChainEventReorgMaterializationRun{}
	mm := entity.ChainEventReorgMaterializationManifest{}
	if err := db.Mysql().Where("run_id = ?", prerequisite.ID).Take(&pm).Error; err != nil {
		return nil, nil, err
	}
	if err := db.Mysql().Where("id = ?", prerequisite.AssessmentRunID).Take(&assessment).Error; err != nil {
		return nil, nil, err
	}
	if err := db.Mysql().Where("run_id = ?", assessment.ID).Take(&am).Error; err != nil {
		return nil, nil, err
	}
	if err := db.Mysql().Where("id = ?", assessment.MaterializationRunID).Take(&materialization).Error; err != nil {
		return nil, nil, err
	}
	if err := db.Mysql().Where("run_id = ?", materialization.ID).Take(&mm).Error; err != nil {
		return nil, nil, err
	}

	attempt := uint(1)
	if latest.ID != 0 {
		attempt = latest.AttemptNo + 1
	}
	run := &entity.ChainEventReorgGeneration{
		GenerationUUID: strings.ToLower(uuid.NewString()), IncidentID: incident.ID, AttemptNo: attempt,
		GenerationKind: reorgGenerationKindCandidate, PrerequisiteRunID: prerequisite.ID, PrerequisiteManifestDigest: pm.ManifestDigest,
		AssessmentRunID: assessment.ID, AssessmentManifestDigest: am.ManifestDigest,
		MaterializationRunID: materialization.ID, MaterializationManifestDigest: mm.ManifestDigest,
		ChainID: assessment.ChainID, BoundaryBlock: assessment.BoundaryBlock, BoundaryBlockHash: assessment.BoundaryBlockHash,
		TargetSafeBlock: assessment.TargetSafeBlock, TargetSafeBlockHash: assessment.TargetSafeBlockHash,
		LegacyContractAddress: materialization.LegacyContractAddress, V2ContractAddress: materialization.ContractAddress,
		LegacySourceRegistered: materialization.LegacySourceRegistered,
		OldEffectRoot:          mm.OldEffectDigest, ReplacementEffectRoot: mm.ReplacementEffectDigest,
		AlgorithmVersion: reorgGenerationAlgorithm, SchemaVersion: reorgGenerationSchema,
	}
	job := &entity.ChainEventReorgGenerationJob{}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requireGenerationInputsTx(tx, *incident, prerequisite, pm, assessment, am, materialization, mm); err != nil {
			return err
		}
		if err := tx.Raw("SELECT CURRENT_TIMESTAMP(6)").Scan(&run.CreatedAt).Error; err != nil {
			return err
		}
		run.DescriptorDigest = generationDescriptorDigest(*run)
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		active := uint8(1)
		*job = entity.ChainEventReorgGenerationJob{RunID: run.ID, IncidentID: incident.ID, State: reorgGenerationStatePreparing,
			Stage: reorgGenerationStageSnapshot, ActiveSlot: &active, SenderRoot: reorgZeroDigest,
			SubmissionRoot: reorgZeroDigest, NonceRoot: reorgZeroDigest, RequirementRoot: reorgZeroDigest, UpdatedAt: time.Now()}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return appendGenerationEventTx(tx, run.ID, "CREATED", "create", "", "created fail-closed candidate generation foundation", 0, reorgZeroDigest)
	})
	return run, job, err
}

func requireGenerationInputsTx(tx *gorm.DB, incident entity.ChainEventReorgIncident, p entity.ChainEventReorgPrerequisiteRun, pm entity.ChainEventReorgPrerequisiteManifest, a entity.ChainEventReorgAssessmentRun, am entity.ChainEventReorgAssessmentManifest, m entity.ChainEventReorgMaterializationRun, mm entity.ChainEventReorgMaterializationManifest) error {
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND active_slot=1 AND status=?", incident.ID, reorgStatusDetected).Take(&entity.ChainEventReorgIncident{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id=1 AND cursor_block=? AND cursor_block_hash=?", incident.CursorBlock, incident.ExpectedBlockHash).Take(&entity.ChainEventSyncState{}).Error; err != nil {
		return errors.New("chain event cursor changed before generation foundation")
	}
	if p.IncidentID != incident.ID || p.AssessmentRunID != a.ID || pm.RunID != p.ID || pm.ManifestDigest != prerequisiteManifestDigest(pm) || pm.Outcome != reorgPrerequisiteOutcomeBlocked || pm.CandidateReady {
		return errors.New("prerequisite manifest binding is invalid")
	}
	if a.IncidentID != incident.ID || a.MaterializationRunID != m.ID || am.RunID != a.ID || am.ManifestDigest != assessmentManifestDigest(am) {
		return errors.New("assessment manifest binding is invalid")
	}
	if m.IncidentID != incident.ID || mm.RunID != m.ID || mm.ManifestDigest != materializationManifestDigest(m.ID, incident.ID, mm.RecoveryManifestDigest, mm.OldEffectCount, mm.ReplacementEffectCount, mm.IgnoredLogCount, mm.OldEffectDigest, mm.ReplacementEffectDigest) {
		return errors.New("materialization manifest binding is invalid")
	}
	return nil
}

func claimGenerationLease(runID uint64, token string) (*entity.ChainEventReorgGenerationJob, error) {
	job := entity.ChainEventReorgGenerationJob{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where("run_id=? AND state=? AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP(6))", runID, reorgGenerationStatePreparing).Updates(map[string]interface{}{
			"lease_token": token, "lease_expires_at": generationLeaseExpiryExpr(),
			"fence_epoch": gorm.Expr("fence_epoch+1"), "attempt_count": gorm.Expr("attempt_count+1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg generation lease is held by another worker")
		}
		return tx.Where("run_id=? AND lease_token=?", runID, token).Take(&job).Error
	})
	return &job, err
}

func processGenerationStage(run *entity.ChainEventReorgGeneration, job *entity.ChainEventReorgGenerationJob, token string) error {
	switch job.Stage {
	case reorgGenerationStageSnapshot:
		return commitGenerationSnapshot(run, job, token)
	case reorgGenerationStageRequirements:
		return commitGenerationRequirements(run, job, token)
	case reorgGenerationStageFinalize:
		return finalizeGenerationRun(run, job, token)
	default:
		return errors.New("unknown generation foundation stage")
	}
}

type generationSnapshot struct {
	Senders      []generationSenderSource
	Submissions  []entity.ChainTxSubmission
	Nonces       []entity.ChainRelayerNonce
	PayloadBytes uint64
}
type generationSenderSource struct {
	Address    string
	Legacy, V2 bool
}

type generationSubmissionIdentity struct {
	ID              uint64
	SenderAddress   string
	ContractAddress string
}

func loadGenerationSnapshotTx(tx *gorm.DB, run *entity.ChainEventReorgGeneration) (*generationSnapshot, error) {
	type rowCount struct{ Count uint64 }
	chainResources := rowCount{}
	if err := tx.Raw(`SELECT COUNT(*) count FROM chain_tx_submission WHERE chain_id=?`, run.ChainID).Scan(&chainResources).Error; err != nil {
		return nil, err
	}
	if chainResources.Count > reorgGenerationMaxRows {
		return nil, errors.New("generation incident-chain submission row limit exceeded")
	}
	chainRows := make([]generationSubmissionIdentity, 0, chainResources.Count)
	lastSender, lastID := "", uint64(0)
	for uint64(len(chainRows)) < chainResources.Count {
		page := make([]generationSubmissionIdentity, 0, reorgGenerationSnapshotPageSize)
		if err := tx.Raw(`SELECT id,sender_address,contract_address FROM chain_tx_submission FORCE INDEX (idx_chain_tx_generation_sender)
			WHERE chain_id=? AND (sender_address>? OR (sender_address=? AND id>?))
			ORDER BY sender_address ASC,id ASC LIMIT ? FOR UPDATE`, run.ChainID, lastSender, lastSender,
			lastID, reorgGenerationSnapshotPageSize).Scan(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return nil, errors.New("generation incident-chain submission source changed during preflight")
		}
		chainRows = append(chainRows, page...)
		last := page[len(page)-1]
		lastSender, lastID = strings.ToLower(last.SenderAddress), last.ID
	}
	senderMap := make(map[string]*generationSenderSource)
	for _, row := range chainRows {
		if !(run.LegacySourceRegistered && strings.EqualFold(row.ContractAddress, run.LegacyContractAddress)) && !strings.EqualFold(row.ContractAddress, run.V2ContractAddress) {
			continue
		}
		address := strings.ToLower(row.SenderAddress)
		item := senderMap[address]
		if item == nil {
			item = &generationSenderSource{Address: address}
			senderMap[address] = item
		}
		item.Legacy = item.Legacy || (run.LegacySourceRegistered && strings.EqualFold(row.ContractAddress, run.LegacyContractAddress))
		item.V2 = item.V2 || strings.EqualFold(row.ContractAddress, run.V2ContractAddress)
	}
	if len(senderMap) > reorgGenerationMaxRows {
		return nil, errors.New("generation sender row limit exceeded")
	}
	senders := make([]generationSenderSource, 0, len(senderMap))
	for _, sender := range senderMap {
		senders = append(senders, *sender)
	}
	sort.Slice(senders, func(i, j int) bool { return senders[i].Address < senders[j].Address })
	addresses := make([]string, len(senders))
	for index := range senders {
		addresses[index] = senders[index].Address
	}
	sort.Strings(addresses)
	result := &generationSnapshot{Senders: senders}
	if len(addresses) == 0 {
		return result, nil
	}
	type submissionResources struct {
		Count        uint64
		PayloadBytes uint64
		MaxCallData  uint64
		MaxRawTx     uint64
	}
	resources := submissionResources{}
	if err := tx.Raw(`SELECT COUNT(*) count,
		COALESCE(SUM(OCTET_LENGTH(call_data)+COALESCE(OCTET_LENGTH(raw_tx),0)),0) payload_bytes,
		COALESCE(MAX(OCTET_LENGTH(call_data)),0) max_call_data,
		COALESCE(MAX(OCTET_LENGTH(raw_tx)),0) max_raw_tx
		FROM chain_tx_submission FORCE INDEX (idx_chain_tx_generation_sender)
		WHERE chain_id=? AND sender_address IN ?`, run.ChainID, addresses).Scan(&resources).Error; err != nil {
		return nil, err
	}
	if resources.Count > reorgGenerationMaxRows || resources.PayloadBytes > reorgGenerationMaxPayload {
		return nil, errors.New("generation submission resource limit exceeded")
	}
	if resources.MaxCallData == 0 || resources.MaxCallData > reorgGenerationMaxRowPayload || resources.MaxRawTx > reorgGenerationMaxRowPayload {
		return nil, errors.New("generation submission row payload limit exceeded")
	}
	rows := make([]entity.ChainTxSubmission, 0, resources.Count)
	lastSender, lastID = "", 0
	for uint64(len(rows)) < resources.Count {
		page := make([]entity.ChainTxSubmission, 0, reorgGenerationSnapshotPageSize)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"chain_id=? AND sender_address IN ? AND (sender_address>? OR (sender_address=? AND id>?))",
			run.ChainID, addresses, lastSender, lastSender, lastID,
		).Order("sender_address ASC,id ASC").Limit(reorgGenerationSnapshotPageSize).Find(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return nil, errors.New("generation submission source changed during resource preflight")
		}
		rows = append(rows, page...)
		last := page[len(page)-1]
		lastSender, lastID = strings.ToLower(last.SenderAddress), last.ID
	}
	if uint64(len(rows)) != resources.Count {
		return nil, errors.New("generation submission source changed during resource preflight")
	}
	type nonceCount struct{ Count uint64 }
	nonceResources := nonceCount{}
	if err := tx.Raw(`SELECT COUNT(*) count FROM chain_relayer_nonce
		WHERE chain_id=? AND sender_address IN ?`, run.ChainID, addresses).Scan(&nonceResources).Error; err != nil {
		return nil, err
	}
	if nonceResources.Count > reorgGenerationMaxRows {
		return nil, errors.New("generation nonce row limit exceeded")
	}
	nonces := make([]entity.ChainRelayerNonce, 0, nonceResources.Count)
	lastSender = ""
	for uint64(len(nonces)) < nonceResources.Count {
		page := make([]entity.ChainRelayerNonce, 0, reorgGenerationSnapshotPageSize)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"chain_id=? AND sender_address IN ? AND sender_address>?", run.ChainID, addresses, lastSender,
		).Order("sender_address ASC").Limit(reorgGenerationSnapshotPageSize).Find(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return nil, errors.New("generation nonce source changed during preflight")
		}
		nonces = append(nonces, page...)
		lastSender = strings.ToLower(page[len(page)-1].SenderAddress)
	}
	result.Submissions, result.Nonces, result.PayloadBytes = rows, nonces, resources.PayloadBytes
	return result, nil
}

func commitGenerationSnapshot(run *entity.ChainEventReorgGeneration, job *entity.ChainEventReorgGenerationJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := lockGenerationIncidentTx(tx, run); err != nil {
			return err
		}
		locked, err := lockGenerationJobTx(tx, run.ID, token, job.FenceEpoch, reorgGenerationStageSnapshot)
		if err != nil {
			return err
		}
		if err := verifyGenerationParentEvidenceTx(tx, run); err != nil {
			return invalidateGenerationRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		snapshot, err := loadGenerationSnapshotTx(tx, run)
		if err != nil {
			return err
		}
		tokenDigest := sha256Hex([]byte(token))
		senderRoot := reorgZeroDigest
		for index, source := range snapshot.Senders {
			row := entity.ChainEventReorgGenerationSender{RunID: run.ID, Ordinal: uint64(index + 1), ChainID: run.ChainID, SenderAddress: source.Address,
				SourceContractRoot: generationSenderSourceDigest(run.ChainID, source), WriterFenceEpoch: job.FenceEpoch,
				WriterLeaseTokenDigest: tokenDigest, PreviousRowDigest: senderRoot, CreatedAt: time.Now()}
			row.RowDigest = generationSenderRowDigest(row)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			senderRoot = row.RowDigest
		}
		submissionRoot := reorgZeroDigest
		for index, source := range snapshot.Submissions {
			row := generationSubmissionSnapshot(run.ID, uint64(index+1), source, job.FenceEpoch, tokenDigest, submissionRoot)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			submissionRoot = row.RowDigest
		}
		nonceRoot := reorgZeroDigest
		for index, source := range snapshot.Nonces {
			row := generationNonceSnapshot(run.ID, uint64(index+1), source, job.FenceEpoch, tokenDigest, nonceRoot)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			nonceRoot = row.RowDigest
		}
		result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where(activeGenerationLeaseWhere+" AND stage=?", run.ID, token, job.FenceEpoch, reorgGenerationStageSnapshot).Updates(map[string]interface{}{
			"stage": reorgGenerationStageRequirements, "sender_count": len(snapshot.Senders), "submission_count": len(snapshot.Submissions),
			"submission_payload_bytes": snapshot.PayloadBytes, "nonce_count": len(snapshot.Nonces), "sender_root": senderRoot,
			"submission_root": submissionRoot, "nonce_root": nonceRoot, "lease_token": "", "lease_expires_at": nil,
			"last_error_code": "", "last_error_message": "", "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("generation lease lost before snapshot checkpoint")
		}
		_ = locked
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func commitGenerationRequirements(run *entity.ChainEventReorgGeneration, job *entity.ChainEventReorgGenerationJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := lockGenerationIncidentTx(tx, run); err != nil {
			return err
		}
		locked, err := lockGenerationJobTx(tx, run.ID, token, job.FenceEpoch, reorgGenerationStageRequirements)
		if err != nil {
			return err
		}
		if err := verifyGenerationParentEvidenceTx(tx, run); err != nil {
			return invalidateGenerationRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		previous, tokenDigest := reorgZeroDigest, sha256Hex([]byte(token))
		for index, item := range generationRequirements {
			row := entity.ChainEventReorgGenerationRequirement{RunID: run.ID, Ordinal: uint64(index + 1), RequirementCode: item.Code,
				EvidenceStatus: item.Status, SafeDetail: item.Detail, WriterFenceEpoch: job.FenceEpoch,
				WriterLeaseTokenDigest: tokenDigest, PreviousRowDigest: previous, CreatedAt: time.Now()}
			row.RowDigest = generationRequirementRowDigest(row)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			previous = row.RowDigest
		}
		result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where(activeGenerationLeaseWhere+" AND stage=?", run.ID, token, job.FenceEpoch, reorgGenerationStageRequirements).Updates(map[string]interface{}{
			"stage": reorgGenerationStageFinalize, "requirement_count": 5, "unmet_count": 4, "verified_count": 1,
			"requirement_root": previous, "lease_token": "", "lease_expires_at": nil, "last_error_code": "", "last_error_message": "",
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("generation lease lost before requirement checkpoint")
		}
		_ = locked
		return nil
	})
}

func finalizeGenerationRun(run *entity.ChainEventReorgGeneration, job *entity.ChainEventReorgGenerationJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := lockGenerationIncidentTx(tx, run); err != nil {
			return err
		}
		locked, err := lockGenerationJobTx(tx, run.ID, token, job.FenceEpoch, reorgGenerationStageFinalize)
		if err != nil {
			return err
		}
		if err := verifyGenerationSourcesTx(tx, run, locked); err != nil {
			return invalidateGenerationRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		tokenDigest := sha256Hex([]byte(token))
		if err := appendGenerationEventTx(tx, run.ID, "COMPLETED", "finalize", "", "completed blocked candidate generation foundation", job.FenceEpoch, tokenDigest); err != nil {
			return err
		}
		terminal := entity.ChainEventReorgGenerationEvent{}
		if err := tx.Where("run_id=? AND sequence_no=2", run.ID).Take(&terminal).Error; err != nil {
			return err
		}
		manifest := entity.ChainEventReorgGenerationManifest{RunID: run.ID, IncidentID: run.IncidentID, DescriptorDigest: run.DescriptorDigest,
			PrerequisiteManifestDigest: run.PrerequisiteManifestDigest, AssessmentManifestDigest: run.AssessmentManifestDigest,
			MaterializationManifestDigest: run.MaterializationManifestDigest, Outcome: reorgGenerationOutcomeBlocked, CandidateReady: false,
			RequirementCount: 5, UnmetCount: 4, VerifiedCount: 1, SenderCount: locked.SenderCount, SubmissionCount: locked.SubmissionCount,
			SubmissionPayloadBytes: locked.SubmissionPayloadBytes, NonceCount: locked.NonceCount, SenderRoot: locked.SenderRoot,
			SubmissionRoot: locked.SubmissionRoot, NonceRoot: locked.NonceRoot, RequirementRoot: locked.RequirementRoot,
			CandidateRoot: reorgZeroDigest, DeltaRoot: reorgZeroDigest, CheckpointRoot: reorgZeroDigest,
			EventCount: 2, EventRoot: terminal.EventHash, WriterFenceEpoch: job.FenceEpoch,
			WriterLeaseTokenDigest: tokenDigest, CompletedAt: time.Now()}
		manifest.ManifestDigest = generationManifestDigest(manifest)
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where(activeGenerationLeaseWhere+" AND stage=?", run.ID, token, job.FenceEpoch, reorgGenerationStageFinalize).Updates(map[string]interface{}{
			"state": reorgGenerationStateCompleted, "stage": reorgGenerationStageDone, "active_slot": nil, "lease_token": "",
			"lease_expires_at": nil, "last_error_code": "", "last_error_message": "", "completed_at": now, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("generation lease lost before finalization")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func lockGenerationIncidentTx(tx *gorm.DB, run *entity.ChainEventReorgGeneration) error {
	incident := entity.ChainEventReorgIncident{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND active_slot=1 AND status=?", run.IncidentID, reorgStatusDetected).Take(&incident).Error; err != nil {
		return err
	}
	if err := tx.Where("id=1 AND cursor_block=? AND cursor_block_hash=?", incident.CursorBlock, incident.ExpectedBlockHash).Take(&entity.ChainEventSyncState{}).Error; err != nil {
		return errors.New("chain event cursor changed during generation foundation")
	}
	return nil
}

func verifyGenerationParentEvidenceTx(tx *gorm.DB, run *entity.ChainEventReorgGeneration) error {
	pm := entity.ChainEventReorgPrerequisiteManifest{}
	am := entity.ChainEventReorgAssessmentManifest{}
	mm := entity.ChainEventReorgMaterializationManifest{}
	if err := tx.Where("run_id=?", run.PrerequisiteRunID).Take(&pm).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id=?", run.AssessmentRunID).Take(&am).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id=?", run.MaterializationRunID).Take(&mm).Error; err != nil {
		return err
	}
	if pm.ManifestDigest != run.PrerequisiteManifestDigest || pm.ManifestDigest != prerequisiteManifestDigest(pm) || pm.Outcome != reorgPrerequisiteOutcomeBlocked || pm.CandidateReady {
		return errors.New("prerequisite source changed during generation foundation")
	}
	if am.ManifestDigest != run.AssessmentManifestDigest || am.ManifestDigest != assessmentManifestDigest(am) {
		return errors.New("assessment source changed during generation foundation")
	}
	if mm.ManifestDigest != run.MaterializationManifestDigest || mm.ManifestDigest != materializationManifestDigest(mm.RunID, run.IncidentID, mm.RecoveryManifestDigest, mm.OldEffectCount, mm.ReplacementEffectCount, mm.IgnoredLogCount, mm.OldEffectDigest, mm.ReplacementEffectDigest) {
		return errors.New("materialization source changed during generation foundation")
	}
	if generationDescriptorDigest(*run) != run.DescriptorDigest {
		return errors.New("generation descriptor digest changed")
	}
	return nil
}

func verifyGenerationSourcesTx(tx *gorm.DB, run *entity.ChainEventReorgGeneration, job *entity.ChainEventReorgGenerationJob) error {
	if err := verifyGenerationParentEvidenceTx(tx, run); err != nil {
		return err
	}
	snapshot, err := loadGenerationSnapshotTx(tx, run)
	if err != nil {
		return err
	}
	if uint64(len(snapshot.Senders)) != job.SenderCount || uint64(len(snapshot.Submissions)) != job.SubmissionCount || uint64(len(snapshot.Nonces)) != job.NonceCount || snapshot.PayloadBytes != job.SubmissionPayloadBytes {
		return errors.New("generation runtime source counts changed")
	}
	storedSenders := make([]entity.ChainEventReorgGenerationSender, 0)
	storedSubmissions := make([]entity.ChainEventReorgGenerationSubmission, 0)
	storedNonces := make([]entity.ChainEventReorgGenerationNonce, 0)
	if err := tx.Where("run_id=?", run.ID).Order("ordinal ASC").Find(&storedSenders).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id=?", run.ID).Order("ordinal ASC").Find(&storedSubmissions).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id=?", run.ID).Order("ordinal ASC").Find(&storedNonces).Error; err != nil {
		return err
	}
	if len(storedSenders) != len(snapshot.Senders) || len(storedSubmissions) != len(snapshot.Submissions) || len(storedNonces) != len(snapshot.Nonces) {
		return errors.New("generation snapshot sets are incomplete")
	}
	senderPrevious := reorgZeroDigest
	for i, source := range snapshot.Senders {
		stored := storedSenders[i]
		if stored.Ordinal != uint64(i+1) || stored.PreviousRowDigest != senderPrevious || stored.RowDigest != generationSenderRowDigest(stored) || stored.SenderAddress != source.Address || stored.SourceContractRoot != generationSenderSourceDigest(run.ChainID, source) {
			return errors.New("generation sender source changed")
		}
		senderPrevious = stored.RowDigest
	}
	submissionPrevious := reorgZeroDigest
	for i, source := range snapshot.Submissions {
		stored := storedSubmissions[i]
		if stored.Ordinal != uint64(i+1) || stored.PreviousRowDigest != submissionPrevious || stored.RowDigest != generationSubmissionRowDigest(stored) || stored.SourceSubmissionID != source.ID || stored.SourceSnapshotDigest != generationSubmissionSourceDigest(source) {
			return errors.New("generation submission source changed")
		}
		submissionPrevious = stored.RowDigest
	}
	noncePrevious := reorgZeroDigest
	for i, source := range snapshot.Nonces {
		stored := storedNonces[i]
		if stored.Ordinal != uint64(i+1) || stored.PreviousRowDigest != noncePrevious || stored.RowDigest != generationNonceRowDigest(stored) || stored.SenderAddress != strings.ToLower(source.SenderAddress) || stored.SourceSnapshotDigest != generationNonceSourceDigest(source) {
			return errors.New("generation nonce source changed")
		}
		noncePrevious = stored.RowDigest
	}
	if rootOfGenerationSenders(storedSenders) != job.SenderRoot || rootOfGenerationSubmissions(storedSubmissions) != job.SubmissionRoot || rootOfGenerationNonces(storedNonces) != job.NonceRoot {
		return errors.New("generation snapshot roots changed")
	}
	var requirements []entity.ChainEventReorgGenerationRequirement
	if err := tx.Where("run_id=?", run.ID).Order("ordinal ASC").Find(&requirements).Error; err != nil {
		return err
	}
	if len(requirements) != 5 || requirements[4].EvidenceStatus != "verified" || rootOfGenerationRequirements(requirements) != job.RequirementRoot {
		return errors.New("generation requirement set changed")
	}
	requirementPrevious := reorgZeroDigest
	for index, stored := range requirements {
		if stored.Ordinal != uint64(index+1) || stored.PreviousRowDigest != requirementPrevious || stored.RowDigest != generationRequirementRowDigest(stored) {
			return errors.New("generation requirement digest chain changed")
		}
		requirementPrevious = stored.RowDigest
	}
	return nil
}

const activeGenerationLeaseWhere = "run_id=? AND lease_token=? AND fence_epoch=? AND state='preparing' AND lease_expires_at>CURRENT_TIMESTAMP(6)"

func lockGenerationJobTx(tx *gorm.DB, runID uint64, token string, epoch uint64, stage string) (*entity.ChainEventReorgGenerationJob, error) {
	job := entity.ChainEventReorgGenerationJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activeGenerationLeaseWhere+" AND stage=?", runID, token, epoch, stage).Take(&job).Error
	return &job, err
}

func invalidateGenerationRunTx(tx *gorm.DB, runID uint64, token string, epoch uint64, code, message string) error {
	job := entity.ChainEventReorgGenerationJob{}
	if err := tx.Where(activeGenerationLeaseWhere, runID, token, epoch).Take(&job).Error; err != nil {
		return err
	}
	message = truncateFailureError(message, 500)
	tokenDigest := sha256Hex([]byte(token))
	if err := appendGenerationEventTx(tx, runID, "INVALIDATED", "invalidate", code, message, epoch, tokenDigest); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where(activeGenerationLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
		"state": reorgGenerationStateInvalidated, "stage": reorgGenerationStageDone, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
		"last_error_code": code, "last_error_message": message, "completed_at": now, "updated_at": now,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("generation lease lost before invalidation")
	}
	return nil
}

func recordGenerationError(runID uint64, token string, epoch uint64, code string, processErr error) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgGenerationJob{}).Where(activeGenerationLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
			"last_error_code": code, "last_error_message": truncateFailureError(processErr.Error(), 500), "lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("generation lease lost before error checkpoint")
		}
		return nil
	})
}

func appendGenerationEventTx(tx *gorm.DB, runID uint64, eventType, stage, code, message string, epoch uint64, tokenDigest string) error {
	previous := entity.ChainEventReorgGenerationEvent{}
	previousHash, sequence := reorgZeroDigest, uint(1)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id=?", runID).Order("sequence_no DESC").Take(&previous).Error
	if err == nil {
		previousHash, sequence = previous.EventHash, previous.SequenceNo+1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := entity.ChainEventReorgGenerationEvent{RunID: runID, SequenceNo: sequence, EventType: eventType, Stage: stage, ErrorCode: code, Message: message,
		WriterFenceEpoch: epoch, WriterLeaseTokenDigest: tokenDigest, PreviousEventHash: previousHash, CreatedAt: time.Now()}
	event.EventHash = generationEventDigest(event)
	return tx.Create(&event).Error
}

func generationSubmissionSnapshot(runID, ordinal uint64, source entity.ChainTxSubmission, epoch uint64, tokenDigest, previous string) entity.ChainEventReorgGenerationSubmission {
	row := entity.ChainEventReorgGenerationSubmission{RunID: runID, Ordinal: ordinal, SourceSubmissionID: source.ID, ProjectID: source.ProjectID, Action: source.Action,
		SourceAttemptNo: source.AttemptNo, ChainID: source.ChainID, SenderAddress: strings.ToLower(source.SenderAddress), ContractAddress: strings.ToLower(source.ContractAddress),
		CallData: source.CallData, CallDataDigest: sha256Hex(source.CallData), ValueRaw: source.ValueRaw, SourceState: source.State, Nonce: source.Nonce, GasLimit: source.GasLimit,
		GasPriceRaw: source.GasPriceRaw, RawTx: source.RawTx, RawTxDigest: nullableBytesDigest(source.RawTx), TxHash: lowerOptional(source.TxHash), SourceActiveSlot: source.ActiveSlot,
		BroadcastCount: source.BroadcastCount, LastBroadcastAt: source.LastBroadcastAt, LastErrorDigest: sha256Hex([]byte(source.LastError)), ReceiptStatus: source.ReceiptStatus,
		ReceiptBlockNumber: source.ReceiptBlockNumber, ReceiptBlockHash: strings.ToLower(source.ReceiptBlockHash), ReceiptGasUsed: source.ReceiptGasUsed,
		FinalEventRecordID: source.FinalEventRecordID, FinalizedAt: source.FinalizedAt, CancelledByAdminID: source.CancelledByAdminID,
		CancelReasonDigest: sha256Hex([]byte(source.CancelReason)), CancelledAt: source.CancelledAt, GenerationDisposition: "unresolved", WriterFenceEpoch: epoch,
		WriterLeaseTokenDigest: tokenDigest, PreviousRowDigest: previous, SourceCreatedAt: source.CreatedAt, SourceUpdatedAt: source.UpdatedAt, CreatedAt: time.Now()}
	row.SourceSnapshotDigest = generationSubmissionSourceDigest(source)
	row.RowDigest = generationSubmissionRowDigest(row)
	return row
}

func generationNonceSnapshot(runID, ordinal uint64, source entity.ChainRelayerNonce, epoch uint64, tokenDigest, previous string) entity.ChainEventReorgGenerationNonce {
	row := entity.ChainEventReorgGenerationNonce{RunID: runID, Ordinal: ordinal, ChainID: source.ChainID, SenderAddress: strings.ToLower(source.SenderAddress), NextNonce: source.NextNonce,
		ActiveSubmissionID: source.ActiveSubmissionID, WriterFenceEpoch: epoch, WriterLeaseTokenDigest: tokenDigest, PreviousRowDigest: previous,
		SourceCreatedAt: source.CreatedAt, SourceUpdatedAt: source.UpdatedAt, CreatedAt: time.Now()}
	row.SourceSnapshotDigest = generationNonceSourceDigest(source)
	row.RowDigest = generationNonceRowDigest(row)
	return row
}

func generationDescriptorDigest(r entity.ChainEventReorgGeneration) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-descriptor.v1", r.GenerationUUID, r.IncidentID, r.AttemptNo, r.GenerationKind, r.PrerequisiteRunID, r.PrerequisiteManifestDigest, r.AssessmentRunID, r.AssessmentManifestDigest, r.MaterializationRunID, r.MaterializationManifestDigest, r.ChainID, r.BoundaryBlock, r.BoundaryBlockHash, r.TargetSafeBlock, r.TargetSafeBlockHash, r.LegacySourceRegistered, r.LegacyContractAddress, r.V2ContractAddress, r.OldEffectRoot, r.ReplacementEffectRoot, r.AlgorithmVersion, r.SchemaVersion, formatGenerationTime(r.CreatedAt))
}
func generationSenderSourceDigest(chainID uint64, s generationSenderSource) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-sender-source.v1", chainID, s.Address, s.Legacy, s.V2)
}
func generationSenderRowDigest(r entity.ChainEventReorgGenerationSender) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-sender.v1", r.RunID, r.Ordinal, r.ChainID, r.SenderAddress, r.SourceContractRoot, r.WriterFenceEpoch, r.WriterLeaseTokenDigest, r.PreviousRowDigest)
}
func generationSubmissionRowDigest(r entity.ChainEventReorgGenerationSubmission) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-submission.v1", r.RunID, r.Ordinal, r.SourceSubmissionID, r.SourceSnapshotDigest, r.GenerationDisposition, r.WriterFenceEpoch, r.WriterLeaseTokenDigest, r.PreviousRowDigest)
}
func generationNonceRowDigest(r entity.ChainEventReorgGenerationNonce) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-nonce.v1", r.RunID, r.Ordinal, r.ChainID, r.SenderAddress, r.NextNonce, optionalUint(r.ActiveSubmissionID), r.SourceSnapshotDigest, r.WriterFenceEpoch, r.WriterLeaseTokenDigest, r.PreviousRowDigest)
}
func generationRequirementRowDigest(r entity.ChainEventReorgGenerationRequirement) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-requirement.v1", r.RunID, r.Ordinal, r.RequirementCode, r.EvidenceStatus, r.SafeDetail, r.WriterFenceEpoch, r.WriterLeaseTokenDigest, r.PreviousRowDigest)
}
func generationEventDigest(r entity.ChainEventReorgGenerationEvent) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-event.v1", r.RunID, r.SequenceNo, r.EventType, r.Stage, r.ErrorCode, r.Message, r.WriterFenceEpoch, r.WriterLeaseTokenDigest, r.PreviousEventHash)
}
func generationManifestDigest(r entity.ChainEventReorgGenerationManifest) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-manifest.v1", r.RunID, r.IncidentID, r.DescriptorDigest, r.PrerequisiteManifestDigest, r.AssessmentManifestDigest, r.MaterializationManifestDigest, r.Outcome, r.CandidateReady, r.RequirementCount, r.UnmetCount, r.VerifiedCount, r.SenderCount, r.SubmissionCount, r.SubmissionPayloadBytes, r.NonceCount, r.CandidateProjectCount, r.CandidateLedgerCount, r.CandidateEntitlementCount, r.CandidateV1FactCount, r.CandidateWalletCount, r.SignedDeltaCount, r.CheckpointCount, r.SenderRoot, r.SubmissionRoot, r.NonceRoot, r.RequirementRoot, r.CandidateRoot, r.DeltaRoot, r.CheckpointRoot, r.EventCount, r.EventRoot, r.WriterFenceEpoch, r.WriterLeaseTokenDigest)
}

func generationSubmissionSourceDigest(s entity.ChainTxSubmission) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-submission-source.v1", s.ID, s.ProjectID, s.Action, s.AttemptNo, s.ChainID, strings.ToLower(s.SenderAddress), strings.ToLower(s.ContractAddress), sha256Hex(s.CallData), s.ValueRaw, s.State, optionalUint(s.Nonce), optionalUint(s.GasLimit), s.GasPriceRaw, nullableBytesDigest(s.RawTx), optionalString(lowerOptional(s.TxHash)), optionalUint8(s.ActiveSlot), s.BroadcastCount, optionalTime(s.LastBroadcastAt), sha256Hex([]byte(s.LastError)), optionalUint(s.ReceiptStatus), optionalUint(s.ReceiptBlockNumber), strings.ToLower(s.ReceiptBlockHash), optionalUint(s.ReceiptGasUsed), optionalUint(s.FinalEventRecordID), optionalTime(s.FinalizedAt), optionalUint(s.CancelledByAdminID), sha256Hex([]byte(s.CancelReason)), optionalTime(s.CancelledAt), formatGenerationTime(s.CreatedAt), formatGenerationTime(s.UpdatedAt))
}
func generationNonceSourceDigest(n entity.ChainRelayerNonce) string {
	return evidenceDigest("joincare.chain-event.reorg-generation-nonce-source.v1", n.ChainID, strings.ToLower(n.SenderAddress), n.NextNonce, optionalUint(n.ActiveSubmissionID), formatGenerationTime(n.CreatedAt), formatGenerationTime(n.UpdatedAt))
}

func nullableBytesDigest(value []byte) string {
	if value == nil {
		return reorgZeroDigest
	}
	return sha256Hex(value)
}
func lowerOptional(value *string) *string {
	if value == nil {
		return nil
	}
	v := strings.ToLower(*value)
	return &v
}
func optionalString(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}
func optionalUint[T ~uint64](value *T) any {
	if value == nil {
		return "null"
	}
	return *value
}
func optionalUint8(value *uint8) any {
	if value == nil {
		return "null"
	}
	return *value
}
func optionalTime(value *time.Time) string {
	if value == nil {
		return "null"
	}
	return formatGenerationTime(*value)
}
func formatGenerationTime(value time.Time) string { return value.Format("2006-01-02T15:04:05.000000Z") }
func rootOfGenerationSenders(rows []entity.ChainEventReorgGenerationSender) string {
	if len(rows) == 0 {
		return reorgZeroDigest
	}
	return rows[len(rows)-1].RowDigest
}
func rootOfGenerationSubmissions(rows []entity.ChainEventReorgGenerationSubmission) string {
	if len(rows) == 0 {
		return reorgZeroDigest
	}
	return rows[len(rows)-1].RowDigest
}
func rootOfGenerationNonces(rows []entity.ChainEventReorgGenerationNonce) string {
	if len(rows) == 0 {
		return reorgZeroDigest
	}
	return rows[len(rows)-1].RowDigest
}
func rootOfGenerationRequirements(rows []entity.ChainEventReorgGenerationRequirement) string {
	if len(rows) == 0 {
		return reorgZeroDigest
	}
	return rows[len(rows)-1].RowDigest
}
