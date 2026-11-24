package entity

import "time"

// FairLaunchAuthorityV1 is the immutable parent identity shared by the
// historical LOCAL-ONLY run child and the controlled test-server/BSC mainnet
// activation child. V3 run_row_id columns reference this parent after 0913.
type FairLaunchAuthorityV1 struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	AuthorityKind     string    `gorm:"column:authority_kind"`
	AuthorityIdentity string    `gorm:"column:authority_identity"`
	Environment       string    `gorm:"column:environment"`
	ChainID           uint64    `gorm:"column:chain_id"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (*FairLaunchAuthorityV1) TableName() string { return "fair_launch_authority_v1" }

type FairLaunchMainnetActivationV1 struct {
	AuthorityRowID              uint64    `gorm:"column:authority_row_id;primaryKey"`
	ActivationID                string    `gorm:"column:activation_id"`
	Environment                 string    `gorm:"column:environment"`
	ChainID                     uint64    `gorm:"column:chain_id"`
	RPCGenesisHash              string    `gorm:"column:rpc_genesis_hash"`
	RPCIdentityDigest           string    `gorm:"column:rpc_identity_digest"`
	ProxyAddress                string    `gorm:"column:proxy_address"`
	ProxyRuntimeCodeHash        string    `gorm:"column:proxy_runtime_code_hash"`
	ImplementationAddress       string    `gorm:"column:implementation_address"`
	ImplementationCodeHash      string    `gorm:"column:implementation_code_hash"`
	ABIDigest                   string    `gorm:"column:abi_digest"`
	SourceID                    uint64    `gorm:"column:source_id"`
	SourceRevisionID            uint64    `gorm:"column:source_revision_id"`
	SourceStartBlock            uint64    `gorm:"column:source_start_block"`
	SourceStartBlockHash        string    `gorm:"column:source_start_block_hash"`
	SourceRegistryDigest        string    `gorm:"column:source_registry_digest"`
	PaymentTokenAllowlistDigest string    `gorm:"column:payment_token_allowlist_digest"`
	ProjectAllowlistDigest      string    `gorm:"column:project_allowlist_digest"`
	ReleaseManifestDigest       string    `gorm:"column:release_manifest_digest"`
	MigrationManifestDigest     string    `gorm:"column:migration_manifest_digest"`
	ActivationManifestDigest    string    `gorm:"column:activation_manifest_digest"`
	SigningKeyID                string    `gorm:"column:signing_key_id"`
	SignatureDigest             string    `gorm:"column:signature_digest"`
	GovernanceEvidenceDigest    string    `gorm:"column:governance_evidence_digest"`
	ValidFrom                   time.Time `gorm:"column:valid_from"`
	ValidUntil                  time.Time `gorm:"column:valid_until"`
	CreatedAt                   time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetActivationV1) TableName() string {
	return "fair_launch_mainnet_activation_v1"
}

type FairLaunchMainnetActivationLeaseV1 struct {
	ID                     uint64     `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID        uint64     `gorm:"column:activation_row_id"`
	ChainID                uint64     `gorm:"column:chain_id"`
	LeaseGeneration        uint64     `gorm:"column:lease_generation"`
	FenceEpoch             uint64     `gorm:"column:fence_epoch"`
	LeaseTokenHash         string     `gorm:"column:lease_token_hash"`
	ComponentSetDigest     string     `gorm:"column:component_set_digest"`
	ApprovalEvidenceDigest string     `gorm:"column:approval_evidence_digest"`
	ValidFrom              time.Time  `gorm:"column:valid_from"`
	ValidUntil             time.Time  `gorm:"column:valid_until"`
	Status                 string     `gorm:"column:status"`
	ActiveSlot             *uint8     `gorm:"column:active_slot"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	RevokedAt              *time.Time `gorm:"column:revoked_at"`
}

func (*FairLaunchMainnetActivationLeaseV1) TableName() string {
	return "fair_launch_mainnet_activation_lease_v1"
}

type FairLaunchMainnetActivationHistoryV1 struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID     uint64    `gorm:"column:activation_row_id"`
	ChainID             uint64    `gorm:"column:chain_id"`
	SequenceNo          uint64    `gorm:"column:sequence_no"`
	PreviousState       *string   `gorm:"column:previous_state"`
	State               string    `gorm:"column:state"`
	PreviousEntryDigest *string   `gorm:"column:previous_entry_digest"`
	EntryDigest         string    `gorm:"column:entry_digest"`
	EvidenceDigest      string    `gorm:"column:evidence_digest"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetActivationHistoryV1) TableName() string {
	return "fair_launch_mainnet_activation_history_v1"
}

