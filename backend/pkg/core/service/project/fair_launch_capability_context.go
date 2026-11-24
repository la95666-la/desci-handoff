package project

import (
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
)

// DecorateFairLaunchCapabilities adds request-stage MFA and identity-separation
// metadata. It never changes a runtime decision from unavailable to available.
func DecorateFairLaunchCapabilities(db *gorm.DB, caps FairLaunchCapabilityEnvelope, projectID, adminID uint64, now time.Time) FairLaunchCapabilityEnvelope {
	for name, descriptor := range caps.Actions {
		descriptor.StepUpMFAVerified = false
		descriptor.StepUpMFAFreshUntil = ""
		descriptor.ForbiddenAdminIDs = []uint64{}
		caps.Actions[name] = descriptor
	}
	if db == nil || projectID == 0 || adminID == 0 {
		return caps
	}
	var rows []entity.FairLaunchActionRequest
	if err := db.Where("project_id=? AND status IN ?", projectID, []string{fairLaunchStatusPending, fairLaunchStatusApprovalBound, fairLaunchStatusApproved, fairLaunchStatusExecutionBound}).Order("id DESC").Find(&rows).Error; err != nil {
		caps.Available = false
		caps.ReasonCode = "action_context_unavailable"
		for name, descriptor := range caps.Actions {
			descriptor.Enabled = false
			descriptor.ReasonCode = caps.ReasonCode
			caps.Actions[name] = descriptor
		}
		return caps
	}
	for name, descriptor := range caps.Actions {
		for _, row := range rows {
			if row.Action != name {
				continue
			}
			descriptor.ForbiddenAdminIDs = append(descriptor.ForbiddenAdminIDs, row.InitiatorAdminID)
			if row.ApproverAdminID != nil {
				descriptor.ForbiddenAdminIDs = append(descriptor.ForbiddenAdminIDs, *row.ApproverAdminID)
			}
			break
		}
		caps.Actions[name] = descriptor
	}
	return caps
}

type FairLaunchExecutionQueueItem struct {
	ID                       uint64                          `json:"id"`
	ProjectID                uint64                          `json:"project_id"`
	Action                   string                          `json:"action"`
	ExecutionDomain          string                          `json:"execution_domain"`
	Status                   string                          `json:"status"`
	InitiatorAdminID         uint64                          `json:"initiator_admin_id"`
	ApproverAdminID          uint64                          `json:"approver_admin_id"`
	ExecutorAdminID          *uint64                         `json:"executor_admin_id,omitempty"`
	ForbiddenAdminIDs        []uint64                        `json:"forbidden_admin_ids"`
	ExecutionExpiresAt       string                          `json:"execution_expires_at"`
	RuntimeGeneration        uint64                          `json:"runtime_generation"`
	PolicyGeneration         uint64                          `json:"policy_generation"`
	SourceRegistryGeneration uint64                          `json:"source_registry_generation"`
	ConfigGeneration         uint64                          `json:"config_generation"`
	StepUpMFAVerified        bool                            `json:"step_up_mfa_verified"`
	StepUpMFAFreshUntil      string                          `json:"step_up_mfa_fresh_until,omitempty"`
	Transitions              []FairLaunchActionTransitionDTO `json:"transitions"`
}

type FairLaunchActionTransitionDTO struct {
	SequenceNo     uint32 `json:"sequence_no"`
	FromStatus     string `json:"from_status"`
	ToStatus       string `json:"to_status"`
	ActorAdminID   uint64 `json:"actor_admin_id"`
	MFAEvidenceRef string `json:"mfa_evidence_ref"`
	EvidenceDigest string `json:"evidence_digest"`
	CreatedAt      string `json:"created_at"`
}

func ListFairLaunchExecutionQueue(db *gorm.DB, projectID, executorID uint64, now time.Time) ([]FairLaunchExecutionQueueItem, error) {
	_ = executorID // forbidden_admin_ids lets the UI explain conflicts; execution rechecks authoritatively.
	var rows []entity.FairLaunchActionRequest
	// Include the whole durable request lifecycle so Admin can audit binding and
	// dispatch transitions. Only approved rows expose the Execute control.
	if err := db.Where("project_id=?", projectID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]FairLaunchExecutionQueueItem, 0, len(rows))
	for _, row := range rows {
		var transitions []entity.FairLaunchActionTransition
		if err := db.Where("action_request_id=?", row.ID).Order("sequence_no ASC").Find(&transitions).Error; err != nil {
			return nil, err
		}
		history := make([]FairLaunchActionTransitionDTO, 0, len(transitions))
		for _, transition := range transitions {
			history = append(history, FairLaunchActionTransitionDTO{SequenceNo: transition.SequenceNo, FromStatus: transition.FromStatus, ToStatus: transition.ToStatus, ActorAdminID: transition.ActorAdminID, MFAEvidenceRef: transition.MFAEvidenceRef, EvidenceDigest: transition.EvidenceDigest, CreatedAt: transition.CreatedAt.UTC().Format(time.RFC3339Nano)})
		}
		approverID := uint64(0)
		forbidden := []uint64{row.InitiatorAdminID}
		if row.ApproverAdminID != nil {
			approverID = *row.ApproverAdminID
			forbidden = append(forbidden, approverID)
		}
		items = append(items, FairLaunchExecutionQueueItem{ID: row.ID, ProjectID: row.ProjectID, Action: row.Action, ExecutionDomain: row.ExecutionDomain, Status: row.Status, InitiatorAdminID: row.InitiatorAdminID, ApproverAdminID: approverID, ExecutorAdminID: row.ExecutorAdminID, ForbiddenAdminIDs: forbidden, ExecutionExpiresAt: row.ExecutionExpiresAt.UTC().Format(time.RFC3339), RuntimeGeneration: row.RuntimeGeneration, PolicyGeneration: row.PolicyGeneration, SourceRegistryGeneration: row.SourceRegistryGeneration, ConfigGeneration: row.ConfigGeneration, StepUpMFAVerified: false, Transitions: history})
	}
	return items, nil
}
