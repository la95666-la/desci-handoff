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

type setRuntimeStub struct {
	*governanceRPCStub
	manifest                                        SetManifest
	badFee, badConfig, badDecimals, changed, failed bool
	reads                                           int
}

func (s *setRuntimeStub) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	s.reads++
	if s.changed && s.reads > 4 {
		return &types.Header{Number: big.NewInt(90), Extra: []byte("replaced")}, nil
	}
	return s.governanceRPCStub.HeaderByNumber(ctx, n)
}

func (s *setRuntimeStub) CodeAt(ctx context.Context, a common.Address, block *big.Int) ([]byte, error) {
	if a == common.HexToAddress(s.manifest.ProjectToken) || a == common.HexToAddress(s.manifest.PaymentToken) {
		panic("Set must not read token bytecode")
	}
	return s.governanceRPCStub.CodeAt(ctx, a, block)
}

func (s *setRuntimeStub) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if s.failed {
		return nil, errors.New("sensitive RPC content")
	}
	if len(call.Data) >= 4 {
		selector := call.Data[:4]
		match := func(signature string) bool { return bytes.Equal(selector, crypto.Keccak256([]byte(signature))[:4]) }
		switch {
		case match("feeReceiver()"):
			if s.badFee {
				return make([]byte, 32), nil
			}
			return common.LeftPadBytes(common.HexToAddress(s.manifest.FeeReceiver).Bytes(), 32), nil
		case match("successFeeRate()"):
			return common.LeftPadBytes(big.NewInt(int64(s.manifest.FeeRate)).Bytes(), 32), nil
		case match("decimals()"):
			if s.badDecimals {
				return nil, nil
			}
			return common.LeftPadBytes(big.NewInt(18).Bytes(), 32), nil
		case match("paymentTokenConfigs(address)"), match("projectTokenConfigs(address)"):
			result := append(common.LeftPadBytes(big.NewInt(1).Bytes(), 32), common.LeftPadBytes(big.NewInt(18).Bytes(), 32)...)
			if s.badConfig {
				result[31] = 0
			}
			return result, nil
		}
	}
	return s.governanceRPCStub.CallContract(ctx, call, block)
}

func TestVerifySetRuntimeAtRequiresIndependentPinnedIdentity(t *testing.T) {
	a := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	proxy, impl, owner, governor := a("1"), a("2"), a("3"), a("4")
	base := &stubPolicyRPC{chainID: big.NewInt(31337), proxy: common.HexToAddress(proxy), project: big.NewInt(19),
		implementation: common.HexToAddress(impl), code: []byte{0x60, 0x80}, implementationCode: []byte{0x60, 0x01},
		slot: common.LeftPadBytes(common.HexToAddress(impl).Bytes(), 32), header: &types.Header{Number: big.NewInt(90)},
		version: v3ContractVersion[:], policy: make([]byte, 96)}
	ownerCode, governorCode := []byte{0x60, 0x02}, []byte{0x60, 0x03}
	m := SetManifest{ChainID: 31337, ProjectID: 19, FundingMode: "capped_presale", ContractVersion: "ido_v3_stablecoin", ProxyAddress: proxy,
		Implementation: impl, OwnerSafe: owner, Governor: governor, Operator: a("9"), ProjectToken: a("5"), PaymentToken: a("6"),
		FeeReceiver: a("7"), FeeRate: 200, PaymentDecimals: 18, StartBlock: 90, Confirmations: 1,
		ProxyCodeHash: crypto.Keccak256Hash(base.code).Hex(), ImplementationCodeHash: crypto.Keccak256Hash(base.implementationCode).Hex(),
		OwnerCodeHash: crypto.Keccak256Hash(ownerCode).Hex(), GovernorCodeHash: crypto.Keccak256Hash(governorCode).Hex()}
	stub := &setRuntimeStub{governanceRPCStub: &governanceRPCStub{stubPolicyRPC: base, owner: common.HexToAddress(owner),
		governor: common.HexToAddress(governor), operator: common.HexToAddress(m.Operator), ownerCode: ownerCode, governorCode: governorCode}, manifest: m}
	check := func(m SetManifest) error {
		stub.reads, base.reads = 0, 0
		return VerifySetRuntimeAt(context.Background(), stub, m)
	}
	if err := check(m); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"fee mismatch", func() { stub.badFee = true }}, {"token not allowed", func() { stub.badConfig = true }},
		{"token decimals invalid", func() { stub.badDecimals = true }}, {"replaced block", func() { stub.changed = true }},
		{"RPC failure", func() { stub.failed = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub.badFee, stub.badConfig, stub.badDecimals, stub.changed, stub.failed = false, false, false, false, false
			tc.change()
			if err := check(m); err == nil || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("unchecked runtime or exposed RPC: %v", err)
			}
		})
	}
	stub.badFee, stub.badConfig, stub.badDecimals, stub.changed, stub.failed = false, false, false, false, false
	m.Confirmations = 2
	if err := check(m); err == nil {
		t.Fatal("insufficient depth accepted")
	}
	m.Confirmations = 1
	m.Operator = a("8")
	if err := check(m); err == nil {
		t.Fatal("wrong operator accepted")
	}
	m.Operator = a("9")
	m.FundingMode = "fair_launch"
	if err := check(m); err == nil {
		t.Fatal("wrong funding mode accepted")
	}
	m.FundingMode = "capped_presale"
	m.ContractVersion = "v2_stablecoin"
	if err := check(m); err == nil {
		t.Fatal("V2 identity accepted")
	}
}
