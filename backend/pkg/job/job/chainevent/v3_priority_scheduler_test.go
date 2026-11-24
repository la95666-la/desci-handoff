package chainevent

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

func TestPriorityActiveFirstRegardlessOfProjectID(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	ids := []uint64{1, 2, 9}
	s.retain(ids, now)
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Pending"}, {ID: 9, Status: "Active"}}
	h, ok := s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID != 9 {
		t.Fatal(h, ok)
	}
	s.record(h, now, now.Add(time.Second), nil)
	h, ok = s.selectNext(rows, map[uint64]bool{9: true}, map[uint64]bool{}, func(uint64) bool { return false }, now.Add(10*time.Second))
	if !ok || h.ID == 9 {
		t.Fatal("active repeated before due", h)
	}
	h, ok = s.selectNext(rows, map[uint64]bool{9: true}, map[uint64]bool{}, func(uint64) bool { return false }, now.Add(32*time.Second))
	if !ok || h.ID != 9 {
		t.Fatal("overdue active did not preempt next ordinary batch", h)
	}
}

func TestPriorityUrgentRefundAndPendingBoundaryPromoted(t *testing.T) {
	for _, h := range []v3PriorityHint{{ID: 9, Status: "Failed", EventBlock: 101, Cursor: 100}, {ID: 9, Status: "Successful", Unresolved: true}, {ID: 9, Status: "Pending", NearBoundary: true}} {
		s := &v3PrioritySchedule{}
		now := time.Now()
		s.retain([]uint64{1, 9}, now)
		got, ok := s.selectNext([]v3PriorityHint{{ID: 1, Status: "Failed"}, h}, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
		if !ok || got.ID != 9 {
			t.Fatal(got, ok)
		}
	}
}

func TestPriorityNormalGetsOpportunityAndAgesWithoutSuccessForgery(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	s.retain([]uint64{1, 2, 9}, now)
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}, {ID: 9, Status: "Active"}}
	for i := 0; i < 2; i++ {
		s.record(rows[2], now, now, errors.New("source failure"))
	}
	h, ok := s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID == 9 {
		t.Fatal("ordinary project starved by urgent burst", h)
	}
	s.highBurst = 1 // one urgent opportunity before an aged normal turn
	h, ok = s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now.Add(91*time.Second))
	if !ok || h.ID == 9 {
		t.Fatal("aging did not protect ordinary work", h)
	}
	if !s.entries[h.ID].lastSuccess.IsZero() {
		t.Fatal("opportunity created false freshness")
	}
}

func TestPriorityRoundRefreshesBetweenAtomicTurnsAndCountsUniqueProjects(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	remaining := 50 * time.Second
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: remaining}
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}, {ID: 9, Status: "Active"}}
	var order []uint64
	refreshes := 0
	result, err := runV3PriorityRound(ctx, []uint64{1, 2, 9}, s, func() time.Time { return now }, func() ([]v3PriorityHint, error) { refreshes++; return rows, nil }, func(uint64) bool { return false },
		func(c context.Context, id uint64) error {
			order = append(order, id)
			now = now.Add(5 * time.Second)
			remaining -= 5 * time.Second
			ctx.set(remaining)
			if len(order) == 2 {
				rows[1].Unresolved = true
			}
			return nil
		}, func(uint64) error { t.Fatal("successful work revoked"); return nil })
	if err != nil || result.Completed != 3 || result.Attempted != 3 || !reflect.DeepEqual(order, []uint64{9, 1, 2}) || refreshes != 3 {
		t.Fatal(order, result, err, refreshes)
	}
}

func TestPriorityCannotStartActiveInClippedTailOrPublishPartial(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 34 * time.Second}
	result, err := runV3PriorityRound(ctx, []uint64{9}, s, func() time.Time { return now }, func() ([]v3PriorityHint, error) { return []v3PriorityHint{{ID: 9, Status: "Active"}}, nil }, func(uint64) bool { return false }, func(context.Context, uint64) error { t.Fatal("clipped active started"); return nil }, func(uint64) error { return nil })
	if err == nil || result.Attempted != 0 || result.Skipped != 1 {
		t.Fatal(result, err)
	}
}

func TestPriorityDeferredAndIntegrityKeepOriginalFailureSemantics(t *testing.T) {
	for _, failure := range []error{&v3ScanDeferred{stage: "consumer_budget_yield"}, rpc.HTTPError{StatusCode: 429}, errV3StreamConflict} {
		s := &v3PrioritySchedule{}
		revokes := 0
		result, err := runV3PriorityRound(context.Background(), []uint64{1, 9}, s, time.Now, func() ([]v3PriorityHint, error) {
			return []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 9, Status: "Active"}}, nil
		}, func(uint64) bool { return false }, func(_ context.Context, id uint64) error {
			if id == 9 {
				return failure
			}
			return nil
		}, func(uint64) error { revokes++; return nil })
		if err == nil || result.Completed != 1 || result.Failed != 1 || (revokes == 0) != v3Deferred(failure) || (result.Deferred == 1) != v3Deferred(failure) {
			t.Fatal(result, err, revokes)
		}
	}
}

