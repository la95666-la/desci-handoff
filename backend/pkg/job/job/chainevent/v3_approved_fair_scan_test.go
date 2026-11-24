package chainevent

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestApprovedFairScanVisitsAllAndIsolatesFailure(t *testing.T) {
	ids := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	for _, fail := range []bool{false, true} {
		var mu sync.Mutex
		var visited, revoked []uint64
		var active, maximum atomic.Int32
		after := uint64(6)
		result, err := fairApprovedV3Scan(context.Background(), ids, &after, time.Second,
			func(ctx context.Context, id uint64) error {
				n := active.Add(1)
				defer active.Add(-1)
				for old := maximum.Load(); n > old; old = maximum.Load() {
					if maximum.CompareAndSwap(old, n) {
						break
					}
				}
				mu.Lock()
				visited = append(visited, id)
				mu.Unlock()
				if fail && id == 1 {
					return errors.New("one project unavailable")
				}
				return nil
			}, func(id uint64) error { mu.Lock(); revoked = append(revoked, id); mu.Unlock(); return nil })
		sort.Slice(visited, func(i, j int) bool { return visited[i] < visited[j] })
		if !reflect.DeepEqual(visited, ids) || result.Attempted != 9 || maximum.Load() > approvedV3ScanWorkers {
			t.Fatalf("coverage=%+v visited=%v max=%d", result, visited, maximum.Load())
		}
		if fail {
			if err == nil || result.Completed != 8 || result.Failed != 1 || !reflect.DeepEqual(revoked, []uint64{1}) {
				t.Fatalf("partial scan reported healthy: %+v %v", result, err)
			}
		} else if err != nil || result.Completed != 9 || len(revoked) != 0 {
			t.Fatalf("complete scan: %+v %v", result, err)
		}
	}
}

func TestApprovedFairScanWrapsAfterCancelledTickWithoutStarvation(t *testing.T) {
	ids := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	after := uint64(0)
	ctx, cancel := context.WithCancel(context.Background())
	var starts atomic.Int32
	var mu sync.Mutex
	var visited, revoked []uint64
	result, err := fairApprovedV3Scan(ctx, ids, &after, time.Second, func(ctx context.Context, id uint64) error {
		mu.Lock()
		visited = append(visited, id)
		mu.Unlock()
		if starts.Add(1) == approvedV3ScanWorkers {
			cancel()
		}
		<-ctx.Done()
		return ctx.Err()
	}, func(id uint64) error { mu.Lock(); revoked = append(revoked, id); mu.Unlock(); return nil })
	if err == nil || result.Completed != 0 || result.Attempted != 3 || len(revoked) != 0 || after != 3 {
		t.Fatalf("cancel coverage=%+v priority=%d", result, after)
	}
	// A new tick starts at the next ID, but also wraps and rechecks old IDs;
	// after is scheduling priority only, never proof or a durable scan cursor.
	var first atomic.Uint64
	seen := map[uint64]int{}
	result, err = fairApprovedV3Scan(context.Background(), ids, &after, time.Second, func(ctx context.Context, id uint64) error {
		first.CompareAndSwap(0, id)
		mu.Lock()
		seen[id]++
		mu.Unlock()
		return nil
	}, func(uint64) error { return nil })
	if err != nil || result.Completed != 9 {
		t.Fatalf("restarted scan incomplete: %+v %v", result, err)
	}
	if first.Load() < 4 || first.Load() > 6 {
		t.Fatalf("resumed at wrong priority %d", first.Load())
	}
	for _, id := range ids {
		if seen[id] != 1 {
			t.Fatalf("project %d skipped/repeated", id)
		}
	}
}

func TestApprovedFairScanBoundsEachProjectAndRejectsEnumeration(t *testing.T) {
	after := uint64(0)
	var revoked atomic.Int32
	result, err := fairApprovedV3Scan(context.Background(), []uint64{1, 2, 3, 4}, &after, 10*time.Millisecond, func(ctx context.Context, id uint64) error {
		if id == 1 {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}, func(uint64) error { revoked.Add(1); return nil })
	if err == nil || result.Completed != 3 || result.Failed != 1 || revoked.Load() != 0 {
		t.Fatalf("project exhausted other budgets: %+v %v", result, err)
	}
	for _, ids := range [][]uint64{{0}, {2, 1}, {1, 1}} {
		_, err := fairApprovedV3Scan(context.Background(), ids, &after, time.Second, func(context.Context, uint64) error { t.Fatal("invalid enumeration scanned"); return nil }, func(uint64) error { return nil })
		if err == nil {
			t.Fatal("invalid enumeration admitted")
		}
	}
}

func TestApprovedScannerBatchFitsBoundedAtomicEmptyRanges(t *testing.T) {
	if approvedV3ProjectBlocks%v3EmptyRangeLimit != 0 || v3EmptyRangeLimit > 16 || approvedV3ProjectBudget >= 50*time.Second {
		t.Fatal("project batch or budget can exhaust whole Job turn")
	}
}
