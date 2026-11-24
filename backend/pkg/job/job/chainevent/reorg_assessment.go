package chainevent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

const (
	reorgAssessmentAlgorithm          = "joincare-reorg-reversibility-assessment-v1"
	reorgAssessmentStatePreparing     = "preparing"
	reorgAssessmentStateCompleted     = "completed"
	reorgAssessmentStateInvalidated   = "invalidated"
	reorgAssessmentReplayExact        = "exact"
	reorgAssessmentReplayConditional  = "conditional"
	reorgAssessmentReplayBlocked      = "blocked"
	reorgAssessmentReplayNoop         = "noop"
	reorgAssessmentOutcomeBlocked     = "BLOCKED"
	reorgAssessmentPageSize           = 100
	reorgAssessmentLeaseTTL           = 2 * time.Minute
	reorgAssessmentMaxEffects         = 99_993
	reorgAssessmentMaxBlockers        = 100_000
	reorgAssessmentMaxPayloadBytes    = 64 * 1024 * 1024
	reorgAssessmentGlobalBlockerCount = 7
)

type ReorgAssessmentJob struct{}

func NewReorgAssessmentJob() model.Add {
	var job cron.Job = new(ReorgAssessmentJob)
	return job.(model.Add)
}

func (j *ReorgAssessmentJob) Add(crontab *cron.Cron) {
	crontab.AddJob("9/10 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(j))
}

func (j *ReorgAssessmentJob) Run() {
	if !chain.V2Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	outerToken, err := newRunLockToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg assessment outer token err: %s", err)
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken, reorgAssessmentLeaseTTL)
	if err != nil || !locked {
		return
	}
	defer func() {
		_ = db.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, outerToken)
	}()

	run, job, err := ensureAssessmentRun()
	if errors.Is(err, errAssessmentAlreadyComplete) || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("prepare reorg assessment run err: %s", err)
		return
	}
	token, err := newAssessmentLeaseToken()
	if err != nil {
		log.GetLogger().Errorf("create reorg assessment lease err: %s", err)
		return
	}
	job, err = claimAssessmentLease(run.ID, token)
	if err != nil {
		return
	}
	if err := processAssessmentPage(ctx, run, job, token); err != nil {
		_ = recordAssessmentError(run.ID, token, job.FenceEpoch, "ASSESSMENT_PREPARE_FAILED", err)
		log.GetLogger().Errorf("prepare reorg assessment page run=%d err: %s", run.ID, err)
	}
}

var errAssessmentAlreadyComplete = errors.New("reorg reversibility assessment is complete")

type assessmentPlanPayload struct {
	SchemaVersion      string `json:"schema_version"`
	SourceEffectDigest string `json:"source_effect_digest"`
	Branch             string `json:"branch"`
	ContractVersion    string `json:"contract_version"`
	EventType          string `json:"event_type"`
	ProjectID          uint64 `json:"project_id"`
	Replayability      string `json:"replayability"`
	ActionKind         string `json:"action_kind"`
	BlockerCode        string `json:"blocker_code,omitempty"`
}

type assessmentDispositionInput struct {
	Replayability string
	ActionKind    string
	BlockerCode   string
	SafeDetail    string
}

type preparedAssessmentDisposition struct {
	Effect  entity.ChainEventReorgMaterializationEffect
	Input   assessmentDispositionInput
	Payload []byte
}

type assessmentGlobalBlocker struct {
	Code   string
	Detail string
}

var assessmentRequiredBlockers = []assessmentGlobalBlocker{
	{Code: "BOUNDARY_STATE_SNAPSHOT_REQUIRED", Detail: "an attested generation boundary state snapshot is not installed"},
	{Code: "V1_SOURCE_ORIGIN_UNATTESTED", Detail: "the legacy contract deployment origin is not frozen as audited evidence"},
	{Code: "GLOBAL_CONFIG_BASELINE_REQUIRED", Detail: "payment token, project token, and platform fee boundary configuration is not attested"},
	{Code: "OUTBOX_GENERATION_BINDING_REQUIRED", Detail: "affected durable transactions do not yet have generation-scoped dispositions"},
	{Code: "GENERATION_CHECKPOINT_SCHEMA_REQUIRED", Detail: "the current checkpoint chain is not generation scoped"},
	{Code: "DUAL_CONTROL_AUTHORIZATION_REQUIRED", Detail: "no dedicated digest-bound four-eyes switch authorization exists"},
	{Code: "INDEPENDENT_CHAIN_PREFLIGHT_REQUIRED", Detail: "no unexpired independently observed canonical preflight is bound to this assessment"},
}

func ensureAssessmentRun() (*entity.ChainEventReorgAssessmentRun, *entity.ChainEventReorgAssessmentJob, error) {
	incident, err := activeRecoveryIncident()
	if err != nil {
		return nil, nil, err
	}
	return ensureAssessmentRunForIncident(incident)
}

