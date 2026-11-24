package chainevent

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/idov3whitelist"
)

type streamRPCFixture struct {
	*v3EmptyWindowRPC
	receipts map[common.Hash]*types.Receipt
	reads    int
	changed  bool
}

func (f *streamRPCFixture) TransactionReceipt(_ context.Context, h common.Hash) (*types.Receipt, error) {
	f.reads++
	r := f.receipts[h]
	if r == nil {
		return nil, errV3Stream
	}
	r = v3CloneReceipt(r)
	if f.changed && f.reads > 1 {
		r.Status = 0
	}
	return r, nil
}
func streamTestFixture() (v3StreamScope, *streamRPCFixture) {
	proxy := common.HexToAddress("0x6767676767676767676767676767676767676767")
	impl := common.HexToAddress("0xaa")
	pay := common.HexToAddress("0x22")
	sale := common.HexToAddress("0x11")
	rt := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	f := &streamRPCFixture{v3EmptyWindowRPC: &v3EmptyWindowRPC{blocks: map[uint64]*v3PinnedScanStub{}, headerReads: map[uint64]int{}, latest: 120}, receipts: map[common.Hash]*types.Receipt{}}
	parent := common.HexToHash("0xa1")
	for n := uint64(90); n <= 110; n++ {
		h := &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: parent}
		parent = h.Hash()
		f.blocks[n] = &v3PinnedScanStub{chain: 31337, head: h, proxy: proxy, impl: impl, payment: pay, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, pay)}
	}
	return v3StreamScope{Chain: 31337, Proxy: proxy, Runtime: rt, Confirmations: 3}, f
}

func TestProxyStreamCollectorCompleteEmptyPrefixAndUniqueReceipts(t *testing.T) {
	s, f := streamTestFixture()
	ctx := context.Background()
	logs := []types.Log{}
	for i, p := range []int64{7, 8} {
		l := types.Log{Address: s.Proxy, BlockNumber: 92, BlockHash: f.blocks[92].head.Hash(), TxHash: common.HexToHash("0xb1"), Index: uint(i), Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(p))}}
		logs = append(logs, l)
	}
	f.logs = logs
	r := &types.Receipt{Status: 1, TxHash: logs[0].TxHash, BlockHash: logs[0].BlockHash, BlockNumber: big.NewInt(92), Logs: []*types.Log{&logs[0], &logs[1]}}
	f.receipts[r.TxHash] = r
	blocks, e := collectV3Stream(ctx, f, s, 91, 93, f.blocks[90].head.Hash())
	if e != nil || len(blocks) != 3 || len(blocks[0].Logs) != 0 || len(blocks[1].Logs) != 2 || f.reads != 2 {
		t.Fatalf("blocks=%d unique receipt reads=%d err=%v", len(blocks), f.reads, e)
	}
	raw, digest, e := v3StreamPayload(blocks[1])
	if e != nil {
		t.Fatal(e)
	}
	var copy v3StreamBlock
	if json.Unmarshal([]byte(raw), &copy) != nil {
		t.Fatal("decode")
	}
	_, again, e := v3StreamPayload(copy)
	if e != nil || again != digest || (*types.Receipt)(copy.Receipts[r.TxHash.Hex()]).BlockNumber.Uint64() != 92 {
		t.Fatal("receipt metadata/digest lost", e)
	}
	f.filters = 0
	f.reads = 0
	f.changed = true
	if _, e = collectV3Stream(ctx, f, s, 91, 93, f.blocks[90].head.Hash()); e == nil {
		t.Fatal("receipt changed on second pass")
	}
}

