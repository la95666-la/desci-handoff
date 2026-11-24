package idov3whitelist

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"strings"
	"testing"
)

func dynamicOwnerFixture() (*governanceRPCStub, Scope, RuntimeIdentity, GovernanceIdentity) {
	proxy, impl := common.HexToAddress("0x1234"), common.HexToAddress("0x2345")
	owner, gov, op := common.HexToAddress("0x3456"), common.HexToAddress("0x4567"), common.HexToAddress("0x5678")
	pc, ic, oc, gc := []byte{1}, []byte{2}, []byte{3}, []byte{4}
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: proxy}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(pc), CodeHash: crypto.Keccak256Hash(ic)}
	expected := GovernanceIdentity{FollowOwner: true, Governor: gov, OwnerCodeHash: crypto.Keccak256Hash(oc), GovernorCodeHash: crypto.Keccak256Hash(gc)}
	base := &stubPolicyRPC{chainID: scope.ChainID, proxy: proxy, project: scope.ProjectID, implementation: impl, implementationCode: ic, slot: common.LeftPadBytes(impl.Bytes(), 32), header: &types.Header{Number: big.NewInt(90)}, code: pc, version: v3ContractVersion[:], policy: make([]byte, 96)}
	return &governanceRPCStub{stubPolicyRPC: base, owner: owner, governor: gov, operator: op, ownerCode: oc, governorCode: gc}, scope, runtime, expected
}

func TestDynamicOwnerAcceptsSuccessiveSafesWithoutChangingTrustRoot(t *testing.T) {
	for _, address := range []string{"0x3456", "0x6789", "0x9876"} {
		s, scope, rt, g := dynamicOwnerFixture()
		s.owner = common.HexToAddress(address)
		got, e := ReadGovernanceAt(context.Background(), s, scope, rt, g, 90, s.header.Hash())
		if e != nil || got.Owner != s.owner {
			t.Fatalf("migrated Safe rejected: %v", e)
		}
		callers, e := ReadLifecycleCallersAt(context.Background(), s, scope, rt, g, s.operator, 90, s.header.Hash())
		if e != nil || callers.Owner != s.owner || callers.Operator != s.operator {
			t.Fatalf("migrated caller identity rejected: %v", e)
		}
		// Legacy signed-manifest mode stays strict, not silently dynamic.
		g.FollowOwner = false
		g.Owner = common.HexToAddress("0xabcd")
		if _, e = ReadGovernanceAt(context.Background(), s, scope, rt, g, 90, s.header.Hash()); e == nil {
			t.Fatal("legacy pinned owner relaxed")
		}
	}
}

func TestDynamicOwnerRejectsMalformedForeignAndChangingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*governanceRPCStub, Scope, RuntimeIdentity, *GovernanceIdentity)
	}{
		{"zero", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) {
			s.owner = common.Address{}
		}},
		{"proxy", func(s *governanceRPCStub, p Scope, _ RuntimeIdentity, _ *GovernanceIdentity) {
			s.owner = p.ProxyAddress
		}},
		{"implementation", func(s *governanceRPCStub, _ Scope, r RuntimeIdentity, _ *GovernanceIdentity) {
			s.owner = r.Implementation
		}},
		{"operator", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.owner = s.operator }},
		{"governor", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.owner = s.governor }},
		{"EOA", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.ownerCode = nil }},
		{"wrong-code", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) {
			s.ownerCode = []byte{99}
		}},
		{"no-trust-root", func(_ *governanceRPCStub, _ Scope, _ RuntimeIdentity, g *GovernanceIdentity) {
			g.OwnerCodeHash = common.Hash{}
		}},
		{"malformed", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.badReturn = true }},
		{"owner-changed", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) {
			s.switchOwnerAfterFirst = true
		}},
		{"reorg", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.changed = true }},
		{"rpc-failure", func(s *governanceRPCStub, _ Scope, _ RuntimeIdentity, _ *GovernanceIdentity) { s.failed = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, scope, rt, g := dynamicOwnerFixture()
			tc.mutate(s, scope, rt, &g)
			_, e := ReadGovernanceAt(context.Background(), s, scope, rt, g, 90, s.header.Hash())
			if e == nil || strings.Contains(e.Error(), "sensitive") {
				t.Fatalf("unsafe identity accepted or secret leaked: %v", e)
			}
		})
	}
}
