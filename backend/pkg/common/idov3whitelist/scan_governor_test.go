package idov3whitelist

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReadScanGovernorAtBindsSignedAddressToFixedBlock(t *testing.T) {
	proxy := common.HexToAddress("0x1234567890123456789012345678901234567890")
	impl := common.HexToAddress("0x2234567890123456789012345678901234567890")
	governor := common.HexToAddress("0x4234567890123456789012345678901234567890")
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(77), ProxyAddress: proxy}
	base := &stubPolicyRPC{chainID: scope.ChainID, proxy: proxy, project: scope.ProjectID,
		implementation: impl, implementationCode: []byte{0x60, 0x01}, slot: common.LeftPadBytes(impl.Bytes(), 32),
		header: &types.Header{Number: big.NewInt(90)}, code: []byte{0x60, 0x80}, version: v3ContractVersion[:], policy: make([]byte, 96)}
	stub := &governanceRPCStub{stubPolicyRPC: base, governor: governor, governorCode: []byte{0x60, 0x03}}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(base.code), CodeHash: crypto.Keccak256Hash(base.implementationCode)}
	read := func(address common.Address) error {
		stub.headReads = 0
		return ReadScanGovernorAt(context.Background(), stub, scope, runtime, address, 90, base.header.Hash())
	}
	if err := read(governor); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []common.Address{{}, proxy, impl, common.HexToAddress("0x5234567890123456789012345678901234567890")} {
		if err := read(bad); err == nil {
			t.Fatal("unauthenticated governor accepted")
		}
	}
	stub.governorCode = nil
	if err := read(governor); err == nil {
		t.Fatal("EOA governor accepted")
	}
	stub.governorCode = []byte{0x60, 0x03}
	stub.wrongGetter = true
	if err := read(governor); err == nil {
		t.Fatal("wrong getter accepted")
	}
	stub.wrongGetter = false
	stub.badReturn = true
	if err := read(governor); err == nil {
		t.Fatal("malformed getter accepted")
	}
	stub.badReturn = false
	stub.changed = true
	if err := read(governor); err == nil {
		t.Fatal("header changed between runtime and governor read")
	}
	stub.changed = false
	stub.failed = true
	if err := read(governor); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("RPC error exposed or accepted: %v", err)
	}
}
