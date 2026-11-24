package idov3whitelist

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// PolicyEvidenceFailureStage exposes only the fixed check that failed, never
// the underlying RPC response, URL, calldata, wallet or approved member set.
type policyEvidenceStageError string

func (e policyEvidenceStageError) Error() string { return "V3 policy evidence unavailable" }

func PolicyEvidenceFailureStage(err error) string {
	var stage policyEvidenceStageError
	if errors.As(err, &stage) {
		switch stage {
		case "request", "scope", "receipt_lookup", "confirmations", "receipt_header",
			"receipt_lookup_deadline", "receipt_lookup_rpc", "receipt_lookup_missing", "receipt_lookup_malformed",
			"receipt_policy_read", "receipt_policy_match", "head_header", "head_policy_read",
			"head_policy_match", "receipt_header_recheck", "head_header_recheck":
			return string(stage)
		}
	}
	return "policy_evidence_unavailable"
}

// EvidenceRPC deliberately grants read operations only. This verifier must
// never be given a signing key or a transaction submission capability.
type EvidenceRPC interface {
	PolicyRPC
	BlockNumber(context.Context) (uint64, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

// PolicyEvidence is a transient, chain-checked observation. It is NOT an
// approval certificate or permission to publish wallet proofs. A later reorg,
// upgrade or membership change invalidates it, even after confirmations.
type PolicyEvidence struct {
	ReceiptBlock PolicyObservation
	Head         PolicyObservation
	TxHash       common.Hash
}

// ReconcilePolicyEvidence checks the executed event and policy at the receipt
// block, then checks that the policy remains unchanged at a confirmed head.
// minConfirmations counts the receipt block as one. The caller must independently
// fetch the immutable full-member approval, validate its two distinct approvers,
// expected old version, exact tx hash, persisted member set, and wallet binding.
// The caller must also choose an independently approved runtime identity and
// revalidate evidence immediately before use. No raw RPC error is returned.
func ReconcilePolicyEvidence(ctx context.Context, rpc EvidenceRPC, scope Scope, runtime RuntimeIdentity, candidate SnapshotCommitment, approvedTxHash common.Hash, minConfirmations uint64) (PolicyEvidence, error) {
	if rpc == nil || approvedTxHash == (common.Hash{}) || minConfirmations == 0 {
		return PolicyEvidence{}, policyEvidenceStageError("request")
	}
	if err := validateScope(scope); err != nil {
		return PolicyEvidence{}, policyEvidenceStageError("scope")
	}
	receipt, err := rpc.TransactionReceipt(ctx, approvedTxHash)
	if stage := policyReceiptLookupCheck(ctx, receipt, err); stage != "" {
		return PolicyEvidence{}, policyEvidenceStageError(stage)
	}
	block := receipt.BlockNumber.Uint64()
	head, err := rpc.BlockNumber(ctx)
	if err != nil || !HasConfirmations(head, block, minConfirmations) {
		return PolicyEvidence{}, policyEvidenceStageError("confirmations")
	}
	// A receipt returned by one RPC is not automatically canonical. Pin its
	// placement to an independently fetched header, before any state reads.
	receiptHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil || receiptHeader == nil || receiptHeader.Number == nil || !receiptHeader.Number.IsUint64() || receiptHeader.Number.Uint64() != block || receiptHeader.Hash() != receipt.BlockHash {
		return PolicyEvidence{}, policyEvidenceStageError("receipt_header")
	}
	atReceipt, err := ReadPolicyAt(ctx, rpc, scope, runtime, block, receipt.BlockHash)
	if err != nil {
		return PolicyEvidence{}, policyEvidenceStageError("receipt_policy_read")
	}
	if err := MatchPolicyReceipt(scope, candidate, approvedTxHash, receipt, atReceipt); err != nil {
		return PolicyEvidence{}, policyEvidenceStageError("receipt_policy_match")
	}
	headHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(head))
	if err != nil || headHeader == nil || headHeader.Number == nil || !headHeader.Number.IsUint64() || headHeader.Number.Uint64() != head {
		return PolicyEvidence{}, policyEvidenceStageError("head_header")
	}
	atHead, err := ReadPolicyAt(ctx, rpc, scope, runtime, head, headHeader.Hash())
	if err != nil {
		return PolicyEvidence{}, policyEvidenceStageError("head_policy_read")
	}
	if !atHead.Policy.Enabled || atHead.Policy.Version != candidate.Version || atHead.Policy.Root != candidate.Root {
		return PolicyEvidence{}, policyEvidenceStageError("head_policy_match")
	}
	// Detect an in-flight reorg after the head reads. Later reorganizations
	// remain possible and must be handled by upstream revalidation.
	again, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil || again == nil || again.Number == nil || !again.Number.IsUint64() || again.Number.Uint64() != block || again.Hash() != receipt.BlockHash {
		return PolicyEvidence{}, policyEvidenceStageError("receipt_header_recheck")
	}
	latestHead, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(head))
	if err != nil || latestHead == nil || latestHead.Number == nil || !latestHead.Number.IsUint64() || latestHead.Number.Uint64() != head || latestHead.Hash() != atHead.BlockHash {
		return PolicyEvidence{}, policyEvidenceStageError("head_header_recheck")
	}
	return PolicyEvidence{ReceiptBlock: atReceipt, Head: atHead, TxHash: approvedTxHash}, nil
}

// A missing receipt is not an RPC failure, and an RPC cancellation is not
// evidence that a correctly supplied transaction hash does not exist.
func policyReceiptLookupCheck(ctx context.Context, receipt *types.Receipt, rpcErr error) string {
	if ctx == nil || ctx.Err() != nil {
		return "receipt_lookup_deadline"
	}
	if rpcErr != nil {
		return "receipt_lookup_rpc"
	}
	if receipt == nil {
		return "receipt_lookup_missing"
	}
	if receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) {
		return "receipt_lookup_malformed"
	}
	return ""
}
