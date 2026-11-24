package chainevent

import (
	"context"
	"errors"
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
	reorgPrerequisiteAlgorithm        = "joincare-reorg-prerequisite-inventory-v1"
	reorgPrerequisiteStatePreparing   = "preparing"
	reorgPrerequisiteStateCompleted   = "completed"
	reorgPrerequisiteStateInvalidated = "invalidated"
	reorgPrerequisiteOutcomeBlocked   = "BLOCKED"
	reorgPrerequisiteStatusUnmet      = "unmet"
	reorgPrerequisiteStatusDeferred   = "deferred"
	reorgPrerequisiteRequirementCount = 7
	reorgPrerequisiteLeaseTTL         = 2 * time.Minute
)

type prerequisiteRequirement struct {
	Code   string
	Status string
	Detail string
}

var prerequisiteRequirements = []prerequisiteRequirement{
	{Code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", Status: reorgPrerequisiteStatusUnmet, Detail: "no trusted full state snapshot is bound to the canonical common-ancestor boundary"},
	{Code: "V1_SOURCE_ORIGIN_UNATTESTED", Status: reorgPrerequisiteStatusUnmet, Detail: "no signed V1 deployment origin and full-history source boundary is installed"},
	{Code: "GLOBAL_CONFIG_BASELINE_REQUIRED", Status: reorgPrerequisiteStatusUnmet, Detail: "no single-boundary V1 and V2 governed configuration baseline is installed"},
	{Code: "OUTBOX_GENERATION_BINDING_REQUIRED", Status: reorgPrerequisiteStatusUnmet, Detail: "the complete chain and relayer nonce domain is not bound to audited generations"},
	{Code: "GENERATION_CHECKPOINT_SCHEMA_REQUIRED", Status: reorgPrerequisiteStatusUnmet, Detail: "generation descriptor, projection, and checkpoint schema is intentionally deferred to B2-B1-B"},
	{Code: "DUAL_CONTROL_AUTHORIZATION_REQUIRED", Status: reorgPrerequisiteStatusDeferred, Detail: "digest-bound dual-control authorization is exclusively a B2-B2 operation"},
	{Code: "INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED", Status: reorgPrerequisiteStatusDeferred, Detail: "independent fixed-block chain preflight is exclusively a B2-B2 operation"},
}

type ReorgPrerequisiteJob struct{}

func NewReorgPrerequisiteJob() model.Add {
	var job cron.Job = new(ReorgPrerequisiteJob)
	return job.(model.Add)
}

func (j *ReorgPrerequisiteJob) Add(crontab *cron.Cron) {
	crontab.AddJob("19/10 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

func (j *ReorgPrerequisiteJob) Run() {
	if !chain.V2Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	outerToken, err := newRunLockToken()
	if err != nil {
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken, reorgPrerequisiteLeaseTTL)
	if err != nil || !locked {
		return
	}
	defer func() {
		_ = db.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken)
	}()

	run, _, err := ensurePrerequisiteRun()
	if errors.Is(err, errPrerequisiteAlreadyComplete) || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("prepare reorg prerequisite run err: %s", err)
		return
	}
	token, err := newAssessmentLeaseToken()
	if err != nil {
		return
	}
	job, err := claimPrerequisiteLease(run.ID, token)
	if err != nil {
		return
	}
	if err := processPrerequisiteStage(run, job, token); err != nil {
		_ = recordPrerequisiteError(run.ID, token, job.FenceEpoch, "PREREQUISITE_INVENTORY_FAILED", err)
		log.GetLogger().Errorf("prepare reorg prerequisite stage run=%d err: %s", run.ID, err)
	}
}

var errPrerequisiteAlreadyComplete = errors.New("reorg prerequisite inventory is complete")

func ensurePrerequisiteRun() (*entity.ChainEventReorgPrerequisiteRun, *entity.ChainEventReorgPrerequisiteJob, error) {
	incident, err := activeRecoveryIncident()
	if err != nil {
		return nil, nil, err
	}
	return ensurePrerequisiteRunForIncident(incident)
}

func ensurePrerequisiteRunForIncident(incident *entity.ChainEventReorgIncident) (*entity.ChainEventReorgPrerequisiteRun, *entity.ChainEventReorgPrerequisiteJob, error) {
	if incident == nil {
		return nil, nil, errors.New("active reorg incident is required")
	}
	assessment := entity.ChainEventReorgAssessmentRun{}
	if err := db.Mysql().Joins("JOIN chain_event_reorg_assessment_job j ON j.run_id=chain_event_reorg_assessment_run.id AND j.state=?", reorgAssessmentStateCompleted).
		Joins("JOIN chain_event_reorg_assessment_manifest m ON m.run_id=chain_event_reorg_assessment_run.id AND m.outcome=?", reorgAssessmentOutcomeBlocked).
		Where("chain_event_reorg_assessment_run.incident_id = ?", incident.ID).Take(&assessment).Error; err != nil {
		return nil, nil, err
	}
	manifest := entity.ChainEventReorgAssessmentManifest{}
	if err := db.Mysql().Where("run_id = ? AND incident_id = ?", assessment.ID, incident.ID).Take(&manifest).Error; err != nil {
		return nil, nil, err
	}
	existing := entity.ChainEventReorgPrerequisiteRun{}
	err := db.Mysql().Where("assessment_run_id = ?", assessment.ID).Take(&existing).Error
	if err == nil {
		job := entity.ChainEventReorgPrerequisiteJob{}
		if err := db.Mysql().Where("run_id = ?", existing.ID).Take(&job).Error; err != nil {
			return nil, nil, err
		}
		if job.State != reorgPrerequisiteStatePreparing {
			return nil, nil, errPrerequisiteAlreadyComplete
		}
		return &existing, &job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	run := &entity.ChainEventReorgPrerequisiteRun{
		RunUUID: uuid.NewString(), IncidentID: incident.ID, AssessmentRunID: assessment.ID,
		AssessmentManifestDigest: manifest.ManifestDigest, AlgorithmVersion: reorgPrerequisiteAlgorithm,
		ChainID: assessment.ChainID, BoundaryBlock: assessment.BoundaryBlock, BoundaryBlockHash: assessment.BoundaryBlockHash,
		ExpectedRequirementCount: reorgPrerequisiteRequirementCount, CreatedAt: time.Now(),
	}
	job := &entity.ChainEventReorgPrerequisiteJob{}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requirePrerequisiteInputsTx(tx, *incident, assessment, manifest); err != nil {
			return err
		}
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		active := uint8(1)
		*job = entity.ChainEventReorgPrerequisiteJob{RunID: run.ID, IncidentID: incident.ID, State: reorgPrerequisiteStatePreparing,
			ActiveSlot: &active, NextRequirementOrdinal: 1, RequirementDigest: reorgZeroDigest,
			ArtifactDigest: reorgZeroDigest, UpdatedAt: time.Now()}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return appendPrerequisiteEventTx(tx, run.ID, "CREATED", "", "created fail-closed prerequisite evidence inventory")
	})
	return run, job, err
}