func ensureAssessmentRunForIncident(incident *entity.ChainEventReorgIncident) (*entity.ChainEventReorgAssessmentRun, *entity.ChainEventReorgAssessmentJob, error) {
	if incident == nil {
		return nil, nil, errors.New("active reorg incident is required")
	}
	materialization := entity.ChainEventReorgMaterializationRun{}
	if err := db.Mysql().Joins("JOIN chain_event_reorg_materialization_job j ON j.run_id=chain_event_reorg_materialization_run.id AND j.state=?", reorgMaterializationStateCompleted).
		Joins("JOIN chain_event_reorg_materialization_manifest m ON m.run_id=chain_event_reorg_materialization_run.id").
		Where("chain_event_reorg_materialization_run.incident_id = ?", incident.ID).Take(&materialization).Error; err != nil {
		return nil, nil, err
	}
	manifest := entity.ChainEventReorgMaterializationManifest{}
	if err := db.Mysql().Where("run_id = ? AND incident_id = ?", materialization.ID, incident.ID).Take(&manifest).Error; err != nil {
		return nil, nil, err
	}
	existing := entity.ChainEventReorgAssessmentRun{}
	err := db.Mysql().Where("materialization_run_id = ?", materialization.ID).Take(&existing).Error
	if err == nil {
		job := entity.ChainEventReorgAssessmentJob{}
		if err := db.Mysql().Where("run_id = ?", existing.ID).Take(&job).Error; err != nil {
			return nil, nil, err
		}
		if job.State != reorgAssessmentStatePreparing {
			return nil, nil, errAssessmentAlreadyComplete
		}
		return &existing, &job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	expected := manifest.OldEffectCount + manifest.ReplacementEffectCount
	if expected > reorgAssessmentMaxEffects {
		return nil, nil, errors.New("reorg assessment effect limit exceeded")
	}
	run := &entity.ChainEventReorgAssessmentRun{
		RunUUID: strings.ToLower(uuid.NewString()), IncidentID: incident.ID, MaterializationRunID: materialization.ID,
		MaterializationManifestDigest: manifest.ManifestDigest, AlgorithmVersion: reorgAssessmentAlgorithm,
		ChainID: materialization.ChainID, BoundaryBlock: materialization.BoundaryBlock, BoundaryBlockHash: materialization.BoundaryBlockHash,
		TargetSafeBlock: materialization.TargetSafeBlock, TargetSafeBlockHash: materialization.TargetSafeBlockHash,
		ExpectedEffectCount:       expected,
		MaterializationEffectRoot: assessmentSourceRoot(manifest), CreatedAt: time.Now(),
	}
	job := &entity.ChainEventReorgAssessmentJob{}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requireAssessmentInputsTx(tx, *incident, materialization, manifest); err != nil {
			return err
		}
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		active := uint8(1)
		*job = entity.ChainEventReorgAssessmentJob{
			RunID: run.ID, IncidentID: incident.ID, State: reorgAssessmentStatePreparing, ActiveSlot: &active,
			NextEffectOrdinal: 1, DispositionDigest: reorgZeroDigest, BlockerDigest: reorgZeroDigest, UpdatedAt: time.Now(),
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return appendAssessmentEventTx(tx, run.ID, "CREATED", "", "created fail-closed reversibility assessment")
	})
	return run, job, err
}

func assessmentSourceRoot(manifest entity.ChainEventReorgMaterializationManifest) string {
	return evidenceDigest("joincare.chain-event.reorg-assessment-source.v1", manifest.RunID, manifest.IncidentID,
		manifest.OldEffectCount, manifest.ReplacementEffectCount, manifest.IgnoredLogCount,
		manifest.OldEffectDigest, manifest.ReplacementEffectDigest, manifest.ManifestDigest)
}

func requireAssessmentInputsTx(tx *gorm.DB, incident entity.ChainEventReorgIncident, run entity.ChainEventReorgMaterializationRun, manifest entity.ChainEventReorgMaterializationManifest) error {
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND active_slot = 1 AND status = ?", incident.ID, reorgStatusDetected).Take(&entity.ChainEventReorgIncident{}).Error; err != nil {
		return err
	}
	state := entity.ChainEventSyncState{}
	if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", run.OldCursorBlock, run.OldCursorBlockHash).Take(&state).Error; err != nil {
		return errors.New("chain event cursor changed before reversibility assessment")
	}
	job := entity.ChainEventReorgMaterializationJob{}
	if err := tx.Where("run_id = ? AND incident_id = ? AND state = ?", run.ID, incident.ID, reorgMaterializationStateCompleted).Take(&job).Error; err != nil {
		return errors.New("completed materialization job is unavailable")
	}
	if manifest.RunID != run.ID || manifest.IncidentID != incident.ID || manifest.ManifestDigest != materializationManifestDigest(run.ID, incident.ID, manifest.RecoveryManifestDigest, manifest.OldEffectCount, manifest.ReplacementEffectCount, manifest.IgnoredLogCount, manifest.OldEffectDigest, manifest.ReplacementEffectDigest) {
		return errors.New("completed materialization manifest identity is invalid")
	}
	var count int64
	if err := tx.Model(&entity.ChainEventReorgMaterializationEffect{}).Where("run_id = ?", run.ID).Count(&count).Error; err != nil {
		return err
	}
	if count < 0 || uint64(count) != manifest.OldEffectCount+manifest.ReplacementEffectCount {
		return errors.New("materialization effect set is incomplete")
	}
	var payloadBytes uint64
	if err := tx.Model(&entity.ChainEventReorgMaterializationEffect{}).Where("run_id = ?", run.ID).
		Select("COALESCE(SUM(OCTET_LENGTH(semantic_payload)),0)").Scan(&payloadBytes).Error; err != nil {
		return err
	}
	if payloadBytes > reorgAssessmentMaxPayloadBytes {
		return errors.New("materialization effect payload limit exceeded")
	}
	return nil
}

func newAssessmentLeaseToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func claimAssessmentLease(runID uint64, token string) (*entity.ChainEventReorgAssessmentJob, error) {
	job := entity.ChainEventReorgAssessmentJob{}
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where("run_id = ? AND state = ? AND (lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP(6))", runID, reorgAssessmentStatePreparing).Updates(map[string]interface{}{
			"lease_token": token, "lease_expires_at": gorm.Expr("DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL 120 SECOND)"),
			"fence_epoch": gorm.Expr("fence_epoch + 1"), "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg assessment lease is held by another worker")
		}
		return tx.Where("run_id = ? AND lease_token = ?", runID, token).Take(&job).Error
	})
	return &job, err
}

func processAssessmentPage(_ context.Context, run *entity.ChainEventReorgAssessmentRun, job *entity.ChainEventReorgAssessmentJob, token string) error {
	if job.BlockerCount == 0 {
		return commitAssessmentPrerequisiteBlockers(run, job, token)
	}
	rows := make([]entity.ChainEventReorgMaterializationEffect, 0, reorgAssessmentPageSize)
	if err := db.Mysql().Where("run_id = ? AND ordinal >= ?", run.MaterializationRunID, job.NextEffectOrdinal).
		Order("ordinal ASC").Limit(reorgAssessmentPageSize).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return finalizeAssessmentRun(run, job, token)
	}
	prepared, err := prepareAssessmentDispositions(rows)
	if err != nil {
		return invalidateAssessmentRun(run.ID, token, job.FenceEpoch, "EFFECT_INVALID", err.Error())
	}
	return commitAssessmentDispositions(run, job, token, prepared)
}

func prepareAssessmentDispositions(effects []entity.ChainEventReorgMaterializationEffect) ([]preparedAssessmentDisposition, error) {
	prepared := make([]preparedAssessmentDisposition, 0, len(effects))
	for _, effect := range effects {
		if effect.EffectDigest == "" || effect.SemanticDigest != sha256Hex(effect.SemanticPayload) {
			return nil, errors.New("materialization effect payload identity changed during assessment")
		}
		input, err := classifyAssessmentEffect(effect)
		if err != nil {
			return nil, err
		}
		if effect.Branch == reorgMaterializationBranchOld {
			input.ActionKind = "reverse_" + input.ActionKind
		} else if effect.Branch == reorgMaterializationBranchReplacement {
			input.ActionKind = "apply_" + input.ActionKind
		} else {
			return nil, errors.New("assessment effect branch is unsupported")
		}
		payload, err := jsonv2.Marshal(assessmentPlanPayload{SchemaVersion: "reorg-assessment-plan-v1", SourceEffectDigest: effect.EffectDigest, Branch: effect.Branch, ContractVersion: effect.ContractVersion, EventType: effect.EventType, ProjectID: effect.ProjectID, Replayability: input.Replayability, ActionKind: input.ActionKind, BlockerCode: input.BlockerCode})
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedAssessmentDisposition{Effect: effect, Input: input, Payload: payload})
	}
	return prepared, nil
}

func commitAssessmentPrerequisiteBlockers(run *entity.ChainEventReorgAssessmentRun, job *entity.ChainEventReorgAssessmentJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockAssessmentJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if locked.BlockerCount != 0 {
			return errors.New("assessment prerequisite blockers were already committed")
		}
		if err := requireActiveMaterializationIncidentTx(tx, run.IncidentID); err != nil {
			return err
		}
		previous := locked.BlockerDigest
		for index, item := range assessmentRequiredBlockers {
			ordinal := uint64(index + 1)
			row := entity.ChainEventReorgAssessmentBlocker{RunID: run.ID, Ordinal: ordinal, Scope: "run", BlockerCode: item.Code, SafeDetail: item.Detail, PreviousRowDigest: previous, CreatedAt: time.Now()}
			row.RowDigest = assessmentBlockerDigest(row)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			previous = row.RowDigest
		}
		result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where(activeAssessmentLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{
			"blocker_count": uint64(len(assessmentRequiredBlockers)), "blocker_digest": previous,
			"lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg assessment lease lost before prerequisite checkpoint")
		}
		return nil
	})
}

