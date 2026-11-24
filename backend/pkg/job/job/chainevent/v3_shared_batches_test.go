package chainevent

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestSharedBatchRoutingNeverSkipsOwnGlobalOrUnknownEvents(t *testing.T) {
	projectTopic := crypto.Keccak256Hash([]byte("ProjectActivated(uint256)"))
	global := crypto.Keccak256Hash([]byte("Paused(address)"))
	for _, tc := range []struct {
		name             string
		logs             []types.Log
		affects, invalid bool
	}{
		{"empty", nil, false, false},
		{"other", []types.Log{{Topics: []common.Hash{projectTopic, common.BigToHash(big.NewInt(2))}}}, false, false},
		{"own", []types.Log{{Topics: []common.Hash{projectTopic, common.BigToHash(big.NewInt(1))}}}, true, false},
		{"other policy", []types.Log{{Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(big.NewInt(2))}}}, false, false},
		{"own policy", []types.Log{{Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(big.NewInt(1))}}}, true, false},
		{"global", []types.Log{{Topics: []common.Hash{global}}}, true, false},
		{"unknown", []types.Log{{Topics: []common.Hash{common.HexToHash("0xff")}}}, false, true},
		{"missing id", []types.Log{{Topics: []common.Hash{projectTopic}}}, false, true},
		{"oversized id", []types.Log{{Topics: []common.Hash{projectTopic, common.BigToHash(new(big.Int).Lsh(big.NewInt(1), 80))}}}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			affects, err := v3BlockAffectsProject(v3StreamBlock{Logs: tc.logs}, 1)
			if affects != tc.affects || (err != nil) != tc.invalid {
				t.Fatal(affects, err)
			}
		})
	}
}

type deploymentCountingRPC struct {
	v3ScannerRPC
	codes, slots, calls int
	fail                bool
}

func (r *deploymentCountingRPC) CodeAt(ctx context.Context, a common.Address, n *big.Int) ([]byte, error) {
	r.codes++
	if r.fail {
		return nil, context.DeadlineExceeded
	}
	return r.v3ScannerRPC.CodeAt(ctx, a, n)
}
func (r *deploymentCountingRPC) StorageAt(ctx context.Context, a common.Address, k common.Hash, n *big.Int) ([]byte, error) {
	r.slots++
	return r.v3ScannerRPC.StorageAt(ctx, a, k, n)
}
func (r *deploymentCountingRPC) CallContract(ctx context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	r.calls++
	return r.v3ScannerRPC.CallContract(ctx, m, n)
}

func TestSharedDeploymentReadsPinnedPrivateAndNotProjectPolicyAuthority(t *testing.T) {
	s, f := streamTestFixture()
	count := &deploymentCountingRPC{v3ScannerRPC: f}
	shared := newV3DeploymentReads(count)
	ctx := context.Background()
	height := big.NewInt(93)
	if _, err := shared.HeaderByNumber(ctx, height); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		code, err := shared.CodeAt(ctx, s.Proxy, height)
		if err != nil {
			t.Fatal(err)
		}
		code[0] = 0
		if _, err := shared.StorageAt(ctx, s.Proxy, common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"), height); err != nil {
			t.Fatal(err)
		}
		if _, err := shared.CallContract(ctx, ethereum.CallMsg{To: &s.Proxy, Data: crypto.Keccak256([]byte("contractVersion()"))[:4]}, height); err != nil {
			t.Fatal(err)
		}
	}
	if count.codes != 1 || count.slots != 1 || count.calls != 1 {
		t.Fatal(count.codes, count.slots, count.calls)
	}
	data := append(crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4], common.LeftPadBytes(big.NewInt(7).Bytes(), 32)...)
	for i := 0; i < 2; i++ {
		if _, err := shared.CallContract(ctx, ethereum.CallMsg{To: &s.Proxy, Data: data}, height); err != nil {
			t.Fatal(err)
		}
	}
	if count.calls != 3 {
		t.Fatal("policy cached", count.calls)
	}
	f.changeHeader = true
	if _, err := shared.HeaderByNumber(ctx, big.NewInt(92)); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.CodeAt(ctx, s.Proxy, big.NewInt(92)); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.HeaderByNumber(ctx, big.NewInt(92)); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.CodeAt(ctx, s.Proxy, big.NewInt(92)); err != nil {
		t.Fatal(err)
	}
	if count.codes != 3 {
		t.Fatal("reorg reused old code namespace", count.codes)
	}
	cold := newV3DeploymentReads(count)
	if _, err := cold.HeaderByNumber(ctx, height); err != nil {
		t.Fatal(err)
	}
	if _, err := cold.CodeAt(ctx, s.Proxy, height); err != nil {
		t.Fatal(err)
	}
	if count.codes != 4 {
		t.Fatal("tick retained public read")
	}
}

