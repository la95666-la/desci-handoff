package entity

import "time"

// FairLaunchPreview is the server-issued, one-time authorization handle for an
// exact Set or action preview. Only its opaque token is returned to Admin.
type FairLaunchPreview struct {
	ID                uint64     `gorm:"column:id;primaryKey"`
	TokenDigest       string     `gorm:"column:token_digest"`
	PreviewKind       string     `gorm:"column:preview_kind"`
	ProjectID         uint64     `gorm:"column:project_id"`
	Action            string     `gorm:"column:action"`
	Operation         string     `gorm:"column:operation"`
	AdminID           uint64     `gorm:"column:admin_id"`
	CanonicalDigest   string     `gorm:"column:canonical_digest"`
	BaseDigest        string     `gorm:"column:base_digest"`
	CanonicalPayload  string     `gorm:"column:canonical_payload"`
	IssuedAt          time.Time  `gorm:"column:issued_at"`
	ExpiresAt         time.Time  `gorm:"column:expires_at"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	ConsumedByAdminID *uint64    `gorm:"column:consumed_by_admin_id"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
}

func (*FairLaunchPreview) TableName() string { return "fair_launch_preview" }

type FairLaunchMFAProvider struct {
	ProviderID      string     `gorm:"column:provider_id;primaryKey"`
	AdmissionDigest string     `gorm:"column:admission_digest"`
	ManifestDigest  string     `gorm:"column:manifest_digest"`
	AdapterIdentity string     `gorm:"column:adapter_identity"`
	Generation      uint64     `gorm:"column:generation"`
	Enabled         bool       `gorm:"column:enabled"`
	RevokedAt       *time.Time `gorm:"column:revoked_at"`
	ValidFrom       time.Time  `gorm:"column:valid_from"`
	ValidUntil      *time.Time `gorm:"column:valid_until"`
}

func (*FairLaunchMFAProvider) TableName() string { return "fair_launch_mfa_provider" }

// FairLaunchMFAAssertion is provisioned only by an approved external MFA
// provider adapter. Production HTTP routes never issue or verify assertions.
type FairLaunchMFAAssertion struct {
	ID                      uint64     `gorm:"column:id;primaryKey"`
	TokenDigest             string     `gorm:"column:token_digest"`
	ProviderID              string     `gorm:"column:provider_id"`
	ProviderEvidenceDigest  string     `gorm:"column:provider_evidence_digest"`
	ProviderAdmissionDigest string     `gorm:"column:provider_admission_digest"`
	ProviderGeneration      uint64     `gorm:"column:provider_generation"`
	ExternallyVerified      bool       `gorm:"column:externally_verified"`
	AdminID                 uint64     `gorm:"column:admin_id"`
	Operation               string     `gorm:"column:operation"`
	Stage                   string     `gorm:"column:stage"`
	ProjectID               uint64     `gorm:"column:project_id"`
	PreviewID               *uint64    `gorm:"column:preview_id"`
	ActionRequestID         *uint64    `gorm:"column:action_request_id"`
	V3PolicyReviewID        *uint64    `gorm:"column:v3_policy_review_id"`
	AdminApprovalID         *uint64    `gorm:"column:admin_approval_id"`
	IssuedAt                time.Time  `gorm:"column:issued_at"`
	ExpiresAt               time.Time  `gorm:"column:expires_at"`
	ConsumedAt              *time.Time `gorm:"column:consumed_at"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
}

func (*FairLaunchMFAAssertion) TableName() string { return "fair_launch_mfa_assertion" }
