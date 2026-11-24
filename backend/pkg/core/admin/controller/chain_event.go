package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

const chainEventFailureSafeSelect = "id,event,block_number,block_hash,tx_hash,log_index,contract_address,contract_version,error_message,status,retry_count,consecutive_count,replay_count,first_failed_at,last_failed_at,replay_requested_at,resolved_at,created_at,updated_at,CASE WHEN status = 'dead_letter' AND event <> 'reconciliation' THEN TRUE ELSE FALSE END AS admin_replayable"
const chainEventReorgSafeSelect = "id,incident_key,chain_id,contract_address,contract_version,cursor_block,expected_block_hash,observed_block_hash,cursor_checkpoint_id,v2_start_block,status,detection_code,detected_at,resolved_at,created_at,updated_at"
const chainEventReorgRecoverySafeSelect = "r.id,r.run_uuid,r.incident_id,r.attempt_no,r.chain_id,r.contract_address,r.legacy_contract_address,r.contract_version,r.algorithm_version,r.boundary_kind,r.boundary_checkpoint_id,r.boundary_block,r.boundary_block_hash,r.target_safe_block,r.target_safe_block_hash,r.confirmation_depth,r.expected_batch_count,r.expected_scanned_blocks,r.requested_at,j.state,j.next_block,j.next_batch_ordinal,j.next_log_ordinal,j.observed_log_count,j.batch_evidence_digest,j.log_evidence_digest,j.attempt_count,j.last_error_code,j.completed_at,j.updated_at"
const chainEventReorgMaterializationSafeSelect = "r.id,r.run_uuid,r.recovery_run_id,r.incident_id,r.algorithm_version,r.recovery_manifest_digest,r.chain_id,r.contract_address,r.legacy_contract_address,r.boundary_block,r.boundary_block_hash,r.old_cursor_block,r.old_cursor_block_hash,r.target_safe_block,r.target_safe_block_hash,r.expected_old_record_count,r.expected_replacement_log_count,r.created_at,j.state,j.next_branch,j.old_effect_count,j.replacement_effect_count,j.ignored_log_count,j.old_effect_digest,j.replacement_effect_digest,j.attempt_count,j.last_error_code,j.completed_at,j.updated_at"
const chainEventReorgAssessmentSafeSelect = "r.id,r.run_uuid,r.incident_id,r.materialization_run_id,r.materialization_manifest_digest,r.algorithm_version,r.chain_id,r.boundary_block,r.boundary_block_hash,r.target_safe_block,r.target_safe_block_hash,r.expected_effect_count,r.created_at,j.state,j.next_effect_ordinal,j.disposition_count,j.exact_count,j.conditional_count,j.blocked_count,j.noop_count,j.blocker_count,j.disposition_digest,j.blocker_digest,j.attempt_count,j.last_error_code,j.completed_at,j.updated_at"
const chainEventReorgPrerequisiteSafeSelect = "r.id,r.run_uuid,r.incident_id,r.assessment_run_id,r.assessment_manifest_digest,r.algorithm_version,r.chain_id,r.boundary_block,r.boundary_block_hash,r.expected_requirement_count,r.created_at,j.state,j.next_requirement_ordinal,j.requirement_count,j.unmet_count,j.untrusted_count,j.deferred_count,j.artifact_count,j.requirement_digest,j.artifact_digest,j.attempt_count,j.last_error_code,j.completed_at,j.updated_at"
const chainEventReorgGenerationSafeSelect = "r.id,r.generation_uuid,r.incident_id,r.attempt_no,r.generation_kind,r.prerequisite_run_id,r.prerequisite_manifest_digest,r.assessment_run_id,r.assessment_manifest_digest,r.materialization_run_id,r.materialization_manifest_digest,r.chain_id,r.boundary_block,r.boundary_block_hash,r.target_safe_block,r.target_safe_block_hash,r.legacy_contract_address,r.v2_contract_address,r.old_effect_root,r.replacement_effect_root,r.algorithm_version,r.schema_version,r.descriptor_digest,r.created_at,j.state,j.stage,j.sender_count,j.submission_count,j.submission_payload_bytes,j.nonce_count,j.requirement_count,j.unmet_count,j.verified_count,j.sender_root,j.submission_root,j.nonce_root,j.requirement_root,j.attempt_count,j.last_error_code,j.completed_at,j.updated_at"