func commitAssessmentDispositions(run *entity.ChainEventReorgAssessmentRun, job *entity.ChainEventReorgAssessmentJob, token string, prepared []preparedAssessmentDisposition) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockAssessmentJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if err := requireActiveMaterializationIncidentTx(tx, run.IncidentID); err != nil {
			return err
		}
		dispositionPrevious, blockerPrevious := locked.DispositionDigest, locked.BlockerDigest
		nextOrdinal := locked.NextEffectOrdinal
		exact, conditional, blocked, noop, blockers := locked.ExactCount, locked.ConditionalCount, locked.BlockedCount, locked.NoopCount, locked.BlockerCount
		for _, item := range prepared {
			effect, input, payload := item.Effect, item.Input, item.Payload
			if effect.Ordinal != nextOrdinal || effect.RunID != run.MaterializationRunID || effect.EffectDigest == "" || effect.SemanticDigest != sha256Hex(effect.SemanticPayload) {
				return errors.New("materialization effect identity changed during assessment")
			}
			disposition := entity.ChainEventReorgAssessmentDisposition{RunID: run.ID, Ordinal: nextOrdinal, MaterializationEffectID: effect.ID, SourceEffectDigest: effect.EffectDigest, Branch: effect.Branch, ContractVersion: effect.ContractVersion, EventType: effect.EventType, ProjectID: effect.ProjectID, Replayability: input.Replayability, ActionKind: input.ActionKind, BlockerCode: input.BlockerCode, PlanPayload: payload, PlanPayloadDigest: sha256Hex(payload), PreviousRowDigest: dispositionPrevious, CreatedAt: time.Now()}
			disposition.RowDigest = assessmentDispositionDigest(disposition)
			if err := tx.Create(&disposition).Error; err != nil {
				return err
			}
			dispositionPrevious = disposition.RowDigest
			switch input.Replayability {
			case reorgAssessmentReplayExact:
				exact++
			case reorgAssessmentReplayConditional:
				conditional++
			case reorgAssessmentReplayBlocked:
				blocked++
			case reorgAssessmentReplayNoop:
				noop++
			default:
				return errors.New("assessment replayability is unsupported")
			}
			if input.BlockerCode != "" {
				blockers++
				dispositionID := disposition.ID
				blocker := entity.ChainEventReorgAssessmentBlocker{RunID: run.ID, Ordinal: blockers, DispositionID: &dispositionID, Scope: "effect", BlockerCode: input.BlockerCode, SafeDetail: input.SafeDetail, PreviousRowDigest: blockerPrevious, CreatedAt: time.Now()}
				blocker.RowDigest = assessmentBlockerDigest(blocker)
				if err := tx.Create(&blocker).Error; err != nil {
					return err
				}
				blockerPrevious = blocker.RowDigest
			}
			nextOrdinal++
		}
		if blockers > reorgAssessmentMaxBlockers {
			return errors.New("reorg assessment blocker limit exceeded")
		}
		result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where(activeAssessmentLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{
			"next_effect_ordinal": nextOrdinal, "disposition_count": nextOrdinal - 1,
			"exact_count": exact, "conditional_count": conditional, "blocked_count": blocked, "noop_count": noop,
			"blocker_count": blockers, "disposition_digest": dispositionPrevious, "blocker_digest": blockerPrevious,
			"lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg assessment lease lost before page checkpoint")
		}
		return nil
	})
}

func invalidateAssessmentRun(runID uint64, token string, epoch uint64, code, message string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		return invalidateAssessmentRunTx(tx, runID, token, epoch, code, message)
	})
}

