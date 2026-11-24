package chainevent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchMainnetWorkerUnavailable = "fair_launch_mainnet_worker_unavailable"
	FairLaunchMainnetWorkerRetryable   = "fair_launch_mainnet_worker_retryable"
)

type FairLaunchMainnetArchiveWorkerResult struct {
	Outcome               string
	ReasonCode            string
	ArchivedThroughBlock  uint64
	ObservedHeadBlock     uint64
	FinalizedThroughBlock uint64
	LagBlocks             uint64
	CursorGeneration      uint64
	Batch                 FairLaunchMainnetArchiveBatchResult
	Failure               FairLaunchMainnetArchiveFailureResult
}

type FairLaunchMainnetArchiveStatus struct {
	ActivationRowID       uint64
	ChainID               uint64
	SourceID              uint64
	SourceRevisionID      uint64
	CursorStatus          string
	CursorCanonical       bool
	ArchivedThroughBlock  uint64
	ObservedHeadBlock     uint64
	FinalizedThroughBlock uint64
	LagBlocks             uint64
	CursorGeneration      uint64
	LastBatchReceiptID    *uint64
	LastRecoveryReceiptID *uint64
	LastErrorCode         *string
	DeadLetterCount       uint64
}

type FairLaunchMainnetSourceRegistry struct {
	seal *fairLaunchMainnetSourceRegistrySeal
}

type fairLaunchMainnetSourceRegistrySeal struct {
	capability           FairLaunchMainnetCapability
	rpc                  FairLaunchMainnetRPC
	authorityRowID       uint64
	chainID              uint64
	sourceID             uint64
	sourceRevisionID     uint64
	sourceRegistryDigest string
	rpcIdentityDigest    string
}