func requirePrerequisiteInputsTx(tx *gorm.DB, incident entity.ChainEventReorgIncident, assessment entity.ChainEventReorgAssessmentRun, manifest entity.ChainEventReorgAssessmentManifest) error {
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND active_slot = 1 AND status = ?", incident.ID, reorgStatusDetected).Take(&entity.ChainEventReorgIncident{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", incident.CursorBlock, incident.ExpectedBlockHash).
		Take(&entity.ChainEventSyncState{}).Error; err != nil {
		return errors.New("chain event cursor changed before prerequisite inventory")
	}
	if err := tx.Where("run_id = ? AND incident_id = ? AND state = ?", assessment.ID, incident.ID, reorgAssessmentStateCompleted).Take(&entity.ChainEventReorgAssessmentJob{}).Error; err != nil {
		return errors.New("completed assessment job is unavailable")
	}
	if manifest.RunID != assessment.ID || manifest.IncidentID != incident.ID ||
		manifest.MaterializationManifestDigest != assessment.MaterializationManifestDigest ||
		manifest.Outcome != reorgAssessmentOutcomeBlocked || manifest.ManifestDigest == "" ||
		manifest.ManifestDigest != assessmentManifestDigest(manifest) {
		return errors.New("completed assessment manifest binding is invalid")
	}
	return nil
}

