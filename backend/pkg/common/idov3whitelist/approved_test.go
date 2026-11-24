package idov3whitelist

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestApprovedSnapshotRequiresExactCanonicalCompleteMembers(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	tree, err := Build(scope, []string{"0x2222222222222222222222222222222222222222", "0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	record := ApprovedSnapshot{Scope: scope, Commitment: commitment, InitiatorID: 7, ApproverID: 8, ApprovalID: 9, ExecutionTx: common.HexToHash("0x01")}
	for _, member := range tree.Members() {
		record.Members = append(record.Members, member.Wallet.Hex())
	}
	evidence := PolicyEvidence{TxHash: record.ExecutionTx, ReceiptBlock: PolicyObservation{Policy: Policy{Enabled: true, Root: commitment.Root, Version: 1}, BlockNumber: 80, BlockHash: common.HexToHash("0x02")}, Head: PolicyObservation{Policy: Policy{Enabled: true, Root: commitment.Root, Version: 1}, BlockNumber: 100, BlockHash: common.HexToHash("0x03")}}
	if got, err := VerifyApprovedSnapshot(record, evidence); err != nil || got.Root != commitment.Root {
		t.Fatalf("valid snapshot rejected: %v", err)
	}
	tests := map[string]func(*ApprovedSnapshot, *PolicyEvidence){
		"same admin":            func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.ApproverID = r.InitiatorID },
		"missing approval":      func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.ApprovalID = 0 },
		"wrong tx":              func(_ *ApprovedSnapshot, e *PolicyEvidence) { e.TxHash = common.HexToHash("0x04") },
		"missing member":        func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.Members = r.Members[:1] },
		"reordered":             func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.Members[0], r.Members[1] = r.Members[1], r.Members[0] },
		"wrong digest":          func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.Commitment.Digest = common.HexToHash("0x05") },
		"stale version":         func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.ExpectedVersion = 1 },
		"wrong project":         func(r *ApprovedSnapshot, _ *PolicyEvidence) { r.Scope.ProjectID = big.NewInt(78) },
		"changed head":          func(_ *ApprovedSnapshot, e *PolicyEvidence) { e.Head.Policy.Version = 2 },
		"missing receipt block": func(_ *ApprovedSnapshot, e *PolicyEvidence) { e.ReceiptBlock.BlockHash = common.Hash{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			r := record
			r.Members = append([]string(nil), record.Members...)
			e := evidence
			mutate(&r, &e)
			if _, err := VerifyApprovedSnapshot(r, e); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}
