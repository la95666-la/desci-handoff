package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchMainnetArchiveUnavailable = "fair_launch_mainnet_archive_unavailable"
	FairLaunchMainnetArchiveRejected    = "fair_launch_mainnet_archive_rejected"
	fairLaunchMainnetArchiveMaxBlocks   = uint64(64)
)

type FairLaunchMainnetFinalizedBlock struct {
	Number           uint64
	Hash             string
	ParentHash       string
	ProviderIdentity string
	Events           []FairLaunchMainnetArchiveLog
}

// FairLaunchMainnetArchiveLog must be assembled from a successful transaction
// receipt, not eth_getLogs alone: TxLogOrdinal is the receipt-local log ordinal
// used by CE-V2 identity, while Event.Log.Index is the RPC block log index.
type FairLaunchMainnetArchiveLog struct {
	Event            *commonchain.FairLaunchTypedEvent
	TransactionIndex uint32
	TxLogOrdinal     uint32
	ProviderIdentity string
	TopicsDataDigest string
	RawLogDigest     string
}

type FairLaunchMainnetArchiveBatch struct {
	Blocks []FairLaunchMainnetFinalizedBlock
}

type FairLaunchMainnetArchive struct {
	seal *fairLaunchMainnetArchiveSeal
}

type fairLaunchMainnetArchiveSeal struct {
	event            *commonchain.FairLaunchTypedEvent
	transactionIndex uint32
	txLogOrdinal     uint32
	providerIdentity string
	topicsDataDigest string
	rawLogDigest     string
	evidence         fairLaunchV3ArchiveEvidence
}

type FairLaunchMainnetArchiveBatchResult struct {
	BatchReceiptID uint64
	FromBlock      uint64
	ThroughBlock   uint64
	EventCount     uint32
	ReceiptDigest  string
	Events         []FairLaunchMainnetArchive
	Replayed       bool
}

type FairLaunchMainnetArchiveFailure struct {
	ReasonCode       string
	BlockNumber      *uint64
	BlockHash        *string
	TransactionIndex *uint32
	TxLogOrdinal     *uint32
	RPCLogIndex      *uint32
	TxHash           *string
}

type FairLaunchMainnetArchiveFailureResult struct {
	DeadLetterID   uint64
	CursorStatus   string
	EvidenceDigest string
	Replayed       bool
}

type FairLaunchMainnetArchiveRecovery struct {
	DeadLetterID                  uint64
	PreviousCursorGeneration      uint64
	RecoveryAction                string
	VerificationProviderSetDigest string
	VerificationCount             uint32
	CanonicalChainDigest          string
	ApprovalEvidenceDigest        string
	RecoveryManifestDigest        string
}

type FairLaunchMainnetArchiveRecoveryResult struct {
	RecoveryReceiptID uint64
	CursorStatus      string
	CursorGeneration  uint64
	LeaseGeneration   uint64
	FenceEpoch        uint64
	ReceiptDigest     string
	Replayed          bool
}

