package chainevent

import (
	"context"
	"joincare_server/pkg/common/projecttracking"
	"testing"
	"time"
)

func TestPriorityAuditFollowupIsSingleUseAndNoFalseSuccess(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	s.retain([]uint64{1, 2}, now)
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}}
	s.record(rows[0], now, now, &v3ScanDeferred{stage: "audit_ready_yield"})
	if !s.entries[1].lastSuccess.IsZero() {
		t.Fatal("audit invented consumption")
	}
	h, ok := s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID != 1 {
		t.Fatal("audit did not reserve consuming opportunity", h)
	}
	s.record(rows[0], now, now, &v3ScanDeferred{stage: "consumer_budget_yield"})
	h, ok = s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID != 2 {
		t.Fatal("failed followup pinned peer", h)
	}
	s.record(rows[0], now, now, nil)
	if s.entries[1].consumeBudget != 0 {
		t.Fatal("successful turn retained escalated reservation")
	}
}

func TestPendingLifecycleDoesNotImplyUnresolvedTransaction(t *testing.T) {
	r := projecttracking.Row{Urgent: true}
	h := v3PriorityHint{ID: 1, Status: "Pending"}
	mergeV3TrackingPriority(&h, r)
	if h.urgent() || h.interval() != 45*time.Second {
		t.Fatal("Pending leaked into unresolved priority", h)
	}
	h.Unresolved = true
	if !h.urgent() {
		t.Fatal("real transaction not promoted")
	}
}

func TestPriorityFollowupStillHonorsUrgentAndBackoff(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	s.retain([]uint64{1, 2, 9}, now)
	rows := []v3PriorityHint{{ID: 1, Status: "Failed"}, {ID: 2, Status: "Failed"}, {ID: 9, Status: "Active"}}
	s.record(rows[0], now, now, &v3ScanDeferred{stage: "audit_ready_yield"})
	h, ok := s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now)
	if !ok || h.ID != 9 {
		t.Fatal("audit followup preempted due Active", h)
	}
	s.highBurst = 2
	h, ok = s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(id uint64) bool { return id == 1 }, now)
	if !ok || h.ID != 2 {
		t.Fatal("audit followup bypassed provider backoff", h)
	}
}

func TestPriorityPollingRespectsDueButBacklogDoesNotWait(t *testing.T) {
	s := &v3PrioritySchedule{}
	now := time.Now()
	s.retain([]uint64{1, 2, 3}, now)
	rows := []v3PriorityHint{{ID: 1, Status: "Set", ProducerTip: 100}, {ID: 2, Status: "Pending", Locked: true, Cursor: 100, ProducerTip: 100}, {ID: 3, Status: "Failed", Locked: true, Cursor: 90, ProducerTip: 100}}
	for _, h := range rows {
		s.record(h, now, now, nil)
	}
	h, ok := s.selectNext(rows, map[uint64]bool{}, map[uint64]bool{}, func(uint64) bool { return false }, now.Add(time.Second))
	if !ok || h.ID != 3 {
		t.Fatal("idle polling displaced backlog", h)
	}
}

// Deterministic wall-clock/load model: independent Producer 2.22 blocks/s,
// two active-priority projects, six terminal backlog projects and three Sets.
// Includes real-size 18s full audits, 10s hot admission, 6s/window, 5s cron gaps,
// 50s round and35s turn reservation. Repeated audits obey an8min validity limit.
func TestPrioritySustainedBacklogConvergesWithTwoUrgentProjects(t *testing.T) {
	start := time.Now()
	now := start
	tip := uint64(20000)
	ids := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	s := &v3PrioritySchedule{}
	cursor := map[uint64]uint64{}
	audit := map[uint64]time.Time{}
	lastActive := map[uint64]time.Time{}
	maxGap := time.Duration(0)
	for _, id := range ids {
		cursor[id] = tip - 6000
	}
	cursor[7], cursor[8] = tip, tip
	for round := 0; round < 150; round++ {
		ctx := &v3BudgetTestContext{Context: context.Background(), remaining: 50 * time.Second}
		remaining := 50 * time.Second
		advance := func(d time.Duration) {
			now = now.Add(d)
			remaining -= d
			ctx.set(remaining)
			tip = 20000 + uint64(now.Sub(start).Seconds()*2.22)
		}
		refresh := func() ([]v3PriorityHint, error) {
			rows := make([]v3PriorityHint, 0, len(ids))
			for _, id := range ids {
				status := "Failed"
				if id == 7 || id == 8 {
					status = "Active"
				}
				if id >= 9 {
					status = "Set"
				}
				rows = append(rows, v3PriorityHint{ID: id, Status: status, Locked: id < 9, Cursor: cursor[id], ProducerTip: tip, FullAudit: id < 9 && (audit[id].IsZero() || now.Sub(audit[id]) > 7*time.Minute)})
			}
			return rows, nil
		}
		_, _ = runV3PriorityRound(ctx, ids, s, func() time.Time { return now }, refresh, func(uint64) bool { return false }, func(c context.Context, id uint64) error {
			turnStart := now
			if id >= 9 {
				advance(10 * time.Millisecond)
				return nil
			}
			if id == 7 || id == 8 {
				if !lastActive[id].IsZero() && now.Sub(lastActive[id]) > maxGap {
					maxGap = now.Sub(lastActive[id])
				}
				lastActive[id] = now
			}
			full := audit[id].IsZero() || now.Sub(audit[id]) > 7*time.Minute
			if full {
				advance(18 * time.Second)
				audit[id] = now
				if id < 7 {
					return &v3ScanDeferred{stage: "audit_ready_yield"}
				}
			} else {
				advance(10 * time.Second)
			}
			from := cursor[id]
			snapshot := tip
			windows := 1
			if !full {
				windows = 3
			}
			for i := 0; i < windows && cursor[id] < snapshot; i++ {
				if i > 0 && (remaining < 15*time.Second || 35*time.Second-now.Sub(turnStart) < 15*time.Second) {
					break
				}
				advance(6 * time.Second)
				n := uint64(512)
				if c.Value(v3CatchupBatchKey{}) == true {
					n = v3CatchupConsumerBlocks
				}
				if snapshot-cursor[id] < n {
					n = snapshot - cursor[id]
				}
				cursor[id] += n
			}
			v3TraceConsumerProgress(c, from, cursor[id], snapshot)
			return nil
		}, func(uint64) error { t.Fatal("valid work revoked"); return nil })
		advance(5 * time.Second)
	}
	for id := uint64(1); id <= 6; id++ {
		lag := tip - cursor[id]
		t.Logf("project%d lag=%d", id, lag)
		if lag >= 6000 {
			t.Fatal("normal backlog grew", id, lag)
		}
	}
	if maxGap > 150*time.Second {
		t.Fatal("active service unbounded", maxGap)
	}
}
