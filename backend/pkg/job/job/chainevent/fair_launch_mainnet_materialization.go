package chainevent

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

const FairLaunchMainnetMaterializationUnavailable = "fair_launch_mainnet_materialization_unavailable"

type FairLaunchMainnetMaterializationInitialization struct {
	ActivationRowID  uint64
	SourceID         uint64
	SourceRevisionID uint64
	SourceBindingID  uint64
	ArchiveEventID   uint64
	EventIdentityID  uint64
	CursorGeneration uint64
	Replayed         bool
}

type FairLaunchMainnetMaterializationResult struct {
	ArchiveEventID   uint64
	EventIdentityID  uint64
	ApplicationID    uint64
	LineageID        uint64
	AttestationID    uint64
	ReceiptID        uint64
	ReceiptDigest    string
	CursorGeneration uint64
	EventName        string
	NoWork           bool
}

var fairLaunchMainnetFinancialEvents = []string{
	"FairLaunchTokensDeposited", "FairLaunchActivated", "FairLaunchContribution", "FairLaunchEnded",
	"FairLaunchSuccessfulReleased", "FairLaunchCancelled", "FairLaunchRefunded", "FairLaunchClaimed",
	"FairLaunchProjectProceedsWithdrawn", "FairLaunchPlatformFeeWithdrawn", "FairLaunchFailedProjectTokensWithdrawn",
}