type FairLaunchMainnetComponentBindingV1 struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID     uint64    `gorm:"column:activation_row_id"`
	ChainID             uint64    `gorm:"column:chain_id"`
	Component           string    `gorm:"column:component"`
	GitCommit           string    `gorm:"column:git_commit"`
	ArtifactSHA256      string    `gorm:"column:artifact_sha256"`
	ArtifactSizeBytes   uint64    `gorm:"column:artifact_size_bytes"`
	BuildManifestDigest string    `gorm:"column:build_manifest_digest"`
	SBOMDigest          string    `gorm:"column:sbom_digest"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetComponentBindingV1) TableName() string {
	return "fair_launch_mainnet_component_binding_v1"
}

type FairLaunchMainnetPaymentTokenV1 struct {
	ID                    uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID       uint64    `gorm:"column:activation_row_id"`
	ChainID               uint64    `gorm:"column:chain_id"`
	TokenSymbol           string    `gorm:"column:token_symbol"`
	TokenAddress          string    `gorm:"column:token_address"`
	TokenDecimals         uint8     `gorm:"column:token_decimals"`
	RuntimeCodeHash       string    `gorm:"column:runtime_code_hash"`
	PolicyDigest          string    `gorm:"column:policy_digest"`
	PolicyVersion         uint32    `gorm:"column:policy_version"`
	MaxActivationValueRaw string    `gorm:"column:max_activation_value_raw"`
	BindingDigest         string    `gorm:"column:binding_digest"`
	CreatedAt             time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetPaymentTokenV1) TableName() string {
	return "fair_launch_mainnet_payment_token_v1"
}

type FairLaunchMainnetProjectAdmissionV1 struct {
	ID                          uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID             uint64    `gorm:"column:activation_row_id"`
	ChainID                     uint64    `gorm:"column:chain_id"`
	ProjectRowID                uint64    `gorm:"column:project_row_id"`
	ProjectID                   uint64    `gorm:"column:project_id"`
	PaymentTokenAddress         string    `gorm:"column:payment_token_address"`
	PaymentTokenDecimals        uint8     `gorm:"column:payment_token_decimals"`
	ProjectTokenAddress         string    `gorm:"column:project_token_address"`
	ProjectTokenDecimals        uint8     `gorm:"column:project_token_decimals"`
	ProjectTokenRuntimeCodeHash string    `gorm:"column:project_token_runtime_code_hash"`
	MaxTotalContributionRaw     string    `gorm:"column:max_total_contribution_raw"`
	MaxWalletContributionRaw    string    `gorm:"column:max_wallet_contribution_raw"`
	AdmissionDigest             string    `gorm:"column:admission_digest"`
	CreatedAt                   time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetProjectAdmissionV1) TableName() string {
	return "fair_launch_mainnet_project_admission_v1"
}

type FairLaunchMainnetWatermarkV1 struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID uint64    `gorm:"column:activation_row_id"`
	ChainID         uint64    `gorm:"column:chain_id"`
	WatermarkKind   string    `gorm:"column:watermark_kind"`
	ProjectID       *uint64   `gorm:"column:project_id"`
	EventIdentityID *uint64   `gorm:"column:event_identity_id"`
	EvidenceDigest  string    `gorm:"column:evidence_digest"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetWatermarkV1) TableName() string {
	return "fair_launch_mainnet_watermark_v1"
}

type FairLaunchMainnetProjectBootstrapReceiptV1 struct {
	ID                    uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID       uint64    `gorm:"column:activation_row_id"`
	ChainID               uint64    `gorm:"column:chain_id"`
	ProjectBindingID      uint64    `gorm:"column:project_binding_id"`
	ProjectID             uint64    `gorm:"column:project_id"`
	EventIdentityID       uint64    `gorm:"column:event_identity_id"`
	PlacementID           uint64    `gorm:"column:placement_id"`
	ObservationID         uint64    `gorm:"column:observation_id"`
	SourceID              uint64    `gorm:"column:source_id"`
	SourceRevisionID      uint64    `gorm:"column:source_revision_id"`
	PlacementGeneration   uint32    `gorm:"column:placement_generation"`
	EffectManifestVersion uint32    `gorm:"column:effect_manifest_version"`
	PayloadDigest         string    `gorm:"column:payload_digest"`
	BindingDigest         string    `gorm:"column:binding_digest"`
	InitialStateDigest    string    `gorm:"column:initial_state_digest"`
	ReceiptDigest         string    `gorm:"column:receipt_digest"`
	CreatedAt             time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetProjectBootstrapReceiptV1) TableName() string {
	return "fair_launch_mainnet_project_bootstrap_receipt_v1"
}

