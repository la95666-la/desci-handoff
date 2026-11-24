package entity

import "time"

// These entities mirror the accepted additive LOCAL-ONLY schema in Deploy
// migrations 0908-0912 at commit 8eef5accc07dcd2540eb7f2d3a469b4e5a7b8431.
// Ordinary production commands do not register them and they confer no runtime
// admission or materialization authority.

type FairLaunchLocalE2ERun struct {
	ID                       uint64     `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunID                    string     `gorm:"column:run_id"`
	Environment              string     `gorm:"column:environment"`
	ExactRPCURL              string     `gorm:"column:exact_rpc_url"`
	RedisEndpoint            string     `gorm:"column:redis_endpoint"`
	ChainID                  uint64     `gorm:"column:chain_id"`
	DatabaseName             string     `gorm:"column:database_name"`
	MySQLServerUUID          string     `gorm:"column:mysql_server_uuid"`
	MySQLVersion             string     `gorm:"column:mysql_version"`
	ContainerID              string     `gorm:"column:container_id"`
	OwnerLabelDigest         string     `gorm:"column:owner_label_digest"`
	SchemaGeneration         uint64     `gorm:"column:schema_generation"`
	BaselineDigest           string     `gorm:"column:baseline_digest"`
	MigrationManifestDigest  string     `gorm:"column:migration_manifest_digest"`
	SchemaDigest             string     `gorm:"column:schema_digest"`
	GrantDigest              string     `gorm:"column:grant_digest"`
	SourceRegistryDigest     string     `gorm:"column:source_registry_digest"`
	HardhatGenesisHash       string     `gorm:"column:hardhat_genesis_hash"`
	ChainManifestDigest      string     `gorm:"column:chain_manifest_digest"`
	DeploymentManifestDigest string     `gorm:"column:deployment_manifest_digest"`
	RuntimeAdmissionDigest   string     `gorm:"column:runtime_admission_digest"`
	CreatedAt                time.Time  `gorm:"column:created_at"`
	ExpiresAt                time.Time  `gorm:"column:expires_at"`
	ClosedAt                 *time.Time `gorm:"column:closed_at"`
	State                    string     `gorm:"column:state"`
}

func (*FairLaunchLocalE2ERun) TableName() string { return "fair_launch_local_e2e_run" }

type FairLaunchLocalE2EObservation struct {
	ID               uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID         uint64    `gorm:"column:run_row_id"`
	ObservationKind  string    `gorm:"column:observation_kind"`
	ObservedValue    string    `gorm:"column:observed_value"`
	ValueDigest      string    `gorm:"column:value_digest"`
	CollectorVersion string    `gorm:"column:collector_version"`
	ObservedAt       time.Time `gorm:"column:observed_at"`
}

func (*FairLaunchLocalE2EObservation) TableName() string {
	return "fair_launch_local_e2e_observation"
}

type FairLaunchSourceRevisionBindingV3 struct {
	ID                        uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                  uint64    `gorm:"column:run_row_id"`
	ChainID                   uint64    `gorm:"column:chain_id"`
	SourceID                  uint64    `gorm:"column:source_id"`
	SourceRevisionID          uint64    `gorm:"column:source_revision_id"`
	FromBlockNumber           uint64    `gorm:"column:from_block_number"`
	FromTransactionIndex      uint32    `gorm:"column:from_transaction_index"`
	FromTxLogOrdinalInclusive uint32    `gorm:"column:from_tx_log_ordinal_inclusive"`
	ToBlockNumber             *uint64   `gorm:"column:to_block_number"`
	ToTransactionIndex        *uint32   `gorm:"column:to_transaction_index"`
	ToTxLogOrdinalExclusive   *uint32   `gorm:"column:to_tx_log_ordinal_exclusive"`
	SourceBindingDigest       string    `gorm:"column:source_binding_digest"`
	CreatedAt                 time.Time `gorm:"column:created_at"`
}

func (*FairLaunchSourceRevisionBindingV3) TableName() string {
	return "fair_launch_source_revision_binding_v3"
}

type FairLaunchProjectBindingV3 struct {
	ID                   uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID             uint64    `gorm:"column:run_row_id"`
	ProjectRowID         uint64    `gorm:"column:project_row_id"`
	ChainID              uint64    `gorm:"column:chain_id"`
	ContractAddress      string    `gorm:"column:contract_address"`
	ContractVersion      string    `gorm:"column:contract_version"`
	ProjectID            uint64    `gorm:"column:project_id"`
	ProjectIDRaw         string    `gorm:"column:project_id_raw"`
	SourceBindingID      uint64    `gorm:"column:source_binding_id"`
	SourceID             uint64    `gorm:"column:source_id"`
	SourceRevisionID     uint64    `gorm:"column:source_revision_id"`
	ProjectTokenAddress  string    `gorm:"column:project_token_address"`
	ProjectTokenDecimals uint8     `gorm:"column:project_token_decimals"`
	PaymentTokenAddress  string    `gorm:"column:payment_token_address"`
	PaymentTokenDecimals uint8     `gorm:"column:payment_token_decimals"`
	BindingDigest        string    `gorm:"column:binding_digest"`
	CreatedAt            time.Time `gorm:"column:created_at"`
}

func (*FairLaunchProjectBindingV3) TableName() string { return "fair_launch_project_binding_v3" }

type FairLaunchProjectStateV3 struct {
	ID                           uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ProjectBindingID             uint64    `gorm:"column:project_binding_id"`
	RunRowID                     uint64    `gorm:"column:run_row_id"`
	ChainID                      uint64    `gorm:"column:chain_id"`
	SourceID                     uint64    `gorm:"column:source_id"`
	ContractAddress              string    `gorm:"column:contract_address"`
	ContractVersion              string    `gorm:"column:contract_version"`
	ProjectID                    uint64    `gorm:"column:project_id"`
	ProjectTokenAddress          string    `gorm:"column:project_token_address"`
	PaymentTokenAddress          string    `gorm:"column:payment_token_address"`
	Status                       string    `gorm:"column:status"`
	TerminalReason               string    `gorm:"column:terminal_reason"`
	SaleTokenAmountRaw           string    `gorm:"column:sale_token_amount_raw"`
	TotalRaisedRaw               string    `gorm:"column:total_raised_raw"`
	TotalTokensDepositedRaw      string    `gorm:"column:total_tokens_deposited_raw"`
	TotalTokensClaimedRaw        string    `gorm:"column:total_tokens_claimed_raw"`
	ParticipantCountRaw          string    `gorm:"column:participant_count_raw"`
	FinalTotalRaisedRaw          *string   `gorm:"column:final_total_raised_raw"`
	FinalPriceNumeratorRaw       *string   `gorm:"column:final_price_numerator_raw"`
	FinalPriceDenominatorRaw     *string   `gorm:"column:final_price_denominator_raw"`
	PlatformFeeReservedRaw       string    `gorm:"column:platform_fee_reserved_raw"`
	ProjectProceedsPaidRaw       string    `gorm:"column:project_proceeds_paid_raw"`
	PlatformFeePaidRaw           string    `gorm:"column:platform_fee_paid_raw"`
	SettlementBlockNumber        *uint64   `gorm:"column:settlement_block_number"`
	SettlementBlockHash          *string   `gorm:"column:settlement_block_hash"`
	SettlementAnchorBlockNumber  *uint64   `gorm:"column:settlement_anchor_block_number"`
	SettlementAnchorBlockHash    *string   `gorm:"column:settlement_anchor_block_hash"`
	SettlementTxHash             *string   `gorm:"column:settlement_tx_hash"`
	SettlementTxLogOrdinal       *uint32   `gorm:"column:settlement_tx_log_ordinal"`
	ReleaseManifestDigest        *string   `gorm:"column:release_manifest_digest"`
	ReleasePolicyDigest          *string   `gorm:"column:release_policy_digest"`
	ReleasePolicyVersion         *uint32   `gorm:"column:release_policy_version"`
	ProjectProceedsWithdrawn     bool      `gorm:"column:project_proceeds_withdrawn"`
	PlatformFeeWithdrawn         bool      `gorm:"column:platform_fee_withdrawn"`
	FailedProjectTokensWithdrawn bool      `gorm:"column:failed_project_tokens_withdrawn"`
	Tombstoned                   bool      `gorm:"column:tombstoned"`
	StateGeneration              uint64    `gorm:"column:state_generation"`
	LastEventIdentityID          *uint64   `gorm:"column:last_event_identity_id"`
	LastPlacementID              *uint64   `gorm:"column:last_placement_id"`
	LastSourceRevisionID         *uint64   `gorm:"column:last_source_revision_id"`
	LastEffectManifestVersion    *uint32   `gorm:"column:last_effect_manifest_version"`
	LastApplicationAttemptID     *uint64   `gorm:"column:last_application_attempt_id"`
	SourceApplicationLineageID   *uint64   `gorm:"column:source_application_lineage_id"`
	BootstrapReceiptID           *uint64   `gorm:"column:bootstrap_receipt_id"`
	StateDigest                  string    `gorm:"column:state_digest"`
	CreatedAt                    time.Time `gorm:"column:created_at"`
	UpdatedAt                    time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchProjectStateV3) TableName() string { return "fair_launch_project_state_v3" }

type FairLaunchPositionV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	ProjectBindingID           uint64    `gorm:"column:project_binding_id"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	ContractAddress            string    `gorm:"column:contract_address"`
	ContractVersion            string    `gorm:"column:contract_version"`
	ProjectID                  uint64    `gorm:"column:project_id"`
	WalletAddress              string    `gorm:"column:wallet_address"`
	PaymentTokenAddress        string    `gorm:"column:payment_token_address"`
	PaymentTokenDecimals       uint8     `gorm:"column:payment_token_decimals"`
	ContributionAmountRaw      string    `gorm:"column:contribution_amount_raw"`
	ContributionAmountDisplay  string    `gorm:"column:contribution_amount_display"`
	ProjectTokenAddress        string    `gorm:"column:project_token_address"`
	ProjectTokenDecimals       uint8     `gorm:"column:project_token_decimals"`
	ClaimedTokensRaw           string    `gorm:"column:claimed_tokens_raw"`
	ClaimedTokensDisplay       string    `gorm:"column:claimed_tokens_display"`
	Refunded                   bool      `gorm:"column:refunded"`
	FirstContributionEventID   *uint64   `gorm:"column:first_contribution_event_id"`
	LastContributionEventID    *uint64   `gorm:"column:last_contribution_event_id"`
	LastClaimEventID           *uint64   `gorm:"column:last_claim_event_id"`
	LastRefundEventID          *uint64   `gorm:"column:last_refund_event_id"`
	LastEventIdentityID        *uint64   `gorm:"column:last_event_identity_id"`
	LastPlacementID            *uint64   `gorm:"column:last_placement_id"`
	LastSourceRevisionID       *uint64   `gorm:"column:last_source_revision_id"`
	LastEffectManifestVersion  *uint32   `gorm:"column:last_effect_manifest_version"`
	LastApplicationAttemptID   *uint64   `gorm:"column:last_application_attempt_id"`
	SourceApplicationLineageID *uint64   `gorm:"column:source_application_lineage_id"`
	StateGeneration            uint64    `gorm:"column:state_generation"`
	StateDigest                string    `gorm:"column:state_digest"`
	Tombstoned                 bool      `gorm:"column:tombstoned"`
	Status                     string    `gorm:"column:status"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
	UpdatedAt                  time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchPositionV3) TableName() string { return "fair_launch_position_v3" }

type FairLaunchGovernanceCurrentV3 struct {
	SourceID                   uint64    `gorm:"column:source_id;primaryKey"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	LatestSourceRevisionID     uint64    `gorm:"column:latest_source_revision_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	ContractAddress            string    `gorm:"column:contract_address"`
	OperatorAddress            string    `gorm:"column:operator_address"`
	PauseGuardianAddress       string    `gorm:"column:pause_guardian_address"`
	UnpauseTimelockAddress     string    `gorm:"column:unpause_timelock_address"`
	TreasuryAddress            string    `gorm:"column:treasury_address"`
	PlatformFeeReceiver        string    `gorm:"column:platform_fee_receiver"`
	PlatformFeeBPS             uint16    `gorm:"column:platform_fee_bps"`
	ContributionsPaused        bool      `gorm:"column:contributions_paused"`
	PayoutsPaused              bool      `gorm:"column:payouts_paused"`
	ClaimsPauseUntil           uint64    `gorm:"column:claims_pause_until"`
	StateGeneration            uint64    `gorm:"column:state_generation"`
	LastEventIdentityID        *uint64   `gorm:"column:last_event_identity_id"`
	LastPlacementID            *uint64   `gorm:"column:last_placement_id"`
	LastSourceRevisionID       *uint64   `gorm:"column:last_source_revision_id"`
	LastEffectManifestVersion  *uint32   `gorm:"column:last_effect_manifest_version"`
	LastApplicationAttemptID   *uint64   `gorm:"column:last_application_attempt_id"`
	SourceApplicationLineageID *uint64   `gorm:"column:source_application_lineage_id"`
	StateDigest                string    `gorm:"column:state_digest"`
	UpdatedAt                  time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchGovernanceCurrentV3) TableName() string {
	return "fair_launch_governance_current_v3"
}

type FairLaunchGovernanceHistoryV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	StateGeneration            uint64    `gorm:"column:state_generation"`
	BeforePayloadJCS           string    `gorm:"column:before_payload_jcs"`
	BeforeDigest               string    `gorm:"column:before_digest"`
	AfterPayloadJCS            string    `gorm:"column:after_payload_jcs"`
	AfterDigest                string    `gorm:"column:after_digest"`
	SourceEventIdentityID      uint64    `gorm:"column:source_event_identity_id"`
	SourcePlacementID          uint64    `gorm:"column:source_placement_id"`
	SourceApplicationAttemptID uint64    `gorm:"column:source_application_attempt_id"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchGovernanceHistoryV3) TableName() string {
	return "fair_launch_governance_history_v3"
}