func TestProxyStreamRejectsUnknownDuplicateOmittedChangedAndReorg(t *testing.T) {
	for _, kind := range []string{"unknown", "duplicate", "omitted", "filter", "header", "wrongchain"} {
		t.Run(kind, func(t *testing.T) {
			s, f := streamTestFixture()
			l := types.Log{Address: s.Proxy, BlockNumber: 92, BlockHash: f.blocks[92].head.Hash(), TxHash: common.HexToHash("0xb1"), Index: 1, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(7))}}
			f.logs = []types.Log{l}
			f.receipts[l.TxHash] = &types.Receipt{Status: 1, TxHash: l.TxHash, BlockHash: l.BlockHash, BlockNumber: big.NewInt(92), Logs: []*types.Log{&l}}
			switch kind {
			case "unknown":
				f.logs[0].Topics = []common.Hash{common.HexToHash("0x99")}
			case "duplicate":
				f.logs = append(f.logs, l)
			case "omitted":
				other := l
				other.Index = 2
				f.receipts[l.TxHash].Logs = append(f.receipts[l.TxHash].Logs, &other)
			case "filter":
				f.changeSecond = true
			case "header":
				f.changeHeader = true
			case "wrongchain":
				s.Chain = 56
			}
			if _, e := collectV3Stream(context.Background(), f, s, 91, 93, f.blocks[90].head.Hash()); e == nil {
				t.Fatal("invalid stream accepted")
			}
		})
	}
}

func TestProxyCoverageRPCRejectsHolesAndCrossProxy(t *testing.T) {
	s, f := streamTestFixture()
	blocks, e := collectV3Stream(context.Background(), f, s, 91, 93, f.blocks[90].head.Hash())
	if e != nil {
		t.Fatal(e)
	}
	r := v3CoverageRPC{v3ScannerRPC: f, proxy: s.Proxy, blocks: map[uint64]v3StreamBlock{91: blocks[0], 93: blocks[2]}}
	if _, e = r.FilterLogs(context.Background(), ethereum.FilterQuery{FromBlock: big.NewInt(91), ToBlock: big.NewInt(93), Addresses: []common.Address{s.Proxy}}); e == nil {
		t.Fatal("coverage gap bridged")
	}
	if _, e = r.HeaderByNumber(context.Background(), big.NewInt(92)); e == nil {
		t.Fatal("missing header fell back to live RPC")
	}
	if _, e = r.TransactionReceipt(context.Background(), common.HexToHash("0xb1")); e == nil {
		t.Fatal("missing receipt fell back")
	}
}

func TestProxyStreamDefaultOffWithoutDraftSchema(t *testing.T) {
	viper.Set("idoV3ProxyStream.mode", "off")
	defer viper.Set("idoV3ProxyStream.mode", nil)
	if c, e := ConfiguredV3ProxyStream(nil); e != nil || c.Mode != "off" {
		t.Fatal(c, e)
	}
	viper.Set("idoV3ProxyStream.mode", "invalid")
	if _, e := ConfiguredV3ProxyStream(nil); e == nil {
		t.Fatal("invalid mode admitted")
	}
}

func TestProxyStreamSingleProducerReadCountDoesNotGrowWithProjects(t *testing.T) {
	s, f := streamTestFixture()
	blocks, e := collectV3Stream(context.Background(), f, s, 91, 106, f.blocks[90].head.Hash())
	if e != nil {
		t.Fatal(e)
	}
	before := 0
	for _, n := range f.headerReads {
		before += n
	}
	coverage := map[uint64]v3StreamBlock{}
	for _, b := range blocks {
		coverage[b.header().Number.Uint64()] = b
	}
	r := v3CoverageRPC{v3ScannerRPC: f, blocks: coverage, proxy: s.Proxy}
	for project := 0; project < 20; project++ {
		for pass := 0; pass < 2; pass++ {
			hs, e := readV3EmptyHeaders(context.Background(), r, 91, 106)
			if e != nil || len(hs) != 16 {
				t.Fatal(e)
			}
		}
	}
	after := 0
	for _, n := range f.headerReads {
		after += n
	}
	if after != before {
		t.Fatalf("project replay issued duplicate headers: %d -> %d", before, after)
	}
	t.Logf("20 project consumers, 2x16 header replay: producer source headers=%d, extra source headers=0 (live consumer boundaries excluded)", before)
}

func TestProxyStreamCancellationFailsClosed(t *testing.T) {
	s, f := streamTestFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	defer cancel()
	if _, e := collectV3Stream(ctx, f, s, 91, 106, f.blocks[90].head.Hash()); e == nil {
		t.Fatal("expired scan published evidence")
	}
}
