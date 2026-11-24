package chainevent

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

// A controllable remaining parent budget avoids timing-sensitive sleeps. The
// actual child contexts still exercise context.WithTimeout and cancellation.
type v3BudgetTestContext struct {
	context.Context
	mu        sync.Mutex
	remaining time.Duration
}

func (c *v3BudgetTestContext) Deadline() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Add(c.remaining), true
}
func (c *v3BudgetTestContext) set(d time.Duration) { c.mu.Lock(); c.remaining = d; c.mu.Unlock() }

func TestV3SerialBudgetResumesUnstartedProjectsAndNeverRevokesThem(t *testing.T) {
	ids := []uint64{1, 2, 3, 4, 5, 6, 7}
	after := uint64(0)
	var visited []uint64
	for tick := 0; tick < 14; tick++ {
		ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
		r, e := fairApprovedV3ScanWorkers(ctx, ids, &after, 35*time.Second, 1, func(child context.Context, id uint64) error {
			deadline, _ := child.Deadline()
			if time.Until(deadline) < 34*time.Second {
				t.Fatal("started clipped project budget")
			}
			visited = append(visited, id)
			ctx.set(14 * time.Second)
			return nil
		}, func(uint64) error { t.Fatal("budget skip revoked project"); return nil })
		if e == nil || r.Attempted != 1 || r.Completed != 1 || r.Skipped != 6 || r.Failed != 0 || r.Deferred != 0 {
			t.Fatal(r, e)
		}
	}
	want := append(append([]uint64{}, ids...), ids...)
	if !reflect.DeepEqual(visited, want) {
		t.Fatalf("starvation: visited=%v want=%v", visited, want)
	}
}

func TestV3CompleteRoundsRotateFirstSlotEvenWhenAllDispatched(t *testing.T) {
	ids := []uint64{1, 2, 3, 4, 5, 6, 7}
	after := uint64(0)
	for tick := 0; tick < 7; tick++ {
		var order []uint64
		r, e := fairApprovedV3ScanWorkers(context.Background(), ids, &after, time.Second, 1, func(_ context.Context, id uint64) error { order = append(order, id); return nil }, func(uint64) error { return nil })
		if e != nil || r.Completed != 7 || r.Skipped != 0 || order[0] != ids[tick] {
			t.Fatal(tick, order, r, e)
		}
	}
}

func TestV3BudgetYieldIsIncompleteButDoesNotHaltOrStarveOthers(t *testing.T) {
	ids := []uint64{1, 2, 3}
	after := uint64(0)
	for tick := 0; tick < 3; tick++ {
		var order []uint64
		r, e := fairApprovedV3ScanWorkers(context.Background(), ids, &after, time.Second, 1, func(_ context.Context, id uint64) error {
			order = append(order, id)
			if id == 3 {
				ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
				defer cancel()
				return requireV3ConsumerWindowBudget(ctx, true)
			}
			return nil
		}, func(uint64) error { t.Fatal("budget yield revoked history"); return nil })
		if e == nil || r.Completed != 2 || r.Failed != 1 || r.Deferred != 1 || r.Skipped != 0 || order[0] != ids[tick] {
			t.Fatal(r, e, order)
		}
	}
}

func TestV3NoBudgetPreservesPriorityAndAdmitsNoWork(t *testing.T) {
	after := uint64(2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e := fairApprovedV3ScanWorkers(ctx, []uint64{1, 2, 3}, &after, 35*time.Second, 1, func(context.Context, uint64) error { t.Fatal("short project started"); return nil }, func(uint64) error { t.Fatal("short budget revoked"); return nil })
	if e == nil || r.Attempted != 0 || r.Completed != 0 || r.Skipped != 3 || after != 2 {
		t.Fatal(r, e, after)
	}
}

func TestV3PartialRoundSurvivesChangingBindingList(t *testing.T) {
	after := uint64(0)
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
	_, _ = fairApprovedV3ScanWorkers(ctx, []uint64{1, 2, 3}, &after, 35*time.Second, 1, func(context.Context, uint64) error { ctx.set(time.Second); return nil }, func(uint64) error { return nil })
	// Removing last-started ID and inserting lower/higher IDs cannot turn
	// scheduling priority into a durable high-water mark that loses bindings.
	var got []uint64
	r, e := fairApprovedV3ScanWorkers(context.Background(), []uint64{2, 3, 4}, &after, time.Second, 1, func(_ context.Context, id uint64) error { got = append(got, id); return nil }, func(uint64) error { return nil })
	if e != nil || r.Completed != 3 || !reflect.DeepEqual(got, []uint64{2, 3, 4}) {
		t.Fatal(r, e, got)
	}
}

func TestV3BudgetSkippedProjectIsNotPlacedInRetryBackoff(t *testing.T) {
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
	s := &v3RetryState{}
	after := uint64(0)
	r, err := fairApprovedV3ScanWorkers(ctx, []uint64{1, 2}, &after, 35*time.Second, 1,
		func(child context.Context, id uint64) error {
			return s.attempt(child, id, func() error { ctx.set(time.Second); return nil })
		}, func(uint64) error { t.Fatal("skipped project revoked"); return nil })
	if err == nil || r.Skipped != 1 || len(s.projects) != 0 || after != 1 {
		t.Fatal(r, err, s.projects, after)
	}
	var next uint64
	r, err = fairApprovedV3ScanWorkers(context.Background(), []uint64{1, 2}, &after, time.Second, 1,
		func(child context.Context, id uint64) error {
			if next == 0 {
				next = id
			}
			return s.attempt(child, id, func() error { return nil })
		}, func(uint64) error { return nil })
	if err != nil || r.Completed != 2 || next != 2 {
		t.Fatal(r, err, next)
	}
}

func TestV3StreamSevenProjectsCompleteWithoutReservingSevenMaximumTimeouts(t *testing.T) {
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
	ids := []uint64{1, 2, 3, 4, 5, 6, 7}
	after := uint64(0)
	remaining := 50 * time.Second
	var visited []uint64
	r, err := fairApprovedV3ScanWithStartBudget(ctx, ids, &after, 35*time.Second, approvedV3StreamAdmissionFloor, 1,
		func(child context.Context, id uint64) error {
			visited = append(visited, id)
			// Representative 4s complete admission + bounded consumer unit.
			// The last projects have <35s, but still >=15s for atomic work.
			if !v3HasProjectBudget(child, 15*time.Second) {
				t.Fatal("window reservation lost", id)
			}
			remaining -= 4 * time.Second
			ctx.set(remaining)
			return nil
		}, func(uint64) error { t.Fatal("valid project revoked"); return nil })
	if err != nil || r.Completed != 7 || r.Skipped != 0 || !reflect.DeepEqual(visited, ids) {
		t.Fatal(r, err, visited)
	}
}

func TestV3StreamPartialAdmissionNeverPublishesFullSuccess(t *testing.T) {
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
	after := uint64(0)
	r, err := fairApprovedV3ScanWithStartBudget(ctx, []uint64{1, 2, 3}, &after, 35*time.Second, approvedV3StreamAdmissionFloor, 1,
		func(child context.Context, id uint64) error {
			if id == 1 {
				ctx.set(10 * time.Second)
				return nil
			}
			if v3HasProjectBudget(child, 15*time.Second) {
				t.Fatal("short consumer window admitted")
			}
			return &v3ScanDeferred{stage: "consumer_budget_yield"}
		}, func(uint64) error { t.Fatal("budget defer revoked"); return nil })
	if err == nil || r.Completed != 1 || r.Deferred != 2 || r.Attempted != 3 {
		t.Fatal(r, err)
	}
}