// BindFairLaunchMainnetSourceRegistry binds an admitted activation/source
// revision to an already-configured RPC caller. It retains neither an endpoint
// nor credentials, starts no goroutine/cron job, and grants no wallet or
// financial-materialization capability.
func BindFairLaunchMainnetSourceRegistry(capability FairLaunchMainnetCapability, rpc FairLaunchMainnetRPC, now time.Time) (FairLaunchMainnetSourceRegistry, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || rpc == nil || admission.Component != "job" || admission.ChainID != fairLaunchMainnetChainID || admission.ConfirmationPolicy != "20_blocks" || !validMainnetNonzeroDigest(admission.SourceRegistryDigest) {
		return FairLaunchMainnetSourceRegistry{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	if err := validateFairLaunchMainnetWorkerRPC(rpc, admission); err != nil {
		return FairLaunchMainnetSourceRegistry{}, err
	}
	return FairLaunchMainnetSourceRegistry{seal: &fairLaunchMainnetSourceRegistrySeal{capability: capability, rpc: rpc, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, sourceRegistryDigest: admission.SourceRegistryDigest, rpcIdentityDigest: admission.RPCIdentityDigest}}, nil
}

// runFairLaunchMainnetArchiveOnce is a package-private one-shot worker seam.
// It is not registered with cron and performs no financial materialization. A
// deterministic collector/repository conflict is recorded only after the
// failed archive transaction has fully rolled back.
func runFairLaunchMainnetArchiveOnce(ctx context.Context, registry FairLaunchMainnetSourceRegistry, database *gorm.DB, now time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
	capability, rpc, admission, err := registry.route(now)
	if err != nil || database == nil {
		return FairLaunchMainnetArchiveWorkerResult{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	cursor, err := loadFairLaunchMainnetArchiveWorkerCursor(database.WithContext(ctx), capability, admission, now)
	if err != nil {
		return FairLaunchMainnetArchiveWorkerResult{}, err
	}
	if err := validateFairLaunchMainnetWorkerRPC(rpc, admission); err != nil {
		return FairLaunchMainnetArchiveWorkerResult{}, err
	}
	if cursor.Status != "active" {
		return fairLaunchMainnetWorkerStoppedResult(cursor), nil
	}
	anchorHash, err := verifyFairLaunchMainnetRPCIdentity(ctx, rpc, admission, cursor.ArchivedThroughBlock)
	if err != nil {
		return handleFairLaunchMainnetWorkerFailure(database.WithContext(ctx), capability, cursor, err, now)
	}
	if anchorHash != cursor.ArchivedThroughBlockHash {
		return handleFairLaunchMainnetWorkerFailure(database.WithContext(ctx), capability, cursor, fairLaunchMainnetCollectorBlocked("canonical_conflict", "archived cursor is no longer canonical"), now)
	}
	observedHead, finalizedThrough, err := fairLaunchMainnetObservedHead(ctx, rpc)
	if err != nil {
		return handleFairLaunchMainnetWorkerFailure(database.WithContext(ctx), capability, cursor, err, now)
	}
	fromBlock, throughBlock, hasWork := planFairLaunchMainnetArchiveRange(cursor.ArchivedThroughBlock, finalizedThrough)
	if !hasWork {
		return FairLaunchMainnetArchiveWorkerResult{Outcome: "no_work", ArchivedThroughBlock: cursor.ArchivedThroughBlock, ObservedHeadBlock: observedHead, FinalizedThroughBlock: finalizedThrough, LagBlocks: fairLaunchMainnetLag(cursor.ArchivedThroughBlock, finalizedThrough), CursorGeneration: cursor.CursorGeneration}, nil
	}
	collected, err := CollectFairLaunchMainnetFinalizedBatch(ctx, capability, rpc, fromBlock, throughBlock, now)
	if err != nil {
		return handleFairLaunchMainnetWorkerFailure(database.WithContext(ctx), capability, cursor, err, now)
	}
	var archived FairLaunchMainnetArchiveBatchResult
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inner error
		archived, inner = ArchiveFairLaunchMainnetCollectedBatchTx(capability, tx, collected, now)
		return inner
	})
	if err != nil {
		if strings.Contains(err.Error(), FairLaunchMainnetArchiveRejected) {
			return handleFairLaunchMainnetWorkerFailure(database.WithContext(ctx), capability, cursor, fairLaunchMainnetCollectorBlocked("archive_conflict", "archive repository conflict"), now)
		}
		return FairLaunchMainnetArchiveWorkerResult{}, fmt.Errorf("%s: archive transaction failed", FairLaunchMainnetWorkerRetryable)
	}
	return FairLaunchMainnetArchiveWorkerResult{Outcome: "archived", ArchivedThroughBlock: archived.ThroughBlock, ObservedHeadBlock: observedHead, FinalizedThroughBlock: finalizedThrough, LagBlocks: fairLaunchMainnetLag(archived.ThroughBlock, finalizedThrough), CursorGeneration: cursor.CursorGeneration + 1, Batch: archived}, nil
}

// LoadFairLaunchMainnetArchiveStatus is read-only operational evidence. It
// neither advances nor unblocks a cursor and does not expose RPC credentials.
func LoadFairLaunchMainnetArchiveStatus(ctx context.Context, registry FairLaunchMainnetSourceRegistry, database *gorm.DB, now time.Time) (FairLaunchMainnetArchiveStatus, error) {
	capability, rpc, admission, err := registry.route(now)
	if err != nil || database == nil {
		return FairLaunchMainnetArchiveStatus{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	cursor, err := loadFairLaunchMainnetArchiveWorkerCursor(database.WithContext(ctx), capability, admission, now)
	if err != nil {
		return FairLaunchMainnetArchiveStatus{}, err
	}
	if err := validateFairLaunchMainnetWorkerRPC(rpc, admission); err != nil {
		return FairLaunchMainnetArchiveStatus{}, err
	}
	cursorCanonical := false
	anchorHash, canonicalErr := verifyFairLaunchMainnetRPCIdentity(ctx, rpc, admission, cursor.ArchivedThroughBlock)
	if canonicalErr != nil {
		failure, retryable, deterministic := classifyFairLaunchMainnetCollectorError(canonicalErr)
		if retryable {
			return FairLaunchMainnetArchiveStatus{}, fmt.Errorf("%s: status RPC unavailable", FairLaunchMainnetWorkerRetryable)
		}
		if !deterministic || failure.ReasonCode != "canonical_conflict" {
			return FairLaunchMainnetArchiveStatus{}, errors.New(FairLaunchMainnetWorkerUnavailable)
		}
	} else {
		cursorCanonical = anchorHash == cursor.ArchivedThroughBlockHash
	}
	observedHead, finalizedThrough, err := fairLaunchMainnetObservedHead(ctx, rpc)
	if err != nil {
		_, retryable, _ := classifyFairLaunchMainnetCollectorError(err)
		if retryable {
			return FairLaunchMainnetArchiveStatus{}, fmt.Errorf("%s: status RPC unavailable", FairLaunchMainnetWorkerRetryable)
		}
		return FairLaunchMainnetArchiveStatus{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	var deadLetterCount int64
	if err := database.WithContext(ctx).Model(&entity.FairLaunchMainnetArchiveDeadLetterV1{}).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID).Count(&deadLetterCount).Error; err != nil || deadLetterCount < 0 {
		return FairLaunchMainnetArchiveStatus{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	return FairLaunchMainnetArchiveStatus{ActivationRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, CursorStatus: cursor.Status, CursorCanonical: cursorCanonical, ArchivedThroughBlock: cursor.ArchivedThroughBlock, ObservedHeadBlock: observedHead, FinalizedThroughBlock: finalizedThrough, LagBlocks: fairLaunchMainnetLag(cursor.ArchivedThroughBlock, finalizedThrough), CursorGeneration: cursor.CursorGeneration, LastBatchReceiptID: cursor.LastBatchReceiptID, LastRecoveryReceiptID: cursor.LastRecoveryReceiptID, LastErrorCode: cursor.LastErrorCode, DeadLetterCount: uint64(deadLetterCount)}, nil
}

func loadFairLaunchMainnetArchiveWorkerCursor(database *gorm.DB, capability FairLaunchMainnetCapability, admission FairLaunchMainnetAdmission, now time.Time) (entity.FairLaunchMainnetArchiveCursorV1, error) {
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	if err := validateFairLaunchMainnetArchiveLeaseTx(database, admission, leaseHash, now); err != nil {
		return entity.FairLaunchMainnetArchiveCursorV1{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := database.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash).Take(&cursor).Error; err != nil || cursor.CursorGeneration == 0 || !fairLaunchLocalHashPattern.MatchString(cursor.ArchivedThroughBlockHash) {
		return entity.FairLaunchMainnetArchiveCursorV1{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	if err := validateFairLaunchMainnetArchiveSourceTx(database, admission); err != nil {
		return entity.FairLaunchMainnetArchiveCursorV1{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	return cursor, nil
}

func validateFairLaunchMainnetWorkerRPC(rpc FairLaunchMainnetRPC, admission FairLaunchMainnetAdmission) error {
	providerDigest, err := FairLaunchMainnetRPCIdentityDigest(rpc.ProviderIdentity())
	if err != nil || providerDigest != admission.RPCIdentityDigest {
		return errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	return nil
}

func (registry FairLaunchMainnetSourceRegistry) route(now time.Time) (FairLaunchMainnetCapability, FairLaunchMainnetRPC, FairLaunchMainnetAdmission, error) {
	if registry.seal == nil || registry.seal.rpc == nil {
		return FairLaunchMainnetCapability{}, nil, FairLaunchMainnetAdmission{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	admission, err := registry.seal.capability.Admission()
	if err != nil || !registry.seal.capability.valid(now) || admission.Component != "job" || admission.ChainID != fairLaunchMainnetChainID || admission.ConfirmationPolicy != "20_blocks" || admission.AuthorityRowID != registry.seal.authorityRowID || admission.ChainID != registry.seal.chainID || admission.SourceID != registry.seal.sourceID || admission.SourceRevisionID != registry.seal.sourceRevisionID || admission.SourceRegistryDigest != registry.seal.sourceRegistryDigest || admission.RPCIdentityDigest != registry.seal.rpcIdentityDigest || !validMainnetNonzeroDigest(admission.SourceRegistryDigest) {
		return FairLaunchMainnetCapability{}, nil, FairLaunchMainnetAdmission{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	if err := validateFairLaunchMainnetWorkerRPC(registry.seal.rpc, admission); err != nil {
		return FairLaunchMainnetCapability{}, nil, FairLaunchMainnetAdmission{}, err
	}
	return registry.seal.capability, registry.seal.rpc, admission, nil
}

func fairLaunchMainnetObservedHead(ctx context.Context, rpc FairLaunchMainnetRPC) (uint64, uint64, error) {
	latest, err := loadFairLaunchMainnetBlockByTag(ctx, rpc, "latest")
	if err != nil {
		return 0, 0, err
	}
	observedHead, ok := parseFairLaunchLocalHexUint64(latest.Number)
	if !ok {
		return 0, 0, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "latest block number invalid")
	}
	if observedHead < fairLaunchMainnetConfirmations {
		return observedHead, 0, nil
	}
	return observedHead, observedHead - fairLaunchMainnetConfirmations, nil
}

func planFairLaunchMainnetArchiveRange(archivedThrough, finalizedThrough uint64) (uint64, uint64, bool) {
	if archivedThrough == math.MaxUint64 || finalizedThrough <= archivedThrough {
		return 0, 0, false
	}
	fromBlock := archivedThrough + 1
	throughBlock := finalizedThrough
	if finalizedThrough-fromBlock+1 > fairLaunchMainnetArchiveMaxBlocks {
		throughBlock = fromBlock + fairLaunchMainnetArchiveMaxBlocks - 1
	}
	return fromBlock, throughBlock, true
}

func handleFairLaunchMainnetWorkerFailure(database *gorm.DB, capability FairLaunchMainnetCapability, cursor entity.FairLaunchMainnetArchiveCursorV1, cause error, now time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
	failure, retryable, block := classifyFairLaunchMainnetCollectorError(cause)
	if retryable {
		return FairLaunchMainnetArchiveWorkerResult{}, fmt.Errorf("%s: collector temporarily unavailable", FairLaunchMainnetWorkerRetryable)
	}
	if !block {
		return FairLaunchMainnetArchiveWorkerResult{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	var blocked FairLaunchMainnetArchiveFailureResult
	if err := database.Transaction(func(tx *gorm.DB) error {
		var inner error
		blocked, inner = BlockFairLaunchMainnetArchiveCursorTx(capability, tx, failure, now)
		return inner
	}); err != nil {
		return FairLaunchMainnetArchiveWorkerResult{}, errors.New(FairLaunchMainnetWorkerUnavailable)
	}
	return FairLaunchMainnetArchiveWorkerResult{Outcome: blocked.CursorStatus, ReasonCode: failure.ReasonCode, ArchivedThroughBlock: cursor.ArchivedThroughBlock, CursorGeneration: cursor.CursorGeneration + 1, Failure: blocked}, nil
}

func fairLaunchMainnetWorkerStoppedResult(cursor entity.FairLaunchMainnetArchiveCursorV1) FairLaunchMainnetArchiveWorkerResult {
	reason := ""
	if cursor.LastErrorCode != nil {
		reason = *cursor.LastErrorCode
	}
	return FairLaunchMainnetArchiveWorkerResult{Outcome: cursor.Status, ReasonCode: reason, ArchivedThroughBlock: cursor.ArchivedThroughBlock, CursorGeneration: cursor.CursorGeneration}
}

func fairLaunchMainnetLag(archivedThrough, finalizedThrough uint64) uint64 {
	if finalizedThrough <= archivedThrough {
		return 0
	}
	return finalizedThrough - archivedThrough
}
