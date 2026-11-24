package project

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedPolicyExecutionDigestBindsReviewActorsAndOriginalHashes(t *testing.T) {
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(19),
		ProxyAddress: common.HexToAddress(v3TokenProxy)}
	tree, err := idov3whitelist.Build(scope, []string{"0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	member := strings.ToLower(tree.Members()[0].Wallet.Hex())
	p := V3PolicyPreview{ChainID: "56", ProjectID: "19", Proxy: strings.ToLower(v3TokenProxy),
		Block: "128805700", BlockHash: "0x" + strings.Repeat("1", 64), State: "pending",
		OldRoot: (common.Hash{}).Hex(), OldVersion: "0", TargetVersion: "1",
		Enabled: true, Root: strings.ToLower(commitment.Root.Hex()),
		MembersDigest:  strings.ToLower(commitment.MembersDigest.Hex()),
		SnapshotDigest: strings.ToLower(commitment.Digest.Hex()), MemberCount: 1,
		CanonicalMembers: []string{member}, AddedMembers: []string{member}, RemovedMembers: []string{}, Reason: "four-eyes"}
	p.PreviewDigest, err = V3PolicyPreviewDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	txHash, safeHash, blockHash := "0x"+strings.Repeat("2", 64), "0x"+strings.Repeat("3", 64), "0x"+strings.Repeat("4", 64)
	set := idov3whitelist.ApprovedSetIdentity{ProjectID: 19, ChainID: 56, Proxy: p.Proxy,
		ProjectToken: "0x2222222222222222222222222222222222222222", TokenDecimals: 18,
		PaymentToken: strings.ToLower(v3TokenUSDT), PaymentDecimals: 18,
		ProjectOwner: "0x3333333333333333333333333333333333333333",
		FeeReceiver:  "0x4444444444444444444444444444444444444444", FeeRate: 100,
		Operator: strings.ToLower(v3DiagnosticOperator), RequestDigest: "0x" + strings.Repeat("a", 64),
		CreateCallHash: "0x" + strings.Repeat("b", 64), CreateDeadline: 1800000000,
		ApprovalID: 5, InitiatorID: 6, ApproverID: 7}
	baseline, err := digestV3ApprovedPolicyExecution(p, set, 7, 8, 9, txHash, safeHash, 128805710, blockHash)
	if err != nil || !v3CanonicalHash(baseline) {
		t.Fatalf("valid identity rejected: %v", err)
	}
	for _, tc := range []struct {
		name    string
		change  func(*V3PolicyPreview, *uint64, *uint64, *uint64, *string, *string, *uint64, *string)
		invalid bool
	}{
		{"different review", func(_ *V3PolicyPreview, r, _, _ *uint64, _, _ *string, _ *uint64, _ *string) { *r = 10 }, false},
		{"different initiator", func(_ *V3PolicyPreview, _, i, _ *uint64, _, _ *string, _ *uint64, _ *string) { *i = 10 }, false},
		{"self approval", func(_ *V3PolicyPreview, _, i, a *uint64, _, _ *string, _ *uint64, _ *string) { *i = *a }, true},
		{"changed transaction", func(_ *V3PolicyPreview, _, _, _ *uint64, tx, _ *string, _ *uint64, _ *string) {
			*tx = "0x" + strings.Repeat("5", 64)
		}, false},
		{"changed Safe hash", func(_ *V3PolicyPreview, _, _, _ *uint64, _, safe *string, _ *uint64, _ *string) {
			*safe = "0x" + strings.Repeat("5", 64)
		}, false},
		{"changed receipt height", func(_ *V3PolicyPreview, _, _, _ *uint64, _, _ *string, block *uint64, _ *string) { *block++ }, false},
		{"changed receipt hash", func(_ *V3PolicyPreview, _, _, _ *uint64, _, _ *string, _ *uint64, hash *string) {
			*hash = "0x" + strings.Repeat("5", 64)
		}, false},
		{"altered review without digest", func(preview *V3PolicyPreview, _, _, _ *uint64, _, _ *string, _ *uint64, _ *string) {
			preview.Reason = "changed"
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preview, review, initiator, approver, tx, safe, height, receipt := p, uint64(7), uint64(8), uint64(9), txHash, safeHash, uint64(128805710), blockHash
			tc.change(&preview, &review, &initiator, &approver, &tx, &safe, &height, &receipt)
			got, err := digestV3ApprovedPolicyExecution(preview, set, review, initiator, approver, tx, safe, height, receipt)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid identity accepted")
				}
			} else if err != nil || got == baseline {
				t.Fatalf("independent evidence not bound: %v", err)
			}
		})
	}
	changedSet := set
	changedSet.ApprovalID++
	if got, err := digestV3ApprovedPolicyExecution(p, changedSet, 7, 8, 9, txHash, safeHash, 128805710, blockHash); err != nil || got == baseline {
		t.Fatal("another Set approval borrowed policy execution")
	}
}
