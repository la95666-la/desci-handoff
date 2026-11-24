package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type governanceRPCStub struct {
	*stubPolicyRPC
	owner, governor, operator common.Address
	ownerCode, governorCode   []byte
	badReturn                 bool
	badSelector               bool
	wrongGetter               bool
	changed                   bool
	headReads                 int
	failed                    bool
	ownerReads                int
	switchOwnerAfterFirst     bool
}

func (s *governanceRPCStub) CodeAt(ctx context.Context, address common.Address, block *big.Int) ([]byte, error) {
	if address == s.owner {
		return s.ownerCode, nil
	}
	if address == s.governor {
		return s.governorCode, nil
	}
	return s.stubPolicyRPC.CodeAt(ctx, address, block)
}

func (s *governanceRPCStub) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if s.failed {
		return nil, errors.New("sensitive governance RPC response")
	}
	if bytes.Equal(call.Data, v3OwnerSelector) || bytes.Equal(call.Data, v3GovernorSelector) || bytes.Equal(call.Data, v3OperatorSelector) {
		if s.badSelector && bytes.Equal(call.Data, v3GovernorSelector) {
			return nil, errors.New("wrong selector")
		}
		address := s.owner
		if bytes.Equal(call.Data, v3OwnerSelector) {
			s.ownerReads++
			if s.switchOwnerAfterFirst && s.ownerReads > 1 {
				address = common.HexToAddress("0x9999")
			}
		}
		if bytes.Equal(call.Data, v3GovernorSelector) {
			address = s.governor
			if s.wrongGetter {
				address = s.owner
			}
		}
		if bytes.Equal(call.Data, v3OperatorSelector) {
			address = s.operator
			if s.wrongGetter {
				address = s.owner
			}
		}
		result := common.LeftPadBytes(address.Bytes(), 32)
		if s.badReturn {
			result[0] = 1
		}
		return result, nil
	}
	return s.stubPolicyRPC.CallContract(ctx, call, block)
}

func TestReadLifecycleCallersAtRequiresPinnedOperatorGetter(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	impl := common.HexToAddress("0x2234567890123456789012345678901234567890")
	owner := common.HexToAddress("0x3234567890123456789012345678901234567890")
	governor := common.HexToAddress("0x4234567890123456789012345678901234567890")
	operator := common.HexToAddress("0x5234567890123456789012345678901234567890")
	proxyCode, implCode, ownerCode, governorCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}, []byte{0x60, 0x02}, []byte{0x60, 0x03}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode)}
	expected := GovernanceIdentity{Owner: owner, Governor: governor, OwnerCodeHash: crypto.Keccak256Hash(ownerCode), GovernorCodeHash: crypto.Keccak256Hash(governorCode)}
	base := &stubPolicyRPC{chainID: scope.ChainID, proxy: scope.ProxyAddress, project: scope.ProjectID,
		implementation: impl, implementationCode: implCode, slot: common.LeftPadBytes(impl.Bytes(), 32),
		header: &types.Header{Number: big.NewInt(90)}, code: proxyCode, version: v3ContractVersion[:], policy: make([]byte, 96)}
	stub := &governanceRPCStub{stubPolicyRPC: base, owner: owner, governor: governor, operator: operator, ownerCode: ownerCode, governorCode: governorCode}
	hash := base.header.Hash()
	read := func(want common.Address) error {
		stub.headReads, base.reads = 0, 0
		got, err := ReadLifecycleCallersAt(context.Background(), stub, scope, runtime, expected, want, 90, hash)
		if err == nil && (got.Owner != owner || got.Operator != operator || got.BlockHash != hash || got.BlockNumber != 90) {
			t.Fatal("wrong lifecycle caller observation")
		}
		return err
	}
	if err := read(operator); err != nil {
		t.Fatalf("correct operator rejected: %v", err)
	}
	for _, bad := range []common.Address{common.Address{}, scope.ProxyAddress, impl, owner, governor, common.HexToAddress("0x6234567890123456789012345678901234567890")} {
		if err := read(bad); err == nil {
			t.Fatal("invalid lifecycle operator accepted")
		}
	}
	stub.wrongGetter = true
	if err := read(operator); err == nil {
		t.Fatal("wrong operator getter accepted")
	}
	stub.wrongGetter = false
	stub.badReturn = true
	if err := read(operator); err == nil {
		t.Fatal("malformed operator getter accepted")
	}
	stub.badReturn = false
	stub.changed = true
	if err := read(operator); err == nil {
		t.Fatal("changed block accepted")
	}
	stub.changed = false
	stub.failed = true
	if err := read(operator); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("RPC failure leaked or accepted: %v", err)
	}
}

