package chainevent

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"testing"
)

type fiveBlockRPC struct {
	v3ScannerRPC
	spans [][2]uint64
	fail  bool
}

func (r *fiveBlockRPC) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	a, b := q.FromBlock.Uint64(), q.ToBlock.Uint64()
	r.spans = append(r.spans, [2]uint64{a, b})
	if b-a >= 5 || r.fail {
		return nil, errV3Stream
	}
	return []types.Log{{BlockNumber: a}}, nil
}
func TestProxyLogsCompleteFiveBlockSegmentation(t *testing.T) {
	r := &fiveBlockRPC{}
	q := ethereum.FilterQuery{FromBlock: big.NewInt(90), ToBlock: big.NewInt(105)}
	for pass := 0; pass < 2; pass++ {
		logs, e := v3ReadBoundedLogs(context.Background(), r, q)
		if e != nil || len(logs) != 4 {
			t.Fatal(logs, e)
		}
	}
	want := [][2]uint64{{90, 94}, {95, 99}, {100, 104}, {105, 105}}
	if len(r.spans) != 8 {
		t.Fatal(r.spans)
	}
	for i, span := range r.spans {
		if span != want[i%4] {
			t.Fatal("gap/overlap", r.spans)
		}
	}
	r.fail = true
	if _, e := v3ReadBoundedLogs(context.Background(), r, q); e == nil {
		t.Fatal("413 treated as empty logs")
	}
}
