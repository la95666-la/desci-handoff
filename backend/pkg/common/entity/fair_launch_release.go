package entity

import "time"

type FairLaunchSourceRegistrySnapshot struct {
	ID                       uint64     `gorm:"column:id;primaryKey"`
	Environment              string     `gorm:"column:environment"`
	Revision                 uint64     `gorm:"column:revision"`
	PayloadDigest            string     `gorm:"column:payload_digest"`
	CanonicalPayload         string     `gorm:"column:canonical_payload;type:longtext"`
	EnvelopeDigest           string     `gorm:"column:envelope_digest"`
	KeySetVersion            uint64     `gorm:"column:key_set_version"`
	ChainID                  uint64     `gorm:"column:chain_id"`
	ProxyAddress             string     `gorm:"column:proxy_address"`
	ImplementationAddress    string     `gorm:"column:implementation_address"`
	ImplementationCodeHash   string     `gorm:"column:implementation_code_hash"`
	ABIDigest                string     `gorm:"column:abi_digest"`
	SourceID                 uint64     `gorm:"column:source_id"`
	SourceRevisionID         uint64     `gorm:"column:source_revision_id"`
	SourceRevisionNo         uint32     `gorm:"column:source_revision_no"`
	RuntimeGeneration        uint64     `gorm:"column:runtime_generation"`
	PolicyGeneration         uint64     `gorm:"column:policy_generation"`
	SourceRegistryGeneration uint64     `gorm:"column:source_registry_generation"`
	ConfigGeneration         uint64     `gorm:"column:config_generation"`
	ValidFrom                time.Time  `gorm:"column:valid_from"`
	ValidUntil               *time.Time `gorm:"column:valid_until"`
	AcceptedAt               time.Time  `gorm:"column:accepted_at"`
}

func (*FairLaunchSourceRegistrySnapshot) TableName() string {
	return "fair_launch_source_registry_snapshot"
}

type FairLaunchFeatureWatermark struct {
	Environment                 string    `gorm:"column:environment;primaryKey"`
	Generation                  uint64    `gorm:"column:generation"`
	CapabilityManifestDigest    string    `gorm:"column:capability_manifest_digest"`
	SourceRegistryRevision      uint64    `gorm:"column:source_registry_revision"`
	SourceRegistryDigest        string    `gorm:"column:source_registry_digest"`
	DeploymentAttestationDigest string    `gorm:"column:deployment_attestation_digest"`
	KeySetVersion               uint64    `gorm:"column:key_set_version"`
	MinimumBackendJobGeneration uint64    `gorm:"column:minimum_backend_job_generation"`
	MinimumFrontendGeneration   uint64    `gorm:"column:minimum_frontend_generation"`
	MinimumAdminGeneration      uint64    `gorm:"column:minimum_admin_generation"`
	SchemaGeneration            uint64    `gorm:"column:schema_generation"`
	EventModelGeneration        uint64    `gorm:"column:event_model_generation"`
	AcceptedAt                  time.Time `gorm:"column:accepted_at"`
}

func (*FairLaunchFeatureWatermark) TableName() string { return "fair_launch_feature_watermark" }

type FairLaunchRestoreRecoveryAttestation struct {
	ID                  uint64     `gorm:"column:id;primaryKey"`
	Environment         string     `gorm:"column:environment"`
	WatermarkGeneration uint64     `gorm:"column:watermark_generation"`
	PayloadDigest       string     `gorm:"column:payload_digest"`
	EnvelopeDigest      string     `gorm:"column:envelope_digest"`
	ReconciliationState string     `gorm:"column:reconciliation_state"`
	InitiatorID         uint64     `gorm:"column:initiator_id"`
	ApproverID          uint64     `gorm:"column:approver_id"`
	ExecutorID          uint64     `gorm:"column:executor_id"`
	AcceptedAt          *time.Time `gorm:"column:accepted_at"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
}

func (*FairLaunchRestoreRecoveryAttestation) TableName() string {
	return "fair_launch_restore_recovery_attestation"
}