type chainEventReorgRecoverySummary struct {
	ID                    uint64     `gorm:"column:id" json:"id"`
	RunUUID               string     `gorm:"column:run_uuid" json:"run_uuid"`
	IncidentID            uint64     `gorm:"column:incident_id" json:"incident_id"`
	AttemptNo             uint       `gorm:"column:attempt_no" json:"attempt_no"`
	ChainID               uint64     `gorm:"column:chain_id" json:"chain_id"`
	ContractAddress       string     `gorm:"column:contract_address" json:"contract_address"`
	LegacyContractAddress string     `gorm:"column:legacy_contract_address" json:"legacy_contract_address"`
	ContractVersion       string     `gorm:"column:contract_version" json:"contract_version"`
	AlgorithmVersion      string     `gorm:"column:algorithm_version" json:"algorithm_version"`
	BoundaryKind          string     `gorm:"column:boundary_kind" json:"boundary_kind"`
	BoundaryCheckpointID  *uint64    `gorm:"column:boundary_checkpoint_id" json:"boundary_checkpoint_id,omitempty"`
	BoundaryBlock         uint64     `gorm:"column:boundary_block" json:"boundary_block"`
	BoundaryBlockHash     string     `gorm:"column:boundary_block_hash" json:"boundary_block_hash"`
	TargetSafeBlock       uint64     `gorm:"column:target_safe_block" json:"target_safe_block"`
	TargetSafeBlockHash   string     `gorm:"column:target_safe_block_hash" json:"target_safe_block_hash"`
	ConfirmationDepth     uint64     `gorm:"column:confirmation_depth" json:"confirmation_depth"`
	ExpectedBatchCount    uint       `gorm:"column:expected_batch_count" json:"expected_batch_count"`
	ExpectedScannedBlocks uint64     `gorm:"column:expected_scanned_blocks" json:"expected_scanned_blocks"`
	RequestedAt           time.Time  `gorm:"column:requested_at" json:"requested_at"`
	State                 string     `gorm:"column:state" json:"state"`
	NextBlock             uint64     `gorm:"column:next_block" json:"next_block"`
	NextBatchOrdinal      uint       `gorm:"column:next_batch_ordinal" json:"next_batch_ordinal"`
	NextLogOrdinal        uint64     `gorm:"column:next_log_ordinal" json:"next_log_ordinal"`
	ObservedLogCount      uint64     `gorm:"column:observed_log_count" json:"observed_log_count"`
	BatchEvidenceDigest   string     `gorm:"column:batch_evidence_digest" json:"batch_evidence_digest"`
	LogEvidenceDigest     string     `gorm:"column:log_evidence_digest" json:"log_evidence_digest"`
	WorkerAttemptCount    uint       `gorm:"column:attempt_count" json:"worker_attempt_count"`
	LastErrorCode         string     `gorm:"column:last_error_code" json:"last_error_code"`
	CompletedAt           *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt             time.Time  `gorm:"column:updated_at" json:"updated_at"`
	ManifestDigest        *string    `gorm:"column:manifest_digest" json:"manifest_digest,omitempty"`
	CompletedBatchCount   uint       `gorm:"column:completed_batch_count" json:"completed_batch_count"`
}

type chainEventReorgMaterializationSummary struct {
	ID                       uint64     `gorm:"column:id" json:"id"`
	RunUUID                  string     `gorm:"column:run_uuid" json:"run_uuid"`
	RecoveryRunID            uint64     `gorm:"column:recovery_run_id" json:"recovery_run_id"`
	IncidentID               uint64     `gorm:"column:incident_id" json:"incident_id"`
	AlgorithmVersion         string     `gorm:"column:algorithm_version" json:"algorithm_version"`
	RecoveryManifestDigest   string     `gorm:"column:recovery_manifest_digest" json:"recovery_manifest_digest"`
	ChainID                  uint64     `gorm:"column:chain_id" json:"chain_id"`
	ContractAddress          string     `gorm:"column:contract_address" json:"contract_address"`
	LegacyContractAddress    string     `gorm:"column:legacy_contract_address" json:"legacy_contract_address"`
	BoundaryBlock            uint64     `gorm:"column:boundary_block" json:"boundary_block"`
	BoundaryBlockHash        string     `gorm:"column:boundary_block_hash" json:"boundary_block_hash"`
	OldCursorBlock           uint64     `gorm:"column:old_cursor_block" json:"old_cursor_block"`
	OldCursorBlockHash       string     `gorm:"column:old_cursor_block_hash" json:"old_cursor_block_hash"`
	TargetSafeBlock          uint64     `gorm:"column:target_safe_block" json:"target_safe_block"`
	TargetSafeBlockHash      string     `gorm:"column:target_safe_block_hash" json:"target_safe_block_hash"`
	ExpectedOldRecordCount   uint64     `gorm:"column:expected_old_record_count" json:"expected_old_record_count"`
	ExpectedReplacementCount uint64     `gorm:"column:expected_replacement_log_count" json:"expected_replacement_log_count"`
	CreatedAt                time.Time  `gorm:"column:created_at" json:"created_at"`
	State                    string     `gorm:"column:state" json:"state"`
	NextBranch               string     `gorm:"column:next_branch" json:"next_branch"`
	OldEffectCount           uint64     `gorm:"column:old_effect_count" json:"old_effect_count"`
	ReplacementEffectCount   uint64     `gorm:"column:replacement_effect_count" json:"replacement_effect_count"`
	IgnoredLogCount          uint64     `gorm:"column:ignored_log_count" json:"ignored_log_count"`
	OldEffectDigest          string     `gorm:"column:old_effect_digest" json:"old_effect_digest"`
	ReplacementEffectDigest  string     `gorm:"column:replacement_effect_digest" json:"replacement_effect_digest"`
	WorkerAttemptCount       uint       `gorm:"column:attempt_count" json:"worker_attempt_count"`
	LastErrorCode            string     `gorm:"column:last_error_code" json:"last_error_code"`
	CompletedAt              *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	UpdatedAt                time.Time  `gorm:"column:updated_at" json:"updated_at"`
	ManifestDigest           *string    `gorm:"column:manifest_digest" json:"manifest_digest,omitempty"`
}