type FairLaunchMainnetArchiveCursorV1 struct {
	ActivationRowID          uint64    `gorm:"column:activation_row_id;primaryKey"`
	ChainID                  uint64    `gorm:"column:chain_id"`
	SourceID                 uint64    `gorm:"column:source_id"`
	SourceRevisionID         uint64    `gorm:"column:source_revision_id"`
	ArchivedThroughBlock     uint64    `gorm:"column:archived_through_block"`
	ArchivedThroughBlockHash string    `gorm:"column:archived_through_block_hash"`
	LeaseGeneration          uint64    `gorm:"column:lease_generation"`
	FenceEpoch               uint64    `gorm:"column:fence_epoch"`
	LeaseTokenHash           string    `gorm:"column:lease_token_hash"`
	Status                   string    `gorm:"column:status"`
	LastErrorCode            *string   `gorm:"column:last_error_code"`
	LastErrorDigest          *string   `gorm:"column:last_error_digest"`
	LastBatchReceiptID       *uint64   `gorm:"column:last_batch_receipt_id"`
	LastRecoveryReceiptID    *uint64   `gorm:"column:last_recovery_receipt_id"`
	CursorGeneration         uint64    `gorm:"column:cursor_generation"`
	CreatedAt                time.Time `gorm:"column:created_at"`
	UpdatedAt                time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchMainnetArchiveCursorV1) TableName() string {
	return "fair_launch_mainnet_archive_cursor_v1"
}

type FairLaunchMainnetArchiveBatchReceiptV1 struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID        uint64    `gorm:"column:activation_row_id"`
	ChainID                uint64    `gorm:"column:chain_id"`
	SourceID               uint64    `gorm:"column:source_id"`
	SourceRevisionID       uint64    `gorm:"column:source_revision_id"`
	LeaseGeneration        uint64    `gorm:"column:lease_generation"`
	FenceEpoch             uint64    `gorm:"column:fence_epoch"`
	FromBlock              uint64    `gorm:"column:from_block"`
	ThroughBlock           uint64    `gorm:"column:through_block"`
	FromParentBlockHash    string    `gorm:"column:from_parent_block_hash"`
	ThroughBlockHash       string    `gorm:"column:through_block_hash"`
	ProviderIdentityDigest string    `gorm:"column:provider_identity_digest"`
	BlockCount             uint32    `gorm:"column:block_count"`
	EventCount             uint32    `gorm:"column:event_count"`
	CoverageDigest         string    `gorm:"column:coverage_digest"`
	PreviousReceiptDigest  *string   `gorm:"column:previous_receipt_digest"`
	ReceiptDigest          string    `gorm:"column:receipt_digest"`
	CreatedAt              time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetArchiveBatchReceiptV1) TableName() string {
	return "fair_launch_mainnet_archive_batch_receipt_v1"
}

type FairLaunchMainnetArchiveBatchEventV1 struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	BatchReceiptID  uint64    `gorm:"column:batch_receipt_id"`
	ActivationRowID uint64    `gorm:"column:activation_row_id"`
	EventOrdinal    uint32    `gorm:"column:event_ordinal"`
	EventIdentityID uint64    `gorm:"column:event_identity_id"`
	PlacementID     uint64    `gorm:"column:placement_id"`
	ObservationID   uint64    `gorm:"column:observation_id"`
	EventName       string    `gorm:"column:event_name"`
	PayloadDigest   string    `gorm:"column:payload_digest"`
	RowDigest       string    `gorm:"column:row_digest"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetArchiveBatchEventV1) TableName() string {
	return "fair_launch_mainnet_archive_batch_event_v1"
}

type FairLaunchMainnetArchiveEventPayloadV1 struct {
	ArchiveEventID   uint64    `gorm:"column:archive_event_id;primaryKey"`
	ActivationRowID  uint64    `gorm:"column:activation_row_id"`
	EventIdentityID  uint64    `gorm:"column:event_identity_id"`
	PlacementID      uint64    `gorm:"column:placement_id"`
	ObservationID    uint64    `gorm:"column:observation_id"`
	TopicCount       uint8     `gorm:"column:topic_count"`
	Topic0           string    `gorm:"column:topic0"`
	Topic1           *string   `gorm:"column:topic1"`
	Topic2           *string   `gorm:"column:topic2"`
	Topic3           *string   `gorm:"column:topic3"`
	DataHex          string    `gorm:"column:data_hex"`
	PayloadRowDigest string    `gorm:"column:payload_row_digest"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetArchiveEventPayloadV1) TableName() string {
	return "fair_launch_mainnet_archive_event_payload_v1"
}