func InitializeFairLaunchMainnetArchiveCursorTx(capability FairLaunchMainnetCapability, tx *gorm.DB, sourceStartBlockHash, sourceParentBlockHash string, now time.Time) error {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || admission.Component != "job" || admission.SourceStartBlock == 0 || sourceStartBlockHash != admission.SourceStartBlockHash || !fairLaunchLocalHashPattern.MatchString(sourceStartBlockHash) || !fairLaunchLocalHashPattern.MatchString(sourceParentBlockHash) {
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	if err := validateFairLaunchMainnetArchiveLeaseTx(tx, admission, leaseHash, now); err != nil {
		return err
	}
	cursor := entity.FairLaunchMainnetArchiveCursorV1{
		ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID,
		ArchivedThroughBlock: admission.SourceStartBlock - 1, ArchivedThroughBlockHash: sourceParentBlockHash,
		LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, LeaseTokenHash: leaseHash,
		Status: "active", CursorGeneration: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	var existing entity.FairLaunchMainnetArchiveCursorV1
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=?", admission.AuthorityRowID).Take(&existing).Error
	if err == nil {
		if existing.ChainID == cursor.ChainID && existing.SourceID == cursor.SourceID && existing.SourceRevisionID == cursor.SourceRevisionID && existing.ArchivedThroughBlock == cursor.ArchivedThroughBlock && existing.ArchivedThroughBlockHash == cursor.ArchivedThroughBlockHash && existing.LeaseGeneration == cursor.LeaseGeneration && existing.FenceEpoch == cursor.FenceEpoch && existing.LeaseTokenHash == cursor.LeaseTokenHash && existing.Status == "active" && existing.LastErrorCode == nil && existing.LastErrorDigest == nil && existing.LastBatchReceiptID == nil && existing.LastRecoveryReceiptID == nil && existing.CursorGeneration == 1 {
			return nil
		}
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	if err := validateFairLaunchMainnetArchiveSourceTx(tx, admission); err != nil {
		return err
	}
	return tx.Create(&cursor).Error
}

// ArchiveFairLaunchMainnetCollectedBatchTx accepts only the opaque result of
// CollectFairLaunchMainnetFinalizedBatch. This prevents callers outside this
// package from bypassing live RPC/finality verification with a fabricated
// repository batch.
func ArchiveFairLaunchMainnetCollectedBatchTx(capability FairLaunchMainnetCapability, tx *gorm.DB, collected FairLaunchMainnetCollectedBatch, now time.Time) (FairLaunchMainnetArchiveBatchResult, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || admission.Component != "job" || !validFairLaunchMainnetCollectedBatch(admission, collected) {
		return FairLaunchMainnetArchiveBatchResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	return archiveFairLaunchMainnetBatchTx(capability, tx, collected.seal.batch, now)
}

func validFairLaunchMainnetCollectedBatch(admission FairLaunchMainnetAdmission, collected FairLaunchMainnetCollectedBatch) bool {
	return collected.seal != nil && collected.seal.authorityRowID == admission.AuthorityRowID && collected.seal.chainID == admission.ChainID && collected.seal.sourceID == admission.SourceID && collected.seal.sourceRevisionID == admission.SourceRevisionID && collected.seal.leaseGeneration == admission.LeaseGeneration && collected.seal.fenceEpoch == admission.FenceEpoch && collected.seal.rpcIdentityDigest == admission.RPCIdentityDigest && collected.seal.batchDigest == fairLaunchMainnetCollectedBatchDigest(collected.seal.batch)
}

// archiveFairLaunchMainnetBatchTx is repository-only. RPC collection/finality
// verification and job scheduling remain deliberately disconnected. The
// function consumes a receipt-derived finalized batch, archives CE-V2 rows,
// writes immutable batch evidence, and advances the source cursor atomically.
func archiveFairLaunchMainnetBatchTx(capability FairLaunchMainnetCapability, tx *gorm.DB, batch FairLaunchMainnetArchiveBatch, now time.Time) (FairLaunchMainnetArchiveBatchResult, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || admission.Component != "job" || len(batch.Blocks) == 0 || len(batch.Blocks) > int(fairLaunchMainnetArchiveMaxBlocks) {
		return FairLaunchMainnetArchiveBatchResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	if err := validateFairLaunchMainnetArchiveLeaseTx(tx, admission, leaseHash, now); err != nil {
		return FairLaunchMainnetArchiveBatchResult{}, err
	}
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND last_error_code IS NULL AND last_error_digest IS NULL", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash).Take(&cursor).Error; err != nil {
		return FairLaunchMainnetArchiveBatchResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	if err := validateFairLaunchMainnetArchiveSourceTx(tx, admission); err != nil {
		return FairLaunchMainnetArchiveBatchResult{}, err
	}
	if batch.Blocks[0].Number <= cursor.ArchivedThroughBlock {
		return replayFairLaunchMainnetArchiveBatchTx(tx, admission, cursor, batch)
	}
	if err := validateFairLaunchMainnetArchiveBatch(admission, cursor, batch); err != nil {
		return FairLaunchMainnetArchiveBatchResult{}, err
	}
	now = now.UTC()
	archives := make([]FairLaunchMainnetArchive, 0)
	eventRows := make([]entity.FairLaunchMainnetArchiveBatchEventV1, 0)
	for _, block := range batch.Blocks {
		for _, archivedLog := range block.Events {
			archive, row, err := archiveFairLaunchMainnetLogTx(tx, admission, archivedLog, now, uint32(len(eventRows)))
			if err != nil {
				return FairLaunchMainnetArchiveBatchResult{}, err
			}
			archives = append(archives, archive)
			eventRows = append(eventRows, row)
		}
	}
	through := batch.Blocks[len(batch.Blocks)-1]
	providerDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_PROVIDER_SET_V1", fairLaunchMainnetArchiveProviders(batch))
	coverageDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_COVERAGE_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "blocks": fairLaunchMainnetArchiveBlockEvidence(batch), "eventRows": fairLaunchMainnetArchiveRowDigests(eventRows)})
	var previousDigest *string
	if cursor.LastBatchReceiptID != nil {
		var previous entity.FairLaunchMainnetArchiveBatchReceiptV1
		if err := tx.Where("id=? AND activation_row_id=?", *cursor.LastBatchReceiptID, admission.AuthorityRowID).Take(&previous).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, errors.New(FairLaunchMainnetArchiveRejected)
		}
		value := previous.ReceiptDigest
		previousDigest = &value
	}
	receipt := entity.FairLaunchMainnetArchiveBatchReceiptV1{
		ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID,
		LeaseGeneration: admission.LeaseGeneration, FenceEpoch: admission.FenceEpoch, FromBlock: batch.Blocks[0].Number, ThroughBlock: through.Number,
		FromParentBlockHash: batch.Blocks[0].ParentHash, ThroughBlockHash: through.Hash, ProviderIdentityDigest: providerDigest,
		BlockCount: uint32(len(batch.Blocks)), EventCount: uint32(len(eventRows)), CoverageDigest: coverageDigest, PreviousReceiptDigest: previousDigest, CreatedAt: now,
	}
	receipt.ReceiptDigest = fairLaunchMainnetArchiveReceiptDigest(receipt)
	if err := tx.Create(&receipt).Error; err != nil {
		return FairLaunchMainnetArchiveBatchResult{}, err
	}
	for index := range eventRows {
		eventRows[index].BatchReceiptID = receipt.ID
		eventRows[index].ActivationRowID = admission.AuthorityRowID
		if err := tx.Create(&eventRows[index]).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, err
		}
		payload, err := fairLaunchMainnetArchivePayloadRow(admission.AuthorityRowID, eventRows[index], archives[index].seal.event.Log, now)
		if err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, err
		}
		if err := tx.Create(&payload).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, err
		}
	}
	cursor.ArchivedThroughBlock = through.Number
	cursor.ArchivedThroughBlockHash = through.Hash
	cursor.LastBatchReceiptID = uint64Pointer(receipt.ID)
	cursor.CursorGeneration++
	cursor.UpdatedAt = now
	update := tx.Model(&entity.FairLaunchMainnetArchiveCursorV1{}).Where("activation_row_id=? AND cursor_generation=? AND status='active'", cursor.ActivationRowID, cursor.CursorGeneration-1).UpdateColumns(map[string]any{"archived_through_block": cursor.ArchivedThroughBlock, "archived_through_block_hash": cursor.ArchivedThroughBlockHash, "last_batch_receipt_id": cursor.LastBatchReceiptID, "cursor_generation": cursor.CursorGeneration, "updated_at": cursor.UpdatedAt})
	if update.Error != nil || update.RowsAffected != 1 {
		if update.Error != nil {
			return FairLaunchMainnetArchiveBatchResult{}, update.Error
		}
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: cursor compare-and-swap failed", FairLaunchMainnetArchiveRejected)
	}
	return FairLaunchMainnetArchiveBatchResult{BatchReceiptID: receipt.ID, FromBlock: receipt.FromBlock, ThroughBlock: receipt.ThroughBlock, EventCount: receipt.EventCount, ReceiptDigest: receipt.ReceiptDigest, Events: archives}, nil
}

