package chainevent

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

func TestPreparedRPCReplaysIndependentReadsWithoutNetworkOrAliases(t *testing.T) {
	ctx := context.Background()
	source := &tickReadStub{}
	r := newV3PreparedRPC(source)
	for i := 0; i < 2; i++ {
		h, e := r.HeaderByNumber(ctx, big.NewInt(90))
		if e != nil {
			t.Fatal(e)
		}
		h.Extra[0] = 9
	}
	hash := common.HexToHash("0xaa")
	receipt, e := r.TransactionReceipt(ctx, hash)
	if e != nil {
		t.Fatal(e)
	}
	receipt.Logs[0].Data[0] = 9
	if source.headers.Load() != 2 {
		t.Fatal("independent preparation reads coalesced")
	}
	r.replay = true
	r.v3ScannerRPC = nil // any unrecorded network fallback panics
	for i := 0; i < 2; i++ {
		h, e := r.HeaderByNumber(ctx, big.NewInt(90))
		if e != nil || h.Extra[0] != 1 {
			t.Fatal("mutable replay header", e)
		}
		h.Extra[0] = 8
	}
	receipt, e = r.TransactionReceipt(ctx, hash)
	if e != nil || receipt.Logs[0].Data[0] != 3 {
		t.Fatal("mutable replay receipt", e)
	}
	if r.position != len(r.answers) {
		t.Fatal("incomplete evidence replay")
	}
	if _, e = r.HeaderByNumber(ctx, big.NewInt(90)); e == nil {
		t.Fatal("unrecorded read accepted")
	}
}

func TestPreparedRPCRejectsChangedQueryOrderCancellationAndErrors(t *testing.T) {
	ctx := context.Background()
	source := &tickReadStub{}
	r := newV3PreparedRPC(source)
	if _, e := r.HeaderByNumber(ctx, big.NewInt(90)); e != nil {
		t.Fatal(e)
	}
	r.replay = true
	if _, e := r.HeaderByNumber(ctx, big.NewInt(91)); e == nil {
		t.Fatal("different height accepted")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := r.HeaderByNumber(cancelCtx, big.NewInt(90)); e == nil {
		t.Fatal("canceled replay accepted")
	}
	r = newV3PreparedRPC(source)
	source.fail.Store(true)
	if _, e := r.HeaderByNumber(ctx, big.NewInt(90)); e == nil || len(r.answers) != 0 {
		t.Fatal("failed response recorded")
	}
}

func TestPreparedRPCDefersNetworkUntilCallerRollback(t *testing.T) {
	ctx := context.Background()
	source := &tickReadStub{}
	r := newV3PreparedRPC(source)
	r.deferNetwork = true
	if _, e := r.HeaderByNumber(ctx, big.NewInt(90)); e == nil || r.pending == nil || source.headers.Load() != 0 {
		t.Fatal("network executed in transaction")
	}
	if e := r.pending(); e != nil || source.headers.Load() != 1 {
		t.Fatal("deferred source read", e)
	}
	r.position = 0
	r.pending = nil
	if _, e := r.HeaderByNumber(ctx, big.NewInt(90)); e != nil || r.pending != nil || source.headers.Load() != 1 {
		t.Fatal("recorded evidence reissued RPC", e)
	}
}