func claimPrerequisiteLease(runID uint64, token string) (*entity.ChainEventReorgPrerequisiteJob, error) {
	job := entity.ChainEventReorgPrerequisiteJob{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where("run_id = ? AND state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP(6))", runID, reorgPrerequisiteStatePreparing).Updates(map[string]interface{}{
			"lease_token": token, "lease_expires_at": gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL 120 SECOND)"),
			"fence_epoch": gorm.Expr("fence_epoch + 1"), "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg prerequisite lease is held by another worker")
		}
		return tx.Where("run_id = ? AND lease_token = ?", runID, token).Take(&job).Error
	})
	return &job, err
}

func processPrerequisiteStage(run *entity.ChainEventReorgPrerequisiteRun, job *entity.ChainEventReorgPrerequisiteJob, token string) error {
	if job.RequirementCount == 0 {
		return commitPrerequisiteRequirements(run, job, token)
	}
	return finalizePrerequisiteRun(run, job, token)
}

func commitPrerequisiteRequirements(run *entity.ChainEventReorgPrerequisiteRun, job *entity.ChainEventReorgPrerequisiteJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockPrerequisiteJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if locked.RequirementCount != 0 || locked.ArtifactCount != 0 {
			return errors.New("prerequisite inventory was already committed")
		}
		if err := verifyPrerequisiteSourcesTx(tx, run, locked, false); err != nil {
			return invalidatePrerequisiteRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		previous := locked.RequirementDigest
		unmet, deferred := uint64(0), uint64(0)
		for index, item := range prerequisiteRequirements {
			row := entity.ChainEventReorgPrerequisiteRequirement{RunID: run.ID, Ordinal: uint64(index + 1), RequirementCode: item.Code,
				EvidenceStatus: item.Status, SafeDetail: item.Detail, PreviousRowDigest: previous, CreatedAt: time.Now()}
			row.RowDigest = prerequisiteRequirementDigest(row)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			previous = row.RowDigest
			if item.Status == reorgPrerequisiteStatusUnmet {
				unmet++
			} else {
				deferred++
			}
		}
		result := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{
			"next_requirement_ordinal": reorgPrerequisiteRequirementCount + 1, "requirement_count": reorgPrerequisiteRequirementCount,
			"unmet_count": unmet, "deferred_count": deferred, "requirement_digest": previous,
			"lease_token": "", "lease_expires_at": nil, "last_error_code": "", "last_error_message": "",
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg prerequisite lease lost before requirement checkpoint")
		}
		return nil
	})
}