// BlockFairLaunchMainnetArchiveCursorTx is the failure-side repository seam.
// Callers must invoke it in a new transaction after the failed archive
// transaction has rolled back. It atomically appends one immutable dead letter
// and blocks the exact source cursor without preserving partial archive rows.
func BlockFairLaunchMainnetArchiveCursorTx(capability FairLaunchMainnetCapability, tx *gorm.DB, failure FairLaunchMainnetArchiveFailure, now time.Time) (FairLaunchMainnetArchiveFailureResult, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || admission.Component != "job" {
		return FairLaunchMainnetArchiveFailureResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	status, ok := fairLaunchMainnetArchiveFailureStatus(failure.ReasonCode)
	if !ok || !validFairLaunchMainnetArchiveFailureCoordinates(failure) {
		return FairLaunchMainnetArchiveFailureResult{}, fmt.Errorf("%s: dead letter invalid", FairLaunchMainnetArchiveRejected)
	}
	failure = normalizeFairLaunchMainnetArchiveFailure(failure)
	evidenceDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_DEAD_LETTER_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "chainId": admission.ChainID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "reasonCode": failure.ReasonCode, "blockNumber": failure.BlockNumber, "blockHash": failure.BlockHash, "transactionIndex": failure.TransactionIndex, "txLogOrdinal": failure.TxLogOrdinal, "rpcLogIndex": failure.RPCLogIndex, "txHash": failure.TxHash})
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	if err := validateFairLaunchMainnetArchiveLeaseTx(tx, admission, leaseHash, now); err != nil {
		return FairLaunchMainnetArchiveFailureResult{}, err
	}
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash).Take(&cursor).Error; err != nil {
		return FairLaunchMainnetArchiveFailureResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	if cursor.Status != "active" {
		if cursor.Status != status || cursor.LastErrorCode == nil || *cursor.LastErrorCode != failure.ReasonCode || cursor.LastErrorDigest == nil || *cursor.LastErrorDigest != evidenceDigest {
			return FairLaunchMainnetArchiveFailureResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
		}
		var existing entity.FairLaunchMainnetArchiveDeadLetterV1
		if err := tx.Where("activation_row_id=? AND evidence_digest=? AND reason_code=?", admission.AuthorityRowID, evidenceDigest, failure.ReasonCode).Take(&existing).Error; err != nil {
			return FairLaunchMainnetArchiveFailureResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
		}
		return FairLaunchMainnetArchiveFailureResult{DeadLetterID: existing.ID, CursorStatus: cursor.Status, EvidenceDigest: evidenceDigest, Replayed: true}, nil
	}
	deadLetter := entity.FairLaunchMainnetArchiveDeadLetterV1{ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, BlockNumber: failure.BlockNumber, BlockHash: failure.BlockHash, TransactionIndex: failure.TransactionIndex, TxLogOrdinal: failure.TxLogOrdinal, RPCLogIndex: failure.RPCLogIndex, TxHash: failure.TxHash, ReasonCode: failure.ReasonCode, EvidenceDigest: evidenceDigest, CreatedAt: now.UTC()}
	if err := tx.Create(&deadLetter).Error; err != nil {
		return FairLaunchMainnetArchiveFailureResult{}, err
	}
	cursor.Status = status
	cursor.LastErrorCode = stringPointer(failure.ReasonCode)
	cursor.LastErrorDigest = stringPointer(evidenceDigest)
	cursor.CursorGeneration++
	cursor.UpdatedAt = now.UTC()
	update := tx.Model(&entity.FairLaunchMainnetArchiveCursorV1{}).Where("activation_row_id=? AND cursor_generation=? AND status='active'", cursor.ActivationRowID, cursor.CursorGeneration-1).UpdateColumns(map[string]any{"status": cursor.Status, "last_error_code": cursor.LastErrorCode, "last_error_digest": cursor.LastErrorDigest, "cursor_generation": cursor.CursorGeneration, "updated_at": cursor.UpdatedAt})
	if update.Error != nil || update.RowsAffected != 1 {
		if update.Error != nil {
			return FairLaunchMainnetArchiveFailureResult{}, update.Error
		}
		return FairLaunchMainnetArchiveFailureResult{}, fmt.Errorf("%s: blocker compare-and-swap failed", FairLaunchMainnetArchiveRejected)
	}
	return FairLaunchMainnetArchiveFailureResult{DeadLetterID: deadLetter.ID, CursorStatus: status, EvidenceDigest: evidenceDigest}, nil
}

