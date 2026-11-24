package entity

import "time"

// FairLaunchPosition is an event-derived wallet projection. It is not an
// entitlement at contribution time; final allocation remains chain-derived.
type FairLaunchPosition struct {
	ID                        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ChainID                   uint64    `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress           string    `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion           string    `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	ProjectID                 uint64    `gorm:"column:project_id;not null" json:"project_id"`
	WalletAddress             string    `gorm:"column:wallet_address;size:42;not null" json:"wallet_address"`
	PaymentTokenAddress       string    `gorm:"column:payment_token_address;size:42;not null" json:"payment_token_address"`
	PaymentTokenDecimals      uint8     `gorm:"column:payment_token_decimals;not null" json:"payment_token_decimals"`
	ContributionAmountRaw     string    `gorm:"column:contribution_amount_raw;size:78;not null;default:'0'" json:"contribution_amount_raw"`
	ContributionAmountDisplay string    `gorm:"column:contribution_amount_display;size:160;not null;default:'0'" json:"contribution_amount_display"`
	ProjectTokenAddress       string    `gorm:"column:project_token_address;size:42;not null" json:"project_token_address"`
	ProjectTokenDecimals      uint8     `gorm:"column:project_token_decimals;not null" json:"project_token_decimals"`
	ClaimedTokensRaw          string    `gorm:"column:claimed_tokens_raw;size:78;not null;default:'0'" json:"claimed_tokens_raw"`
	ClaimedTokensDisplay      string    `gorm:"column:claimed_tokens_display;size:160;not null;default:'0'" json:"claimed_tokens_display"`
	Refunded                  bool      `gorm:"column:refunded;not null;default:0" json:"refunded"`
	FirstContributionEventID  *uint64   `gorm:"column:first_contribution_event_id" json:"first_contribution_event_id,omitempty"`
	LastContributionEventID   *uint64   `gorm:"column:last_contribution_event_id" json:"last_contribution_event_id,omitempty"`
	LastClaimEventID          *uint64   `gorm:"column:last_claim_event_id" json:"last_claim_event_id,omitempty"`
	LastRefundEventID         *uint64   `gorm:"column:last_refund_event_id" json:"last_refund_event_id,omitempty"`
	Status                    string    `gorm:"column:status;size:20;not null;default:canonical" json:"status"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP(6)" json:"created_at"`
	UpdatedAt                 time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP(6);autoUpdateTime" json:"updated_at"`
}

func (*FairLaunchPosition) TableName() string { return "fair_launch_position" }

// ChainContractSource is the materialized source identity. Signed revision
// envelopes remain the authorization source; database rows are never trusted
// merely because they exist.
type ChainContractSource struct {
	ID              uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ChainID         uint64     `gorm:"column:chain_id;not null" json:"chain_id"`
	ContractAddress string     `gorm:"column:contract_address;size:42;not null" json:"contract_address"`
	ContractVersion string     `gorm:"column:contract_version;size:32;not null" json:"contract_version"`
	FundingMode     string     `gorm:"column:funding_mode;size:32;not null" json:"funding_mode"`
	RegisteredAt    time.Time  `gorm:"column:registered_at;not null" json:"registered_at"`
	RetiredAt       *time.Time `gorm:"column:retired_at" json:"retired_at,omitempty"`
	StartBlock      uint64     `gorm:"column:start_block;not null" json:"start_block"`
	Enabled         bool       `gorm:"column:enabled;not null" json:"enabled"`
}

func (*ChainContractSource) TableName() string { return "chain_contract_source" }

type ChainContractSourceRevision struct {
	ID                        uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	SourceID                  uint64    `gorm:"column:source_id;not null" json:"source_id"`
	RevisionNo                uint32    `gorm:"column:revision_no;not null" json:"revision_no"`
	FromBlockNumber           uint64    `gorm:"column:from_block_number;not null" json:"from_block_number"`
	FromTransactionIndex      uint32    `gorm:"column:from_transaction_index;not null" json:"from_transaction_index"`
	FromTxLogOrdinalInclusive uint32    `gorm:"column:from_tx_log_ordinal_inclusive;not null" json:"from_tx_log_ordinal_inclusive"`
	ToBlockNumber             *uint64   `gorm:"column:to_block_number" json:"to_block_number,omitempty"`
	ToTransactionIndex        *uint32   `gorm:"column:to_transaction_index" json:"to_transaction_index,omitempty"`
	ToTxLogOrdinalExclusive   *uint32   `gorm:"column:to_tx_log_ordinal_exclusive" json:"to_tx_log_ordinal_exclusive,omitempty"`
	ActivationBlockHash       string    `gorm:"column:activation_block_hash;size:66;not null" json:"activation_block_hash"`
	ProxyKind                 string    `gorm:"column:proxy_kind;size:32;not null" json:"proxy_kind"`
	ImplementationAddress     string    `gorm:"column:implementation_address;size:42;not null" json:"implementation_address"`
	ImplementationCodeHash    string    `gorm:"column:implementation_code_hash;size:64;not null" json:"implementation_code_hash"`
	DependencyManifestDigest  string    `gorm:"column:dependency_manifest_digest;size:64;not null" json:"dependency_manifest_digest"`
	ABIVersion                string    `gorm:"column:abi_version;size:64;not null" json:"abi_version"`
	ABIDigest                 string    `gorm:"column:abi_digest;size:64;not null" json:"abi_digest"`
	ConfirmationPolicy        string    `gorm:"column:confirmation_policy;size:32;not null" json:"confirmation_policy"`
	GovernanceTxHash          string    `gorm:"column:governance_tx_hash;size:66;not null" json:"governance_tx_hash"`
	GovernanceOperationID     string    `gorm:"column:governance_operation_id;size:66;not null" json:"governance_operation_id"`
	ActivationEventIdentityID *uint64   `gorm:"column:activation_event_identity_id" json:"activation_event_identity_id,omitempty"`
	PredecessorRevisionID     *uint64   `gorm:"column:predecessor_revision_id" json:"predecessor_revision_id,omitempty"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*ChainContractSourceRevision) TableName() string { return "chain_contract_source_revision" }