func TestSharedProjectQuietChecksEndStateWithoutRepeatedStartPolicyReads(t *testing.T) {
	s, f := streamTestFixture()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: 7, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	blocks := map[uint64]v3StreamBlock{}
	for n := uint64(91); n <= 93; n++ {
		blocks[n] = v3StreamBlock{Header: (*v3StreamHeader)(f.blocks[n].head)}
	}
	count := &deploymentCountingRPC{v3ScannerRPC: f}
	u, err := prepareV3ProjectQuiet(context.Background(), v3CoverageRPC{v3ScannerRPC: count, blocks: blocks, proxy: s.Proxy}, s, b, s.Runtime, 91, 93)
	if err != nil || len(u.headers) != 3 || count.calls != 3 {
		t.Fatal(u, err, count.calls)
	} // version+policy+project, once
	blocks[92] = v3StreamBlock{Header: (*v3StreamHeader)(f.blocks[92].head), Logs: []types.Log{{Topics: []common.Hash{v3PolicyUpdateTopic, common.BigToHash(big.NewInt(7))}}}}
	if _, err := prepareV3ProjectQuiet(context.Background(), v3CoverageRPC{v3ScannerRPC: f, blocks: blocks, proxy: s.Proxy}, s, b, s.Runtime, 91, 93); err == nil {
		t.Fatal("own event treated as quiet")
	}
}

func TestSharedQuietOtherProjectReceiptIsCheckedAndNotReplayed(t *testing.T) {
	s, f := streamTestFixture()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: 7, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	l := types.Log{Address: s.Proxy, BlockNumber: 92, BlockHash: f.blocks[92].head.Hash(), TxHash: common.HexToHash("0xab"), Index: 1, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(8))}}
	r := &types.Receipt{Status: 1, BlockNumber: big.NewInt(92), BlockHash: l.BlockHash, TxHash: l.TxHash, Logs: []*types.Log{&l}}
	blocks := map[uint64]v3StreamBlock{}
	for n := uint64(91); n <= 93; n++ {
		blocks[n] = v3StreamBlock{Header: (*v3StreamHeader)(f.blocks[n].head)}
	}
	blocks[92] = v3StreamBlock{Header: (*v3StreamHeader)(f.blocks[92].head), Logs: []types.Log{l}, Receipts: map[string]*v3StreamReceipt{l.TxHash.Hex(): (*v3StreamReceipt)(r)}}
	u, err := prepareV3ProjectQuiet(context.Background(), v3CoverageRPC{v3ScannerRPC: f, blocks: blocks, proxy: s.Proxy}, s, b, s.Runtime, 91, 93)
	if err != nil || !u.empty || len(u.logs) != 0 || len(u.headers) != 3 {
		t.Fatal("other project lost quiet optimization", u, err)
	}
	r.Status = 0
	if _, err := prepareV3ProjectQuiet(context.Background(), v3CoverageRPC{v3ScannerRPC: f, blocks: blocks, proxy: s.Proxy}, s, b, s.Runtime, 91, 93); err == nil {
		t.Fatal("invalid other-project receipt ignored")
	}
}

func TestSharedDeploymentErrorsAndCanceledContextsNeverMemoize(t *testing.T) {
	s, f := streamTestFixture()
	count := &deploymentCountingRPC{v3ScannerRPC: f, fail: true}
	shared := newV3DeploymentReads(count)
	ctx := context.Background()
	height := big.NewInt(93)
	shared.HeaderByNumber(ctx, height)
	if _, err := shared.CodeAt(ctx, s.Proxy, height); err == nil {
		t.Fatal("source failure ignored")
	}
	count.fail = false
	if _, err := shared.CodeAt(ctx, s.Proxy, height); err != nil || count.codes != 2 {
		t.Fatal("source error memoized", err, count.codes)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := shared.CodeAt(ctx, s.Proxy, height); err == nil {
		t.Fatal("canceled cached result returned")
	}
}