// recoverFairLaunchMainnetArchiveCursorTx is package-private. Human/provider
// verification and replacement-lease admission happen outside this repository;
// this seam only persists exact evidence and performs the receipt-backed cursor
// transition atomically.
func recoverFairLaunchMainnetArchiveCursorTx(capability FairLaunchMainnetCapability, tx *gorm.DB, recovery FairLaunchMainnetArchiveRecovery, now time.Time) (FairLaunchMainnetArchiveRecoveryResult, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || admission.Component != "job" || recovery.DeadLetterID == 0 || recovery.PreviousCursorGeneration == 0 || !validFairLaunchMainnetArchiveRecovery(recovery) {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	var replacementLease entity.FairLaunchMainnetActivationLeaseV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND active_slot=1 AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash, now.UTC(), now.UTC()).Take(&replacementLease).Error; err != nil {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	var existingReceipt entity.FairLaunchMainnetArchiveRecoveryReceiptV1
	if err := tx.Where("activation_row_id=? AND dead_letter_id=? AND previous_cursor_generation=?", admission.AuthorityRowID, recovery.DeadLetterID, recovery.PreviousCursorGeneration).Take(&existingReceipt).Error; err == nil {
		return replayFairLaunchMainnetArchiveRecoveryTx(tx, admission, replacementLease, recovery, existingReceipt)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND cursor_generation=? AND status IN('blocked','reorg_detected')", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, recovery.PreviousCursorGeneration).Take(&cursor).Error; err != nil || cursor.LastErrorCode == nil || cursor.LastErrorDigest == nil || cursor.LastRecoveryReceiptID != nil {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	var deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1
	if err := tx.Where("id=? AND activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND reason_code=? AND evidence_digest=?", recovery.DeadLetterID, admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, *cursor.LastErrorCode, *cursor.LastErrorDigest).Take(&deadLetter).Error; err != nil {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	var previousLease entity.FairLaunchMainnetActivationLeaseV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status IN('revoked','expired') AND active_slot IS NULL", admission.AuthorityRowID, admission.ChainID, cursor.LeaseGeneration, cursor.FenceEpoch, cursor.LeaseTokenHash).Take(&previousLease).Error; err != nil || replacementLease.LeaseGeneration <= previousLease.LeaseGeneration || replacementLease.FenceEpoch <= previousLease.FenceEpoch {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	receipt := entity.FairLaunchMainnetArchiveRecoveryReceiptV1{ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, DeadLetterID: deadLetter.ID, PreviousStatus: cursor.Status, ReasonCode: deadLetter.ReasonCode, PreviousErrorDigest: deadLetter.EvidenceDigest, ArchivedThroughBlock: cursor.ArchivedThroughBlock, ArchivedThroughBlockHash: cursor.ArchivedThroughBlockHash, LastBatchReceiptID: cursor.LastBatchReceiptID, PreviousCursorGeneration: cursor.CursorGeneration, PreviousLeaseID: previousLease.ID, ReplacementLeaseID: replacementLease.ID, RecoveryAction: recovery.RecoveryAction, ConfirmationPolicy: admission.ConfirmationPolicy, VerificationProviderSetDigest: recovery.VerificationProviderSetDigest, VerificationCount: recovery.VerificationCount, CanonicalChainDigest: recovery.CanonicalChainDigest, ApprovalEvidenceDigest: recovery.ApprovalEvidenceDigest, RecoveryManifestDigest: recovery.RecoveryManifestDigest, CreatedAt: now.UTC()}
	if !fairLaunchMainnetArchiveRecoveryMatches(receipt.PreviousStatus, receipt.ReasonCode, receipt.RecoveryAction, receipt.VerificationCount) {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	receipt.ReceiptDigest = fairLaunchMainnetArchiveRecoveryReceiptDigest(receipt)
	if err := tx.Create(&receipt).Error; err != nil {
		return FairLaunchMainnetArchiveRecoveryResult{}, err
	}
	nextGeneration := cursor.CursorGeneration + 1
	update := tx.Model(&entity.FairLaunchMainnetArchiveCursorV1{}).Where("activation_row_id=? AND cursor_generation=? AND status=? AND last_error_digest=?", cursor.ActivationRowID, cursor.CursorGeneration, cursor.Status, deadLetter.EvidenceDigest).UpdateColumns(map[string]any{"lease_generation": replacementLease.LeaseGeneration, "fence_epoch": replacementLease.FenceEpoch, "lease_token_hash": replacementLease.LeaseTokenHash, "status": "active", "last_error_code": nil, "last_error_digest": nil, "last_recovery_receipt_id": receipt.ID, "cursor_generation": nextGeneration, "updated_at": now.UTC()})
	if update.Error != nil || update.RowsAffected != 1 {
		if update.Error != nil {
			return FairLaunchMainnetArchiveRecoveryResult{}, update.Error
		}
		return FairLaunchMainnetArchiveRecoveryResult{}, fmt.Errorf("%s: recovery compare-and-swap failed", FairLaunchMainnetArchiveRejected)
	}
	return FairLaunchMainnetArchiveRecoveryResult{RecoveryReceiptID: receipt.ID, CursorStatus: "active", CursorGeneration: nextGeneration, LeaseGeneration: replacementLease.LeaseGeneration, FenceEpoch: replacementLease.FenceEpoch, ReceiptDigest: receipt.ReceiptDigest}, nil
}

func replayFairLaunchMainnetArchiveBatchTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, cursor entity.FairLaunchMainnetArchiveCursorV1, batch FairLaunchMainnetArchiveBatch) (FairLaunchMainnetArchiveBatchResult, error) {
	through := batch.Blocks[len(batch.Blocks)-1]
	if through.Number > cursor.ArchivedThroughBlock {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay overlaps cursor", FairLaunchMainnetArchiveRejected)
	}
	var receipt entity.FairLaunchMainnetArchiveBatchReceiptV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND from_block=? AND through_block=? AND from_parent_block_hash=? AND through_block_hash=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, batch.Blocks[0].Number, through.Number, batch.Blocks[0].ParentHash, through.Hash).Take(&receipt).Error; err != nil || receipt.LeaseGeneration != admission.LeaseGeneration || receipt.FenceEpoch != admission.FenceEpoch || receipt.BlockCount != uint32(len(batch.Blocks)) {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay receipt conflict", FairLaunchMainnetArchiveRejected)
	}
	if receipt.ProviderIdentityDigest != fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_PROVIDER_SET_V1", fairLaunchMainnetArchiveProviders(batch)) {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay provider conflict", FairLaunchMainnetArchiveRejected)
	}
	incoming := fairLaunchMainnetArchiveLogs(batch)
	if receipt.EventCount != uint32(len(incoming)) {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay event count conflict", FairLaunchMainnetArchiveRejected)
	}
	var persisted []entity.FairLaunchMainnetArchiveBatchEventV1
	if err := tx.Where("batch_receipt_id=? AND activation_row_id=?", receipt.ID, admission.AuthorityRowID).Order("event_ordinal ASC").Find(&persisted).Error; err != nil || len(persisted) != len(incoming) {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay event rows unavailable", FairLaunchMainnetArchiveRejected)
	}
	archives := make([]FairLaunchMainnetArchive, 0, len(incoming))
	rowDigests := make([]string, len(persisted))
	for index, archivedLog := range incoming {
		block := fairLaunchMainnetArchiveBlockForLog(batch, archivedLog)
		if err := validateFairLaunchMainnetArchiveLog(admission, block, archivedLog); err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, err
		}
		payloadDigest, err := commonchain.FairLaunchTypedEventDigest(archivedLog.Event)
		if err != nil || persisted[index].EventOrdinal != uint32(index) || persisted[index].EventName != archivedLog.Event.Name || persisted[index].PayloadDigest != payloadDigest {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay typed payload conflict", FairLaunchMainnetArchiveRejected)
		}
		var identity entity.ChainEventIdentityV2
		if err := tx.Where("id=? AND chain_id=? AND tx_hash=? AND tx_log_ordinal=?", persisted[index].EventIdentityID, admission.ChainID, strings.ToLower(archivedLog.Event.Log.TxHash.Hex()), archivedLog.TxLogOrdinal).Take(&identity).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay identity conflict", FairLaunchMainnetArchiveRejected)
		}
		var placement entity.ChainEventPlacementV2
		if err := tx.Where("id=? AND event_identity_id=? AND block_number=? AND block_hash=? AND transaction_index=? AND rpc_log_index=? AND source_revision_id=? AND canonical_slot=1 AND finalized_at IS NOT NULL AND noncanonical_at IS NULL", persisted[index].PlacementID, identity.ID, archivedLog.Event.Log.BlockNumber, strings.ToLower(archivedLog.Event.Log.BlockHash.Hex()), archivedLog.TransactionIndex, uint32(archivedLog.Event.Log.Index), admission.SourceRevisionID).Take(&placement).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay placement conflict", FairLaunchMainnetArchiveRejected)
		}
		var observation entity.ChainEventPlacementObservationV2
		if err := tx.Where("id=? AND placement_id=? AND event_identity_id=? AND provider_identity=? AND emitting_address=? AND topics_data_digest=? AND raw_log_digest=?", persisted[index].ObservationID, placement.ID, identity.ID, archivedLog.ProviderIdentity, admission.ProxyAddress, stripCE(archivedLog.TopicsDataDigest), stripCE(archivedLog.RawLogDigest)).Take(&observation).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay observation conflict", FairLaunchMainnetArchiveRejected)
		}
		expectedRowDigest := fairLaunchMainnetArchiveEventRowDigest(admission, persisted[index].EventOrdinal, identity.ID, placement.ID, observation.ID, persisted[index].EventName, persisted[index].PayloadDigest)
		if persisted[index].RowDigest != expectedRowDigest {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay event digest conflict", FairLaunchMainnetArchiveRejected)
		}
		var persistedPayload entity.FairLaunchMainnetArchiveEventPayloadV1
		if err := tx.Where("archive_event_id=? AND activation_row_id=? AND event_identity_id=? AND placement_id=? AND observation_id=?", persisted[index].ID, admission.AuthorityRowID, identity.ID, placement.ID, observation.ID).Take(&persistedPayload).Error; err != nil {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay event payload unavailable", FairLaunchMainnetArchiveRejected)
		}
		expectedPayload, err := fairLaunchMainnetArchivePayloadRow(admission.AuthorityRowID, persisted[index], archivedLog.Event.Log, persistedPayload.CreatedAt)
		if err != nil || !fairLaunchMainnetArchivePayloadEqual(persistedPayload, expectedPayload) {
			return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay event payload conflict", FairLaunchMainnetArchiveRejected)
		}
		rowDigests[index] = persisted[index].RowDigest
		evidence := fairLaunchV3ArchiveEvidence{IdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, PlacementGeneration: placement.Generation, SourceRevisionID: placement.SourceRevisionID, BlockNumber: placement.BlockNumber, BlockHash: placement.BlockHash, TransactionIndex: placement.TransactionIndex, TxLogOrdinal: archivedLog.TxLogOrdinal, RPCLogIndex: placement.RPCLogIndex}
		archives = append(archives, FairLaunchMainnetArchive{seal: &fairLaunchMainnetArchiveSeal{event: archivedLog.Event, transactionIndex: archivedLog.TransactionIndex, txLogOrdinal: archivedLog.TxLogOrdinal, providerIdentity: archivedLog.ProviderIdentity, topicsDataDigest: stripCE(archivedLog.TopicsDataDigest), rawLogDigest: stripCE(archivedLog.RawLogDigest), evidence: evidence}})
	}
	coverageDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_COVERAGE_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "blocks": fairLaunchMainnetArchiveBlockEvidence(batch), "eventRows": rowDigests})
	if receipt.CoverageDigest != coverageDigest || receipt.ReceiptDigest != fairLaunchMainnetArchiveReceiptDigest(receipt) {
		return FairLaunchMainnetArchiveBatchResult{}, fmt.Errorf("%s: replay batch digest conflict", FairLaunchMainnetArchiveRejected)
	}
	return FairLaunchMainnetArchiveBatchResult{BatchReceiptID: receipt.ID, FromBlock: receipt.FromBlock, ThroughBlock: receipt.ThroughBlock, EventCount: receipt.EventCount, ReceiptDigest: receipt.ReceiptDigest, Events: archives, Replayed: true}, nil
}