func (s *governanceRPCStub) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	s.headReads++
	if s.changed && s.headReads == 3 {
		return &types.Header{Number: block, Extra: []byte("replacement")}, nil
	}
	return s.stubPolicyRPC.HeaderByNumber(ctx, block)
}

func TestReadGovernanceAtIsPinnedReadOnlyAndNotApproval(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	impl := common.HexToAddress("0x2234567890123456789012345678901234567890")
	owner := common.HexToAddress("0x3234567890123456789012345678901234567890")
	governor := common.HexToAddress("0x4234567890123456789012345678901234567890")
	proxyCode, implCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}
	ownerCode, governorCode := []byte{0x60, 0x02}, []byte{0x60, 0x03}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode)}
	expected := GovernanceIdentity{Owner: owner, Governor: governor, OwnerCodeHash: crypto.Keccak256Hash(ownerCode), GovernorCodeHash: crypto.Keccak256Hash(governorCode)}
	policy := make([]byte, 96) // consistent disabled, version 0
	slot := common.LeftPadBytes(impl.Bytes(), 32)
	base := &stubPolicyRPC{chainID: big.NewInt(31337), proxy: scope.ProxyAddress, project: scope.ProjectID,
		implementation: impl, implementationCode: implCode, slot: slot, header: &types.Header{Number: big.NewInt(90)},
		code: proxyCode, version: v3ContractVersion[:], policy: policy}
	stub := &governanceRPCStub{stubPolicyRPC: base, owner: owner, governor: governor, ownerCode: ownerCode, governorCode: governorCode}
	hash := base.header.Hash()
	read := func(want GovernanceIdentity) error {
		stub.headReads, base.reads = 0, 0
		observed, err := ReadGovernanceAt(context.Background(), stub, scope, runtime, want, 90, hash)
		if err == nil && (observed.BlockHash != hash || observed.BlockNumber != 90 || observed.Owner != owner || observed.Governor != governor) {
			t.Fatal("wrong governance observation")
		}
		return err
	}
	if err := read(expected); err != nil {
		t.Fatalf("pinned governance observation rejected: %v", err)
	}
	bad := expected
	bad.OwnerCodeHash = common.HexToHash("0x01")
	if err := read(bad); err == nil {
		t.Fatal("wrong owner code accepted")
	}
	bad = expected
	bad.GovernorCodeHash = common.HexToHash("0x01")
	if err := read(bad); err == nil {
		t.Fatal("wrong governor code accepted")
	}
	bad = expected
	bad.Owner = common.HexToAddress("0x5234567890123456789012345678901234567890")
	if err := read(bad); err == nil {
		t.Fatal("wrong owner accepted")
	}
	bad = expected
	bad.Governor = impl
	if err := read(bad); err == nil {
		t.Fatal("implementation as governor accepted")
	}
	bad = expected
	bad.Governor = owner
	if err := read(bad); err == nil {
		t.Fatal("same contract for owner and governor accepted")
	}
	stub.badReturn = true
	if err := read(expected); err == nil {
		t.Fatal("malformed address ABI accepted")
	}
	stub.badReturn = false
	stub.badSelector = true
	if err := read(expected); err == nil {
		t.Fatal("missing governor read accepted")
	}
	stub.badSelector = false
	stub.wrongGetter = true
	if err := read(expected); err == nil {
		t.Fatal("governor getter mismatch accepted")
	}
	stub.wrongGetter = false
	stub.governorCode = nil
	if err := read(expected); err == nil {
		t.Fatal("governor with no code accepted")
	}
	stub.governorCode = governorCode
	stub.changed = true
	if err := read(expected); err == nil {
		t.Fatal("replaced block accepted")
	}
	stub.changed = false
	stub.failed = true
	if err := read(expected); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw RPC leak or error accepted: %v", err)
	}
}