type chainEventReorgAssessmentSummary struct {
	ID                            uint64                                    `gorm:"column:id" json:"id"`
	RunUUID                       string                                    `gorm:"column:run_uuid" json:"run_uuid"`
	IncidentID                    uint64                                    `gorm:"column:incident_id" json:"incident_id"`
	MaterializationRunID          uint64                                    `gorm:"column:materialization_run_id" json:"materialization_run_id"`
	MaterializationManifestDigest string                                    `gorm:"column:materialization_manifest_digest" json:"materialization_manifest_digest"`
	AlgorithmVersion              string                                    `gorm:"column:algorithm_version" json:"algorithm_version"`
	ChainID                       uint64                                    `gorm:"column:chain_id" json:"chain_id"`
	BoundaryBlock                 uint64                                    `gorm:"column:boundary_block" json:"boundary_block"`
	BoundaryBlockHash             string                                    `gorm:"column:boundary_block_hash" json:"boundary_block_hash"`
	TargetSafeBlock               uint64                                    `gorm:"column:target_safe_block" json:"target_safe_block"`
	TargetSafeBlockHash           string                                    `gorm:"column:target_safe_block_hash" json:"target_safe_block_hash"`
	ExpectedEffectCount           uint64                                    `gorm:"column:expected_effect_count" json:"expected_effect_count"`
	State                         string                                    `gorm:"column:state" json:"state"`
	NextEffectOrdinal             uint64                                    `gorm:"column:next_effect_ordinal" json:"next_effect_ordinal"`
	DispositionCount              uint64                                    `gorm:"column:disposition_count" json:"disposition_count"`
	ExactCount                    uint64                                    `gorm:"column:exact_count" json:"exact_count"`
	ConditionalCount              uint64                                    `gorm:"column:conditional_count" json:"conditional_count"`
	BlockedCount                  uint64                                    `gorm:"column:blocked_count" json:"blocked_count"`
	NoopCount                     uint64                                    `gorm:"column:noop_count" json:"noop_count"`
	BlockerCount                  uint64                                    `gorm:"column:blocker_count" json:"blocker_count"`
	DispositionDigest             string                                    `gorm:"column:disposition_digest" json:"disposition_digest"`
	BlockerDigest                 string                                    `gorm:"column:blocker_digest" json:"blocker_digest"`
	WorkerAttemptCount            uint                                      `gorm:"column:attempt_count" json:"worker_attempt_count"`
	LastErrorCode                 string                                    `gorm:"column:last_error_code" json:"last_error_code"`
	CompletedAt                   *time.Time                                `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt                     time.Time                                 `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                     time.Time                                 `gorm:"column:updated_at" json:"updated_at"`
	Outcome                       *string                                   `gorm:"column:outcome" json:"outcome,omitempty"`
	ManifestDigest                *string                                   `gorm:"column:manifest_digest" json:"manifest_digest,omitempty"`
	SwitchAuthorized              bool                                      `gorm:"-" json:"switch_authorized"`
	Blockers                      []entity.ChainEventReorgAssessmentBlocker `gorm:"-" json:"blockers,omitempty"`
}

