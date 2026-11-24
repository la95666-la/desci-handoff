package chainevent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchMainnetRecoveryUnavailable = "fair_launch_mainnet_recovery_unavailable"
	FairLaunchMainnetRecoveryRetryable   = "fair_launch_mainnet_recovery_retryable"
	FairLaunchMainnetRecoveryRejected    = "fair_launch_mainnet_recovery_rejected"
	fairLaunchMainnetRecoverySealTTL     = 5 * time.Minute
)

type FairLaunchMainnetRecoveryVerificationRequest struct {
	DeadLetterID             uint64
	PreviousCursorGeneration uint64
}

type FairLaunchMainnetRecoveryProvider struct {
	seal *fairLaunchMainnetRecoveryProviderSeal
}

type fairLaunchMainnetRecoveryProviderSeal struct {
	rpc              FairLaunchMainnetRPC
	provider         entity.FairLaunchMainnetRecoveryProviderV1
	authorityRowID   uint64
	chainID          uint64
	sourceID         uint64
	sourceRevisionID uint64
	leaseGeneration  uint64
	fenceEpoch       uint64
}

type FairLaunchMainnetVerifiedRecovery struct {
	seal *fairLaunchMainnetVerifiedRecoverySeal
}

type fairLaunchMainnetVerifiedRecoverySeal struct {
	recovery         FairLaunchMainnetArchiveRecovery
	authorityRowID   uint64
	chainID          uint64
	sourceID         uint64
	sourceRevisionID uint64
	leaseGeneration  uint64
	fenceEpoch       uint64
	verifiedAt       time.Time
	expiresAt        time.Time
	verificationHash string
}

type fairLaunchMainnetRecoveryBlock struct {
	Number     uint64
	Hash       string
	ParentHash string
}