func classifyAssessmentEffect(effect entity.ChainEventReorgMaterializationEffect) (assessmentDispositionInput, error) {
	if effect.Classification == reorgMaterializationEffectIgnored {
		if effect.EventType != "ignored" || effect.ProjectID != 0 || effect.Branch != reorgMaterializationBranchReplacement {
			return assessmentDispositionInput{}, errors.New("ignored materialization effect identity is invalid")
		}
		return assessmentDispositionInput{Replayability: reorgAssessmentReplayNoop, ActionKind: "approved_abi_noop"}, nil
	}
	if effect.Classification != reorgMaterializationEffectSemantic {
		return assessmentDispositionInput{}, errors.New("unknown materialization effect classification")
	}
	if effect.ProjectID > 9223372036854775807 {
		return assessmentDispositionInput{}, errors.New("assessment project identity exceeds signed database range")
	}
	if err := validateAssessmentSemanticPayload(effect); err != nil {
		return assessmentDispositionInput{}, err
	}
	if effect.ContractVersion == constant.ContractVersionV1Legacy {
		switch effect.EventType {
		case commonChain.Purchase:
			return blockedAssessment("v1_participant_delta", "V1_UID_TIME_NOTICE_UNRECOVERABLE", "legacy purchase UID, timestamps, and notification identity are not event-derived"), nil
		case commonChain.Claimed:
			return blockedAssessment("v1_claim_delta", "V1_CLAIM_DERIVED_STATE_UNRECOVERABLE", "legacy claim timestamps, vesting-relative next claim, row identity, and notices are not reconstructible"), nil
		case commonChain.Refunded:
			return blockedAssessment("v1_refund_delta", "V1_REFUND_ROW_BINDING_UNPROVEN", "legacy refund rows lack log-index identity and notification delivery cannot be reconstructed"), nil
		case commonChain.ProjectEnded, commonChain.ProjectCancelled:
			return blockedAssessment("v1_lifecycle_transition", "V1_LIFECYCLE_BEFORE_IMAGE_MISSING", "legacy lifecycle prior status and withdrawal flag are not present in the semantic payload"), nil
		case commonChain.PlatformFeesUpdated:
			return blockedAssessment("v1_global_config_transition", "GLOBAL_CONFIG_BASELINE_REQUIRED", "legacy platform fee configuration has no authoritative boundary baseline"), nil
		case commonChain.ProjectCreated, commonChain.ProjectTokensDeposited, commonChain.ProjectActivated, commonChain.RaisedFundsWithdrawn, commonChain.ProjectTokensWithdrawn:
			return blockedAssessment("v1_projection_delta", "V1_PROJECT_ID_RAW_UNATTESTED", "legacy old-branch project identity is not bound to preserved uint256 topic evidence"), nil
		default:
			return assessmentDispositionInput{}, fmt.Errorf("unsupported legacy assessment event %q", effect.EventType)
		}
	}
	if effect.ContractVersion != constant.ContractVersionV2Stablecoin {
		return assessmentDispositionInput{}, errors.New("assessment contract version is unsupported")
	}
	switch effect.EventType {
	case commonChain.V2ProjectTokensDeposited:
		return exactAssessment("v2_token_deposit_ledger_and_total_deposited_delta"), nil
	case commonChain.V2Purchased:
		return conditionalAssessment("v2_purchase_payment_entitlement_and_participant_delta", "BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V2 payment and entitlement amounts are exact, but participant count depends on prior project-wallet purchase state"), nil
	case commonChain.V2Refunded:
		return exactAssessment("v2_payment_refund_fact"), nil
	case commonChain.V2Claimed:
		return exactAssessment("v2_project_token_claim_fact"), nil
	case commonChain.V2ProjectProceedsWithdrawn:
		return conditionalAssessment("v2_project_proceeds_and_fund_withdrawn_flag", "BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V2 proceeds amounts are exact, but reversing the project fund-withdrawn flag requires its boundary before-image"), nil
	case commonChain.V2PlatformFeeWithdrawn:
		return conditionalAssessment("v2_platform_fee_fact_and_platform_fee_withdrawn_flag", "BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V2 platform fee amount is exact, but reversing the platform-fee-withdrawn flag requires its boundary before-image"), nil
	case commonChain.V2ProjectTokensWithdrawn:
		return conditionalAssessment("v2_token_withdrawal_fact_and_token_withdrawn_flag", "BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V2 project-token withdrawal amount is exact, but reversing the token-withdrawn flag requires its boundary before-image"), nil
	case commonChain.V2ProjectCreated, commonChain.V2ProjectActivated, commonChain.V2ProjectEnded, commonChain.V2ProjectCancelled:
		return conditionalAssessment("v2_lifecycle_transition", "BOUNDARY_STATE_SNAPSHOT_REQUIRED", "V2 lifecycle after-image is deterministic but requires an exact boundary project state and outbox binding"), nil
	case reorgV2PaymentTokenConfigured, reorgV2ProjectTokenConfigured, reorgV2PlatformFeesUpdated:
		return conditionalAssessment("v2_global_config_transition", "GLOBAL_CONFIG_BASELINE_REQUIRED", "V2 configuration event is exact but the boundary configuration is not attested"), nil
	default:
		return assessmentDispositionInput{}, fmt.Errorf("unsupported V2 assessment event %q", effect.EventType)
	}
}

