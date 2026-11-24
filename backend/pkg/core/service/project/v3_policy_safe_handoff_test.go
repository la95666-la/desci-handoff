package project

import (
	"bytes"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3PolicySafeHandoffBindsApprovedPreviewCall(t *testing.T) {
	scope := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(93),
		ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111")}
	tree, err := idov3whitelist.Build(scope, []string{"0x3333333333333333333333333333333333333333"})
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := tree.CommitSnapshot(scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	preview := V3PolicyPreview{ChainID: "56", ProjectID: "93", Proxy: strings.ToLower(scope.ProxyAddress.Hex()),
		Block: "90", BlockHash: strings.ToLower(common.HexToHash("0x1234").Hex()), State: "pending",
		OldRoot: (common.Hash{}).Hex(), OldVersion: "0", TargetVersion: "1", Enabled: true,
		Root: strings.ToLower(commitment.Root.Hex()), MembersDigest: strings.ToLower(commitment.MembersDigest.Hex()),
		SnapshotDigest: strings.ToLower(commitment.Digest.Hex()), MemberCount: 1, Reason: "review",
		CanonicalMembers: []string{strings.ToLower(tree.Members()[0].Wallet.Hex())},
		AddedMembers:     []string{strings.ToLower(tree.Members()[0].Wallet.Hex())}, RemovedMembers: []string{}}
	preview.PreviewDigest, err = V3PolicyPreviewDigest(preview)
	if err != nil {
		t.Fatal(err)
	}
	governor := "0x2222222222222222222222222222222222222222"
	handoff, err := makeV3PolicySafeHandoff(preview, 11, governor)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.ChainID != "56" || handoff.ProjectID != "93" || handoff.Proxy != preview.Proxy ||
		handoff.Governor != governor || handoff.ReviewID != "11" || handoff.PreviewDigest != preview.PreviewDigest ||
		handoff.Value != "0" || handoff.Operation != "CALL" || handoff.Root != preview.Root ||
		handoff.ExpectedVersion != "0" || handoff.TargetVersion != "1" {
		t.Fatalf("handoff identity mismatch: %+v", handoff)
	}
	expected, err := idov3whitelist.PackPolicyChangeCall(scope, idov3whitelist.PolicyChange{
		ExpectedVersion: 0, TargetVersion: 1, Enabled: true, Root: commitment.Root, Candidate: &commitment})
	if err != nil || !bytes.Equal(common.FromHex(handoff.Calldata), expected) ||
		!bytes.Equal(expected[:4], crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4]) {
		t.Fatal("handoff calldata differs from exact reviewed policy change")
	}
	for _, wrong := range []struct {
		name    string
		preview V3PolicyPreview
	}{
		{"foreign project", func() V3PolicyPreview { p := preview; p.ProjectID = "94"; return p }()},
		{"changed root", func() V3PolicyPreview { p := preview; p.Root = common.HexToHash("0x01").Hex(); return p }()},
		{"missing digest", func() V3PolicyPreview { p := preview; p.PreviewDigest = ""; return p }()},
		{"changed review reason", func() V3PolicyPreview { p := preview; p.Reason = "unreviewed"; return p }()},
	} {
		t.Run(wrong.name, func(t *testing.T) {
			if _, err := makeV3PolicySafeHandoff(wrong.preview, 11, governor); err == nil {
				t.Fatal("invalid review yielded Safe handoff")
			}
		})
	}
}

func TestV3FirstEnableHandoffScopeIsReadOnlyPendingV0(t *testing.T) {
	base := V3PolicyPreview{State: "pending", OldVersion: "0", TargetVersion: "1", Enabled: true}
	if !v3FirstEnablePolicyReview(base) {
		t.Fatal("first-enable review cannot use canonical forward cursor")
	}
	for _, change := range []func(*V3PolicyPreview){
		func(p *V3PolicyPreview) { p.State = "active" },
		func(p *V3PolicyPreview) { p.OldEnabled = true },
		func(p *V3PolicyPreview) { p.OldVersion = "1" },
		func(p *V3PolicyPreview) { p.TargetVersion = "2" },
		func(p *V3PolicyPreview) { p.Enabled = false },
	} {
		p := base
		change(&p)
		if v3FirstEnablePolicyReview(p) {
			t.Fatalf("non-initial handoff accepted: %+v", p)
		}
	}
}
