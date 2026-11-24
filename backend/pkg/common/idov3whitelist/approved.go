package idov3whitelist

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
)

// ApprovedSnapshot is a *caller-supplied* immutable full-member record. This
// package cannot authenticate its database provenance or the identities and
// permissions of either Admin. Never construct it from Warmup registrations.
type ApprovedSnapshot struct {
	Scope           Scope
	ExpectedVersion uint64
	Commitment      SnapshotCommitment
	Members         []string
	InitiatorID     uint64
	ApproverID      uint64
	ApprovalID      uint64
	ExecutionTx     common.Hash
}

// VerifyApprovedSnapshot detects incomplete, reordered, altered or
// cross-project member records and mismatched approval/chain evidence. This
// is a necessary check, not a proof of actual four-eye approval, receipt
// authenticity, current canonicality, or permission to distribute proofs.
func VerifyApprovedSnapshot(record ApprovedSnapshot, evidence PolicyEvidence) (*Tree, error) {
	if err := validateScope(record.Scope); err != nil {
		return nil, err
	}
	if record.InitiatorID == 0 || record.ApproverID == 0 || record.InitiatorID == record.ApproverID || record.ApprovalID == 0 || record.ExecutionTx == (common.Hash{}) || evidence.TxHash != record.ExecutionTx ||
		record.ExpectedVersion == ^uint64(0) || record.Commitment.Version != record.ExpectedVersion+1 || len(record.Members) == 0 || len(record.Members) > MaxWallets || uint32(len(record.Members)) != record.Commitment.MemberCount {
		return nil, errors.New("V3 snapshot approval or execution binding is invalid")
	}
	tree, err := Build(record.Scope, record.Members)
	if err != nil {
		return nil, err
	}
	// Build sorts the leaves; require the persisted list to already be in the
	// same canonical order, never silently repair an incomplete audit record.
	for i, member := range tree.Members() {
		if record.Members[i] != member.Wallet.Hex() {
			return nil, errors.New("V3 snapshot member order or encoding is not canonical")
		}
	}
	candidate, err := tree.CommitSnapshot(record.Scope, record.Commitment.Version)
	if err != nil || candidate != record.Commitment {
		return nil, errors.New("V3 approved snapshot commitment mismatch")
	}
	if !evidence.ReceiptBlock.Policy.Enabled || !evidence.Head.Policy.Enabled || evidence.ReceiptBlock.Policy.Version != candidate.Version || evidence.Head.Policy.Version != candidate.Version ||
		evidence.ReceiptBlock.Policy.Root != candidate.Root || evidence.Head.Policy.Root != candidate.Root || evidence.ReceiptBlock.BlockNumber == 0 || evidence.Head.BlockNumber < evidence.ReceiptBlock.BlockNumber || evidence.ReceiptBlock.BlockHash == (common.Hash{}) || evidence.Head.BlockHash == (common.Hash{}) {
		return nil, errors.New("V3 approved snapshot chain evidence mismatch")
	}
	return tree, nil
}