type FairLaunchPolicyCurrentV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	LatestSourceRevisionID     uint64    `gorm:"column:latest_source_revision_id"`
	PolicyKind                 string    `gorm:"column:policy_kind"`
	SubjectAddress             string    `gorm:"column:subject_address"`
	PolicyVersion              uint32    `gorm:"column:policy_version"`
	Enabled                    bool      `gorm:"column:enabled"`
	EmergencyBlocked           bool      `gorm:"column:emergency_blocked"`
	TokenDecimals              *uint8    `gorm:"column:token_decimals"`
	RuntimeCodeHash            *string   `gorm:"column:runtime_code_hash"`
	PolicyDigest               string    `gorm:"column:policy_digest"`
	AttestationDigest          *string   `gorm:"column:attestation_digest"`
	StateGeneration            uint64    `gorm:"column:state_generation"`
	LastEventIdentityID        uint64    `gorm:"column:last_event_identity_id"`
	LastPlacementID            uint64    `gorm:"column:last_placement_id"`
	LastSourceRevisionID       uint64    `gorm:"column:last_source_revision_id"`
	LastEffectManifestVersion  uint32    `gorm:"column:last_effect_manifest_version"`
	LastApplicationAttemptID   uint64    `gorm:"column:last_application_attempt_id"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	StateDigest                string    `gorm:"column:state_digest"`
	UpdatedAt                  time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchPolicyCurrentV3) TableName() string { return "fair_launch_policy_current_v3" }

type FairLaunchPolicyHistoryV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	PolicyCurrentID            uint64    `gorm:"column:policy_current_id"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	StateGeneration            uint64    `gorm:"column:state_generation"`
	BeforePayloadJCS           string    `gorm:"column:before_payload_jcs"`
	BeforeDigest               string    `gorm:"column:before_digest"`
	AfterPayloadJCS            string    `gorm:"column:after_payload_jcs"`
	AfterDigest                string    `gorm:"column:after_digest"`
	SourceEventIdentityID      uint64    `gorm:"column:source_event_identity_id"`
	SourcePlacementID          uint64    `gorm:"column:source_placement_id"`
	SourceApplicationAttemptID uint64    `gorm:"column:source_application_attempt_id"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchPolicyHistoryV3) TableName() string { return "fair_launch_policy_history_v3" }

type FairLaunchApplicationLineageV3 struct {
	ID                     uint64     `gorm:"column:id;primaryKey;autoIncrement:true"`
	CEApplicationAttemptID uint64     `gorm:"column:ce_application_attempt_id"`
	RunRowID               uint64     `gorm:"column:run_row_id"`
	ChainID                uint64     `gorm:"column:chain_id"`
	SourceBindingID        uint64     `gorm:"column:source_binding_id"`
	SourceID               uint64     `gorm:"column:source_id"`
	EventIdentityID        uint64     `gorm:"column:event_identity_id"`
	PlacementID            uint64     `gorm:"column:placement_id"`
	SourceRevisionID       uint64     `gorm:"column:source_revision_id"`
	EffectManifestVersion  uint32     `gorm:"column:effect_manifest_version"`
	PlacementGeneration    uint32     `gorm:"column:placement_generation"`
	PredecessorLineageID   *uint64    `gorm:"column:predecessor_lineage_id"`
	ReversesLineageID      *uint64    `gorm:"column:reverses_lineage_id"`
	Disposition            string     `gorm:"column:disposition"`
	ExpectedEffect         uint8      `gorm:"column:expected_effect"`
	PayloadDigest          string     `gorm:"column:payload_digest"`
	EffectDigest           string     `gorm:"column:effect_digest"`
	LedgerDigest           string     `gorm:"column:ledger_digest"`
	MutationDigest         string     `gorm:"column:mutation_digest"`
	State                  string     `gorm:"column:state"`
	AppliedAt              *time.Time `gorm:"column:applied_at"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
}