func validateFairLaunchMainnetArchiveSourceTx(tx *gorm.DB, admission FairLaunchMainnetAdmission) error {
	var source entity.ChainContractSource
	if err := tx.Where("id=? AND chain_id=? AND contract_address=? AND contract_version=? AND funding_mode=? AND start_block=? AND enabled=1 AND retired_at IS NULL", admission.SourceID, admission.ChainID, admission.ProxyAddress, fairLaunchLocalContractVersion, fairLaunchLocalFundingMode, admission.SourceStartBlock).Take(&source).Error; err != nil {
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	var revision entity.ChainContractSourceRevision
	if err := tx.Where("id=? AND source_id=? AND from_block_number=? AND from_transaction_index=0 AND from_tx_log_ordinal_inclusive=0 AND to_block_number IS NULL AND to_transaction_index IS NULL AND to_tx_log_ordinal_exclusive IS NULL AND activation_block_hash=? AND proxy_kind='uups' AND implementation_address=? AND implementation_code_hash=? AND abi_digest=? AND confirmation_policy=?", admission.SourceRevisionID, admission.SourceID, admission.SourceStartBlock, admission.SourceStartBlockHash, admission.ImplementationAddress, admission.ImplementationCodeHash, admission.ABIDigest, admission.ConfirmationPolicy).Take(&revision).Error; err != nil {
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	return nil
}

func validateFairLaunchMainnetArchiveLeaseTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, leaseHash string, now time.Time) error {
	var lease entity.FairLaunchMainnetActivationLeaseV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND active_slot=1 AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash, now.UTC(), now.UTC()).Take(&lease).Error; err != nil {
		return errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	return nil
}

func validateFairLaunchMainnetArchiveBatch(admission FairLaunchMainnetAdmission, cursor entity.FairLaunchMainnetArchiveCursorV1, batch FairLaunchMainnetArchiveBatch) error {
	first := batch.Blocks[0]
	if cursor.ArchivedThroughBlock == math.MaxUint64 || first.Number != cursor.ArchivedThroughBlock+1 || first.ParentHash != cursor.ArchivedThroughBlockHash {
		return fmt.Errorf("%s: batch predecessor mismatch", FairLaunchMainnetArchiveRejected)
	}
	if first.Number == admission.SourceStartBlock && first.Hash != admission.SourceStartBlockHash {
		return fmt.Errorf("%s: source start hash mismatch", FairLaunchMainnetArchiveRejected)
	}
	previousHash := first.ParentHash
	previousNumber := first.Number - 1
	for _, block := range batch.Blocks {
		providerDigest, providerErr := FairLaunchMainnetRPCIdentityDigest(block.ProviderIdentity)
		if block.Number != previousNumber+1 || block.ParentHash != previousHash || !fairLaunchLocalHashPattern.MatchString(block.Hash) || !fairLaunchLocalHashPattern.MatchString(block.ParentHash) || providerErr != nil || providerDigest != admission.RPCIdentityDigest {
			return fmt.Errorf("%s: non-contiguous finalized blocks", FairLaunchMainnetArchiveRejected)
		}
		var previousTx uint32
		var previousLog uint32
		for index, event := range block.Events {
			if err := validateFairLaunchMainnetArchiveLog(admission, block, event); err != nil {
				return err
			}
			if index > 0 && (event.TransactionIndex < previousTx || event.TransactionIndex == previousTx && event.TxLogOrdinal <= previousLog) {
				return fmt.Errorf("%s: archived event order invalid", FairLaunchMainnetArchiveRejected)
			}
			previousTx, previousLog = event.TransactionIndex, event.TxLogOrdinal
		}
		previousNumber, previousHash = block.Number, block.Hash
	}
	return nil
}

func validateFairLaunchMainnetArchiveLog(admission FairLaunchMainnetAdmission, block FairLaunchMainnetFinalizedBlock, archivedLog FairLaunchMainnetArchiveLog) error {
	event := archivedLog.Event
	if event == nil || event.Name == "" || event.Log.Removed || strings.ToLower(event.Log.Address.Hex()) != admission.ProxyAddress || event.Log.BlockNumber != block.Number || strings.ToLower(event.Log.BlockHash.Hex()) != block.Hash || uint64(event.Log.TxIndex) != uint64(archivedLog.TransactionIndex) || uint64(event.Log.Index) > math.MaxUint32 || archivedLog.ProviderIdentity != block.ProviderIdentity || !fairLaunchLocalDigestPattern.MatchString(stripCE(archivedLog.TopicsDataDigest)) || !fairLaunchLocalDigestPattern.MatchString(stripCE(archivedLog.RawLogDigest)) {
		return fmt.Errorf("%s: archived log envelope invalid", FairLaunchMainnetArchiveRejected)
	}
	if len(event.Log.Topics) == 0 {
		return fmt.Errorf("%s: archived event topic missing", FairLaunchMainnetArchiveRejected)
	}
	name, ok := commonchain.FairLaunchEventName(event.Log.Topics[0])
	if !ok || name != event.Name {
		return fmt.Errorf("%s: archived event is not frozen", FairLaunchMainnetArchiveRejected)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil {
		return fmt.Errorf("%s: typed event invalid", FairLaunchMainnetArchiveRejected)
	}
	decoded, err := commonchain.DecodeFairLaunchTypedLog(event.Log)
	if err != nil || decoded == nil || decoded.Name != event.Name {
		return fmt.Errorf("%s: raw log decode mismatch", FairLaunchMainnetArchiveRejected)
	}
	decodedDigest, err := commonchain.FairLaunchTypedEventDigest(decoded)
	if err != nil || decodedDigest != payloadDigest {
		return fmt.Errorf("%s: typed event does not match raw log", FairLaunchMainnetArchiveRejected)
	}
	expectedTopicsDataDigest, expectedRawLogDigest := fairLaunchLocalLogDigests(event.Log)
	if stripCE(archivedLog.TopicsDataDigest) != expectedTopicsDataDigest || stripCE(archivedLog.RawLogDigest) != expectedRawLogDigest {
		return fmt.Errorf("%s: archived log digest mismatch", FairLaunchMainnetArchiveRejected)
	}
	return nil
}

func archiveFairLaunchMainnetLogTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, archivedLog FairLaunchMainnetArchiveLog, now time.Time, eventOrdinal uint32) (FairLaunchMainnetArchive, entity.FairLaunchMainnetArchiveBatchEventV1, error) {
	event := archivedLog.Event
	txHash := strings.ToLower(event.Log.TxHash.Hex())
	blockHash := strings.ToLower(event.Log.BlockHash.Hex())
	if !fairLaunchLocalHashPattern.MatchString(txHash) || txHash == "0x"+strings.Repeat("0", 64) {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, fmt.Errorf("%s: transaction identity invalid", FairLaunchMainnetArchiveRejected)
	}
	var identity entity.ChainEventIdentityV2
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND tx_hash=? AND tx_log_ordinal=?", admission.ChainID, txHash, archivedLog.TxLogOrdinal).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		identity = entity.ChainEventIdentityV2{ChainID: admission.ChainID, TxHash: txHash, TxLogOrdinal: archivedLog.TxLogOrdinal, CreatedAt: now}
		if err = tx.Create(&identity).Error; err != nil {
			return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
		}
	} else if err != nil {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
	}
	var placement entity.ChainEventPlacementV2
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_identity_id=? AND canonical_slot=1", identity.ID).Take(&placement).Error
	if err == nil {
		if placement.BlockNumber != event.Log.BlockNumber || placement.BlockHash != blockHash || placement.TransactionIndex != archivedLog.TransactionIndex || placement.RPCLogIndex != uint32(event.Log.Index) || placement.SourceRevisionID != admission.SourceRevisionID || placement.FinalizedAt == nil || placement.NoncanonicalAt != nil {
			return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, fmt.Errorf("%s: canonical placement conflict", FairLaunchMainnetArchiveRejected)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		var generation uint32
		if err := tx.Model(&entity.ChainEventPlacementV2{}).Where("event_identity_id=?", identity.ID).Select("COALESCE(MAX(generation),0)").Scan(&generation).Error; err != nil {
			return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
		}
		generation++
		canonical := uint8(1)
		placement = entity.ChainEventPlacementV2{EventIdentityID: identity.ID, Generation: generation, BlockNumber: event.Log.BlockNumber, BlockHash: blockHash, TransactionIndex: archivedLog.TransactionIndex, RPCLogIndex: uint32(event.Log.Index), SourceRevisionID: admission.SourceRevisionID, CanonicalSlot: &canonical, FinalizedAt: localTimePointer(now)}
		if err := tx.Create(&placement).Error; err != nil {
			return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
		}
	} else {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
	}
	var openConflictCount int64
	if err := tx.Model(&entity.ChainEventPayloadConflictV2{}).Where("event_identity_id=? AND state='open'", identity.ID).Count(&openConflictCount).Error; err != nil || openConflictCount != 0 {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, fmt.Errorf("%s: event payload conflict", FairLaunchMainnetArchiveRejected)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, err
	}
	observation := entity.ChainEventPlacementObservationV2{PlacementID: placement.ID, EventIdentityID: identity.ID, ProviderIdentity: archivedLog.ProviderIdentity, EmittingAddress: admission.ProxyAddress, TopicsDataDigest: stripCE(archivedLog.TopicsDataDigest), RawLogDigest: stripCE(archivedLog.RawLogDigest), ObservedAt: now}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&observation)
	if result.Error != nil {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, result.Error
	}
	if result.RowsAffected == 0 {
		if err := tx.Where("placement_id=? AND event_identity_id=? AND provider_identity=? AND emitting_address=? AND topics_data_digest=? AND raw_log_digest=?", placement.ID, identity.ID, archivedLog.ProviderIdentity, admission.ProxyAddress, stripCE(archivedLog.TopicsDataDigest), stripCE(archivedLog.RawLogDigest)).Take(&observation).Error; err != nil {
			return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, fmt.Errorf("%s: observation replay conflict", FairLaunchMainnetArchiveRejected)
		}
	}
	var observationCount int64
	if err := tx.Model(&entity.ChainEventPlacementObservationV2{}).Where("placement_id=? AND event_identity_id=? AND (emitting_address<>? OR topics_data_digest<>? OR raw_log_digest<>?)", placement.ID, identity.ID, admission.ProxyAddress, stripCE(archivedLog.TopicsDataDigest), stripCE(archivedLog.RawLogDigest)).Count(&observationCount).Error; err != nil || observationCount != 0 {
		return FairLaunchMainnetArchive{}, entity.FairLaunchMainnetArchiveBatchEventV1{}, fmt.Errorf("%s: observation payload conflict", FairLaunchMainnetArchiveRejected)
	}
	evidence := fairLaunchV3ArchiveEvidence{IdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, PlacementGeneration: placement.Generation, SourceRevisionID: placement.SourceRevisionID, BlockNumber: placement.BlockNumber, BlockHash: placement.BlockHash, TransactionIndex: placement.TransactionIndex, TxLogOrdinal: archivedLog.TxLogOrdinal, RPCLogIndex: placement.RPCLogIndex}
	row := entity.FairLaunchMainnetArchiveBatchEventV1{EventOrdinal: eventOrdinal, EventIdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, EventName: event.Name, PayloadDigest: payloadDigest, CreatedAt: now}
	row.RowDigest = fairLaunchMainnetArchiveEventRowDigest(admission, row.EventOrdinal, row.EventIdentityID, row.PlacementID, row.ObservationID, row.EventName, row.PayloadDigest)
	return FairLaunchMainnetArchive{seal: &fairLaunchMainnetArchiveSeal{event: event, transactionIndex: archivedLog.TransactionIndex, txLogOrdinal: archivedLog.TxLogOrdinal, providerIdentity: archivedLog.ProviderIdentity, topicsDataDigest: stripCE(archivedLog.TopicsDataDigest), rawLogDigest: stripCE(archivedLog.RawLogDigest), evidence: evidence}}, row, nil
}

