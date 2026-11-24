package chainevent

import (
	"context"
	"testing"
	"time"
)

func TestNormalFullAuditHasIndependent35SecondTurnAndNoFakeSuccess(t *testing.T) {
	now := time.Now()
	ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
	cache := &v3PeriodicAudit{}
	s := &v3PrioritySchedule{}
	if !cache.needsFull(1, now) {
		t.Fatal("cold cache treated as reuse")
	}
	result, err := runV3PriorityRound(ctx, []uint64{1}, s, func() time.Time { return now }, func() ([]v3PriorityHint, error) {
		return []v3PriorityHint{{ID: 1, Status: "Failed", Locked: true, FullAudit: true}}, nil
	}, func(uint64) bool { return false }, func(child context.Context, id uint64) error {
		deadline, _ := child.Deadline()
		if time.Until(deadline) < 34*time.Second {
			t.Fatal("full audit got ordinary 25s budget")
		}
		if child.Value(v3NormalAuditTurnKey{}) != true {
			t.Fatal("audit not split from consumer")
		}
		return &v3ScanDeferred{stage: "audit_ready_yield"}
	}, func(uint64) error { t.Fatal("audit yield revoked project"); return nil })
	if err == nil || result.Completed != 0 || result.Deferred != 1 || !s.entries[1].lastSuccess.IsZero() {
		t.Fatal(result, err)
	}
}

func TestNormalBudgetYieldEscalatesNextConsumerWithoutProviderBackoff(t *testing.T) {
	now := time.Now()
	s := &v3PrioritySchedule{}
	s.retain([]uint64{1}, now)
	s.record(v3PriorityHint{ID: 1, Status: "Failed"}, now, now, &v3ScanDeferred{stage: "consumer_budget_yield"})
	result, err := runV3PriorityRound(context.Background(), []uint64{1}, s, time.Now, func() ([]v3PriorityHint, error) { return []v3PriorityHint{{ID: 1, Status: "Failed"}}, nil }, func(uint64) bool { return false }, func(child context.Context, id uint64) error {
		d, _ := child.Deadline()
		if time.Until(d) < 34*time.Second {
			t.Fatal("yield repeated fixed insufficient quantum")
		}
		return nil
	}, func(uint64) error { return nil })
	if err != nil || result.Completed != 1 {
		t.Fatal(result, err)
	}
}