// InitializeFairLaunchMainnetMaterializationTx initializes the authority-neutral
// V3 cursor and the receipt-backed mainnet cursor at the exact archived
// FairLaunchProjectCreated bootstrap anchor. Profile creation remains a
// separately governed DB admission; this function cannot mint or widen it.
func InitializeFairLaunchMainnetMaterializationTx(capability FairLaunchMainnetCapability, tx *gorm.DB, now time.Time) (FairLaunchMainnetMaterializationInitialization, error) {
	admission, err := capability.Admission()
	if err != nil || tx == nil || !capability.valid(now) || admission.Component != "job" {
		return FairLaunchMainnetMaterializationInitialization{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	authority, err := capability.v3Authority(now)
	if err != nil {
		return FairLaunchMainnetMaterializationInitialization{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	now = now.UTC()
	if err := validateFairLaunchMainnetMaterializationLeaseTx(tx, admission, authority.LeaseToken, now); err != nil {
		return FairLaunchMainnetMaterializationInitialization{}, err
	}
	var profile entity.FairLaunchMainnetMaterializationProfileV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND source_binding_id=? AND effect_manifest_version=? AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.SourceBindingID, FairLaunchEffectManifestVersion, now, now).Take(&profile).Error; err != nil {
		return FairLaunchMainnetMaterializationInitialization{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if !fairLaunchMainnetMaterializationProfileValid(profile) {
		return FairLaunchMainnetMaterializationInitialization{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	anchor, err := loadFairLaunchMainnetMaterializationAnchorTx(tx, admission, profile)
	if err != nil {
		return FairLaunchMainnetMaterializationInitialization{}, err
	}
	generic := entity.SourceMaterializationCursor{
		SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID,
		MaterializedThroughBlock: anchor.placement.BlockNumber, MaterializedThroughBlockHash: anchor.placement.BlockHash,
		MaterializedThroughTxIndex: anchor.placement.TransactionIndex, MaterializedThroughTxLogOrdinal: anchor.identity.TxLogOrdinal,
		LastEventIdentityID: uint64Pointer(anchor.identity.ID), LeaseToken: authority.LeaseToken, FenceEpoch: admission.FenceEpoch,
		Status: "active", LastError: "", UpdatedAt: now, CursorGeneration: 1,
	}
	mainnet := entity.FairLaunchMainnetMaterializationCursorV1{
		ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, SourceBindingID: admission.SourceBindingID,
		MaterializedThroughBlock: anchor.placement.BlockNumber, MaterializedThroughBlockHash: anchor.placement.BlockHash,
		MaterializedThroughTxIndex: anchor.placement.TransactionIndex, MaterializedThroughTxLogOrdinal: anchor.identity.TxLogOrdinal,
		LastEventIdentityID: uint64Pointer(anchor.identity.ID), LastArchiveEventID: uint64Pointer(anchor.archiveEvent.ID),
		LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, LeaseTokenHash: authority.LeaseToken,
		Status: "active", CursorGeneration: 1, CreatedAt: now, UpdatedAt: now,
	}
	replayed, err := initializeFairLaunchMainnetDoubleCursorTx(tx, generic, mainnet)
	if err != nil {
		return FairLaunchMainnetMaterializationInitialization{}, err
	}
	return FairLaunchMainnetMaterializationInitialization{ActivationRowID: admission.AuthorityRowID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, SourceBindingID: admission.SourceBindingID, ArchiveEventID: anchor.archiveEvent.ID, EventIdentityID: anchor.identity.ID, CursorGeneration: mainnet.CursorGeneration, Replayed: replayed}, nil
}

// materializeFairLaunchMainnetNextTx is deliberately package-private and owns
// no transaction, scheduler or service lifecycle. Its caller must provide the
// transaction boundary. One invocation applies at most one archived financial
// event so every receipt/cursor transition has a narrow idempotency boundary.
func materializeFairLaunchMainnetNextTx(capability FairLaunchMainnetCapability, tx *gorm.DB, now time.Time) (FairLaunchMainnetMaterializationResult, error) {
	admission, err := capability.Admission()
	if err != nil || tx == nil || !capability.valid(now) || admission.Component != "job" {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	authority, err := capability.v3Authority(now)
	if err != nil {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	now = now.UTC()
	if err := validateFairLaunchMainnetMaterializationLeaseTx(tx, admission, authority.LeaseToken, now); err != nil {
		return FairLaunchMainnetMaterializationResult{}, err
	}
	var profile entity.FairLaunchMainnetMaterializationProfileV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND source_binding_id=? AND effect_manifest_version=? AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.SourceBindingID, FairLaunchEffectManifestVersion, now, now).Take(&profile).Error; err != nil {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if !fairLaunchMainnetMaterializationProfileValid(profile) {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var cursor entity.FairLaunchMainnetMaterializationCursorV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND source_binding_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active'", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.SourceBindingID, admission.LeaseGeneration, admission.FenceEpoch, authority.LeaseToken).Take(&cursor).Error; err != nil {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var generic entity.SourceMaterializationCursor
	if err := tx.Where("source_id=? AND source_revision_id=? AND lease_token=? AND fence_epoch=? AND status='active' AND last_error=''", admission.SourceID, admission.SourceRevisionID, authority.LeaseToken, admission.FenceEpoch).Take(&generic).Error; err != nil || !fairLaunchDoubleCursorPositionEqual(generic, cursor) {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if err := validateFairLaunchMainnetMaterializationCursorReceiptTx(tx, admission, cursor); err != nil {
		return FairLaunchMainnetMaterializationResult{}, err
	}
	var archiveCursor entity.FairLaunchMainnetArchiveCursorV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND last_error_code IS NULL AND last_error_digest IS NULL", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.LeaseGeneration, admission.FenceEpoch, authority.LeaseToken).Take(&archiveCursor).Error; err != nil || archiveCursor.ArchivedThroughBlock < cursor.MaterializedThroughBlock || archiveCursor.ArchivedThroughBlock == cursor.MaterializedThroughBlock && archiveCursor.ArchivedThroughBlockHash != cursor.MaterializedThroughBlockHash {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	archiveEvent, identity, placement, observation, payload, batchReceipt, err := loadNextFairLaunchMainnetFinancialEventTx(tx, admission, cursor)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FairLaunchMainnetMaterializationResult{CursorGeneration: cursor.CursorGeneration, NoWork: true}, nil
	}
	if err != nil {
		return FairLaunchMainnetMaterializationResult{}, err
	}
	if compareFairLaunchPosition(placement.BlockNumber, placement.TransactionIndex, identity.TxLogOrdinal, archiveCursor.ArchivedThroughBlock, ^uint32(0), ^uint32(0)) > 0 {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	log, err := fairLaunchMainnetLogFromPayload(payload, identity, placement, observation)
	if err != nil {
		return FairLaunchMainnetMaterializationResult{}, err
	}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil || event.Name != archiveEvent.EventName || !fairLaunchV3ForwardEvents[event.Name] {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil || payloadDigest != archiveEvent.PayloadDigest {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	write := FairLaunchCEWrite{
		Event: event, Context: FairLaunchMaterializationContext{ChainID: admission.ChainID, TxLogOrdinal: identity.TxLogOrdinal, PlacementGeneration: placement.Generation, SourceRevisionID: admission.SourceRevisionID, EffectVersion: FairLaunchEffectManifestVersion, Canonical: true, Finalized: true},
		ProviderIdentity: observation.ProviderIdentity, TopicsDataDigest: observation.TopicsDataDigest, RawLogDigest: observation.RawLogDigest,
		TransactionIndex: placement.TransactionIndex, SourceID: admission.SourceID, LeaseToken: authority.LeaseToken, FenceEpoch: admission.FenceEpoch,
	}
	result, err := applyFairLaunchV3ForwardCoreTx(authority, tx, write, fairLaunchV3ArchiveEvidence{IdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, PlacementGeneration: placement.Generation, SourceRevisionID: placement.SourceRevisionID, BlockNumber: placement.BlockNumber, BlockHash: placement.BlockHash, TransactionIndex: placement.TransactionIndex, TxLogOrdinal: identity.TxLogOrdinal, RPCLogIndex: placement.RPCLogIndex}, now)
	if err != nil || result.Effect.Disposition != FairLaunchEffectExact || result.ApplicationID == 0 {
		if err != nil {
			return FairLaunchMainnetMaterializationResult{}, err
		}
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var lineage entity.FairLaunchApplicationLineageV3
	if err := tx.Where("ce_application_attempt_id=? AND run_row_id=? AND chain_id=? AND source_binding_id=? AND source_id=? AND event_identity_id=? AND placement_id=? AND source_revision_id=? AND effect_manifest_version=? AND disposition='forward' AND state='applied'", result.ApplicationID, admission.AuthorityRowID, admission.ChainID, admission.SourceBindingID, admission.SourceID, identity.ID, placement.ID, admission.SourceRevisionID, FairLaunchEffectManifestVersion).Take(&lineage).Error; err != nil {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var attestation entity.FairLaunchEffectAttestationV3
	if err := tx.Where("source_application_lineage_id=? AND event_identity_id=? AND placement_id=?", lineage.ID, identity.ID, placement.ID).Take(&attestation).Error; err != nil || !fairLaunchAttestationValid(attestation, lineage) {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var previousReceiptDigest *string
	if cursor.LastMaterializationReceiptID != nil {
		var previous entity.FairLaunchMainnetMaterializationReceiptV1
		if err := tx.Where("id=? AND activation_row_id=?", *cursor.LastMaterializationReceiptID, admission.AuthorityRowID).Take(&previous).Error; err != nil {
			return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		if previous.ReceiptDigest != fairLaunchMainnetMaterializationReceiptDigest(previous) {
			return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		previousReceiptDigest = stringPointer(previous.ReceiptDigest)
	}
	receipt := entity.FairLaunchMainnetMaterializationReceiptV1{
		ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, SourceBindingID: admission.SourceBindingID,
		ArchiveBatchReceiptID: batchReceipt.ID, ArchiveEventID: archiveEvent.ID, EventIdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID,
		ApplicationAttemptID: result.ApplicationID, SourceApplicationLineageID: lineage.ID, AttestationID: attestation.ID, EffectManifestVersion: FairLaunchEffectManifestVersion,
		PayloadDigest: lineage.PayloadDigest, EffectDigest: lineage.EffectDigest, LedgerDigest: lineage.LedgerDigest, MutationDigest: lineage.MutationDigest, AttestationDigest: attestation.AttestationDigest,
		LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, PreviousCursorGeneration: cursor.CursorGeneration, PreviousReceiptDigest: previousReceiptDigest, CreatedAt: now,
	}
	receipt.ReceiptDigest = fairLaunchMainnetMaterializationReceiptDigest(receipt)
	if err := tx.Create(&receipt).Error; err != nil {
		return FairLaunchMainnetMaterializationResult{}, err
	}
	cursor.MaterializedThroughBlock = placement.BlockNumber
	cursor.MaterializedThroughBlockHash = placement.BlockHash
	cursor.MaterializedThroughTxIndex = placement.TransactionIndex
	cursor.MaterializedThroughTxLogOrdinal = identity.TxLogOrdinal
	cursor.LastEventIdentityID = uint64Pointer(identity.ID)
	cursor.LastArchiveEventID = uint64Pointer(archiveEvent.ID)
	cursor.LastMaterializationReceiptID = uint64Pointer(receipt.ID)
	cursor.CursorGeneration++
	cursor.UpdatedAt = now
	update := tx.Model(&entity.FairLaunchMainnetMaterializationCursorV1{}).Where("activation_row_id=? AND cursor_generation=? AND status='active'", admission.AuthorityRowID, receipt.PreviousCursorGeneration).Updates(map[string]any{
		"materialized_through_block": cursor.MaterializedThroughBlock, "materialized_through_block_hash": cursor.MaterializedThroughBlockHash,
		"materialized_through_tx_index": cursor.MaterializedThroughTxIndex, "materialized_through_tx_log_ordinal": cursor.MaterializedThroughTxLogOrdinal,
		"last_event_identity_id": identity.ID, "last_archive_event_id": archiveEvent.ID, "last_materialization_receipt_id": receipt.ID,
		"cursor_generation": cursor.CursorGeneration, "updated_at": now,
	})
	if update.Error != nil {
		return FairLaunchMainnetMaterializationResult{}, update.Error
	}
	if update.RowsAffected != 1 {
		return FairLaunchMainnetMaterializationResult{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return FairLaunchMainnetMaterializationResult{ArchiveEventID: archiveEvent.ID, EventIdentityID: identity.ID, ApplicationID: result.ApplicationID, LineageID: lineage.ID, AttestationID: attestation.ID, ReceiptID: receipt.ID, ReceiptDigest: receipt.ReceiptDigest, CursorGeneration: cursor.CursorGeneration, EventName: event.Name}, nil
}

type fairLaunchMainnetMaterializationAnchor struct {
	archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	identity     entity.ChainEventIdentityV2
	placement    entity.ChainEventPlacementV2
}

func loadFairLaunchMainnetMaterializationAnchorTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, profile entity.FairLaunchMainnetMaterializationProfileV1) (fairLaunchMainnetMaterializationAnchor, error) {
	var bootstrap entity.FairLaunchMainnetProjectBootstrapReceiptV1
	if err := tx.Where("id=? AND activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND event_identity_id=?", profile.BootstrapReceiptID, admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, profile.InitialEventIdentityID).Take(&bootstrap).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	expectedBootstrapDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": bootstrap.ActivationRowID, "chainId": bootstrap.ChainID, "projectBindingId": bootstrap.ProjectBindingID, "projectId": bootstrap.ProjectID, "eventIdentityId": bootstrap.EventIdentityID, "placementId": bootstrap.PlacementID, "observationId": bootstrap.ObservationID, "sourceId": bootstrap.SourceID, "sourceRevisionId": bootstrap.SourceRevisionID, "placementGeneration": bootstrap.PlacementGeneration, "effectManifestVersion": bootstrap.EffectManifestVersion, "payloadDigest": bootstrap.PayloadDigest, "bindingDigest": bootstrap.BindingDigest, "initialStateDigest": bootstrap.InitialStateDigest})
	if bootstrap.ReceiptDigest != expectedBootstrapDigest {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	if err := tx.Where("id=? AND activation_row_id=? AND event_identity_id=? AND placement_id=? AND observation_id=? AND event_name='FairLaunchProjectCreated' AND payload_digest=?", profile.BootstrapArchiveEventID, admission.AuthorityRowID, bootstrap.EventIdentityID, bootstrap.PlacementID, bootstrap.ObservationID, bootstrap.PayloadDigest).Take(&archiveEvent).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if archiveEvent.RowDigest != fairLaunchMainnetArchiveEventRowDigest(admission, archiveEvent.EventOrdinal, archiveEvent.EventIdentityID, archiveEvent.PlacementID, archiveEvent.ObservationID, archiveEvent.EventName, archiveEvent.PayloadDigest) {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var identity entity.ChainEventIdentityV2
	if err := tx.Where("id=? AND chain_id=?", bootstrap.EventIdentityID, admission.ChainID).Take(&identity).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var placement entity.ChainEventPlacementV2
	if err := tx.Where("id=? AND event_identity_id=? AND source_revision_id=? AND canonical_slot=1 AND finalized_at IS NOT NULL AND noncanonical_at IS NULL", bootstrap.PlacementID, identity.ID, admission.SourceRevisionID).Take(&placement).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var observation entity.ChainEventPlacementObservationV2
	if err := tx.Where("id=? AND event_identity_id=? AND placement_id=? AND emitting_address=?", bootstrap.ObservationID, identity.ID, placement.ID, admission.ProxyAddress).Take(&observation).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	var payload entity.FairLaunchMainnetArchiveEventPayloadV1
	if err := tx.Where("archive_event_id=? AND activation_row_id=? AND event_identity_id=? AND placement_id=? AND observation_id=?", archiveEvent.ID, admission.AuthorityRowID, identity.ID, placement.ID, observation.ID).Take(&payload).Error; err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	log, err := fairLaunchMainnetLogFromPayload(payload, identity, placement, observation)
	if err != nil {
		return fairLaunchMainnetMaterializationAnchor{}, err
	}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil || event == nil || event.Name != archiveEvent.EventName {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	digest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil || digest != archiveEvent.PayloadDigest || digest != bootstrap.PayloadDigest {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if profile.InitialBlockNumber != placement.BlockNumber || profile.InitialBlockHash != placement.BlockHash || profile.InitialTransactionIndex != placement.TransactionIndex || profile.InitialTxLogOrdinal != identity.TxLogOrdinal || profile.InitialEventIdentityID != identity.ID {
		return fairLaunchMainnetMaterializationAnchor{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return fairLaunchMainnetMaterializationAnchor{archiveEvent: archiveEvent, identity: identity, placement: placement}, nil
}

func loadNextFairLaunchMainnetFinancialEventTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, cursor entity.FairLaunchMainnetMaterializationCursorV1) (entity.FairLaunchMainnetArchiveBatchEventV1, entity.ChainEventIdentityV2, entity.ChainEventPlacementV2, entity.ChainEventPlacementObservationV2, entity.FairLaunchMainnetArchiveEventPayloadV1, entity.FairLaunchMainnetArchiveBatchReceiptV1, error) {
	type nextRow struct {
		ArchiveEventID uint64 `gorm:"column:archive_event_id"`
	}
	var next nextRow
	err := tx.Table("fair_launch_mainnet_archive_batch_event_v1 AS e").
		Select("e.id AS archive_event_id").
		Joins("JOIN chain_event_identity_v2 i ON i.id=e.event_identity_id AND i.chain_id=?", admission.ChainID).
		Joins("JOIN chain_event_placement_v2 p ON p.id=e.placement_id AND p.event_identity_id=i.id AND p.source_revision_id=?", admission.SourceRevisionID).
		Where("e.activation_row_id=? AND e.event_name IN ? AND p.canonical_slot=1 AND p.finalized_at IS NOT NULL AND p.noncanonical_at IS NULL", admission.AuthorityRowID, fairLaunchMainnetFinancialEvents).
		Where("p.block_number>? OR (p.block_number=? AND (p.transaction_index>? OR (p.transaction_index=? AND i.tx_log_ordinal>?)))", cursor.MaterializedThroughBlock, cursor.MaterializedThroughBlock, cursor.MaterializedThroughTxIndex, cursor.MaterializedThroughTxIndex, cursor.MaterializedThroughTxLogOrdinal).
		Order("p.block_number ASC, p.transaction_index ASC, i.tx_log_ordinal ASC").Limit(1).Take(&next).Error
	if err != nil {
		return entity.FairLaunchMainnetArchiveBatchEventV1{}, entity.ChainEventIdentityV2{}, entity.ChainEventPlacementV2{}, entity.ChainEventPlacementObservationV2{}, entity.FairLaunchMainnetArchiveEventPayloadV1{}, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var archiveEvent entity.FairLaunchMainnetArchiveBatchEventV1
	if err := tx.Where("id=? AND activation_row_id=?", next.ArchiveEventID, admission.AuthorityRowID).Take(&archiveEvent).Error; err != nil {
		return archiveEvent, entity.ChainEventIdentityV2{}, entity.ChainEventPlacementV2{}, entity.ChainEventPlacementObservationV2{}, entity.FairLaunchMainnetArchiveEventPayloadV1{}, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var identity entity.ChainEventIdentityV2
	if err := tx.Where("id=? AND chain_id=?", archiveEvent.EventIdentityID, admission.ChainID).Take(&identity).Error; err != nil {
		return archiveEvent, identity, entity.ChainEventPlacementV2{}, entity.ChainEventPlacementObservationV2{}, entity.FairLaunchMainnetArchiveEventPayloadV1{}, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var placement entity.ChainEventPlacementV2
	if err := tx.Where("id=? AND event_identity_id=? AND source_revision_id=? AND canonical_slot=1 AND finalized_at IS NOT NULL AND noncanonical_at IS NULL", archiveEvent.PlacementID, identity.ID, admission.SourceRevisionID).Take(&placement).Error; err != nil {
		return archiveEvent, identity, placement, entity.ChainEventPlacementObservationV2{}, entity.FairLaunchMainnetArchiveEventPayloadV1{}, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var observation entity.ChainEventPlacementObservationV2
	if err := tx.Where("id=? AND event_identity_id=? AND placement_id=? AND emitting_address=?", archiveEvent.ObservationID, identity.ID, placement.ID, admission.ProxyAddress).Take(&observation).Error; err != nil {
		return archiveEvent, identity, placement, observation, entity.FairLaunchMainnetArchiveEventPayloadV1{}, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var payload entity.FairLaunchMainnetArchiveEventPayloadV1
	if err := tx.Where("archive_event_id=? AND activation_row_id=? AND event_identity_id=? AND placement_id=? AND observation_id=?", archiveEvent.ID, admission.AuthorityRowID, identity.ID, placement.ID, observation.ID).Take(&payload).Error; err != nil {
		return archiveEvent, identity, placement, observation, payload, entity.FairLaunchMainnetArchiveBatchReceiptV1{}, err
	}
	var receipt entity.FairLaunchMainnetArchiveBatchReceiptV1
	if err := tx.Where("id=? AND activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=?", archiveEvent.BatchReceiptID, admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID).Take(&receipt).Error; err != nil {
		return archiveEvent, identity, placement, observation, payload, receipt, err
	}
	if archiveEvent.RowDigest != fairLaunchMainnetArchiveEventRowDigest(admission, archiveEvent.EventOrdinal, archiveEvent.EventIdentityID, archiveEvent.PlacementID, archiveEvent.ObservationID, archiveEvent.EventName, archiveEvent.PayloadDigest) || receipt.ReceiptDigest != fairLaunchMainnetArchiveReceiptDigest(receipt) {
		return archiveEvent, identity, placement, observation, payload, receipt, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return archiveEvent, identity, placement, observation, payload, receipt, nil
}

func initializeFairLaunchMainnetDoubleCursorTx(tx *gorm.DB, generic entity.SourceMaterializationCursor, mainnet entity.FairLaunchMainnetMaterializationCursorV1) (bool, error) {
	var existingGeneric entity.SourceMaterializationCursor
	genericErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_id=?", generic.SourceID).Take(&existingGeneric).Error
	var existingMainnet entity.FairLaunchMainnetMaterializationCursorV1
	mainnetErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=?", mainnet.ActivationRowID).Take(&existingMainnet).Error
	if genericErr == nil || mainnetErr == nil {
		if genericErr != nil || mainnetErr != nil || !fairLaunchGenericCursorEqual(existingGeneric, generic) || !fairLaunchMainnetCursorEqual(existingMainnet, mainnet) {
			return false, errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		return true, nil
	}
	if !errors.Is(genericErr, gorm.ErrRecordNotFound) || !errors.Is(mainnetErr, gorm.ErrRecordNotFound) {
		return false, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if err := tx.Create(&generic).Error; err != nil {
		return false, err
	}
	if err := tx.Create(&mainnet).Error; err != nil {
		return false, err
	}
	return false, nil
}

func validateFairLaunchMainnetMaterializationLeaseTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, leaseHash string, now time.Time) error {
	var lease entity.FairLaunchMainnetActivationLeaseV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND active_slot=1 AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash, now, now).Take(&lease).Error; err != nil {
		return errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return nil
}

func fairLaunchMainnetLogFromPayload(payload entity.FairLaunchMainnetArchiveEventPayloadV1, identity entity.ChainEventIdentityV2, placement entity.ChainEventPlacementV2, observation entity.ChainEventPlacementObservationV2) (types.Log, error) {
	if payload.TopicCount == 0 || payload.TopicCount > 4 || !common.IsHexAddress(observation.EmittingAddress) || !fairLaunchLocalHashPattern.MatchString(identity.TxHash) || !fairLaunchLocalHashPattern.MatchString(placement.BlockHash) {
		return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	topicText := []string{payload.Topic0}
	for _, optional := range []*string{payload.Topic1, payload.Topic2, payload.Topic3} {
		if optional != nil {
			topicText = append(topicText, *optional)
		}
	}
	if len(topicText) != int(payload.TopicCount) {
		return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	topics := make([]common.Hash, len(topicText))
	for index, value := range topicText {
		if !fairLaunchLocalHashPattern.MatchString(value) {
			return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		topics[index] = common.HexToHash(value)
	}
	if len(payload.DataHex) < 2 || !strings.HasPrefix(payload.DataHex, "0x") || len(payload.DataHex)%2 != 0 {
		return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	data, err := hex.DecodeString(payload.DataHex[2:])
	if err != nil || len(data) > 65536 {
		return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	log := types.Log{Address: common.HexToAddress(observation.EmittingAddress), Topics: topics, Data: data, BlockNumber: placement.BlockNumber, TxHash: common.HexToHash(identity.TxHash), BlockHash: common.HexToHash(placement.BlockHash), Index: uint(placement.RPCLogIndex)}
	topicsDigest, rawDigest := fairLaunchLocalLogDigests(log)
	if topicsDigest != observation.TopicsDataDigest || rawDigest != observation.RawLogDigest {
		return types.Log{}, fmt.Errorf("%s: archive payload digest mismatch", FairLaunchMainnetMaterializationUnavailable)
	}
	expected, err := fairLaunchMainnetArchivePayloadRow(payload.ActivationRowID, entity.FairLaunchMainnetArchiveBatchEventV1{ID: payload.ArchiveEventID, EventIdentityID: payload.EventIdentityID, PlacementID: payload.PlacementID, ObservationID: payload.ObservationID}, log, payload.CreatedAt)
	if err != nil || !fairLaunchMainnetArchivePayloadEqual(payload, expected) {
		return types.Log{}, errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return log, nil
}

func fairLaunchGenericCursorEqual(left, right entity.SourceMaterializationCursor) bool {
	return left.SourceID == right.SourceID && left.SourceRevisionID == right.SourceRevisionID && left.MaterializedThroughBlock == right.MaterializedThroughBlock && left.MaterializedThroughBlockHash == right.MaterializedThroughBlockHash && left.MaterializedThroughTxIndex == right.MaterializedThroughTxIndex && left.MaterializedThroughTxLogOrdinal == right.MaterializedThroughTxLogOrdinal && nullableUint64(left.LastEventIdentityID) == nullableUint64(right.LastEventIdentityID) && left.LeaseToken == right.LeaseToken && left.FenceEpoch == right.FenceEpoch && left.Status == right.Status && left.LastError == right.LastError && left.CursorGeneration == right.CursorGeneration
}

func fairLaunchMainnetCursorEqual(left, right entity.FairLaunchMainnetMaterializationCursorV1) bool {
	return left.ActivationRowID == right.ActivationRowID && left.ChainID == right.ChainID && left.SourceID == right.SourceID && left.SourceRevisionID == right.SourceRevisionID && left.SourceBindingID == right.SourceBindingID && left.MaterializedThroughBlock == right.MaterializedThroughBlock && left.MaterializedThroughBlockHash == right.MaterializedThroughBlockHash && left.MaterializedThroughTxIndex == right.MaterializedThroughTxIndex && left.MaterializedThroughTxLogOrdinal == right.MaterializedThroughTxLogOrdinal && nullableUint64(left.LastEventIdentityID) == nullableUint64(right.LastEventIdentityID) && nullableUint64(left.LastArchiveEventID) == nullableUint64(right.LastArchiveEventID) && left.LastMaterializationReceiptID == nil && right.LastMaterializationReceiptID == nil && left.LeaseGeneration == right.LeaseGeneration && left.FenceEpoch == right.FenceEpoch && left.LeaseTokenHash == right.LeaseTokenHash && left.Status == right.Status && left.CursorGeneration == right.CursorGeneration
}

func fairLaunchDoubleCursorPositionEqual(generic entity.SourceMaterializationCursor, mainnet entity.FairLaunchMainnetMaterializationCursorV1) bool {
	return generic.SourceID == mainnet.SourceID && generic.SourceRevisionID == mainnet.SourceRevisionID && generic.MaterializedThroughBlock == mainnet.MaterializedThroughBlock && generic.MaterializedThroughBlockHash == mainnet.MaterializedThroughBlockHash && generic.MaterializedThroughTxIndex == mainnet.MaterializedThroughTxIndex && generic.MaterializedThroughTxLogOrdinal == mainnet.MaterializedThroughTxLogOrdinal && nullableUint64(generic.LastEventIdentityID) == nullableUint64(mainnet.LastEventIdentityID) && generic.LeaseToken == mainnet.LeaseTokenHash && generic.FenceEpoch == mainnet.FenceEpoch && generic.Status == "active" && generic.LastError == "" && mainnet.Status == "active" && generic.CursorGeneration == mainnet.CursorGeneration
}

func validateFairLaunchMainnetMaterializationCursorReceiptTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, cursor entity.FairLaunchMainnetMaterializationCursorV1) error {
	if cursor.LastMaterializationReceiptID == nil {
		if cursor.CursorGeneration != 1 {
			return errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		return nil
	}
	var receipt entity.FairLaunchMainnetMaterializationReceiptV1
	if err := tx.Where("id=? AND activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND source_binding_id=?", *cursor.LastMaterializationReceiptID, admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.SourceBindingID).Take(&receipt).Error; err != nil {
		return errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	if receipt.ReceiptDigest != fairLaunchMainnetMaterializationReceiptDigest(receipt) || receipt.ArchiveEventID != nullableUint64(cursor.LastArchiveEventID) || receipt.EventIdentityID != nullableUint64(cursor.LastEventIdentityID) || receipt.PreviousCursorGeneration+1 != cursor.CursorGeneration || receipt.LeaseGeneration != cursor.LeaseGeneration || receipt.FenceEpoch != cursor.FenceEpoch {
		return errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	seen := map[string]bool{receipt.ReceiptDigest: true}
	for receipt.PreviousReceiptDigest != nil {
		if seen[*receipt.PreviousReceiptDigest] || uint64(len(seen)) >= cursor.CursorGeneration {
			return errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		var previous entity.FairLaunchMainnetMaterializationReceiptV1
		if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND source_binding_id=? AND receipt_digest=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.SourceBindingID, *receipt.PreviousReceiptDigest).Take(&previous).Error; err != nil || previous.ReceiptDigest != fairLaunchMainnetMaterializationReceiptDigest(previous) || previous.PreviousCursorGeneration+1 != receipt.PreviousCursorGeneration || previous.LeaseGeneration != cursor.LeaseGeneration || previous.FenceEpoch != cursor.FenceEpoch {
			return errors.New(FairLaunchMainnetMaterializationUnavailable)
		}
		seen[previous.ReceiptDigest] = true
		receipt = previous
	}
	if receipt.PreviousCursorGeneration != 1 || uint64(len(seen))+1 != cursor.CursorGeneration {
		return errors.New(FairLaunchMainnetMaterializationUnavailable)
	}
	return nil
}

func fairLaunchMainnetMaterializationReceiptDigest(receipt entity.FairLaunchMainnetMaterializationReceiptV1) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_MATERIALIZATION_RECEIPT_V1", map[string]any{
		"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "sourceBindingId": receipt.SourceBindingID,
		"archiveBatchReceiptId": receipt.ArchiveBatchReceiptID, "archiveEventId": receipt.ArchiveEventID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID,
		"applicationAttemptId": receipt.ApplicationAttemptID, "sourceApplicationLineageId": receipt.SourceApplicationLineageID, "attestationId": receipt.AttestationID, "effectManifestVersion": receipt.EffectManifestVersion,
		"payloadDigest": receipt.PayloadDigest, "effectDigest": receipt.EffectDigest, "ledgerDigest": receipt.LedgerDigest, "mutationDigest": receipt.MutationDigest, "attestationDigest": receipt.AttestationDigest,
		"leaseGeneration": receipt.LeaseGeneration, "fenceEpoch": receipt.FenceEpoch, "previousCursorGeneration": receipt.PreviousCursorGeneration, "previousReceiptDigest": receipt.PreviousReceiptDigest,
	})
}

func FairLaunchMainnetFinancialEffectManifestDigest() string {
	rows := make([]FairLaunchEventEffectManifest, 0, len(fairLaunchMainnetFinancialEvents))
	for _, eventName := range fairLaunchMainnetFinancialEvents {
		manifest, ok := FairLaunchEffectManifest(eventName)
		if !ok {
			return ""
		}
		rows = append(rows, manifest)
	}
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_FINANCIAL_EFFECT_MANIFEST_V1", rows)
}

func fairLaunchMainnetMaterializationProfileDigest(profile entity.FairLaunchMainnetMaterializationProfileV1) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_MATERIALIZATION_PROFILE_V1", map[string]any{
		"activationRowId": profile.ActivationRowID, "chainId": profile.ChainID, "sourceId": profile.SourceID, "sourceRevisionId": profile.SourceRevisionID, "sourceBindingId": profile.SourceBindingID,
		"bootstrapReceiptId": profile.BootstrapReceiptID, "bootstrapArchiveEventId": profile.BootstrapArchiveEventID, "initialEventIdentityId": profile.InitialEventIdentityID,
		"effectManifestVersion": profile.EffectManifestVersion, "effectManifestDigest": profile.EffectManifestDigest,
		"initialBlockNumber": profile.InitialBlockNumber, "initialBlockHash": profile.InitialBlockHash, "initialTransactionIndex": profile.InitialTransactionIndex, "initialTxLogOrdinal": profile.InitialTxLogOrdinal,
		"approvalEvidenceDigest": profile.ApprovalEvidenceDigest, "validFrom": profile.ValidFrom.UTC().Format(time.RFC3339Nano), "validUntil": profile.ValidUntil.UTC().Format(time.RFC3339Nano), "createdAt": profile.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func fairLaunchMainnetMaterializationProfileValid(profile entity.FairLaunchMainnetMaterializationProfileV1) bool {
	return profile.EffectManifestVersion == FairLaunchEffectManifestVersion && profile.EffectManifestDigest == FairLaunchMainnetFinancialEffectManifestDigest() && profile.ProfileDigest == fairLaunchMainnetMaterializationProfileDigest(profile)
}

func nullableUint64(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}