type chainEventReorgPrerequisiteSummary struct {
	ID                       uint64                                          `gorm:"column:id" json:"id"`
	RunUUID                  string                                          `gorm:"column:run_uuid" json:"run_uuid"`
	IncidentID               uint64                                          `gorm:"column:incident_id" json:"incident_id"`
	AssessmentRunID          uint64                                          `gorm:"column:assessment_run_id" json:"assessment_run_id"`
	AssessmentManifestDigest string                                          `gorm:"column:assessment_manifest_digest" json:"assessment_manifest_digest"`
	AlgorithmVersion         string                                          `gorm:"column:algorithm_version" json:"algorithm_version"`
	ChainID                  uint64                                          `gorm:"column:chain_id" json:"chain_id"`
	BoundaryBlock            uint64                                          `gorm:"column:boundary_block" json:"boundary_block"`
	BoundaryBlockHash        string                                          `gorm:"column:boundary_block_hash" json:"boundary_block_hash"`
	ExpectedRequirementCount uint64                                          `gorm:"column:expected_requirement_count" json:"expected_requirement_count"`
	State                    string                                          `gorm:"column:state" json:"state"`
	NextRequirementOrdinal   uint64                                          `gorm:"column:next_requirement_ordinal" json:"next_requirement_ordinal"`
	RequirementCount         uint64                                          `gorm:"column:requirement_count" json:"requirement_count"`
	UnmetCount               uint64                                          `gorm:"column:unmet_count" json:"unmet_count"`
	UntrustedCount           uint64                                          `gorm:"column:untrusted_count" json:"untrusted_count"`
	DeferredCount            uint64                                          `gorm:"column:deferred_count" json:"deferred_count"`
	ArtifactCount            uint64                                          `gorm:"column:artifact_count" json:"artifact_count"`
	RequirementDigest        string                                          `gorm:"column:requirement_digest" json:"requirement_digest"`
	ArtifactDigest           string                                          `gorm:"column:artifact_digest" json:"artifact_digest"`
	WorkerAttemptCount       uint                                            `gorm:"column:attempt_count" json:"worker_attempt_count"`
	LastErrorCode            string                                          `gorm:"column:last_error_code" json:"last_error_code"`
	CompletedAt              *time.Time                                      `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt                time.Time                                       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                time.Time                                       `gorm:"column:updated_at" json:"updated_at"`
	Outcome                  *string                                         `gorm:"column:outcome" json:"outcome,omitempty"`
	CandidateReady           bool                                            `gorm:"column:candidate_ready" json:"candidate_ready"`
	SatisfiedCount           uint64                                          `gorm:"column:satisfied_count" json:"satisfied_count"`
	EventDigest              *string                                         `gorm:"column:event_digest" json:"event_digest,omitempty"`
	ManifestDigest           *string                                         `gorm:"column:manifest_digest" json:"manifest_digest,omitempty"`
	Requirements             []entity.ChainEventReorgPrerequisiteRequirement `gorm:"-" json:"requirements,omitempty"`
}

type chainEventReorgGenerationSummary struct {
	ID                            uint64                                        `gorm:"column:id" json:"id"`
	GenerationUUID                string                                        `gorm:"column:generation_uuid" json:"generation_uuid"`
	IncidentID                    uint64                                        `gorm:"column:incident_id" json:"incident_id"`
	AttemptNo                     uint                                          `gorm:"column:attempt_no" json:"attempt_no"`
	GenerationKind                string                                        `gorm:"column:generation_kind" json:"generation_kind"`
	PrerequisiteRunID             uint64                                        `gorm:"column:prerequisite_run_id" json:"prerequisite_run_id"`
	PrerequisiteManifestDigest    string                                        `gorm:"column:prerequisite_manifest_digest" json:"prerequisite_manifest_digest"`
	AssessmentRunID               uint64                                        `gorm:"column:assessment_run_id" json:"assessment_run_id"`
	AssessmentManifestDigest      string                                        `gorm:"column:assessment_manifest_digest" json:"assessment_manifest_digest"`
	MaterializationRunID          uint64                                        `gorm:"column:materialization_run_id" json:"materialization_run_id"`
	MaterializationManifestDigest string                                        `gorm:"column:materialization_manifest_digest" json:"materialization_manifest_digest"`
	ChainID                       uint64                                        `gorm:"column:chain_id" json:"chain_id"`
	BoundaryBlock                 uint64                                        `gorm:"column:boundary_block" json:"boundary_block"`
	BoundaryBlockHash             string                                        `gorm:"column:boundary_block_hash" json:"boundary_block_hash"`
	TargetSafeBlock               uint64                                        `gorm:"column:target_safe_block" json:"target_safe_block"`
	TargetSafeBlockHash           string                                        `gorm:"column:target_safe_block_hash" json:"target_safe_block_hash"`
	LegacyContractAddress         string                                        `gorm:"column:legacy_contract_address" json:"legacy_contract_address"`
	V2ContractAddress             string                                        `gorm:"column:v2_contract_address" json:"v2_contract_address"`
	OldEffectRoot                 string                                        `gorm:"column:old_effect_root" json:"old_effect_root"`
	ReplacementEffectRoot         string                                        `gorm:"column:replacement_effect_root" json:"replacement_effect_root"`
	AlgorithmVersion              string                                        `gorm:"column:algorithm_version" json:"algorithm_version"`
	SchemaVersion                 string                                        `gorm:"column:schema_version" json:"schema_version"`
	DescriptorDigest              string                                        `gorm:"column:descriptor_digest" json:"descriptor_digest"`
	State                         string                                        `gorm:"column:state" json:"state"`
	Stage                         string                                        `gorm:"column:stage" json:"stage"`
	SenderCount                   uint64                                        `gorm:"column:sender_count" json:"sender_count"`
	SubmissionCount               uint64                                        `gorm:"column:submission_count" json:"submission_count"`
	SubmissionPayloadBytes        uint64                                        `gorm:"column:submission_payload_bytes" json:"submission_payload_bytes"`
	NonceCount                    uint64                                        `gorm:"column:nonce_count" json:"nonce_count"`
	RequirementCount              uint64                                        `gorm:"column:requirement_count" json:"requirement_count"`
	UnmetCount                    uint64                                        `gorm:"column:unmet_count" json:"unmet_count"`
	VerifiedCount                 uint64                                        `gorm:"column:verified_count" json:"verified_count"`
	SenderRoot                    string                                        `gorm:"column:sender_root" json:"sender_root"`
	SubmissionRoot                string                                        `gorm:"column:submission_root" json:"submission_root"`
	NonceRoot                     string                                        `gorm:"column:nonce_root" json:"nonce_root"`
	RequirementRoot               string                                        `gorm:"column:requirement_root" json:"requirement_root"`
	WorkerAttemptCount            uint                                          `gorm:"column:attempt_count" json:"worker_attempt_count"`
	LastErrorCode                 string                                        `gorm:"column:last_error_code" json:"last_error_code"`
	CompletedAt                   *time.Time                                    `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt                     time.Time                                     `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                     time.Time                                     `gorm:"column:updated_at" json:"updated_at"`
	Outcome                       *string                                       `gorm:"column:outcome" json:"outcome,omitempty"`
	CandidateReady                bool                                          `gorm:"column:candidate_ready" json:"candidate_ready"`
	ManifestDigest                *string                                       `gorm:"column:manifest_digest" json:"manifest_digest,omitempty"`
	Requirements                  []entity.ChainEventReorgGenerationRequirement `gorm:"-" json:"requirements,omitempty"`
}

type chainEventSyncSafeSnapshot struct {
	ID                       uint64     `json:"id"`
	Initialized              bool       `json:"initialized"`
	CursorBlock              uint64     `json:"cursor_block"`
	CursorBlockHash          string     `json:"cursor_block_hash"`
	V2Registered             bool       `json:"v2_registered"`
	V2ContractAddress        string     `json:"v2_contract_address"`
	V2StartBlock             uint64     `json:"v2_start_block"`
	V2ImplementationAddress  string     `json:"v2_implementation_address"`
	V2ImplementationCodeHash string     `json:"v2_implementation_code_hash"`
	LatestBlock              uint64     `json:"latest_block"`
	SafeBlock                uint64     `json:"safe_block"`
	LagBlocks                uint64     `json:"lag_blocks"`
	LastStartedAt            *time.Time `json:"last_started_at,omitempty"`
	LastSuccessAt            *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt            *time.Time `json:"last_failure_at,omitempty"`
	LastErrorCode            string     `json:"last_error_code"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

func safeChainEventSyncState(state entity.ChainEventSyncState) chainEventSyncSafeSnapshot {
	errorCode := ""
	if state.LastError != "" {
		errorCode = "CHAIN_EVENT_SYNC_FAILED"
	}
	return chainEventSyncSafeSnapshot{
		ID: state.ID, Initialized: state.Initialized, CursorBlock: state.CursorBlock, CursorBlockHash: state.CursorBlockHash,
		V2Registered: state.V2Registered, V2ContractAddress: state.V2ContractAddress, V2StartBlock: state.V2StartBlock,
		V2ImplementationAddress: state.V2ImplementationAddress, V2ImplementationCodeHash: state.V2ImplementationCodeHash,
		LatestBlock: state.LatestBlock, SafeBlock: state.SafeBlock, LagBlocks: state.LagBlocks,
		LastStartedAt: state.LastStartedAt, LastSuccessAt: state.LastSuccessAt, LastFailureAt: state.LastFailureAt,
		LastErrorCode: errorCode, CreatedAt: state.CreatedAt, UpdatedAt: state.UpdatedAt,
	}
}

var errChainEventNotReplayable = errors.New("failure is not replayable; reconciliation records require manual accounting and compensation")
var errChainEventReorgActive = errors.New("failure replay is disabled while a chain event reorg incident is active")

// ChainEventHealth GET /admin/v1/chain-events/health
func ChainEventHealth(c *gin.Context) {
	state := entity.ChainEventSyncState{}
	if err := db.Mysql().Where("id = 1").Take(&state).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event sync state is unavailable"})
		return
	}
	counts := make([]struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}, 0, 3)
	if err := db.Mysql().Model(&entity.ChainEventFailure{}).
		Select("status, count(*) AS count").Group("status").Scan(&counts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event failure counts are unavailable"})
		return
	}
	failureCounts := map[string]int64{
		constant.ChainEventFailureRetrying:   0,
		constant.ChainEventFailureDeadLetter: 0,
		constant.ChainEventFailureResolved:   0,
	}
	for _, count := range counts {
		failureCounts[count.Status] = count.Count
	}
	staleAfter := 3 * time.Minute
	stale := state.LastStartedAt == nil || state.UpdatedAt.IsZero() || time.Since(state.UpdatedAt) > staleAfter
	maxHealthyLag := viper.GetUint64("chainEvent.maxHealthyLagBlocks")
	if maxHealthyLag == 0 {
		maxHealthyLag = 1000
	}
	lagging := state.LagBlocks > maxHealthyLag
	activeReorgs, activeIncident, err := loadActiveChainEventReorg(db.Mysql())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg state is unavailable"})
		return
	}
	activeRecovery, err := loadActiveChainEventReorgRecovery(db.Mysql(), activeIncident)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg recovery state is unavailable"})
		return
	}
	activeMaterialization, err := loadActiveChainEventReorgMaterialization(db.Mysql(), activeIncident)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg materialization state is unavailable"})
		return
	}
	activeAssessment, err := loadActiveChainEventReorgAssessment(db.Mysql(), activeIncident)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg assessment state is unavailable"})
		return
	}
	activePrerequisite, err := loadActiveChainEventReorgPrerequisite(db.Mysql(), activeIncident)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg prerequisite state is unavailable"})
		return
	}
	activeGeneration, err := loadActiveChainEventReorgGeneration(db.Mysql(), activeIncident)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain event reorg generation state is unavailable"})
		return
	}
	var blockedSubmissions int64
	staleSubmissionBefore := time.Now().Add(-staleAfter)
	if err := db.Mysql().Model(&entity.ChainTxSubmission{}).Where(
		"state = ? OR (state = ? AND NOT EXISTS (SELECT 1 FROM chain_tx_submission newer WHERE newer.chain_id = chain_tx_submission.chain_id AND newer.contract_address = chain_tx_submission.contract_address AND newer.project_id = chain_tx_submission.project_id AND newer.action = chain_tx_submission.action AND newer.attempt_no > chain_tx_submission.attempt_no)) OR (state IN ? AND created_at < ?) OR (state = ? AND updated_at < ?)",
		constant.ChainTxStateAmbiguous, constant.ChainTxStateFinalRevert,
		[]string{constant.ChainTxStateIntent, constant.ChainTxStateReady, constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown, constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert}, staleSubmissionBefore,
		constant.ChainTxStateFinalSuccess, staleSubmissionBefore,
	).Count(&blockedSubmissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_HEALTH_FAILED", "message": "chain transaction health is unavailable"})
		return
	}
	healthy := state.Initialized && state.LastSuccessAt != nil && !stale && !lagging && state.LastError == "" && failureCounts[constant.ChainEventFailureDeadLetter] == 0 && blockedSubmissions == 0 && activeReorgs == 0
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"sync": safeChainEventSyncState(state), "failure_counts": failureCounts, "healthy": healthy,
		"stale": stale, "stale_after_seconds": int(staleAfter.Seconds()),
		"lagging": lagging, "max_healthy_lag_blocks": maxHealthyLag,
		"blocked_chain_transactions": blockedSubmissions,
		"active_reorg_incidents":     activeReorgs, "active_reorg_incident": activeIncident,
		"active_reorg_recovery":        activeRecovery,
		"active_reorg_materialization": activeMaterialization,
		"active_reorg_assessment":      activeAssessment,
		"active_reorg_prerequisite":    activePrerequisite,
		"active_reorg_generation":      activeGeneration,
	}})
}

