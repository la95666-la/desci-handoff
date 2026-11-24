package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type stubPolicyRPC struct {
	chainID            *big.Int
	proxy              common.Address
	project            *big.Int
	implementation     common.Address
	implementationCode []byte
	slot               []byte
	header             *types.Header
	code               []byte
	version            []byte
	policy             []byte
	reads              int
	change             bool
	fail               bool
}

func (s *stubPolicyRPC) ChainID(context.Context) (*big.Int, error) {
	if s.fail {
		return nil, errors.New("sensitive RPC endpoint data")
	}
	return s.chainID, nil
}
func (s *stubPolicyRPC) HeaderByNumber(_ context.Context, _ *big.Int) (*types.Header, error) {
	s.reads++
	if s.change && s.reads == 2 {
		return &types.Header{Number: big.NewInt(77), Extra: []byte("replacement")}, nil
	}
	return s.header, nil
}
func (s *stubPolicyRPC) CodeAt(_ context.Context, address common.Address, block *big.Int) ([]byte, error) {
	if block.Cmp(s.header.Number) != 0 {
		return nil, errors.New("wrong snapshot contract")
	}
	if address == s.proxy {
		return s.code, nil
	}
	if address == s.implementation {
		return s.implementationCode, nil
	}
	return nil, errors.New("wrong snapshot contract")
}
func (s *stubPolicyRPC) StorageAt(_ context.Context, address common.Address, key common.Hash, block *big.Int) ([]byte, error) {
	if address != s.proxy || key != eip1967ImplementationSlot || block.Cmp(s.header.Number) != 0 {
		return nil, errors.New("wrong proxy implementation lookup")
	}
	return s.slot, nil
}
func (s *stubPolicyRPC) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if call.To == nil || *call.To != s.proxy || block.Cmp(s.header.Number) != 0 {
		return nil, errors.New("wrong snapshot contract")
	}
	if len(call.Data) == 4 && bytes.Equal(call.Data, v3VersionSelector) {
		return s.version, nil
	}
	if len(call.Data) == 36 && bytes.Equal(call.Data[:4], v3PolicySelector) {
		if new(big.Int).SetBytes(call.Data[4:]).Cmp(s.project) != 0 {
			return nil, errors.New("wrong project ID")
		}
		return s.policy, nil
	}
	return nil, errors.New("unexpected selector")
}

func TestReadPolicyAtPinnedIdentityAndStrictABI(t *testing.T) {
	const buyer = "0x1111111111111111111111111111111111111111"
	scope := Scope{ChainID: big.NewInt(31337), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"), ProjectID: big.NewInt(41376991395446785)}
	tree, err := Build(scope, []string{buyer})
	if err != nil {
		t.Fatal(err)
	}
	policyData := make([]byte, 96)
	policyData[31] = 1
	copy(policyData[32:64], tree.Root[:])
	policyData[95] = 7
	implementation := common.HexToAddress("0x2234567890123456789012345678901234567890")
	proxyCode := []byte{0x60, 0x80}
	implementationCode := []byte{0x60, 0x01}
	runtime := RuntimeIdentity{Implementation: implementation, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implementationCode)}
	slot := make([]byte, 32)
	copy(slot[12:], implementation[:])
	stub := &stubPolicyRPC{
		chainID: big.NewInt(31337), proxy: scope.ProxyAddress, project: scope.ProjectID,
		implementation: implementation, implementationCode: implementationCode, slot: slot,
		header: &types.Header{Number: big.NewInt(77)}, code: proxyCode,
		version: v3ContractVersion[:], policy: policyData,
	}
	hash := stub.header.Hash()
	read := func(identity Scope, want RuntimeIdentity, digest common.Hash) (PolicyObservation, error) {
		return ReadPolicyAt(context.Background(), stub, identity, want, 77, digest)
	}
	result, err := read(scope, runtime, hash)
	if err != nil || result.BlockNumber != 77 || result.BlockHash != hash {
		t.Fatalf("valid policy snapshot rejected: %+v err=%v", result, err)
	}
	version, proof, err := tree.ProofForPolicy(scope, result.Policy, buyer)
	if err != nil || version != 7 || len(proof) != 0 {
		t.Fatalf("observed policy not compatible with proof: version=%d proof=%v err=%v", version, proof, err)
	}
	if _, err := read(scope, runtime, common.HexToHash("0x01")); err == nil {
		t.Fatal("block hash mismatch admitted")
	}
	if _, err := ReadPolicyAt(context.Background(), stub, scope, runtime, 0, hash); err == nil {
		t.Fatal("unpinned block accepted")
	}
	if _, err := ReadPolicyAt(context.Background(), stub, scope, runtime, 77, common.Hash{}); err == nil {
		t.Fatal("empty block hash accepted")
	}
	badScope := scope
	badScope.ChainID = big.NewInt(56)
	if _, err := read(badScope, runtime, hash); err == nil {
		t.Fatal("chain mismatch admitted")
	}
	badScope = scope
	badScope.ProjectID = big.NewInt(42)
	if _, err := read(badScope, runtime, hash); err == nil {
		t.Fatal("wrong project policy selector accepted")
	}
	badRuntime := runtime
	badRuntime.ProxyCodeHash = common.HexToHash("0x01")
	if _, err := read(scope, badRuntime, hash); err == nil {
		t.Fatal("proxy code hash mismatch admitted")
	}
	badRuntime = runtime
	badRuntime.CodeHash = common.HexToHash("0x01")
	if _, err := read(scope, badRuntime, hash); err == nil {
		t.Fatal("implementation code hash mismatch admitted")
	}
	badRuntime = runtime
	badRuntime.Implementation = scope.ProxyAddress
	if _, err := read(scope, badRuntime, hash); err == nil {
		t.Fatal("proxy accepted as implementation")
	}
	stub.slot = make([]byte, 32)
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("wrong EIP-1967 implementation slot admitted")
	}
	stub.slot = slot
	stub.version = make([]byte, 32)
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("wrong contract version admitted")
	}
	stub.version = v3ContractVersion[:]
	stub.policy = make([]byte, 95)
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("short policy result accepted")
	}
	stub.policy = append([]byte{}, policyData...)
	stub.policy[64] = 1 // dirty high bits in uint64 ABI word
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("noncanonical uint64 ABI admitted")
	}
	stub.policy = append([]byte{}, policyData...)
	stub.policy[30] = 1 // bool word != 0 or 1
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("noncanonical bool ABI admitted")
	}
	stub.policy = append([]byte{}, policyData...)
	stub.policy[32] = 0
	for i := 32; i < 64; i++ {
		stub.policy[i] = 0
	}
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("enabled zero root admitted")
	}
	stub.policy = policyData
	stub.policy = make([]byte, 96)
	disabled, err := read(scope, runtime, hash)
	if err != nil || disabled.Policy.Enabled || disabled.Policy.Version != 0 || disabled.Policy.Root != (common.Hash{}) {
		t.Fatalf("initial disabled policy rejected: %+v err=%v", disabled, err)
	}
	stub.policy = policyData
	stub.code = nil
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("missing code admitted")
	}
	stub.code = proxyCode
	stub.change, stub.reads = true, 0
	if _, err := read(scope, runtime, hash); err == nil {
		t.Fatal("mid-read reorg admitted")
	}
	stub.change, stub.fail, stub.reads = false, true, 0
	if _, err := read(scope, runtime, hash); err == nil || bytes.Contains([]byte(err.Error()), []byte("sensitive")) {
		t.Fatalf("RPC error leaked or accepted: %v", err)
	}
}