func TestSchedulingYieldDoesNotAccumulateRPCBackoff(t *testing.T) {
	s := &v3RetryState{}
	for i := 0; i < 5; i++ {
		err := s.attempt(context.Background(), 9, func() error { return &v3ScanDeferred{stage: "consumer_budget_yield"} })
		if !v3Deferred(err) || s.blocked(9) || len(s.projects) != 0 {
			t.Fatal(err, s.projects)
		}
	}
	err := s.attempt(context.Background(), 9, func() error { return rpc.HTTPError{StatusCode: 429} })
	if !v3Deferred(err) || !s.blocked(9) {
		t.Fatal("429 lost real backoff", err)
	}
	if s.blocked(1) {
		t.Fatal("one source failure blocked unrelated project")
	}
}

func TestPriorityRefreshFailureAndRestartNeverInventCompletion(t *testing.T) {
	s := &v3PrioritySchedule{}
	r, err := runV3PriorityRound(context.Background(), []uint64{1}, s, time.Now, func() ([]v3PriorityHint, error) { return nil, errors.New("metadata unavailable") }, func(uint64) bool { return false }, func(context.Context, uint64) error { t.Fatal("no metadata scan"); return nil }, func(uint64) error { return nil })
	if err == nil || r.Completed != 0 {
		t.Fatal(r, err)
	}
	s.retain([]uint64{2}, time.Now())
	if len(s.entries) != 1 || !s.entries[2].lastSuccess.IsZero() {
		t.Fatal(s.entries)
	}
}

func TestPriorityActiveDoesNotWaitBehindWholeHistoricalRound(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	var activeStarts []time.Time
	ordinary := map[uint64]int{}
	ids := []uint64{1, 2, 3, 4, 9}
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}, {ID: 3, Status: "Failed"}, {ID: 4, Status: "Set"}, {ID: 9, Status: "Active"}}
	for round := 0; round < 15; round++ {
		remaining := 50 * time.Second
		ctx := &v3BudgetTestContext{Context: context.Background(), remaining: remaining}
		_, _ = runV3PriorityRound(ctx, ids, s, func() time.Time { return now }, func() ([]v3PriorityHint, error) { return rows, nil }, func(uint64) bool { return false }, func(_ context.Context, id uint64) error {
			if id == 9 {
				activeStarts = append(activeStarts, now)
			} else {
				ordinary[id]++
			}
			now = now.Add(10 * time.Second)
			remaining -= 10 * time.Second
			ctx.set(remaining)
			return nil
		}, func(uint64) error { t.Fatal("valid project revoked"); return nil })
		now = now.Add(5 * time.Second)
	}
	if len(activeStarts) < 5 || len(ordinary) != 4 {
		t.Fatal("priority or fairness lost", len(activeStarts), ordinary)
	}
	for i := 1; i < len(activeStarts); i++ {
		if gap := activeStarts[i].Sub(activeStarts[i-1]); gap > 65*time.Second {
			t.Fatal("active waiting behind full ordinary rotation", gap)
		}
	}
}

func TestPriorityBlockedSourceIsSkippedButDoesNotGrantRoundSuccess(t *testing.T) {
	result, err := runV3PriorityRound(context.Background(), []uint64{1, 9}, &v3PrioritySchedule{}, time.Now, func() ([]v3PriorityHint, error) {
		return []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 9, Status: "Active"}}, nil
	}, func(id uint64) bool { return id == 9 }, func(_ context.Context, id uint64) error {
		if id == 9 {
			t.Fatal("source backoff bypassed")
		}
		return nil
	}, func(uint64) error { return nil })
	if err == nil || result.Completed != 1 || result.Skipped != 1 {
		t.Fatal(result, err)
	}
}

func TestPriorityOwnQuantumExpiryIsNotProviderBackoff(t *testing.T) {
	s := &v3RetryState{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, v3PriorityTurnBudgetKey{}, true)
	err := s.attempt(ctx, 9, func() error { <-ctx.Done(); return ctx.Err() })
	if !v3SchedulingYield(err) || s.blocked(9) || len(s.projects) != 0 {
		t.Fatal(err, s.projects)
	}
}

func TestPriorityYieldClassificationSurvivesRedactedTrackerWrappers(t *testing.T) {
	e := (&v3TrackedRPC{}).finish(context.Background(), "consumer_prepare_or_publish", &v3ScanDeferred{stage: "consumer_budget_yield"})
	if !v3SchedulingYield(e) {
		t.Fatal("wrapper converted budget pressure to source failure", e)
	}
	if v3SchedulingYield(&v3ScanDeferred{stage: "rpc", cause: rpc.HTTPError{StatusCode: 429}}) {
		t.Fatal("429 lost provider backoff")
	}
}

func TestPriorityYieldingOrdinaryProjectCannotPinBackgroundSlot(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	s.retain([]uint64{1, 2}, now)
	s.record(v3PriorityHint{ID: 1, Status: "Failed"}, now, now, &v3ScanDeferred{stage: "consumer_budget_yield"})
	h, ok := s.selectNext([]v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}}, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID != 2 {
		t.Fatal("yielding ordinary project starved never-attempted peer", h)
	}
}