type FairLaunchMainnetMaterializationProfileV1 struct {
	ActivationRowID         uint64    `gorm:"column:activation_row_id;primaryKey"`
	ChainID                 uint64    `gorm:"column:chain_id"`
	SourceID                uint64    `gorm:"column:source_id"`
	SourceRevisionID        uint64    `gorm:"column:source_revision_id"`
	SourceBindingID         uint64    `gorm:"column:source_binding_id"`
	BootstrapReceiptID      uint64    `gorm:"column:bootstrap_receipt_id"`
	BootstrapArchiveEventID uint64    `gorm:"column:bootstrap_archive_event_id"`
	InitialEventIdentityID  uint64    `gorm:"column:initial_event_identity_id"`
	EffectManifestVersion   uint32    `gorm:"column:effect_manifest_version"`
	EffectManifestDigest    string    `gorm:"column:effect_manifest_digest"`
	InitialBlockNumber      uint64    `gorm:"column:initial_block_number"`
	InitialBlockHash        string    `gorm:"column:initial_block_hash"`
	InitialTransactionIndex uint32    `gorm:"column:initial_transaction_index"`
	InitialTxLogOrdinal     uint32    `gorm:"column:initial_tx_log_ordinal"`
	ProfileDigest           string    `gorm:"column:profile_digest"`
	ApprovalEvidenceDigest  string    `gorm:"column:approval_evidence_digest"`
	ValidFrom               time.Time `gorm:"column:valid_from"`
	ValidUntil              time.Time `gorm:"column:valid_until"`
	CreatedAt               time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetMaterializationProfileV1) TableName() string {
	return "fair_launch_mainnet_materialization_profile_v1"
}

type FairLaunchMainnetMaterializationCursorV1 struct {
	ActivationRowID                 uint64    `gorm:"column:activation_row_id;primaryKey"`
	ChainID                         uint64    `gorm:"column:chain_id"`
	SourceID                        uint64    `gorm:"column:source_id"`
	SourceRevisionID                uint64    `gorm:"column:source_revision_id"`
	SourceBindingID                 uint64    `gorm:"column:source_binding_id"`
	MaterializedThroughBlock        uint64    `gorm:"column:materialized_through_block"`
	MaterializedThroughBlockHash    string    `gorm:"column:materialized_through_block_hash"`
	MaterializedThroughTxIndex      uint32    `gorm:"column:materialized_through_tx_index"`
	MaterializedThroughTxLogOrdinal uint32    `gorm:"column:materialized_through_tx_log_ordinal"`
	LastEventIdentityID             *uint64   `gorm:"column:last_event_identity_id"`
	LastArchiveEventID              *uint64   `gorm:"column:last_archive_event_id"`
	LastMaterializationReceiptID    *uint64   `gorm:"column:last_materialization_receipt_id"`
	LeaseGeneration                 uint64    `gorm:"column:lease_generation"`
	FenceEpoch                      uint64    `gorm:"column:fence_epoch"`
	LeaseTokenHash                  string    `gorm:"column:lease_token_hash"`
	Status                          string    `gorm:"column:status"`
	CursorGeneration                uint64    `gorm:"column:cursor_generation"`
	CreatedAt                       time.Time `gorm:"column:created_at"`
	UpdatedAt                       time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchMainnetMaterializationCursorV1) TableName() string {
	return "fair_launch_mainnet_materialization_cursor_v1"
}

type FairLaunchMainnetMaterializationReceiptV1 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID            uint64    `gorm:"column:activation_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	SourceBindingID            uint64    `gorm:"column:source_binding_id"`
	ArchiveBatchReceiptID      uint64    `gorm:"column:archive_batch_receipt_id"`
	ArchiveEventID             uint64    `gorm:"column:archive_event_id"`
	EventIdentityID            uint64    `gorm:"column:event_identity_id"`
	PlacementID                uint64    `gorm:"column:placement_id"`
	ObservationID              uint64    `gorm:"column:observation_id"`
	ApplicationAttemptID       uint64    `gorm:"column:application_attempt_id"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	AttestationID              uint64    `gorm:"column:attestation_id"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	PayloadDigest              string    `gorm:"column:payload_digest"`
	EffectDigest               string    `gorm:"column:effect_digest"`
	LedgerDigest               string    `gorm:"column:ledger_digest"`
	MutationDigest             string    `gorm:"column:mutation_digest"`
	AttestationDigest          string    `gorm:"column:attestation_digest"`
	LeaseGeneration            uint64    `gorm:"column:lease_generation"`
	FenceEpoch                 uint64    `gorm:"column:fence_epoch"`
	PreviousCursorGeneration   uint64    `gorm:"column:previous_cursor_generation"`
	PreviousReceiptDigest      *string   `gorm:"column:previous_receipt_digest"`
	ReceiptDigest              string    `gorm:"column:receipt_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetMaterializationReceiptV1) TableName() string {
	return "fair_launch_mainnet_materialization_receipt_v1"
}