func (*FairLaunchApplicationLineageV3) TableName() string {
	return "fair_launch_application_lineage_v3"
}

type FairLaunchReversalSerializationGuardV3 struct {
	ReversedLineageID uint64     `gorm:"column:reversed_lineage_id;primaryKey"`
	ReversalLineageID *uint64    `gorm:"column:reversal_lineage_id"`
	RunRowID          uint64     `gorm:"column:run_row_id"`
	ChainID           uint64     `gorm:"column:chain_id"`
	SourceBindingID   uint64     `gorm:"column:source_binding_id"`
	SourceID          uint64     `gorm:"column:source_id"`
	SourceRevisionID  uint64     `gorm:"column:source_revision_id"`
	State             string     `gorm:"column:state"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	BoundAt           *time.Time `gorm:"column:bound_at"`
	ClosedAt          *time.Time `gorm:"column:closed_at"`
}

func (*FairLaunchReversalSerializationGuardV3) TableName() string {
	return "fair_launch_reversal_serialization_guard_v3"
}

type FairLaunchEffectGuardV3 struct {
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	EventIdentityID            uint64    `gorm:"column:event_identity_id;primaryKey"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version;primaryKey"`
	PlacementGeneration        uint32    `gorm:"column:placement_generation;primaryKey"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	State                      string    `gorm:"column:state"`
	ActiveSlot                 *uint8    `gorm:"column:active_slot"`
	PayloadDigest              string    `gorm:"column:payload_digest"`
	EffectDigest               string    `gorm:"column:effect_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchEffectGuardV3) TableName() string { return "fair_launch_effect_guard_v3" }

type FairLaunchEffectJournalV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	EventIdentityID            uint64    `gorm:"column:event_identity_id"`
	PlacementID                uint64    `gorm:"column:placement_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	EffectKind                 string    `gorm:"column:effect_kind"`
	AggregateKind              string    `gorm:"column:aggregate_kind"`
	AggregateKey               string    `gorm:"column:aggregate_key"`
	MutationOrdinal            uint32    `gorm:"column:mutation_ordinal"`
	BeforePayloadJCS           string    `gorm:"column:before_payload_jcs"`
	BeforeDigest               string    `gorm:"column:before_digest"`
	AfterPayloadJCS            string    `gorm:"column:after_payload_jcs"`
	AfterDigest                string    `gorm:"column:after_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchEffectJournalV3) TableName() string { return "fair_launch_effect_journal_v3" }

type FairLaunchLineageReversalV3 struct {
	ReversedLineageID uint64    `gorm:"column:reversed_lineage_id;primaryKey"`
	ReversalLineageID uint64    `gorm:"column:reversal_lineage_id"`
	ReversedAt        time.Time `gorm:"column:reversed_at"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (*FairLaunchLineageReversalV3) TableName() string { return "fair_launch_lineage_reversal_v3" }

type FairLaunchJournalReversalV3 struct {
	SourceJournalID   uint64    `gorm:"column:source_journal_id;primaryKey"`
	ReversedLineageID uint64    `gorm:"column:reversed_lineage_id"`
	ReversalLineageID uint64    `gorm:"column:reversal_lineage_id"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (*FairLaunchJournalReversalV3) TableName() string { return "fair_launch_journal_reversal_v3" }

type FairLaunchReversalEvidenceItemV3 struct {
	ReversedLineageID   uint64    `gorm:"column:reversed_lineage_id;primaryKey"`
	ReversalLineageID   uint64    `gorm:"column:reversal_lineage_id"`
	EvidenceOrdinal     uint64    `gorm:"column:evidence_ordinal;primaryKey"`
	EvidenceKind        string    `gorm:"column:evidence_kind"`
	EvidenceID          uint64    `gorm:"column:evidence_id"`
	EvidenceDigest      string    `gorm:"column:evidence_digest"`
	PreviousChainDigest string    `gorm:"column:previous_chain_digest"`
	ItemChainDigest     string    `gorm:"column:item_chain_digest"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

func (*FairLaunchReversalEvidenceItemV3) TableName() string {
	return "fair_launch_reversal_evidence_item_v3"
}

type FairLaunchReversalEvidenceManifestV3 struct {
	ReversedLineageID           uint64    `gorm:"column:reversed_lineage_id;primaryKey"`
	ReversalLineageID           uint64    `gorm:"column:reversal_lineage_id"`
	SourceJournalCount          uint64    `gorm:"column:source_journal_count"`
	DependencyCount             uint64    `gorm:"column:dependency_count"`
	AttestationCount            uint64    `gorm:"column:attestation_count"`
	CursorCheckpointCount       uint64    `gorm:"column:cursor_checkpoint_count"`
	GovernanceHistoryCount      uint64    `gorm:"column:governance_history_count"`
	PolicyHistoryCount          uint64    `gorm:"column:policy_history_count"`
	SourceLedgerCount           uint64    `gorm:"column:source_ledger_count"`
	SourceGovernanceLedgerCount uint64    `gorm:"column:source_governance_ledger_count"`
	EvidenceItemCount           uint64    `gorm:"column:evidence_item_count"`
	FinalItemChainDigest        string    `gorm:"column:final_item_chain_digest"`
	ManifestDigest              string    `gorm:"column:manifest_digest"`
	CreatedAt                   time.Time `gorm:"column:created_at"`
}

func (*FairLaunchReversalEvidenceManifestV3) TableName() string {
	return "fair_launch_reversal_evidence_manifest_v3"
}

type FairLaunchReversalClosureV3 struct {
	ReversedLineageID                 uint64    `gorm:"column:reversed_lineage_id;primaryKey"`
	ReversalLineageID                 uint64    `gorm:"column:reversal_lineage_id"`
	EvidenceManifestDigest            string    `gorm:"column:evidence_manifest_digest"`
	SourceJournalCount                uint64    `gorm:"column:source_journal_count"`
	LinkedJournalCount                uint64    `gorm:"column:linked_journal_count"`
	DependencyCount                   uint64    `gorm:"column:dependency_count"`
	AttestationCount                  uint64    `gorm:"column:attestation_count"`
	CursorCheckpointCount             uint64    `gorm:"column:cursor_checkpoint_count"`
	GovernanceHistoryCount            uint64    `gorm:"column:governance_history_count"`
	PolicyHistoryCount                uint64    `gorm:"column:policy_history_count"`
	SourceLedgerCount                 uint64    `gorm:"column:source_ledger_count"`
	CompensationLedgerCount           uint64    `gorm:"column:compensation_ledger_count"`
	SourceGovernanceLedgerCount       uint64    `gorm:"column:source_governance_ledger_count"`
	CompensationGovernanceLedgerCount uint64    `gorm:"column:compensation_governance_ledger_count"`
	ClosureDigest                     string    `gorm:"column:closure_digest"`
	CreatedAt                         time.Time `gorm:"column:created_at"`
}

func (*FairLaunchReversalClosureV3) TableName() string { return "fair_launch_reversal_closure_v3" }

type FairLaunchEffectDependencyV3 struct {
	ID                    uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	PredecessorLineageID  uint64    `gorm:"column:predecessor_lineage_id"`
	DependentLineageID    uint64    `gorm:"column:dependent_lineage_id"`
	RunRowID              uint64    `gorm:"column:run_row_id"`
	ChainID               uint64    `gorm:"column:chain_id"`
	SourceBindingID       uint64    `gorm:"column:source_binding_id"`
	SourceID              uint64    `gorm:"column:source_id"`
	SourceRevisionID      uint64    `gorm:"column:source_revision_id"`
	EffectManifestVersion uint32    `gorm:"column:effect_manifest_version"`
	AggregateKind         string    `gorm:"column:aggregate_kind"`
	AggregateKey          string    `gorm:"column:aggregate_key"`
	DependencyDigest      string    `gorm:"column:dependency_digest"`
	CreatedAt             time.Time `gorm:"column:created_at"`
}

func (*FairLaunchEffectDependencyV3) TableName() string { return "fair_launch_effect_dependency_v3" }

type FairLaunchEffectAttestationV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceBindingID            uint64    `gorm:"column:source_binding_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	EventIdentityID            uint64    `gorm:"column:event_identity_id"`
	PlacementID                uint64    `gorm:"column:placement_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	PayloadDigest              string    `gorm:"column:payload_digest"`
	EffectDigest               string    `gorm:"column:effect_digest"`
	LedgerDigest               string    `gorm:"column:ledger_digest"`
	MutationDigest             string    `gorm:"column:mutation_digest"`
	LedgerRowCount             uint64    `gorm:"column:ledger_row_count"`
	MutationRowCount           uint64    `gorm:"column:mutation_row_count"`
	ProjectMutationCount       uint64    `gorm:"column:project_mutation_count"`
	PositionMutationCount      uint64    `gorm:"column:position_mutation_count"`
	GovernanceMutationCount    uint64    `gorm:"column:governance_mutation_count"`
	PolicyMutationCount        uint64    `gorm:"column:policy_mutation_count"`
	AttestationDigest          string    `gorm:"column:attestation_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchEffectAttestationV3) TableName() string {
	return "fair_launch_effect_attestation_v3"
}

type FairLaunchCursorCheckpointV3 struct {
	ID                           uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                     uint64    `gorm:"column:run_row_id"`
	ChainID                      uint64    `gorm:"column:chain_id"`
	SourceBindingID              uint64    `gorm:"column:source_binding_id"`
	SourceID                     uint64    `gorm:"column:source_id"`
	SourceRevisionID             uint64    `gorm:"column:source_revision_id"`
	PredecessorBlockNumber       uint64    `gorm:"column:predecessor_block_number"`
	PredecessorBlockHash         string    `gorm:"column:predecessor_block_hash"`
	PredecessorTxIndex           uint32    `gorm:"column:predecessor_tx_index"`
	PredecessorTxLogOrdinal      uint32    `gorm:"column:predecessor_tx_log_ordinal"`
	PredecessorEventIdentityID   *uint64   `gorm:"column:predecessor_event_identity_id"`
	Reason                       string    `gorm:"column:reason"`
	ReversalApplicationLineageID uint64    `gorm:"column:reversal_application_lineage_id"`
	CheckpointDigest             string    `gorm:"column:checkpoint_digest"`
	CreatedAt                    time.Time `gorm:"column:created_at"`
}

func (*FairLaunchCursorCheckpointV3) TableName() string { return "fair_launch_cursor_checkpoint_v3" }

type FairLaunchLedgerEntryV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	ProjectBindingID           uint64    `gorm:"column:project_binding_id"`
	ProjectID                  uint64    `gorm:"column:project_id"`
	ContractAddress            string    `gorm:"column:contract_address"`
	ContractVersion            string    `gorm:"column:contract_version"`
	WalletAddress              string    `gorm:"column:wallet_address"`
	CounterpartyAddress        string    `gorm:"column:counterparty_address"`
	EntryType                  string    `gorm:"column:entry_type"`
	AssetRole                  string    `gorm:"column:asset_role"`
	FlowDirection              string    `gorm:"column:flow_direction"`
	TokenAddress               string    `gorm:"column:token_address"`
	TokenDecimals              uint8     `gorm:"column:token_decimals"`
	AmountRaw                  string    `gorm:"column:amount_raw"`
	AmountDisplay              string    `gorm:"column:amount_display"`
	SourceEventIdentityID      uint64    `gorm:"column:source_event_identity_id"`
	SourcePlacementID          uint64    `gorm:"column:source_placement_id"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	EntryOrdinal               uint32    `gorm:"column:entry_ordinal"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	ReversalOfLedgerID         *uint64   `gorm:"column:reversal_of_ledger_id"`
	CompensationGeneration     *uint32   `gorm:"column:compensation_generation"`
	RowDigest                  string    `gorm:"column:row_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchLedgerEntryV3) TableName() string { return "fair_launch_ledger_entry_v3" }

type FairLaunchSourceTokenBindingV3 struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID            uint64    `gorm:"column:run_row_id"`
	ChainID             uint64    `gorm:"column:chain_id"`
	SourceID            uint64    `gorm:"column:source_id"`
	ContractAddress     string    `gorm:"column:contract_address"`
	TokenAddress        string    `gorm:"column:token_address"`
	TokenDecimals       uint8     `gorm:"column:token_decimals"`
	MetadataVersion     uint32    `gorm:"column:metadata_version"`
	AdmissionGeneration uint64    `gorm:"column:admission_generation"`
	RuntimeCodeHash     string    `gorm:"column:runtime_code_hash"`
	MetadataDigest      string    `gorm:"column:metadata_digest"`
	AdmittedAt          time.Time `gorm:"column:admitted_at"`
}

func (*FairLaunchSourceTokenBindingV3) TableName() string {
	return "fair_launch_source_token_binding_v3"
}

type FairLaunchGovernanceLedgerEntryV3 struct {
	ID                         uint64    `gorm:"column:id;primaryKey;autoIncrement:true"`
	RunRowID                   uint64    `gorm:"column:run_row_id"`
	ChainID                    uint64    `gorm:"column:chain_id"`
	SourceID                   uint64    `gorm:"column:source_id"`
	ContractAddress            string    `gorm:"column:contract_address"`
	TokenBindingID             uint64    `gorm:"column:token_binding_id"`
	EntryType                  string    `gorm:"column:entry_type"`
	AssetRole                  string    `gorm:"column:asset_role"`
	FlowDirection              string    `gorm:"column:flow_direction"`
	TokenAddress               string    `gorm:"column:token_address"`
	ReceiverAddress            string    `gorm:"column:receiver_address"`
	TokenDecimals              uint8     `gorm:"column:token_decimals"`
	AmountRaw                  string    `gorm:"column:amount_raw"`
	AmountDisplay              string    `gorm:"column:amount_display"`
	SourceApplicationLineageID uint64    `gorm:"column:source_application_lineage_id"`
	SourceEventIdentityID      uint64    `gorm:"column:source_event_identity_id"`
	SourcePlacementID          uint64    `gorm:"column:source_placement_id"`
	SourceRevisionID           uint64    `gorm:"column:source_revision_id"`
	EntryOrdinal               uint32    `gorm:"column:entry_ordinal"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	ReversalOfLedgerID         *uint64   `gorm:"column:reversal_of_ledger_id"`
	CompensationGeneration     *uint32   `gorm:"column:compensation_generation"`
	RowDigest                  string    `gorm:"column:row_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FairLaunchGovernanceLedgerEntryV3) TableName() string {
	return "fair_launch_governance_ledger_entry_v3"
}