func fairLaunchMainnetArchiveProviders(batch FairLaunchMainnetArchiveBatch) []string {
	providers := make([]string, 0)
	seen := map[string]bool{}
	for _, block := range batch.Blocks {
		if !seen[block.ProviderIdentity] {
			seen[block.ProviderIdentity] = true
			providers = append(providers, block.ProviderIdentity)
		}
		for _, event := range block.Events {
			if !seen[event.ProviderIdentity] {
				seen[event.ProviderIdentity] = true
				providers = append(providers, event.ProviderIdentity)
			}
		}
	}
	sort.Strings(providers)
	return providers
}

func fairLaunchMainnetArchiveBlockEvidence(batch FairLaunchMainnetArchiveBatch) []map[string]any {
	rows := make([]map[string]any, 0, len(batch.Blocks))
	for _, block := range batch.Blocks {
		rows = append(rows, map[string]any{"number": block.Number, "hash": block.Hash, "parentHash": block.ParentHash, "providerIdentity": block.ProviderIdentity})
	}
	return rows
}

func fairLaunchMainnetArchiveRowDigests(rows []entity.FairLaunchMainnetArchiveBatchEventV1) []string {
	digests := make([]string, len(rows))
	for index := range rows {
		digests[index] = rows[index].RowDigest
	}
	return digests
}