func finalizePrerequisiteRun(run *entity.ChainEventReorgPrerequisiteRun, job *entity.ChainEventReorgPrerequisiteJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockPrerequisiteJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if err := verifyPrerequisiteSourcesTx(tx, run, locked, true); err != nil {
			return invalidatePrerequisiteRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		if err := appendPrerequisiteEventTx(tx, run.ID, "COMPLETED", "", "completed blocked prerequisite declaration"); err != nil {
			return err
		}
		terminalEvent := entity.ChainEventReorgPrerequisiteEvent{}
		if err := tx.Where("run_id = ? AND sequence_no = ? AND event_type = ?", run.ID, 2, "COMPLETED").Take(&terminalEvent).Error; err != nil {
			return err
		}
		manifest := entity.ChainEventReorgPrerequisiteManifest{RunID: run.ID, IncidentID: run.IncidentID,
			AssessmentManifestDigest: run.AssessmentManifestDigest, Outcome: reorgPrerequisiteOutcomeBlocked, CandidateReady: false,
			RequirementCount: locked.RequirementCount, SatisfiedCount: 0, UnmetCount: locked.UnmetCount,
			UntrustedCount: locked.UntrustedCount, DeferredCount: locked.DeferredCount, ArtifactCount: 0,
			RequirementDigest: locked.RequirementDigest, ArtifactDigest: locked.ArtifactDigest,
			EventDigest: terminalEvent.EventHash, CompletedAt: time.Now()}
		manifest.ManifestDigest = prerequisiteManifestDigest(manifest)
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{
			"state": reorgPrerequisiteStateCompleted, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
			"last_error_code": "", "last_error_message": "", "completed_at": now, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg prerequisite lease lost before finalization")
		}
		return nil
	})
}

func verifyPrerequisiteSourcesTx(tx *gorm.DB, run *entity.ChainEventReorgPrerequisiteRun, job *entity.ChainEventReorgPrerequisiteJob, requireRequirements bool) error {
	incident := entity.ChainEventReorgIncident{}
	if err := tx.Where("id = ? AND active_slot = 1 AND status = ?", run.IncidentID, reorgStatusDetected).Take(&incident).Error; err != nil {
		return err
	}
	if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", incident.CursorBlock, incident.ExpectedBlockHash).
		Take(&entity.ChainEventSyncState{}).Error; err != nil {
		return errors.New("chain event cursor changed during prerequisite inventory")
	}
	assessment := entity.ChainEventReorgAssessmentRun{}
	if err := tx.Where("id = ? AND incident_id = ?", run.AssessmentRunID, run.IncidentID).Take(&assessment).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id = ? AND incident_id = ? AND state = ?", assessment.ID, run.IncidentID, reorgAssessmentStateCompleted).Take(&entity.ChainEventReorgAssessmentJob{}).Error; err != nil {
		return errors.New("completed assessment changed during prerequisite inventory")
	}
	manifest := entity.ChainEventReorgAssessmentManifest{}
	if err := tx.Where("run_id = ? AND incident_id = ? AND outcome = ?", assessment.ID, run.IncidentID, reorgAssessmentOutcomeBlocked).Take(&manifest).Error; err != nil {
		return err
	}
	if manifest.ManifestDigest != run.AssessmentManifestDigest || manifest.ManifestDigest != assessmentManifestDigest(manifest) || assessment.ChainID != run.ChainID || assessment.BoundaryBlock != run.BoundaryBlock || assessment.BoundaryBlockHash != run.BoundaryBlockHash {
		return errors.New("assessment source changed during prerequisite inventory")
	}
	var artifacts int64
	if err := tx.Model(&entity.ChainEventReorgPrerequisiteArtifact{}).Where("run_id = ?", run.ID).Count(&artifacts).Error; err != nil {
		return err
	}
	if artifacts != 0 || job.ArtifactCount != 0 || job.ArtifactDigest != reorgZeroDigest {
		return errors.New("B1-A v1 forbids prerequisite artifacts")
	}
	if requireRequirements {
		var requirements int64
		if err := tx.Model(&entity.ChainEventReorgPrerequisiteRequirement{}).Where("run_id = ?", run.ID).Count(&requirements).Error; err != nil {
			return err
		}
		if requirements != reorgPrerequisiteRequirementCount || job.RequirementCount != reorgPrerequisiteRequirementCount || job.UnmetCount != 5 || job.UntrustedCount != 0 || job.DeferredCount != 2 || job.NextRequirementOrdinal != reorgPrerequisiteRequirementCount+1 {
			return errors.New("prerequisite requirement set is incomplete")
		}
	}
	return nil
}

const activePrerequisiteLeaseWhere = "run_id = ? AND lease_token = ? AND fence_epoch = ? AND state = 'preparing' AND lease_expires_at > CURRENT_TIMESTAMP(6)"