func validateAssessmentSemanticPayload(effect entity.ChainEventReorgMaterializationEffect) error {
	if effect.ContractVersion == constant.ContractVersionV1Legacy {
		var event commonChain.Data
		if err := jsonv2.Unmarshal(effect.SemanticPayload, &event); err != nil {
			return fmt.Errorf("decode V1 assessment payload: %w", err)
		}
		if effect.EventType == commonChain.PlatformFeesUpdated {
			if effect.ProjectID != 0 || event.ProjectID != 0 {
				return errors.New("V1 global assessment project identity is invalid")
			}
			return nil
		}
		if event.ProjectID <= 0 || uint64(event.ProjectID) != effect.ProjectID {
			return errors.New("V1 assessment project identity is invalid")
		}
		return nil
	}
	if effect.ContractVersion != constant.ContractVersionV2Stablecoin {
		return errors.New("assessment semantic contract version is unsupported")
	}
	if effect.ProjectID == 0 {
		var event materializationV2GlobalEvent
		if err := jsonv2.Unmarshal(effect.SemanticPayload, &event); err != nil {
			return fmt.Errorf("decode V2 global assessment payload: %w", err)
		}
		if event.Event != effect.EventType || event.ContractAddress.Hex() != effect.ContractAddress || event.BlockNumber != effect.BlockNumber || event.BlockHash != effect.BlockHash || event.TxHash != effect.TxHash || event.LogIndex != effect.LogIndex {
			return errors.New("V2 global assessment payload identity mismatch")
		}
		switch event.Event {
		case reorgV2PaymentTokenConfigured, reorgV2ProjectTokenConfigured:
			if event.Token == nil || *event.Token == (common.Address{}) || event.Allowed == nil || event.Decimals == nil || *event.Decimals == 0 || *event.Decimals > 18 || event.FeeRate != nil || event.FeeReceiver != nil {
				return errors.New("V2 token configuration assessment payload is invalid")
			}
		case reorgV2PlatformFeesUpdated:
			if event.FeeRate == nil || *event.FeeRate > 1000 || event.FeeReceiver == nil || *event.FeeReceiver == (common.Address{}) || event.Token != nil || event.Allowed != nil || event.Decimals != nil {
				return errors.New("V2 platform fee assessment payload is invalid")
			}
		default:
			return errors.New("V2 global assessment event is unsupported")
		}
		return nil
	}
	var event commonChain.V2Event
	if err := jsonv2.Unmarshal(effect.SemanticPayload, &event); err != nil {
		return fmt.Errorf("decode V2 assessment payload: %w", err)
	}
	if event.Event != effect.EventType || event.ProjectID != effect.ProjectID || event.ContractAddress.Hex() != effect.ContractAddress || event.BlockNumber != effect.BlockNumber || event.BlockHash != effect.BlockHash || event.TxHash != effect.TxHash || event.LogIndex != effect.LogIndex {
		return errors.New("V2 assessment payload identity mismatch")
	}
	for _, raw := range assessmentV2RawValues(event) {
		if raw == "" {
			continue
		}
		canonical, err := money.NormalizeRawUint256(raw)
		if err != nil || canonical != raw {
			return errors.New("V2 assessment raw amount is not canonical uint256")
		}
	}
	return validateAssessmentV2EventFields(event)
}

func assessmentV2RawValues(event commonChain.V2Event) []string {
	return []string{event.TokenPriceRaw, event.HardCapRaw, event.SoftCapRaw, event.MaxPerUserRaw, event.StartTimeRaw, event.EndTimeRaw, event.PaymentRaw, event.ProjectTokenRaw, event.TotalRaisedRaw, event.PlatformFeeRaw}
}

func validateAssessmentV2EventFields(event commonChain.V2Event) error {
	requireAddress := func(name string, value common.Address) error {
		if value == (common.Address{}) {
			return fmt.Errorf("V2 assessment %s is zero", name)
		}
		return nil
	}
	requireRaw := func(name, value string) error {
		if value == "" {
			return fmt.Errorf("V2 assessment %s is missing", name)
		}
		return nil
	}
	switch event.Event {
	case commonChain.V2ProjectCreated:
		for name, address := range map[string]common.Address{"projectToken": event.ProjectToken, "paymentToken": event.PaymentToken, "projectOwner": event.ProjectOwner, "feeReceiver": event.FeeReceiver} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		if event.ProjectDecimals == 0 || event.ProjectDecimals > 18 || event.PaymentDecimals == 0 || event.PaymentDecimals > 18 || event.FeeRate > 1000 {
			return errors.New("V2 assessment project creation configuration is invalid")
		}
		for name, raw := range map[string]string{"tokenPriceRaw": event.TokenPriceRaw, "hardCapRaw": event.HardCapRaw, "softCapRaw": event.SoftCapRaw, "maxPerUserRaw": event.MaxPerUserRaw, "startTimeRaw": event.StartTimeRaw, "endTimeRaw": event.EndTimeRaw} {
			if err := requireRaw(name, raw); err != nil {
				return err
			}
		}
	case commonChain.V2ProjectTokensDeposited:
		if err := requireAddress("projectToken", event.ProjectToken); err != nil {
			return err
		}
		return requireRaw("projectTokenAmountRaw", event.ProjectTokenRaw)
	case commonChain.V2ProjectActivated:
		return nil
	case commonChain.V2Purchased:
		for name, address := range map[string]common.Address{"user": event.User, "paymentToken": event.PaymentToken, "projectToken": event.ProjectToken} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		if err := requireRaw("paymentAmountRaw", event.PaymentRaw); err != nil {
			return err
		}
		return requireRaw("projectTokenAmountRaw", event.ProjectTokenRaw)
	case commonChain.V2ProjectEnded:
		if event.Status != 2 && event.Status != 3 {
			return errors.New("V2 assessment terminal status is invalid")
		}
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
		return requireRaw("totalRaisedRaw", event.TotalRaisedRaw)
	case commonChain.V2ProjectCancelled:
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
		return requireRaw("totalRaisedRaw", event.TotalRaisedRaw)
	case commonChain.V2Refunded:
		if err := requireAddress("user", event.User); err != nil {
			return err
		}
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
		return requireRaw("paymentAmountRaw", event.PaymentRaw)
	case commonChain.V2Claimed:
		if err := requireAddress("user", event.User); err != nil {
			return err
		}
		if err := requireAddress("projectToken", event.ProjectToken); err != nil {
			return err
		}
		return requireRaw("projectTokenAmountRaw", event.ProjectTokenRaw)
	case commonChain.V2ProjectProceedsWithdrawn:
		for name, address := range map[string]common.Address{"paymentToken": event.PaymentToken, "projectOwner": event.ProjectOwner} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		if err := requireRaw("paymentAmountRaw", event.PaymentRaw); err != nil {
			return err
		}
		return requireRaw("platformFeeRaw", event.PlatformFeeRaw)
	case commonChain.V2PlatformFeeWithdrawn:
		for name, address := range map[string]common.Address{"paymentToken": event.PaymentToken, "feeReceiver": event.FeeReceiver} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		return requireRaw("paymentAmountRaw", event.PaymentRaw)
	case commonChain.V2ProjectTokensWithdrawn:
		for name, address := range map[string]common.Address{"projectToken": event.ProjectToken, "projectOwner": event.ProjectOwner} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		return requireRaw("projectTokenAmountRaw", event.ProjectTokenRaw)
	default:
		return fmt.Errorf("V2 assessment event %q is unsupported", event.Event)
	}
	return nil
}

