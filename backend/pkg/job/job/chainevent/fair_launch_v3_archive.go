package chainevent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

type FairLaunchLocalE2EArchive struct {
	seal *fairLaunchLocalE2EArchiveSeal
}

type fairLaunchLocalE2EArchiveSeal struct {
	IdentityID          uint64
	PlacementID         uint64
	ObservationID       uint64
	PlacementGeneration uint32
	SourceRevisionID    uint64
	BlockNumber         uint64
	BlockHash           string
	TransactionIndex    uint32
	TxLogOrdinal        uint32
	RPCLogIndex         uint32
}

func (archive FairLaunchLocalE2EArchive) valid(capability FairLaunchLocalE2ECapability) bool {
	return archive.seal != nil && capability.seal != nil && archive.seal.SourceRevisionID == capability.seal.admission.SourceRevisionID && archive.seal.IdentityID > 0 && archive.seal.PlacementID > 0 && archive.seal.ObservationID > 0 && archive.seal.PlacementGeneration > 0
}

// ArchiveFairLaunchLocalE2ETx performs only chain identity/placement/archive
// work under local_ingest. Financial projection is a separate local_job
// transaction and consumes the returned immutable IDs.
func ArchiveFairLaunchLocalE2ETx(capability FairLaunchLocalE2ECapability, tx *gorm.DB, in FairLaunchCEWrite, now time.Time) (FairLaunchLocalE2EArchive, error) {
	if !capability.valid(now) {
		return FairLaunchLocalE2EArchive{}, errors.New(FairLaunchLocalE2ECapabilityRequired)
	}
	if tx == nil || in.Event == nil || now.IsZero() || in.Context.ChainID != fairLaunchLocalExpectedChainID || in.Context.SourceRevisionID != capability.seal.admission.SourceRevisionID || in.Context.EffectVersion != FairLaunchEffectManifestVersion || !in.Context.Canonical || !in.Context.Finalized || in.SourceID != capability.seal.admission.SourceID || in.ProviderIdentity == "" || !fairLaunchLocalDigestPattern.MatchString(stripCE(in.TopicsDataDigest)) || !fairLaunchLocalDigestPattern.MatchString(stripCE(in.RawLogDigest)) {
		return FairLaunchLocalE2EArchive{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	if strings.ToLower(in.Event.Log.Address.Hex()) == "0x0000000000000000000000000000000000000000" || in.Event.Log.TxHash.Hex() == "0x0000000000000000000000000000000000000000000000000000000000000000" || in.Event.Log.BlockHash.Hex() == "0x0000000000000000000000000000000000000000000000000000000000000000" || in.Event.Log.BlockNumber <= capability.seal.admission.SignedTipBlock {
		return FairLaunchLocalE2EArchive{}, fmt.Errorf("%s: event identity/placement invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := verifyFairLaunchLocalLiveBlock(capability.seal.rpc, tx.Statement.Context, in.Event.Log.BlockNumber, strings.ToLower(in.Event.Log.BlockHash.Hex())); err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	var source entity.ChainContractSource
	if err := tx.Where("id=? AND chain_id=? AND contract_address=? AND contract_version=? AND funding_mode=? AND enabled=1 AND retired_at IS NULL", capability.seal.admission.SourceID, fairLaunchLocalExpectedChainID, strings.ToLower(in.Event.Log.Address.Hex()), fairLaunchLocalContractVersion, fairLaunchLocalFundingMode).Take(&source).Error; err != nil {
		return FairLaunchLocalE2EArchive{}, fmt.Errorf("%s: event source mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(in.Event)
	if err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	txHash := strings.ToLower(in.Event.Log.TxHash.Hex())
	blockHash := strings.ToLower(in.Event.Log.BlockHash.Hex())
	var identity entity.ChainEventIdentityV2
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND tx_hash=? AND tx_log_ordinal=?", fairLaunchLocalExpectedChainID, txHash, in.Context.TxLogOrdinal).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		identity = entity.ChainEventIdentityV2{ChainID: fairLaunchLocalExpectedChainID, TxHash: txHash, TxLogOrdinal: in.Context.TxLogOrdinal, CreatedAt: now.UTC()}
		if err = tx.Create(&identity).Error; err != nil {
			return FairLaunchLocalE2EArchive{}, err
		}
	} else if err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	var existing entity.ChainEventPlacementV2
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_identity_id=? AND canonical_slot=1", identity.ID).Take(&existing).Error
	if err == nil {
		if existing.BlockNumber != in.Event.Log.BlockNumber || existing.BlockHash != blockHash || existing.TransactionIndex != in.TransactionIndex || existing.RPCLogIndex != uint32(in.Event.Log.Index) || existing.SourceRevisionID != in.Context.SourceRevisionID || existing.FinalizedAt == nil || existing.NoncanonicalAt != nil {
			return FairLaunchLocalE2EArchive{}, fmt.Errorf("%s: canonical placement conflict", FairLaunchLocalE2EAdmissionRejected)
		}
		observation, err := upsertFairLaunchLocalObservation(tx, identity.ID, existing.ID, in, payloadDigest, now.UTC())
		if err != nil {
			return FairLaunchLocalE2EArchive{}, err
		}
		return FairLaunchLocalE2EArchive{seal: &fairLaunchLocalE2EArchiveSeal{IdentityID: identity.ID, PlacementID: existing.ID, ObservationID: observation.ID, PlacementGeneration: existing.Generation, SourceRevisionID: existing.SourceRevisionID, BlockNumber: existing.BlockNumber, BlockHash: existing.BlockHash, TransactionIndex: existing.TransactionIndex, TxLogOrdinal: in.Context.TxLogOrdinal, RPCLogIndex: existing.RPCLogIndex}}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return FairLaunchLocalE2EArchive{}, err
	}
	var generation uint32
	if err := tx.Model(&entity.ChainEventPlacementV2{}).Where("event_identity_id=?", identity.ID).Select("COALESCE(MAX(generation),0)").Scan(&generation).Error; err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	generation++
	canonical := uint8(1)
	placement := entity.ChainEventPlacementV2{EventIdentityID: identity.ID, Generation: generation, BlockNumber: in.Event.Log.BlockNumber, BlockHash: blockHash, TransactionIndex: in.TransactionIndex, RPCLogIndex: uint32(in.Event.Log.Index), SourceRevisionID: in.Context.SourceRevisionID, CanonicalSlot: &canonical, FinalizedAt: localTimePointer(now.UTC())}
	if err := tx.Create(&placement).Error; err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	observation, err := upsertFairLaunchLocalObservation(tx, identity.ID, placement.ID, in, payloadDigest, now.UTC())
	if err != nil {
		return FairLaunchLocalE2EArchive{}, err
	}
	return FairLaunchLocalE2EArchive{seal: &fairLaunchLocalE2EArchiveSeal{IdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, PlacementGeneration: generation, SourceRevisionID: placement.SourceRevisionID, BlockNumber: placement.BlockNumber, BlockHash: placement.BlockHash, TransactionIndex: placement.TransactionIndex, TxLogOrdinal: in.Context.TxLogOrdinal, RPCLogIndex: placement.RPCLogIndex}}, nil
}

// ProcessFairLaunchLocalE2E executes the required two-transaction pipeline.
// Archive identity/placement commits first under local_ingest. Financial V3
// application then runs atomically under local_job; failure leaves a durable,
// retriable observation without partial financial state.
func ProcessFairLaunchLocalE2E(capability FairLaunchLocalE2ECapability, ingestDB, jobDB *gorm.DB, in FairLaunchCEWrite, now time.Time) (FairLaunchCEWriteResult, error) {
	if !capability.valid(now) || ingestDB == nil || jobDB == nil || in.LocalArchive != nil {
		return FairLaunchCEWriteResult{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var archive FairLaunchLocalE2EArchive
	if err := ingestDB.Transaction(func(tx *gorm.DB) error {
		var err error
		archive, err = ArchiveFairLaunchLocalE2ETx(capability, tx, in, now.UTC())
		return err
	}); err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	in.LocalArchive = &archive
	in.LeaseToken = capability.seal.lease
	in.FenceEpoch = capability.seal.fence
	var result FairLaunchCEWriteResult
	if err := jobDB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = ApplyFairLaunchCEV2LocalTx(capability, tx, in, now.UTC())
		return err
	}); err != nil {
		stage := "financial_apply"
		var staged *fairLaunchLocalFinancialStageError
		if errors.As(err, &staged) {
			stage = staged.Stage
		}
		return FairLaunchCEWriteResult{}, &FairLaunchLocalE2ERejectedEvent{Status: "ARCHIVED_FINANCIAL_REJECTED", EventIdentityID: archive.seal.IdentityID, PlacementID: archive.seal.PlacementID, ObservationID: archive.seal.ObservationID, Stage: stage, Reason: FairLaunchLocalE2EAdmissionRejected}
	}
	return result, nil
}

type FairLaunchLocalE2ERejectedEvent struct {
	Status          string `json:"status"`
	EventIdentityID uint64 `json:"eventIdentityId"`
	PlacementID     uint64 `json:"placementId"`
	ObservationID   uint64 `json:"observationId"`
	Stage           string `json:"stage"`
	Reason          string `json:"reason"`
}

func (rejected *FairLaunchLocalE2ERejectedEvent) Error() string {
	return rejected.Reason
}

func upsertFairLaunchLocalObservation(tx *gorm.DB, identityID, placementID uint64, in FairLaunchCEWrite, payloadDigest string, now time.Time) (entity.ChainEventPlacementObservationV2, error) {
	observation := entity.ChainEventPlacementObservationV2{PlacementID: placementID, EventIdentityID: identityID, ProviderIdentity: in.ProviderIdentity, EmittingAddress: strings.ToLower(in.Event.Log.Address.Hex()), TopicsDataDigest: stripCE(in.TopicsDataDigest), RawLogDigest: stripCE(in.RawLogDigest), ObservedAt: now}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&observation)
	if result.Error != nil {
		return entity.ChainEventPlacementObservationV2{}, result.Error
	}
	if result.RowsAffected == 0 {
		if err := tx.Where("placement_id=? AND provider_identity=? AND raw_log_digest=? AND event_identity_id=? AND emitting_address=? AND topics_data_digest=?", placementID, in.ProviderIdentity, stripCE(in.RawLogDigest), identityID, strings.ToLower(in.Event.Log.Address.Hex()), stripCE(in.TopicsDataDigest)).Take(&observation).Error; err != nil {
			return entity.ChainEventPlacementObservationV2{}, fmt.Errorf("%s: archived observation replay mismatch", FairLaunchLocalE2EAdmissionRejected)
		}
	}
	_ = payloadDigest // Typed digest is re-derived and bound by the job transaction.
	return observation, nil
}

func localTimePointer(value time.Time) *time.Time { return &value }
