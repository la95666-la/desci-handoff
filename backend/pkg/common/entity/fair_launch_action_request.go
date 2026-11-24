package entity

import "time"

type FairLaunchActionRequest struct {
	ID                       uint64     `gorm:"column:id;primaryKey"`
	ProjectID                uint64     `gorm:"column:project_id"`
	CanonicalDigest          string     `gorm:"column:canonical_digest"`
	CanonicalPayload         string     `gorm:"column:canonical_payload;type:longtext"`
	Action                   string     `gorm:"column:action"`
	ExecutionDomain          string     `gorm:"column:execution_domain"`
	InitiatorAdminID         uint64     `gorm:"column:initiator_admin_id"`
	ApproverAdminID          *uint64    `gorm:"column:approver_admin_id"`
	ExecutorAdminID          *uint64    `gorm:"column:executor_admin_id"`
	Status                   string     `gorm:"column:status"`
	PreviewExpiresAt         time.Time  `gorm:"column:preview_expires_at"`
	ApprovalExpiresAt        time.Time  `gorm:"column:approval_expires_at"`
	ExecutionExpiresAt       time.Time  `gorm:"column:execution_expires_at"`
	InitiatorMFAEvidenceRef  string     `gorm:"column:initiator_mfa_evidence_ref"`
	InitiatorMFAVerifiedAt   time.Time  `gorm:"column:initiator_mfa_verified_at"`
	ApproverMFAEvidenceRef   *string    `gorm:"column:approver_mfa_evidence_ref"`
	ApproverMFAVerifiedAt    *time.Time `gorm:"column:approver_mfa_verified_at"`
	ExecutorMFAEvidenceRef   *string    `gorm:"column:executor_mfa_evidence_ref"`
	ExecutorMFAVerifiedAt    *time.Time `gorm:"column:executor_mfa_verified_at"`
	RuntimeGeneration        uint64     `gorm:"column:runtime_generation"`
	PolicyGeneration         uint64     `gorm:"column:policy_generation"`
	SourceRegistryGeneration uint64     `gorm:"column:source_registry_generation"`
	ConfigGeneration         uint64     `gorm:"column:config_generation"`
	SubmissionID             *uint64    `gorm:"column:submission_id"`
	TimelockOperationRowID   *uint64    `gorm:"column:timelock_operation_row_id"`
	SafeTxID                 *uint64    `gorm:"column:safe_tx_id"`
	ExternalObservationRef   *string    `gorm:"column:external_observation_ref"`
	AdminApprovalID          *uint64    `gorm:"column:admin_approval_id"`
	PreviewID                uint64     `gorm:"column:preview_id"`
	CreatedAt                time.Time  `gorm:"column:created_at"`
	UpdatedAt                time.Time  `gorm:"column:updated_at"`
}

func (*FairLaunchActionRequest) TableName() string { return "fair_launch_action_request" }

type FairLaunchActionTransition struct {
	ID              uint64    `gorm:"column:id;primaryKey"`
	ActionRequestID uint64    `gorm:"column:action_request_id"`
	SequenceNo      uint32    `gorm:"column:sequence_no"`
	FromStatus      string    `gorm:"column:from_status"`
	ToStatus        string    `gorm:"column:to_status"`
	ActorAdminID    uint64    `gorm:"column:actor_admin_id"`
	MFAEvidenceRef  string    `gorm:"column:mfa_evidence_ref"`
	EvidenceDigest  string    `gorm:"column:evidence_digest"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (*FairLaunchActionTransition) TableName() string { return "fair_launch_action_transition" }