func exactAssessment(action string) assessmentDispositionInput {
	return assessmentDispositionInput{Replayability: reorgAssessmentReplayExact, ActionKind: action}
}

func conditionalAssessment(action, code, detail string) assessmentDispositionInput {
	return assessmentDispositionInput{Replayability: reorgAssessmentReplayConditional, ActionKind: action, BlockerCode: code, SafeDetail: detail}
}

func blockedAssessment(action, code, detail string) assessmentDispositionInput {
	return assessmentDispositionInput{Replayability: reorgAssessmentReplayBlocked, ActionKind: action, BlockerCode: code, SafeDetail: detail}
}

func finalizeAssessmentRun(run *entity.ChainEventReorgAssessmentRun, job *entity.ChainEventReorgAssessmentJob, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		locked, err := lockAssessmentJobTx(tx, run.ID, token, job.FenceEpoch)
		if err != nil {
			return err
		}
		if err := verifyAssessmentSourcesTx(tx, run, locked); err != nil {
			return invalidateAssessmentRunTx(tx, run.ID, token, job.FenceEpoch, "SOURCE_DRIFT", err.Error())
		}
		if locked.BlockerCount < reorgAssessmentGlobalBlockerCount {
			return errors.New("reorg assessment mandatory blockers are incomplete")
		}
		manifest := entity.ChainEventReorgAssessmentManifest{RunID: run.ID, IncidentID: run.IncidentID, MaterializationManifestDigest: run.MaterializationManifestDigest, Outcome: reorgAssessmentOutcomeBlocked, DispositionCount: locked.DispositionCount, ExactCount: locked.ExactCount, ConditionalCount: locked.ConditionalCount, BlockedCount: locked.BlockedCount, NoopCount: locked.NoopCount, BlockerCount: locked.BlockerCount, DispositionDigest: locked.DispositionDigest, BlockerDigest: locked.BlockerDigest, CompletedAt: time.Now()}
		manifest.ManifestDigest = assessmentManifestDigest(manifest)
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		if err := appendAssessmentEventTx(tx, run.ID, "COMPLETED", "", "completed fail-closed reversibility assessment"); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where(activeAssessmentLeaseWhere, run.ID, token, job.FenceEpoch).Updates(map[string]interface{}{"state": reorgAssessmentStateCompleted, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("reorg assessment lease lost before finalization")
		}
		return nil
	})
}

func verifyAssessmentSourcesTx(tx *gorm.DB, run *entity.ChainEventReorgAssessmentRun, job *entity.ChainEventReorgAssessmentJob) error {
	if err := requireActiveMaterializationIncidentTx(tx, run.IncidentID); err != nil {
		return err
	}
	materialization := entity.ChainEventReorgMaterializationRun{}
	if err := tx.Where("id = ? AND incident_id = ?", run.MaterializationRunID, run.IncidentID).Take(&materialization).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id = ? AND incident_id = ? AND state = ?", run.MaterializationRunID, run.IncidentID, reorgMaterializationStateCompleted).Take(&entity.ChainEventReorgMaterializationJob{}).Error; err != nil {
		return errors.New("completed materialization job changed during assessment")
	}
	if err := tx.Where("id = 1 AND cursor_block = ? AND cursor_block_hash = ?", materialization.OldCursorBlock, materialization.OldCursorBlockHash).Take(&entity.ChainEventSyncState{}).Error; err != nil {
		return errors.New("chain event cursor changed during assessment")
	}
	manifest := entity.ChainEventReorgMaterializationManifest{}
	if err := tx.Where("run_id = ? AND incident_id = ?", run.MaterializationRunID, run.IncidentID).Take(&manifest).Error; err != nil {
		return err
	}
	if manifest.ManifestDigest != run.MaterializationManifestDigest || manifest.ManifestDigest != materializationManifestDigest(materialization.ID, run.IncidentID, manifest.RecoveryManifestDigest, manifest.OldEffectCount, manifest.ReplacementEffectCount, manifest.IgnoredLogCount, manifest.OldEffectDigest, manifest.ReplacementEffectDigest) || assessmentSourceRoot(manifest) != run.MaterializationEffectRoot || job.DispositionCount != run.ExpectedEffectCount || job.NextEffectOrdinal != run.ExpectedEffectCount+1 {
		return errors.New("assessment materialization source changed or is incomplete")
	}
	var dispositionCount int64
	if err := tx.Model(&entity.ChainEventReorgAssessmentDisposition{}).Where("run_id = ?", run.ID).Count(&dispositionCount).Error; err != nil {
		return err
	}
	if uint64(dispositionCount) != run.ExpectedEffectCount || job.ExactCount+job.ConditionalCount+job.BlockedCount+job.NoopCount != job.DispositionCount {
		return errors.New("assessment disposition set is incomplete")
	}
	var blockerCount int64
	if err := tx.Model(&entity.ChainEventReorgAssessmentBlocker{}).Where("run_id = ?", run.ID).Count(&blockerCount).Error; err != nil {
		return err
	}
	if uint64(blockerCount) != job.BlockerCount || job.BlockerCount < reorgAssessmentGlobalBlockerCount {
		return errors.New("assessment blocker set is incomplete")
	}
	return nil
}

