package entity

import "time"

type ChainIngestionCursor struct {
	ChainID                  uint64    `gorm:"column:chain_id;primaryKey"`
	ArchivedThroughBlock     uint64    `gorm:"column:archived_through_block"`
	ArchivedThroughBlockHash string    `gorm:"column:archived_through_block_hash"`
	LastTxIndex              uint32    `gorm:"column:last_tx_index"`
	LastLogIndex             uint32    `gorm:"column:last_log_index"`
	LeaseToken               string    `gorm:"column:lease_token"`
	FenceEpoch               uint64    `gorm:"column:fence_epoch"`
	Status                   string    `gorm:"column:status"`
	LastError                string    `gorm:"column:last_error"`
	UpdatedAt                time.Time `gorm:"column:updated_at"`
	CursorGeneration         uint64    `gorm:"column:cursor_generation"`
}

func (*ChainIngestionCursor) TableName() string { return "chain_ingestion_cursor" }

type SourceMaterializationCursor struct {
	SourceID                        uint64    `gorm:"column:source_id;primaryKey"`
	SourceRevisionID                uint64    `gorm:"column:source_revision_id"`
	MaterializedThroughBlock        uint64    `gorm:"column:materialized_through_block"`
	MaterializedThroughBlockHash    string    `gorm:"column:materialized_through_block_hash"`
	MaterializedThroughTxIndex      uint32    `gorm:"column:materialized_through_tx_index"`
	MaterializedThroughTxLogOrdinal uint32    `gorm:"column:materialized_through_tx_log_ordinal"`
	LastEventIdentityID             *uint64   `gorm:"column:last_event_identity_id"`
	LeaseToken                      string    `gorm:"column:lease_token"`
	FenceEpoch                      uint64    `gorm:"column:fence_epoch"`
	Status                          string    `gorm:"column:status"`
	LastError                       string    `gorm:"column:last_error"`
	UpdatedAt                       time.Time `gorm:"column:updated_at"`
	CursorGeneration                uint64    `gorm:"column:cursor_generation"`
}

func (*SourceMaterializationCursor) TableName() string { return "source_materialization_cursor" }

type SourceIngestionCoverage struct {
	ID             uint64 `gorm:"column:id;primaryKey"`
	SourceID       uint64 `gorm:"column:source_id"`
	FromBlock      uint64 `gorm:"column:from_block"`
	ThroughBlock   uint64 `gorm:"column:through_block"`
	CoverageDigest string `gorm:"column:coverage_digest"`
	Finalized      bool   `gorm:"column:finalized"`
}

func (*SourceIngestionCoverage) TableName() string { return "source_ingestion_coverage" }

type ChainEventPlacementObservationV2 struct {
	ID               uint64    `gorm:"column:id;primaryKey"`
	PlacementID      uint64    `gorm:"column:placement_id"`
	EventIdentityID  uint64    `gorm:"column:event_identity_id"`
	ProviderIdentity string    `gorm:"column:provider_identity"`
	EmittingAddress  string    `gorm:"column:emitting_address"`
	TopicsDataDigest string    `gorm:"column:topics_data_digest"`
	RawLogDigest     string    `gorm:"column:raw_log_digest"`
	ObservedAt       time.Time `gorm:"column:observed_at"`
}

func (*ChainEventPlacementObservationV2) TableName() string {
	return "chain_event_placement_observation_v2"
}

type ChainEventPayloadConflictV2 struct {
	ID                       uint64     `gorm:"column:id;primaryKey"`
	EventIdentityID          uint64     `gorm:"column:event_identity_id"`
	FirstObservationID       uint64     `gorm:"column:first_observation_id"`
	ConflictingObservationID uint64     `gorm:"column:conflicting_observation_id"`
	State                    string     `gorm:"column:state"`
	EvidenceDigest           string     `gorm:"column:evidence_digest"`
	DetectedAt               time.Time  `gorm:"column:detected_at"`
	ResolvedAt               *time.Time `gorm:"column:resolved_at"`
}

func (*ChainEventPayloadConflictV2) TableName() string { return "chain_event_payload_conflict_v2" }

type ChainEventIdentityQuarantineV2 struct {
	ID             uint64     `gorm:"column:id;primaryKey"`
	ChainID        uint64     `gorm:"column:chain_id"`
	TxHash         string     `gorm:"column:tx_hash"`
	ReasonCode     string     `gorm:"column:reason_code"`
	EvidenceDigest string     `gorm:"column:evidence_digest"`
	State          string     `gorm:"column:state"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	ResolvedAt     *time.Time `gorm:"column:resolved_at"`
}

func (*ChainEventIdentityQuarantineV2) TableName() string {
	return "chain_event_identity_quarantine_v2"
}
