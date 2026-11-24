package entity

import "time"

type FairLaunchSigningGeneration struct {
	ID                      uint64     `gorm:"column:id;primaryKey"`
	SubmissionID            uint64     `gorm:"column:submission_id"`
	Generation              uint32     `gorm:"column:generation"`
	ChainID                 uint64     `gorm:"column:chain_id"`
	ExpectedSignerAddress   string     `gorm:"column:expected_signer_address"`
	Nonce                   uint64     `gorm:"column:nonce"`
	SigningDigest           string     `gorm:"column:signing_digest"`
	UnsignedTxDigest        string     `gorm:"column:unsigned_tx_digest"`
	RawTx                   []byte     `gorm:"column:raw_tx"`
	RawTxDigest             string     `gorm:"column:raw_tx_digest"`
	TxHash                  string     `gorm:"column:tx_hash"`
	State                   string     `gorm:"column:state"`
	PredecessorGenerationID *uint64    `gorm:"column:predecessor_generation_id"`
	CommittedAt             *time.Time `gorm:"column:committed_at"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
}

func (*FairLaunchSigningGeneration) TableName() string { return "fair_launch_signing_generation" }

type FairLaunchGatewaySigningJournal struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	SubmissionID          uint64    `gorm:"column:submission_id"`
	SigningGenerationID   uint64    `gorm:"column:signing_generation_id"`
	JournalGeneration     uint32    `gorm:"column:journal_generation"`
	ChainID               uint64    `gorm:"column:chain_id"`
	ExpectedSignerAddress string    `gorm:"column:expected_signer_address"`
	Nonce                 uint64    `gorm:"column:nonce"`
	SigningDigest         string    `gorm:"column:signing_digest"`
	RequestDigest         string    `gorm:"column:request_digest"`
	RawTx                 []byte    `gorm:"column:raw_tx"`
	RawTxDigest           string    `gorm:"column:raw_tx_digest"`
	TxHash                string    `gorm:"column:tx_hash"`
	State                 string    `gorm:"column:state"`
	LastErrorCode         string    `gorm:"column:last_error_code"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (*FairLaunchGatewaySigningJournal) TableName() string {
	return "fair_launch_gateway_signing_journal"
}

type ChainEventIdentityV2 struct {
	ID           uint64    `gorm:"column:id;primaryKey"`
	ChainID      uint64    `gorm:"column:chain_id"`
	TxHash       string    `gorm:"column:tx_hash"`
	TxLogOrdinal uint32    `gorm:"column:tx_log_ordinal"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (*ChainEventIdentityV2) TableName() string { return "chain_event_identity_v2" }

type ChainEventPlacementV2 struct {
	ID               uint64     `gorm:"column:id;primaryKey"`
	EventIdentityID  uint64     `gorm:"column:event_identity_id"`
	Generation       uint32     `gorm:"column:generation"`
	BlockNumber      uint64     `gorm:"column:block_number"`
	BlockHash        string     `gorm:"column:block_hash"`
	TransactionIndex uint32     `gorm:"column:transaction_index"`
	RPCLogIndex      uint32     `gorm:"column:rpc_log_index"`
	SourceRevisionID uint64     `gorm:"column:source_revision_id"`
	CanonicalSlot    *uint8     `gorm:"column:canonical_slot"`
	FinalizedAt      *time.Time `gorm:"column:finalized_at"`
	NoncanonicalAt   *time.Time `gorm:"column:noncanonical_at"`
}

func (*ChainEventPlacementV2) TableName() string { return "chain_event_placement_v2" }

type ChainEventApplicationAttemptV2 struct {
	ID                       uint64     `gorm:"column:id;primaryKey"`
	EventIdentityID          uint64     `gorm:"column:event_identity_id"`
	PlacementID              uint64     `gorm:"column:placement_id"`
	EffectVersion            uint32     `gorm:"column:effect_version"`
	ExpectedEffect           uint8      `gorm:"column:expected_effect"`
	AttemptNo                uint32     `gorm:"column:attempt_no"`
	PredecessorApplicationID *uint64    `gorm:"column:predecessor_application_id"`
	Disposition              string     `gorm:"column:disposition"`
	EffectDigest             string     `gorm:"column:effect_digest"`
	AppliedAt                *time.Time `gorm:"column:applied_at"`
	ReversedAt               *time.Time `gorm:"column:reversed_at"`
}

func (*ChainEventApplicationAttemptV2) TableName() string {
	return "chain_event_application_attempt_v2"
}

type FundLedgerV2Shadow struct {
	ID                         uint64    `gorm:"column:id;primaryKey"`
	SourceApplicationAttemptID uint64    `gorm:"column:source_application_attempt_id"`
	EntryOrdinal               uint32    `gorm:"column:entry_ordinal"`
	EffectManifestVersion      uint32    `gorm:"column:effect_manifest_version"`
	TxLogOrdinal               uint32    `gorm:"column:tx_log_ordinal"`
	RPCLogIndex                uint32    `gorm:"column:rpc_log_index"`
	TokenAddress               string    `gorm:"column:token_address"`
	AmountRaw                  string    `gorm:"column:amount_raw"`
	CounterpartyAddress        string    `gorm:"column:counterparty_address"`
	ReversalOfLedgerID         *uint64   `gorm:"column:reversal_of_ledger_id"`
	CompensationGeneration     *uint32   `gorm:"column:compensation_generation"`
	RowDigest                  string    `gorm:"column:row_digest"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
}

func (*FundLedgerV2Shadow) TableName() string { return "fund_ledger_v2_shadow" }