// BindFairLaunchMainnetRecoveryProvider binds an already-configured RPC caller
// to one immutable, activation-scoped provider admission. The returned value is
// opaque and retains no URL, credential or secret-bearing configuration.
func BindFairLaunchMainnetRecoveryProvider(capability FairLaunchMainnetCapability, database *gorm.DB, rpc FairLaunchMainnetRPC, now time.Time) (FairLaunchMainnetRecoveryProvider, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || database == nil || rpc == nil || admission.Component != "job" || admission.ChainID != fairLaunchMainnetChainID {
		return FairLaunchMainnetRecoveryProvider{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	identity := rpc.ProviderIdentity()
	identityDigest, err := FairLaunchMainnetRPCIdentityDigest(identity)
	if err != nil {
		return FairLaunchMainnetRecoveryProvider{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	var provider entity.FairLaunchMainnetRecoveryProviderV1
	if err := database.Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND provider_identity=? AND rpc_identity_digest=? AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, identity, identityDigest, now.UTC(), now.UTC()).Take(&provider).Error; err != nil || !validFairLaunchMainnetRecoveryProviderAdmission(provider, admission, identity, identityDigest, now) {
		return FairLaunchMainnetRecoveryProvider{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	return FairLaunchMainnetRecoveryProvider{seal: &fairLaunchMainnetRecoveryProviderSeal{rpc: rpc, provider: provider, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch}}, nil
}

// VerifyFairLaunchMainnetArchiveRecovery is read-only. It verifies the exact
// blocked cursor against 2-8 independent public provider identities, requires
// the activation's admitted provider to participate, and returns an opaque
// recovery handoff. It stores no endpoint, credential, lease token or RPC.
func VerifyFairLaunchMainnetArchiveRecovery(ctx context.Context, capability FairLaunchMainnetCapability, database *gorm.DB, providers []FairLaunchMainnetRecoveryProvider, request FairLaunchMainnetRecoveryVerificationRequest, now time.Time) (FairLaunchMainnetVerifiedRecovery, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || database == nil || admission.Component != "job" || admission.ChainID != fairLaunchMainnetChainID || admission.ConfirmationPolicy != "20_blocks" || request.DeadLetterID == 0 || request.PreviousCursorGeneration == 0 || len(providers) < 2 || len(providers) > 8 {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	leaseHash := fmt.Sprintf("%x", capability.seal.leaseHash)
	var replacementLease entity.FairLaunchMainnetActivationLeaseV1
	if err := database.WithContext(ctx).Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status='active' AND active_slot=1 AND valid_from<=? AND valid_until>?", admission.AuthorityRowID, admission.ChainID, admission.LeaseGeneration, admission.FenceEpoch, leaseHash, now.UTC(), now.UTC()).Take(&replacementLease).Error; err != nil || !validMainnetNonzeroDigest(replacementLease.ApprovalEvidenceDigest) {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	var cursor entity.FairLaunchMainnetArchiveCursorV1
	if err := database.WithContext(ctx).Where("activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND cursor_generation=? AND status='reorg_detected'", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, request.PreviousCursorGeneration).Take(&cursor).Error; err != nil || cursor.LastErrorCode == nil || cursor.LastErrorDigest == nil || cursor.LastRecoveryReceiptID != nil {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	if *cursor.LastErrorCode != "canonical_conflict" && *cursor.LastErrorCode != "rpc_inconsistent" {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	var deadLetter entity.FairLaunchMainnetArchiveDeadLetterV1
	if err := database.WithContext(ctx).Where("id=? AND activation_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=? AND reason_code=? AND evidence_digest=?", request.DeadLetterID, admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID, *cursor.LastErrorCode, *cursor.LastErrorDigest).Take(&deadLetter).Error; err != nil {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	var previousLease entity.FairLaunchMainnetActivationLeaseV1
	if err := database.WithContext(ctx).Where("activation_row_id=? AND chain_id=? AND lease_generation=? AND fence_epoch=? AND lease_token_hash=? AND status IN('revoked','expired') AND active_slot IS NULL", admission.AuthorityRowID, admission.ChainID, cursor.LeaseGeneration, cursor.FenceEpoch, cursor.LeaseTokenHash).Take(&previousLease).Error; err != nil || replacementLease.LeaseGeneration <= previousLease.LeaseGeneration || replacementLease.FenceEpoch <= previousLease.FenceEpoch {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	if err := validateFairLaunchMainnetArchiveSourceTx(database.WithContext(ctx), admission); err != nil {
		return FairLaunchMainnetVerifiedRecovery{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	providerEvidence, canonicalBlocks, err := verifyFairLaunchMainnetRecoveryProviders(ctx, providers, admission, cursor, now)
	if err != nil {
		return FairLaunchMainnetVerifiedRecovery{}, err
	}
	recoveryAction := "canonical_reverified"
	if deadLetter.ReasonCode == "rpc_inconsistent" {
		recoveryAction = "provider_consensus_reverified"
	}
	providerSetDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_RECOVERY_PROVIDER_SET_V1", providerEvidence)
	canonicalChainDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_RECOVERY_CANONICAL_CHAIN_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "chainId": admission.ChainID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "deadLetterId": deadLetter.ID, "previousCursorGeneration": cursor.CursorGeneration, "blocks": canonicalBlocks})
	recoveryManifestDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_RECOVERY_MANIFEST_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "chainId": admission.ChainID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "deadLetterId": deadLetter.ID, "reasonCode": deadLetter.ReasonCode, "previousErrorDigest": deadLetter.EvidenceDigest, "archivedThroughBlock": cursor.ArchivedThroughBlock, "archivedThroughBlockHash": cursor.ArchivedThroughBlockHash, "lastBatchReceiptId": cursor.LastBatchReceiptID, "previousCursorGeneration": cursor.CursorGeneration, "previousLeaseId": previousLease.ID, "replacementLeaseId": replacementLease.ID, "replacementLeaseGeneration": replacementLease.LeaseGeneration, "replacementFenceEpoch": replacementLease.FenceEpoch, "recoveryAction": recoveryAction, "confirmationPolicy": admission.ConfirmationPolicy, "verificationProviderSetDigest": providerSetDigest, "verificationCount": len(providerEvidence), "canonicalChainDigest": canonicalChainDigest, "approvalEvidenceDigest": replacementLease.ApprovalEvidenceDigest})
	recovery := FairLaunchMainnetArchiveRecovery{DeadLetterID: deadLetter.ID, PreviousCursorGeneration: cursor.CursorGeneration, RecoveryAction: recoveryAction, VerificationProviderSetDigest: providerSetDigest, VerificationCount: uint32(len(providerEvidence)), CanonicalChainDigest: canonicalChainDigest, ApprovalEvidenceDigest: replacementLease.ApprovalEvidenceDigest, RecoveryManifestDigest: recoveryManifestDigest}
	verifiedAt := now.UTC()
	expiresAt := verifiedAt.Add(fairLaunchMainnetRecoverySealTTL)
	verificationHash := fairLaunchMainnetVerifiedRecoveryDigest(admission, recovery, verifiedAt, expiresAt)
	return FairLaunchMainnetVerifiedRecovery{seal: &fairLaunchMainnetVerifiedRecoverySeal{recovery: recovery, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch, verifiedAt: verifiedAt, expiresAt: expiresAt, verificationHash: verificationHash}}, nil
}

// RecoverFairLaunchMainnetArchiveCursorTx accepts only the verifier's opaque
// handoff. Raw digest-bearing recovery input remains package-private.
func RecoverFairLaunchMainnetArchiveCursorTx(capability FairLaunchMainnetCapability, tx *gorm.DB, verified FairLaunchMainnetVerifiedRecovery, now time.Time) (FairLaunchMainnetArchiveRecoveryResult, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || tx == nil || verified.seal == nil || now.Before(verified.seal.verifiedAt) || !now.Before(verified.seal.expiresAt) || verified.seal.expiresAt.Sub(verified.seal.verifiedAt) != fairLaunchMainnetRecoverySealTTL || verified.seal.authorityRowID != admission.AuthorityRowID || verified.seal.chainID != admission.ChainID || verified.seal.sourceID != admission.SourceID || verified.seal.sourceRevisionID != admission.SourceRevisionID || verified.seal.leaseGeneration != admission.LeaseGeneration || verified.seal.fenceEpoch != admission.FenceEpoch || verified.seal.verificationHash != fairLaunchMainnetVerifiedRecoveryDigest(admission, verified.seal.recovery, verified.seal.verifiedAt, verified.seal.expiresAt) {
		return FairLaunchMainnetArchiveRecoveryResult{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	return recoverFairLaunchMainnetArchiveCursorTx(capability, tx, verified.seal.recovery, now)
}

func verifyFairLaunchMainnetRecoveryProviders(ctx context.Context, providers []FairLaunchMainnetRecoveryProvider, admission FairLaunchMainnetAdmission, cursor entity.FairLaunchMainnetArchiveCursorV1, now time.Time) ([]map[string]any, []fairLaunchMainnetRecoveryBlock, error) {
	providerEvidence := make([]map[string]any, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	admittedProviderPresent := false
	var consensus []fairLaunchMainnetRecoveryBlock
	for _, binding := range providers {
		rpc, provider, err := binding.route(admission, now)
		if err != nil || seen[provider.ProviderIdentity] {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryUnavailable)
		}
		seen[provider.ProviderIdentity] = true
		if provider.RPCIdentityDigest == admission.RPCIdentityDigest && provider.ProviderRole == "primary" {
			admittedProviderPresent = true
		} else if provider.RPCIdentityDigest == admission.RPCIdentityDigest || provider.ProviderRole != "independent" {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryUnavailable)
		}
		first, err := collectFairLaunchMainnetRecoveryWindow(ctx, rpc, admission, cursor.ArchivedThroughBlock)
		if err != nil {
			return nil, nil, classifyFairLaunchMainnetRecoveryRPCError(err)
		}
		if first[0].Hash != cursor.ArchivedThroughBlockHash {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryRejected)
		}
		if consensus == nil {
			consensus = first
		} else if !equalFairLaunchMainnetRecoveryBlocks(consensus, first) {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryRejected)
		}
		providerEvidence = append(providerEvidence, map[string]any{"providerRowId": provider.ID, "providerIdentity": provider.ProviderIdentity, "rpcIdentityDigest": provider.RPCIdentityDigest, "providerRole": provider.ProviderRole, "admissionManifestDigest": provider.AdmissionManifestDigest, "approvalEvidenceDigest": provider.ApprovalEvidenceDigest})
	}
	if !admittedProviderPresent {
		return nil, nil, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	for _, binding := range providers {
		rpc, _, err := binding.route(admission, now)
		if err != nil {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryUnavailable)
		}
		second, err := collectFairLaunchMainnetRecoveryWindow(ctx, rpc, admission, cursor.ArchivedThroughBlock)
		if err != nil {
			return nil, nil, classifyFairLaunchMainnetRecoveryRPCError(err)
		}
		if !equalFairLaunchMainnetRecoveryBlocks(consensus, second) {
			return nil, nil, errors.New(FairLaunchMainnetRecoveryRejected)
		}
	}
	sort.Slice(providerEvidence, func(i, j int) bool {
		return providerEvidence[i]["providerIdentity"].(string) < providerEvidence[j]["providerIdentity"].(string)
	})
	return providerEvidence, consensus, nil
}

func (binding FairLaunchMainnetRecoveryProvider) route(admission FairLaunchMainnetAdmission, now time.Time) (FairLaunchMainnetRPC, entity.FairLaunchMainnetRecoveryProviderV1, error) {
	if binding.seal == nil || binding.seal.rpc == nil || binding.seal.authorityRowID != admission.AuthorityRowID || binding.seal.chainID != admission.ChainID || binding.seal.sourceID != admission.SourceID || binding.seal.sourceRevisionID != admission.SourceRevisionID || binding.seal.leaseGeneration != admission.LeaseGeneration || binding.seal.fenceEpoch != admission.FenceEpoch {
		return nil, entity.FairLaunchMainnetRecoveryProviderV1{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	provider := binding.seal.provider
	identity := binding.seal.rpc.ProviderIdentity()
	identityDigest, err := FairLaunchMainnetRPCIdentityDigest(identity)
	if err != nil || !validFairLaunchMainnetRecoveryProviderAdmission(provider, admission, identity, identityDigest, now) {
		return nil, entity.FairLaunchMainnetRecoveryProviderV1{}, errors.New(FairLaunchMainnetRecoveryUnavailable)
	}
	return binding.seal.rpc, provider, nil
}

func validFairLaunchMainnetRecoveryProviderAdmission(provider entity.FairLaunchMainnetRecoveryProviderV1, admission FairLaunchMainnetAdmission, identity, identityDigest string, now time.Time) bool {
	return provider.ID != 0 && provider.ActivationRowID == admission.AuthorityRowID && provider.ChainID == admission.ChainID && provider.SourceID == admission.SourceID && provider.SourceRevisionID == admission.SourceRevisionID && provider.ProviderIdentity == identity && provider.RPCIdentityDigest == identityDigest && !now.Before(provider.ValidFrom) && now.Before(provider.ValidUntil) && validMainnetNonzeroDigest(provider.AdmissionManifestDigest) && validMainnetNonzeroDigest(provider.ApprovalEvidenceDigest) && (provider.ProviderRole == "primary" && provider.RPCIdentityDigest == admission.RPCIdentityDigest || provider.ProviderRole == "independent" && provider.RPCIdentityDigest != admission.RPCIdentityDigest)
}

func collectFairLaunchMainnetRecoveryWindow(ctx context.Context, rpc FairLaunchMainnetRPC, admission FairLaunchMainnetAdmission, anchorBlock uint64) ([]fairLaunchMainnetRecoveryBlock, error) {
	if err := verifyFairLaunchMainnetRPCNetwork(ctx, rpc, admission); err != nil {
		return nil, err
	}
	latest, err := loadFairLaunchMainnetBlockByTag(ctx, rpc, "latest")
	if err != nil {
		return nil, err
	}
	latestNumber, ok := parseFairLaunchLocalHexUint64(latest.Number)
	if !ok {
		return nil, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "latest block number invalid")
	}
	if latestNumber < anchorBlock+fairLaunchMainnetConfirmations {
		return nil, fairLaunchMainnetCollectorRetryable("confirmation depth unavailable")
	}
	blocks := make([]fairLaunchMainnetRecoveryBlock, 0, fairLaunchMainnetConfirmations+1)
	var previousHash string
	for number := anchorBlock; number <= anchorBlock+fairLaunchMainnetConfirmations; number++ {
		block, err := loadFairLaunchMainnetBlock(ctx, rpc, number)
		if err != nil {
			return nil, err
		}
		if number > anchorBlock && block.ParentHash != previousHash {
			return nil, fairLaunchMainnetCollectorBlocked("canonical_conflict", "recovery confirmation chain mismatch")
		}
		blocks = append(blocks, fairLaunchMainnetRecoveryBlock{Number: number, Hash: block.Hash, ParentHash: block.ParentHash})
		previousHash = block.Hash
	}
	return blocks, nil
}

func classifyFairLaunchMainnetRecoveryRPCError(err error) error {
	_, retryable, deterministic := classifyFairLaunchMainnetCollectorError(err)
	if retryable {
		return errors.New(FairLaunchMainnetRecoveryRetryable)
	}
	if deterministic {
		return errors.New(FairLaunchMainnetRecoveryRejected)
	}
	return errors.New(FairLaunchMainnetRecoveryUnavailable)
}

func equalFairLaunchMainnetRecoveryBlocks(left, right []fairLaunchMainnetRecoveryBlock) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func fairLaunchMainnetVerifiedRecoveryDigest(admission FairLaunchMainnetAdmission, recovery FairLaunchMainnetArchiveRecovery, verifiedAt, expiresAt time.Time) string {
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_VERIFIED_RECOVERY_V1", map[string]any{"activationRowId": admission.AuthorityRowID, "chainId": admission.ChainID, "sourceId": admission.SourceID, "sourceRevisionId": admission.SourceRevisionID, "leaseGeneration": admission.LeaseGeneration, "fenceEpoch": admission.FenceEpoch, "deadLetterId": recovery.DeadLetterID, "previousCursorGeneration": recovery.PreviousCursorGeneration, "recoveryAction": recovery.RecoveryAction, "verificationProviderSetDigest": recovery.VerificationProviderSetDigest, "verificationCount": recovery.VerificationCount, "canonicalChainDigest": recovery.CanonicalChainDigest, "approvalEvidenceDigest": recovery.ApprovalEvidenceDigest, "recoveryManifestDigest": recovery.RecoveryManifestDigest, "verifiedAt": verifiedAt.UTC().Format(time.RFC3339Nano), "expiresAt": expiresAt.UTC().Format(time.RFC3339Nano)})
}
