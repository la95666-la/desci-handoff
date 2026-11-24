package chainevent

import (
	"context"
	"joincare_server/pkg/common/idov3whitelist"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestConsumerPreparedStreamFences(t *testing.T) {
	before := v3StreamCursor{Mode: "stream", State: "running", Epoch: 2, AnchorBlock: 90, CursorBlock: 100, CursorHash: "a", Revision: 5}
	if !v3ConsumerStreamProgress(before, before) {
		t.Fatal("unchanged stream rejected")
	}
	forward := before
	forward.CursorBlock += 32
	forward.Revision++
	forward.CursorHash = "b"
	if !v3ConsumerStreamProgress(before, forward) {
		t.Fatal("producer progress rejected")
	}
	cases := []v3StreamCursor{}
	for _, mutate := range []func(*v3StreamCursor){
		func(c *v3StreamCursor) { c.State = "halted" }, func(c *v3StreamCursor) { c.Mode = "shadow" },
		func(c *v3StreamCursor) { c.Epoch++ }, func(c *v3StreamCursor) { c.AnchorBlock++ },
		func(c *v3StreamCursor) { c.CursorBlock-- }, func(c *v3StreamCursor) { c.Revision++ },
		func(c *v3StreamCursor) { c.CursorHash = "b" }, func(c *v3StreamCursor) { c.CursorBlock++; c.Revision += 2 },
		func(c *v3StreamCursor) { c.CursorBlock++ },
	} {
		c := before
		mutate(&c)
		cases = append(cases, c)
	}
	for i, c := range cases {
		if v3ConsumerStreamProgress(before, c) {
			t.Fatalf("invalid progress %d accepted", i)
		}
	}
}

func TestPreparedEmptyRejectsChangedStatePolicyAndCancellation(t *testing.T) {
	s, rpc := streamTestFixture()
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: 7, ProjectToken: common.HexToAddress("0x11"), PaymentToken: common.HexToAddress("0x22")}
	blocks := map[uint64]v3StreamBlock{}
	for n := uint64(90); n <= 93; n++ {
		blocks[n] = v3StreamBlock{Header: (*v3StreamHeader)(rpc.blocks[n].head), Logs: []types.Log{}}
	}
	r := v3CoverageRPC{v3ScannerRPC: rpc, blocks: blocks, proxy: s.Proxy}
	ctx := context.Background()
	if _, e := prepareV3ConsumerEmpty(ctx, r, s, b, s.Runtime, 91, 93); e != nil {
		t.Fatal(e)
	}
	original := rpc.blocks[93].tuple
	rpc.blocks[93].tuple = append([]byte(nil), original...)
	rpc.blocks[93].tuple[32+9*32+31]++
	if _, e := prepareV3ConsumerEmpty(ctx, r, s, b, s.Runtime, 91, 93); e == nil {
		t.Fatal("immutable tuple change accepted")
	}
	rpc.blocks[93].tuple = original
	rpc.blocks[93].policy = make([]byte, 96)
	rpc.blocks[93].policy[95] = 1
	if _, e := prepareV3ConsumerEmpty(ctx, r, s, b, s.Runtime, 91, 93); e == nil {
		t.Fatal("policy change without event accepted")
	}
	rpc.blocks[93].policy = nil
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := prepareV3ConsumerEmpty(canceled, r, s, b, s.Runtime, 91, 93); e == nil {
		t.Fatal("canceled preparation accepted")
	}
}

func TestConsumerCoverageCopiesArePrivate(t *testing.T) {
	s, rpc := streamTestFixture()
	blocks, e := collectV3Stream(context.Background(), rpc, s, 91, 93, rpc.blocks[90].head.Hash())
	if e != nil {
		t.Fatal(e)
	}
	source := map[uint64]v3StreamBlock{91: blocks[0]}
	source[91].header().Extra = []byte{1}
	b := source[91]
	b.Logs = []types.Log{{Data: []byte{2}, Topics: []common.Hash{common.HexToHash("0xaa")}}}
	b.Receipts = map[string]*v3StreamReceipt{"r": (*v3StreamReceipt)(&types.Receipt{BlockNumber: big.NewInt(91), Logs: []*types.Log{{Data: []byte{3}}}})}
	source[91] = b
	copy := cloneV3Coverage(source)
	copy[91].header().Extra[0] = 8
	copy[91].Logs[0].Data[0] = 8
	copy[91].Receipts["r"].Logs[0].Data[0] = 8
	if source[91].header().Extra[0] != 1 || source[91].Logs[0].Data[0] != 2 || source[91].Receipts["r"].Logs[0].Data[0] != 3 {
		t.Fatal("shared evidence alias")
	}
}