func loadActiveChainEventReorgGeneration(database *gorm.DB, incident *entity.ChainEventReorgIncident) (*chainEventReorgGenerationSummary, error) {
	if incident == nil {
		return nil, nil
	}
	summary := chainEventReorgGenerationSummary{}
	err := database.Table("chain_event_reorg_generation r").
		Joins("JOIN chain_event_reorg_generation_job j ON j.run_id=r.id").
		Joins("LEFT JOIN chain_event_reorg_generation_manifest m ON m.run_id=r.id").
		Where("r.incident_id = ?", incident.ID).
		Select(chainEventReorgGenerationSafeSelect + ",m.outcome,m.candidate_ready,m.manifest_digest").
		Order("r.attempt_no DESC").Limit(1).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	if summary.ID == 0 {
		return nil, nil
	}
	if err := database.Select("id,run_id,ordinal,requirement_code,evidence_status,safe_detail,row_digest,created_at").
		Where("run_id = ?", summary.ID).Order("ordinal ASC").Limit(5).Find(&summary.Requirements).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func loadActiveChainEventReorgPrerequisite(database *gorm.DB, incident *entity.ChainEventReorgIncident) (*chainEventReorgPrerequisiteSummary, error) {
	if incident == nil {
		return nil, nil
	}
	summary := chainEventReorgPrerequisiteSummary{}
	err := database.Table("chain_event_reorg_prerequisite_run r").
		Joins("JOIN chain_event_reorg_prerequisite_job j ON j.run_id=r.id").
		Joins("LEFT JOIN chain_event_reorg_prerequisite_manifest m ON m.run_id=r.id").
		Where("r.incident_id = ?", incident.ID).
		Select(chainEventReorgPrerequisiteSafeSelect + ",m.outcome,m.candidate_ready,m.satisfied_count,m.event_digest,m.manifest_digest").
		Order("r.id DESC").Limit(1).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	if summary.ID == 0 {
		return nil, nil
	}
	if err := database.Select("id,run_id,ordinal,requirement_code,evidence_status,artifact_id,safe_detail,row_digest,created_at").
		Where("run_id = ?", summary.ID).Order("ordinal ASC").Limit(7).Find(&summary.Requirements).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func loadActiveChainEventReorgAssessment(database *gorm.DB, incident *entity.ChainEventReorgIncident) (*chainEventReorgAssessmentSummary, error) {
	if incident == nil {
		return nil, nil
	}
	summary := chainEventReorgAssessmentSummary{}
	err := database.Table("chain_event_reorg_assessment_run r").
		Joins("JOIN chain_event_reorg_assessment_job j ON j.run_id=r.id").
		Joins("LEFT JOIN chain_event_reorg_assessment_manifest m ON m.run_id=r.id").
		Where("r.incident_id = ?", incident.ID).
		Select(chainEventReorgAssessmentSafeSelect + ",m.outcome,m.manifest_digest").
		Order("r.id DESC").Limit(1).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	if summary.ID == 0 {
		return nil, nil
	}
	summary.SwitchAuthorized = false
	if err := database.Select("id,run_id,ordinal,disposition_id,scope,blocker_code,safe_detail,row_digest,created_at").
		Where("run_id = ?", summary.ID).Order("ordinal ASC").Limit(200).Find(&summary.Blockers).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func loadActiveChainEventReorgMaterialization(database *gorm.DB, incident *entity.ChainEventReorgIncident) (*chainEventReorgMaterializationSummary, error) {
	if incident == nil {
		return nil, nil
	}
	summary := chainEventReorgMaterializationSummary{}
	err := database.Table("chain_event_reorg_materialization_run r").
		Joins("JOIN chain_event_reorg_materialization_job j ON j.run_id=r.id").
		Joins("LEFT JOIN chain_event_reorg_materialization_manifest m ON m.run_id=r.id").
		Where("r.incident_id = ?", incident.ID).
		Select(chainEventReorgMaterializationSafeSelect + ",m.manifest_digest").
		Order("r.id DESC").Limit(1).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	if summary.ID == 0 {
		return nil, nil
	}
	return &summary, nil
}

func loadActiveChainEventReorgRecovery(database *gorm.DB, incident *entity.ChainEventReorgIncident) (*chainEventReorgRecoverySummary, error) {
	if incident == nil {
		return nil, nil
	}
	summary := chainEventReorgRecoverySummary{}
	err := database.Table("chain_event_reorg_recovery_run r").
		Joins("JOIN chain_event_reorg_recovery_job j ON j.run_id=r.id").
		Joins("LEFT JOIN chain_event_reorg_recovery_manifest m ON m.run_id=r.id").
		Where("r.incident_id = ?", incident.ID).
		Select(chainEventReorgRecoverySafeSelect + ",m.manifest_digest,(SELECT COUNT(*) FROM chain_event_reorg_recovery_batch b WHERE b.run_id=r.id) AS completed_batch_count").
		Order("r.attempt_no DESC").Limit(1).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	if summary.ID == 0 {
		return nil, nil
	}
	return &summary, nil
}

func loadActiveChainEventReorg(database *gorm.DB) (int64, *entity.ChainEventReorgIncident, error) {
	var count int64
	query := database.Model(&entity.ChainEventReorgIncident{}).Where("active_slot = 1 AND status = ?", "detected")
	if err := query.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	if count == 0 {
		return 0, nil, nil
	}
	if count != 1 {
		return count, nil, errors.New("multiple active reorg incidents violate the singleton operational boundary")
	}
	incident := entity.ChainEventReorgIncident{}
	if err := query.Select(chainEventReorgSafeSelect).Take(&incident).Error; err != nil {
		return count, nil, err
	}
	return count, &incident, nil
}

// ListChainEventFailures GET /admin/v1/chain-events/failures?status=&page=&page_size=
func ListChainEventFailures(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := db.Mysql().Model(&entity.ChainEventFailure{})
	if status := c.Query("status"); status != "" {
		if !validChainEventFailureStatus(status) {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_EVENT_BAD_STATUS", "message": "invalid failure status"})
			return
		}
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_FAILURE_LIST_FAILED", "message": err.Error()})
		return
	}
	rows := make([]entity.ChainEventFailure, 0, pageSize)
	if err := query.Select(chainEventFailureSafeSelect).Order("last_failed_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_FAILURE_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": pageSize,
	}})
}

// GetChainEventFailure GET /admin/v1/chain-events/failures/:id
func GetChainEventFailure(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_EVENT_BAD_ID", "message": "invalid failure id"})
		return
	}
	failure := entity.ChainEventFailure{}
	if err := db.Mysql().Select(chainEventFailureSafeSelect).Where("id = ?", id).Take(&failure).Error; err != nil {
		status := http.StatusInternalServerError
		code := "CHAIN_EVENT_FAILURE_GET_FAILED"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
			code = "CHAIN_EVENT_FAILURE_NOT_FOUND"
		}
		c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
		return
	}
	if err := db.Mysql().Where("failure_id = ?", id).Order("replay_no DESC").Find(&failure.ReplayAudits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_FAILURE_GET_FAILED", "message": err.Error()})
		return
	}
	if uint(len(failure.ReplayAudits)) != failure.ReplayCount {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "CHAIN_EVENT_REPLAY_AUDIT_MISMATCH", "message": "replay audit history does not match replay count"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": failure})
}