type FairLaunchMainnetArchiveDeadLetterV1 struct {
	ID               uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID  uint64    `gorm:"column:activation_row_id"`
	ChainID          uint64    `gorm:"column:chain_id"`
	SourceID         uint64    `gorm:"column:source_id"`
	SourceRevisionID uint64    `gorm:"column:source_revision_id"`
	BlockNumber      *uint64   `gorm:"column:block_number"`
	BlockHash        *string   `gorm:"column:block_hash"`
	TransactionIndex *uint32   `gorm:"column:transaction_index"`
	TxLogOrdinal     *uint32   `gorm:"column:tx_log_ordinal"`
	RPCLogIndex      *uint32   `gorm:"column:rpc_log_index"`
	TxHash           *string   `gorm:"column:tx_hash"`
	ReasonCode       string    `gorm:"column:reason_code"`
	EvidenceDigest   string    `gorm:"column:evidence_digest"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetArchiveDeadLetterV1) TableName() string {
	return "fair_launch_mainnet_archive_dead_letter_v1"
}

type FairLaunchMainnetArchiveRecoveryReceiptV1 struct {
	ID                            uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID               uint64    `gorm:"column:activation_row_id"`
	ChainID                       uint64    `gorm:"column:chain_id"`
	SourceID                      uint64    `gorm:"column:source_id"`
	SourceRevisionID              uint64    `gorm:"column:source_revision_id"`
	DeadLetterID                  uint64    `gorm:"column:dead_letter_id"`
	PreviousStatus                string    `gorm:"column:previous_status"`
	ReasonCode                    string    `gorm:"column:reason_code"`
	PreviousErrorDigest           string    `gorm:"column:previous_error_digest"`
	ArchivedThroughBlock          uint64    `gorm:"column:archived_through_block"`
	ArchivedThroughBlockHash      string    `gorm:"column:archived_through_block_hash"`
	LastBatchReceiptID            *uint64   `gorm:"column:last_batch_receipt_id"`
	PreviousCursorGeneration      uint64    `gorm:"column:previous_cursor_generation"`
	PreviousLeaseID               uint64    `gorm:"column:previous_lease_id"`
	ReplacementLeaseID            uint64    `gorm:"column:replacement_lease_id"`
	RecoveryAction                string    `gorm:"column:recovery_action"`
	ConfirmationPolicy            string    `gorm:"column:confirmation_policy"`
	VerificationProviderSetDigest string    `gorm:"column:verification_provider_set_digest"`
	VerificationCount             uint32    `gorm:"column:verification_count"`
	CanonicalChainDigest          string    `gorm:"column:canonical_chain_digest"`
	ApprovalEvidenceDigest        string    `gorm:"column:approval_evidence_digest"`
	RecoveryManifestDigest        string    `gorm:"column:recovery_manifest_digest"`
	ReceiptDigest                 string    `gorm:"column:receipt_digest"`
	CreatedAt                     time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetArchiveRecoveryReceiptV1) TableName() string {
	return "fair_launch_mainnet_archive_recovery_receipt_v1"
}

type FairLaunchMainnetRecoveryProviderV1 struct {
	ID                      uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ActivationRowID         uint64    `gorm:"column:activation_row_id"`
	ChainID                 uint64    `gorm:"column:chain_id"`
	SourceID                uint64    `gorm:"column:source_id"`
	SourceRevisionID        uint64    `gorm:"column:source_revision_id"`
	ProviderIdentity        string    `gorm:"column:provider_identity"`
	RPCIdentityDigest       string    `gorm:"column:rpc_identity_digest"`
	ProviderRole            string    `gorm:"column:provider_role"`
	AdmissionManifestDigest string    `gorm:"column:admission_manifest_digest"`
	ApprovalEvidenceDigest  string    `gorm:"column:approval_evidence_digest"`
	ValidFrom               time.Time `gorm:"column:valid_from"`
	ValidUntil              time.Time `gorm:"column:valid_until"`
	CreatedAt               time.Time `gorm:"column:created_at"`
}

func (*FairLaunchMainnetRecoveryProviderV1) TableName() string {
	return "fair_launch_mainnet_recovery_provider_v1"
}