func lockPrerequisiteJobTx(tx *gorm.DB, runID uint64, token string, epoch uint64) (*entity.ChainEventReorgPrerequisiteJob, error) {
	job := entity.ChainEventReorgPrerequisiteJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activePrerequisiteLeaseWhere, runID, token, epoch).Take(&job).Error
	return &job, err
}

func invalidatePrerequisiteRunTx(tx *gorm.DB, runID uint64, token string, epoch uint64, code, message string) error {
	if _, err := lockPrerequisiteJobTx(tx, runID, token, epoch); err != nil {
		return err
	}
	if err := appendPrerequisiteEventTx(tx, runID, "INVALIDATED", code, truncateFailureError(message, 500)); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
		"state": reorgPrerequisiteStateInvalidated, "active_slot": nil, "lease_token": "", "lease_expires_at": nil,
		"last_error_code": code, "last_error_message": truncateFailureError(message, 500), "completed_at": now, "updated_at": now,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("reorg prerequisite lease lost before invalidation")
	}
	return nil
}

func recordPrerequisiteError(runID uint64, token string, epoch uint64, code string, processErr error) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgPrerequisiteJob{}).Where(activePrerequisiteLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{
			"last_error_code": code, "last_error_message": truncateFailureError(processErr.Error(), 500),
			"lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg prerequisite lease lost before error checkpoint")
		}
		return nil
	})
}

func appendPrerequisiteEventTx(tx *gorm.DB, runID uint64, eventType, code, message string) error {
	previous := entity.ChainEventReorgPrerequisiteEvent{}
	previousHash, sequence := reorgZeroDigest, uint(1)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ?", runID).Order("sequence_no DESC").Take(&previous).Error
	if err == nil {
		previousHash, sequence = previous.EventHash, previous.SequenceNo+1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := entity.ChainEventReorgPrerequisiteEvent{RunID: runID, SequenceNo: sequence, EventType: eventType,
		ErrorCode: code, Message: message, PreviousEventHash: previousHash, CreatedAt: time.Now()}
	event.EventHash = prerequisiteEventDigest(event)
	return tx.Create(&event).Error
}

func prerequisiteRequirementDigest(row entity.ChainEventReorgPrerequisiteRequirement) string {
	artifactID := uint64(0)
	if row.ArtifactID != nil {
		artifactID = *row.ArtifactID
	}
	return evidenceDigest("joincare.chain-event.reorg-prerequisite-requirement.v1", row.RunID, row.Ordinal,
		row.RequirementCode, row.EvidenceStatus, artifactID, row.SafeDetail, row.PreviousRowDigest)
}

func prerequisiteArtifactDigest(row entity.ChainEventReorgPrerequisiteArtifact) string {
	return evidenceDigest("joincare.chain-event.reorg-prerequisite-artifact.v1", row.RunID, row.Ordinal,
		row.RequirementCode, row.ArtifactKind, row.SchemaVersion, row.SourceKind, row.SourceDigest,
		row.SafeSummary, row.PreviousRowDigest)
}

func prerequisiteEventDigest(row entity.ChainEventReorgPrerequisiteEvent) string {
	return evidenceDigest("joincare.chain-event.reorg-prerequisite-event.v1", row.RunID, uint64(row.SequenceNo),
		row.EventType, row.ErrorCode, row.Message, row.PreviousEventHash)
}

func prerequisiteManifestDigest(row entity.ChainEventReorgPrerequisiteManifest) string {
	return evidenceDigest("joincare.chain-event.reorg-prerequisite-manifest.v1", row.RunID, row.IncidentID,
		row.AssessmentManifestDigest, row.Outcome, row.CandidateReady, row.RequirementCount, row.SatisfiedCount,
		row.UnmetCount, row.UntrustedCount, row.DeferredCount, row.ArtifactCount, row.RequirementDigest,
		row.ArtifactDigest, row.EventDigest)
}