// ReplayChainEventFailure POST /admin/v1/chain-events/failures/:id/replay
func ReplayChainEventFailure(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_EVENT_BAD_ID", "message": "invalid failure id"})
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_EVENT_REPLAY_REASON_REQUIRED", "message": "replay reason is required"})
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	reasonLength := utf8.RuneCountInString(request.Reason)
	if reasonLength < 3 || reasonLength > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "CHAIN_EVENT_REPLAY_REASON_INVALID", "message": "replay reason must be 3-500 characters"})
		return
	}
	adminID := currentAdminID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "CHAIN_EVENT_ADMIN_REQUIRED", "message": "authenticated administrator is required"})
		return
	}
	if err := replayChainEventFailure(id, adminID, request.Reason); err != nil {
		status, code := http.StatusInternalServerError, "CHAIN_EVENT_REPLAY_FAILED"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status, code = http.StatusNotFound, "CHAIN_EVENT_FAILURE_NOT_FOUND"
		} else if errors.Is(err, errAdminProjectExecuteRevoked) {
			status, code = http.StatusForbidden, "CHAIN_EVENT_ADMIN_REVOKED"
		} else if errors.Is(err, errChainEventNotReplayable) {
			status, code = http.StatusConflict, "CHAIN_EVENT_NOT_REPLAYABLE"
		} else if errors.Is(err, errChainEventReorgActive) {
			status, code = http.StatusConflict, "CHAIN_EVENT_REORG_ACTIVE"
		}
		c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "status": constant.ChainEventFailureRetrying, "reason": request.Reason}})
}