func fairLaunchMainnetArchiveReceiptDigest(receipt entity.FairLaunchMainnetArchiveBatchReceiptV1) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_BATCH_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "leaseGeneration": receipt.LeaseGeneration, "fenceEpoch": receipt.FenceEpoch, "fromBlock": receipt.FromBlock, "throughBlock": receipt.ThroughBlock, "fromParentBlockHash": receipt.FromParentBlockHash, "throughBlockHash": receipt.ThroughBlockHash, "providerIdentityDigest": receipt.ProviderIdentityDigest, "blockCount": receipt.BlockCount, "eventCount": receipt.EventCount, "coverageDigest": receipt.CoverageDigest, "previousReceiptDigest": receipt.PreviousReceiptDigest})
}

func fairLaunchMainnetArchiveEventRowDigest(admission FairLaunchMainnetAdmission, eventOrdinal uint32, identityID, placementID, observationID uint64, eventName, payloadDigest string) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_BATCH_EVENT_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "eventOrdinal": eventOrdinal, "eventIdentityId": identityID, "placementId": placementID, "observationId": observationID, "eventName": eventName, "payloadDigest": payloadDigest})
}

func fairLaunchMainnetArchivePayloadRow(activationRowID uint64, eventRow entity.FairLaunchMainnetArchiveBatchEventV1, log types.Log, createdAt time.Time) (entity.FairLaunchMainnetArchiveEventPayloadV1, error) {
	if activationRowID == 0 || eventRow.ID == 0 || eventRow.EventIdentityID == 0 || eventRow.PlacementID == 0 || eventRow.ObservationID == 0 || len(log.Topics) == 0 || len(log.Topics) > 4 || len(log.Data) > 65536 {
		return entity.FairLaunchMainnetArchiveEventPayloadV1{}, fmt.Errorf("%s: archive payload invalid", FairLaunchMainnetArchiveRejected)
	}
	topics := make([]string, len(log.Topics))
	for index := range log.Topics {
		topics[index] = strings.ToLower(log.Topics[index].Hex())
	}
	row := entity.FairLaunchMainnetArchiveEventPayloadV1{
		ArchiveEventID: eventRow.ID, ActivationRowID: activationRowID, EventIdentityID: eventRow.EventIdentityID,
		PlacementID: eventRow.PlacementID, ObservationID: eventRow.ObservationID, TopicCount: uint8(len(topics)),
		Topic0: topics[0], DataHex: "0x" + hex.EncodeToString(log.Data), CreatedAt: createdAt.UTC(),
	}
	if len(topics) > 1 {
		row.Topic1 = stringPointer(topics[1])
	}
	if len(topics) > 2 {
		row.Topic2 = stringPointer(topics[2])
	}
	if len(topics) > 3 {
		row.Topic3 = stringPointer(topics[3])
	}
	parts := []string{"joincare:fair-launch:mainnet-archive-payload:v1", fmt.Sprint(row.ArchiveEventID), fmt.Sprint(row.ActivationRowID), fmt.Sprint(row.EventIdentityID), fmt.Sprint(row.PlacementID), fmt.Sprint(row.ObservationID), fmt.Sprint(row.TopicCount), row.Topic0, nullableString(row.Topic1), nullableString(row.Topic2), nullableString(row.Topic3), row.DataHex}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	row.PayloadRowDigest = hex.EncodeToString(digest[:])
	return row, nil
}

