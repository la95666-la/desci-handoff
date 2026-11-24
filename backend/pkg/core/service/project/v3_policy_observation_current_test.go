package project

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
	"joincare_server/pkg/common/idov3whitelist"
)

type observationCurrentFake struct {
	proxy, impl, sale, payment common.Address
	header                     *types.Header
	head                       uint64
	headCalls, headerCalls     int
	policy                     []byte
	project                    []byte
	reorg, fail                bool
}

func (f *observationCurrentFake) ChainID(context.Context) (*big.Int, error) {
	return big.NewInt(56), nil
}
func (f *observationCurrentFake) BlockNumber(context.Context) (uint64, error) {
	f.headCalls++
	if f.fail {
		return 0, errors.New("RPC unavailable")
	}
	return f.head, nil
}
func (f *observationCurrentFake) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	f.headerCalls++
	f.head++ // deterministic normal growth during all reads
	if n == nil || n.Cmp(f.header.Number) != 0 {
		return nil, errors.New("wrong pinned block")
	}
	h := types.CopyHeader(f.header)
	if f.reorg && f.headerCalls >= 2 {
		h.Extra = []byte("replacement")
	}
	return h, nil
}
func (f *observationCurrentFake) CodeAt(_ context.Context, a common.Address, _ *big.Int) ([]byte, error) {
	if a == f.proxy {
		return []byte{0x60, 0x80}, nil
	}
	if a == f.impl {
		return []byte{0x60, 0x01}, nil
	}
	return nil, errors.New("wrong address")
}
func (f *observationCurrentFake) StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error) {
	return common.LeftPadBytes(f.impl.Bytes(), 32), nil
}
func (f *observationCurrentFake) CallContract(_ context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	f.head++
	if c.To == nil || *c.To != f.proxy || n.Cmp(f.header.Number) != 0 || len(c.Data) < 4 {
		return nil, errors.New("wrong pinned call")
	}
	if bytes.Equal(c.Data, crypto.Keccak256([]byte("contractVersion()"))[:4]) {
		h := crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1"))
		return h.Bytes(), nil
	}
	if len(c.Data) != 36 || new(big.Int).SetBytes(c.Data[4:]).Uint64() != 7 {
		return nil, errors.New("wrong project ID")
	}
	if bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]) {
		return append([]byte(nil), f.policy...), nil
	}
	if bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("getProject(uint256)"))[:4]) {
		return append([]byte(nil), f.project...), nil
	}
	return nil, errors.New("unsupported call")
}

func observationCurrentFixture() (idov3whitelist.Scope, idov3whitelist.AdmittedProject, V3PolicyPreview, *observationCurrentFake) {
	f := &observationCurrentFake{proxy: common.HexToAddress("0x11"), impl: common.HexToAddress("0x44"), sale: common.HexToAddress("0x22"), payment: common.HexToAddress("0x33"), head: 90, header: &types.Header{Number: big.NewInt(90)}, policy: make([]byte, 96)}
	root := common.HexToHash("0x1234")
	f.policy[31] = 1
	copy(f.policy[32:64], root[:])
	f.policy[95] = 1
	f.project = make([]byte, 32+23*32+128)
	word := func(index int, n *big.Int) {
		copy(f.project[32+index*32:32+(index+1)*32], n.FillBytes(make([]byte, 32)))
	}
	copy(f.project[:32], big.NewInt(32).FillBytes(make([]byte, 32)))
	word(0, big.NewInt(7))
	for i, a := range map[int]common.Address{1: f.sale, 2: f.payment, 3: f.sale, 4: f.payment} {
		word(i, new(big.Int).SetBytes(a.Bytes()))
	}
	for i, n := range map[int]int64{5: 18, 6: 18, 7: 200, 9: 20, 10: 1000, 11: 500, 12: 900, 13: 100, 14: 200, 17: 400, 21: 23 * 32, 22: 23*32 + 64} {
		word(i, big.NewInt(n))
	}
	for i, n := range []int64{1, 300, 1, 10000} {
		copy(f.project[32+23*32+i*32:], big.NewInt(n).FillBytes(make([]byte, 32)))
	}
	s := idov3whitelist.Scope{ChainID: big.NewInt(56), ProjectID: big.NewInt(7), ProxyAddress: f.proxy}
	a := idov3whitelist.AdmittedProject{Binding: idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: 7, ProxyAddress: f.proxy, ProjectToken: f.sale, PaymentToken: f.payment}, Runtime: idov3whitelist.RuntimeIdentity{Implementation: f.impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}}
	p := V3PolicyPreview{TargetVersion: "1", Enabled: true, Root: root.Hex()}
	return s, a, p, f
}

func TestObservationCurrentPolicyAllowsNormalChainGrowth(t *testing.T) {
	s, a, p, f := observationCurrentFixture()
	if e := checkApprovedV3ObservationCurrentPolicy(context.Background(), f, s, a, p); e != nil {
		t.Fatal(e)
	}
	if f.head <= 90 || f.headCalls != 1 {
		t.Fatalf("normal growth not modeled or frozen-head check reintroduced: head=%d headCalls=%d", f.head, f.headCalls)
	}
	if f.headerCalls < 3 {
		t.Fatal("canonical anchor rechecks omitted")
	}
}

func TestObservationCurrentPolicyRetainsIdentityAndPolicyChecks(t *testing.T) {
	for _, kind := range []string{"root", "version", "enabled", "reorg", "terminal", "identity", "active-public", "rpc", "state-transition"} {
		t.Run(kind, func(t *testing.T) {
			s, a, p, f := observationCurrentFixture()
			switch kind {
			case "root":
				p.Root = common.HexToHash("0x99").Hex()
			case "version":
				p.TargetVersion = "2"
			case "enabled":
				p.Enabled = false
			case "reorg":
				f.reorg = true
			case "terminal":
				f.project[32+8*32+31] = 4 // Cancelled
			case "identity":
				a.Binding.ProjectToken = common.HexToAddress("0x55")
			case "active-public":
				p.Enabled = false
				p.Root = (common.Hash{}).Hex()
				p.TargetVersion = "0"
				f.policy = make([]byte, 96)
				f.project[32+8*32+31] = 1
			case "rpc":
				f.fail = true
			case "state-transition":
				p.State = "active" // chain is Pending; policy equality alone is not enough
			}
			if e := checkApprovedV3ObservationCurrentPolicy(context.Background(), f, s, a, p); e == nil {
				t.Fatal("invalid current evidence admitted")
			}
		})
	}
}