const activeAssessmentLeaseWhere = "run_id = ? AND lease_token = ? AND fence_epoch = ? AND state = 'preparing' AND lease_expires_at > CURRENT_TIMESTAMP(6)"

func lockAssessmentJobTx(tx *gorm.DB, runID uint64, token string, epoch uint64) (*entity.ChainEventReorgAssessmentJob, error) {
	job := entity.ChainEventReorgAssessmentJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activeAssessmentLeaseWhere, runID, token, epoch).Take(&job).Error
	return &job, err
}

func invalidateAssessmentRunTx(tx *gorm.DB, runID uint64, token string, epoch uint64, code, message string) error {
	if _, err := lockAssessmentJobTx(tx, runID, token, epoch); err != nil {
		return err
	}
	if err := appendAssessmentEventTx(tx, runID, "INVALIDATED", code, truncateFailureError(message, 500)); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where(activeAssessmentLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{"state": reorgAssessmentStateInvalidated, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "last_error_code": code, "last_error_message": truncateFailureError(message, 500), "completed_at": now, "updated_at": now})
	if result.Error != nil || result.RowsAffected != 1 {
		return errors.New("reorg assessment lease lost before invalidation")
	}
	return nil
}

func recordAssessmentError(runID uint64, token string, epoch uint64, code string, processErr error) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventReorgAssessmentJob{}).Where(activeAssessmentLeaseWhere, runID, token, epoch).Updates(map[string]interface{}{"last_error_code": code, "last_error_message": truncateFailureError(processErr.Error(), 500), "lease_token": "", "lease_expires_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(6)")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("reorg assessment lease lost before error checkpoint")
		}
		return nil
	})
}

func appendAssessmentEventTx(tx *gorm.DB, runID uint64, eventType, code, message string) error {
	previous := entity.ChainEventReorgAssessmentEvent{}
	previousHash, sequence := reorgZeroDigest, uint(1)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ?", runID).Order("sequence_no DESC").Take(&previous).Error
	if err == nil {
		previousHash, sequence = previous.EventHash, previous.SequenceNo+1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := entity.ChainEventReorgAssessmentEvent{RunID: runID, SequenceNo: sequence, EventType: eventType, ErrorCode: code, Message: message, PreviousEventHash: previousHash, CreatedAt: time.Now()}
	event.EventHash = assessmentEventDigest(event)
	return tx.Create(&event).Error
}

func assessmentDispositionDigest(row entity.ChainEventReorgAssessmentDisposition) string {
	return evidenceDigest("joincare.chain-event.reorg-assessment-disposition.v1", row.RunID, row.Ordinal, row.MaterializationEffectID, row.SourceEffectDigest, row.Branch, row.ContractVersion, row.EventType, row.ProjectID, row.Replayability, row.ActionKind, row.BlockerCode, row.PlanPayloadDigest, row.PreviousRowDigest)
}

func assessmentBlockerDigest(row entity.ChainEventReorgAssessmentBlocker) string {
	dispositionID := uint64(0)
	if row.DispositionID != nil {
		dispositionID = *row.DispositionID
	}
	return evidenceDigest("joincare.chain-event.reorg-assessment-blocker.v1", row.RunID, row.Ordinal, dispositionID, row.Scope, row.BlockerCode, row.SafeDetail, row.PreviousRowDigest)
}

func assessmentEventDigest(row entity.ChainEventReorgAssessmentEvent) string {
	return evidenceDigest("joincare.chain-event.reorg-assessment-event.v1", row.RunID, uint64(row.SequenceNo), row.EventType, row.ErrorCode, row.Message, row.PreviousEventHash)
}

func assessmentManifestDigest(row entity.ChainEventReorgAssessmentManifest) string {
	return evidenceDigest("joincare.chain-event.reorg-assessment-manifest.v1", row.RunID, row.IncidentID, row.MaterializationManifestDigest, row.Outcome, row.DispositionCount, row.ExactCount, row.ConditionalCount, row.BlockedCount, row.NoopCount, row.BlockerCount, row.DispositionDigest, row.BlockerDigest)
}