func fairLaunchMainnetArchivePayloadEqual(left, right entity.FairLaunchMainnetArchiveEventPayloadV1) bool {
	return left.ArchiveEventID == right.ArchiveEventID && left.ActivationRowID == right.ActivationRowID && left.EventIdentityID == right.EventIdentityID && left.PlacementID == right.PlacementID && left.ObservationID == right.ObservationID && left.TopicCount == right.TopicCount && left.Topic0 == right.Topic0 && nullableString(left.Topic1) == nullableString(right.Topic1) && nullableString(left.Topic2) == nullableString(right.Topic2) && nullableString(left.Topic3) == nullableString(right.Topic3) && left.DataHex == right.DataHex && left.PayloadRowDigest == right.PayloadRowDigest
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func fairLaunchMainnetArchiveLogs(batch FairLaunchMainnetArchiveBatch) []FairLaunchMainnetArchiveLog {
	logs := make([]FairLaunchMainnetArchiveLog, 0)
	for _, block := range batch.Blocks {
		logs = append(logs, block.Events...)
	}
	return logs
}

func fairLaunchMainnetArchiveBlockForLog(batch FairLaunchMainnetArchiveBatch, archivedLog FairLaunchMainnetArchiveLog) FairLaunchMainnetFinalizedBlock {
	if archivedLog.Event == nil {
		return FairLaunchMainnetFinalizedBlock{}
	}
	for _, block := range batch.Blocks {
		if block.Number == archivedLog.Event.Log.BlockNumber {
			return block
		}
	}
	return FairLaunchMainnetFinalizedBlock{}
}

func fairLaunchMainnetArchiveFailureStatus(reason string) (string, bool) {
	switch reason {
	case "canonical_conflict", "rpc_inconsistent":
		return "reorg_detected", true
	case "unknown_event", "malformed_log", "receipt_mismatch", "archive_conflict":
		return "blocked", true
	default:
		return "", false
	}
}

func validFairLaunchMainnetArchiveRecovery(recovery FairLaunchMainnetArchiveRecovery) bool {
	return validMainnetNonzeroDigest(recovery.VerificationProviderSetDigest) && recovery.VerificationCount >= 1 && recovery.VerificationCount <= 8 && validMainnetNonzeroDigest(recovery.CanonicalChainDigest) && validMainnetNonzeroDigest(recovery.ApprovalEvidenceDigest) && validMainnetNonzeroDigest(recovery.RecoveryManifestDigest)
}

func fairLaunchMainnetArchiveRecoveryMatches(status, reason, action string, verificationCount uint32) bool {
	switch reason {
	case "canonical_conflict":
		return status == "reorg_detected" && action == "canonical_reverified" && verificationCount >= 2
	case "rpc_inconsistent":
		return status == "reorg_detected" && action == "provider_consensus_reverified" && verificationCount >= 2
	case "unknown_event", "malformed_log":
		return status == "blocked" && action == "decoder_remediated"
	case "receipt_mismatch":
		return status == "blocked" && action == "receipt_reverified"
	case "archive_conflict":
		return status == "blocked" && action == "repository_remediated"
	default:
		return false
	}
}

func fairLaunchMainnetArchiveRecoveryReceiptDigest(receipt entity.FairLaunchMainnetArchiveRecoveryReceiptV1) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_ARCHIVE_RECOVERY_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "deadLetterId": receipt.DeadLetterID, "previousStatus": receipt.PreviousStatus, "reasonCode": receipt.ReasonCode, "previousErrorDigest": receipt.PreviousErrorDigest, "archivedThroughBlock": receipt.ArchivedThroughBlock, "archivedThroughBlockHash": receipt.ArchivedThroughBlockHash, "lastBatchReceiptId": receipt.LastBatchReceiptID, "previousCursorGeneration": receipt.PreviousCursorGeneration, "previousLeaseId": receipt.PreviousLeaseID, "replacementLeaseId": receipt.ReplacementLeaseID, "recoveryAction": receipt.RecoveryAction, "confirmationPolicy": receipt.ConfirmationPolicy, "verificationProviderSetDigest": receipt.VerificationProviderSetDigest, "verificationCount": receipt.VerificationCount, "canonicalChainDigest": receipt.CanonicalChainDigest, "approvalEvidenceDigest": receipt.ApprovalEvidenceDigest, "recoveryManifestDigest": receipt.RecoveryManifestDigest})
}

func replayFairLaunchMainnetArchiveRecoveryTx(tx *gorm.DB, admission FairLaunchMainnetAdmission, replacementLease entity.FairLaunchMainnetActivationLeaseV1, recovery FairLaunchMainnetArchiveRecovery, receipt entity.FairLaunchMainnetArchiveRecoveryReceiptV1) (FairLaunchMainnetArchiveRecoveryResult, error) {
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND status='active' AND cursor_generation=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND last_error_code IS NULL AND last_error_digest IS NULL AND last_recovery_receipt_id=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, receipt.PreviousCursorGeneration+1, replacementLease.LeaseGeneration, replacementLease.FenceEpoch, replacementLease.LeaseTokenHash, receipt.ID).Take(&cursor).Error; err != nil || receipt.ReplacementLeaseID != replacementLease.ID || receipt.RecoveryAction != recovery.RecoveryAction || receipt.VerificationProviderSetDigest != recovery.VerificationProviderSetDigest || receipt.VerificationCount != recovery.VerificationCount || receipt.CanonicalChainDigest != recovery.CanonicalChainDigest || receipt.ApprovalEvidenceDigest != recovery.ApprovalEvidenceDigest || receipt.RecoveryManifestDigest != recovery.RecoveryManifestDigest || receipt.ReceiptDigest != fairLaunchMainnetArchiveRecoveryReceiptDigest(receipt) {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetArchiveUnavailable)
	}
	return FairLaunchMainnetArchiveRecoveryResult{RecoveryReceiptID: receipt.ID, CursorStatus: cursor.Status, CursorGeneration: cursor.CursorGeneration, LeaseGeneration: cursor.LeaseGeneration, FenceEpoch: cursor.FenceEpoch, ReceiptDigest: receipt.ReceiptDigest, Replayed: true}, nil
}

func validFairLaunchMainnetArchiveFailureCoordinates(failure FairLaunchMainnetArchiveFailure) bool {
	none := failure.BlockNumber == nil && failure.BlockHash == nil && failure.TransactionIndex == nil && failure.TxLogOrdinal == nil && failure.RPCLogIndex == nil && failure.TxHash == nil
	if none {
		return true
	}
	return failure.BlockNumber != nil && failure.BlockHash != nil && fairLaunchLocalHashPattern.MatchString(*failure.BlockHash) && failure.TransactionIndex != nil && failure.RPCLogIndex != nil && failure.TxHash != nil && fairLaunchLocalHashPattern.MatchString(*failure.TxHash)
}

func normalizeFairLaunchMainnetArchiveFailure(failure FairLaunchMainnetArchiveFailure) FairLaunchMainnetArchiveFailure {
	if failure.BlockHash != nil {
		value := strings.ToLower(*failure.BlockHash)
		failure.BlockHash = &value
	}
	if failure.TxHash != nil {
		value := strings.ToLower(*failure.TxHash)
		failure.TxHash = &value
	}
	return failure
}
