package chainevent

import (
	"context"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

type concurrentEmptyHeaders struct {
	v3ScannerRPC
	mu              sync.Mutex
	reads           map[uint64]int
	active, maximum atomic.Int32
	wrong, missing  uint64
	delay           time.Duration
}

func (r *concurrentEmptyHeaders) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	count := r.active.Add(1)
	defer r.active.Add(-1)
	for old := r.maximum.Load(); count > old; old = r.maximum.Load() {
		if r.maximum.CompareAndSwap(old, count) {
			break
		}
	}
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	h := n.Uint64()
	r.mu.Lock()
	r.reads[h]++
	r.mu.Unlock()
	if h == r.missing {
		return nil, nil
	}
	if h == r.wrong {
		h++
	}
	return &types.Header{Number: new(big.Int).SetUint64(h), Time: h}, nil
}

func TestEmptyHeaderWindowReadsEveryHeightTwiceWithBoundedConcurrency(t *testing.T) {
	rpc := &concurrentEmptyHeaders{reads: map[uint64]int{}, delay: time.Millisecond}
	for pass := 0; pass < 2; pass++ {
		headers, err := readV3EmptyHeaders(context.Background(), rpc, 100, 115)
		if err != nil || len(headers) != 16 {
			t.Fatalf("window unavailable: count=%d err=%v", len(headers), err)
		}
		for i, h := range headers {
			if h == nil || h.Number.Uint64() != 100+uint64(i) {
				t.Fatal("height reordered or omitted")
			}
		}
	}
	for height := uint64(100); height <= 115; height++ {
		if rpc.reads[height] != 2 {
			t.Fatalf("height %d not independently re-read", height)
		}
	}
	if rpc.maximum.Load() > 4 || rpc.maximum.Load() < 2 {
		t.Fatalf("header concurrency=%d", rpc.maximum.Load())
	}
}

func TestEmptyHeadersFailClosedOnMissingWrongCancelledAndOversizedRange(t *testing.T) {
	for _, kind := range []string{"missing", "wrong", "cancelled", "oversized"} {
		rpc := &concurrentEmptyHeaders{reads: map[uint64]int{}}
		ctx, cancel := context.WithCancel(context.Background())
		end := uint64(115)
		switch kind {
		case "missing":
			rpc.missing = 105
		case "wrong":
			rpc.wrong = 105
		case "cancelled":
			cancel()
		case "oversized":
			end = 116
		}
		if headers, err := readV3EmptyHeaders(ctx, rpc, 100, end); err == nil || headers != nil {
			t.Fatalf("%s window accepted", kind)
		}
		cancel()
	}
}

func TestFairScanProducesProgressForEveryProjectWithoutPublishingPartialHealth(t *testing.T) {
	ids := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	after := uint64(0)
	var mu sync.Mutex
	cursors := map[uint64]uint64{}
	result, err := fairApprovedV3Scan(context.Background(), ids, &after, time.Second,
		func(ctx context.Context, id uint64) error {
			if id == 1 {
				return context.DeadlineExceeded
			}
			mu.Lock()
			cursors[id] += approvedV3ProjectBlocks
			mu.Unlock()
			return nil
		}, func(uint64) error { return nil })
	if err == nil || result.Completed != 8 {
		t.Fatal("partial enumeration reported healthy")
	}
	for _, id := range ids[1:] {
		if cursors[id] != 128 {
			t.Fatalf("project %d starved", id)
		}
	}
	// Failed priority is not permanently skipped on the next tick.
	result, err = fairApprovedV3Scan(context.Background(), ids, &after, time.Second, func(ctx context.Context, id uint64) error { mu.Lock(); cursors[id] += 128; mu.Unlock(); return nil }, func(uint64) error { return nil })
	if err != nil || result.Completed != 9 || cursors[1] != 128 {
		t.Fatal("failed project lost after fair rotation")
	}
}