func replayChainEventFailure(id, adminID uint64, reason string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		syncState := entity.ChainEventSyncState{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = 1").Take(&syncState).Error; err != nil {
			return err
		}
		if err := requireCurrentProjectExecuteAdmin(tx, adminID); err != nil {
			return err
		}
		var activeReorgs int64
		if err := tx.Model(&entity.ChainEventReorgIncident{}).Where("active_slot = 1 AND status = ?", "detected").Count(&activeReorgs).Error; err != nil {
			return err
		}
		if activeReorgs != 0 {
			return errChainEventReorgActive
		}
		failure := entity.ChainEventFailure{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,event,status,replay_count").Where("id = ?", id).Take(&failure).Error; err != nil {
			return err
		}
		if !chainEventAdminReplayable(failure) {
			return errChainEventNotReplayable
		}
		now := time.Now()
		replayNo := nextChainEventReplayNo(failure)
		audit := entity.ChainEventReplayAudit{
			FailureID: id, ReplayNo: replayNo, AdminID: adminID,
			Reason: reason, RequestedAt: now,
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		result := tx.Model(&entity.ChainEventFailure{}).
			Where("id = ? AND status = ? AND event <> ? AND replay_count = ?", id, constant.ChainEventFailureDeadLetter, "reconciliation", failure.ReplayCount).
			Updates(map[string]interface{}{
				"status":              constant.ChainEventFailureRetrying,
				"consecutive_count":   0,
				"replay_count":        replayNo,
				"replay_requested_at": now,
				"resolved_at":         nil,
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errChainEventNotReplayable
		}
		return nil
	})
}

func chainEventAdminReplayable(failure entity.ChainEventFailure) bool {
	return failure.Status == constant.ChainEventFailureDeadLetter && failure.Event != "reconciliation"
}

func nextChainEventReplayNo(failure entity.ChainEventFailure) uint {
	return failure.ReplayCount + 1
}

func validChainEventFailureStatus(status string) bool {
	switch status {
	case constant.ChainEventFailureRetrying, constant.ChainEventFailureDeadLetter, constant.ChainEventFailureResolved:
		return true
	default:
		return false
	}
}
