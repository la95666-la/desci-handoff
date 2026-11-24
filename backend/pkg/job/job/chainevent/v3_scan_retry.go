package chainevent

import (
	"context"
	"errors"
	"sync"
	"time"

	"joincare_server/pkg/common/log"
)

// Scheduling only, never evidence. Restart loses backoff but cannot grant any
// freshness. No sleeps inside a project turn and no retry of signing/writes.
type v3RetryState struct {
	mu       sync.Mutex
	projects map[uint64]v3RetryEntry
}
type v3RetryEntry struct {
	failures uint
	after    time.Time
}
type v3RetryKey struct{}

func v3SchedulingReason(err error) string {
	if errors.Is(err, errV3StreamConflict) {
		return "source_or_integrity"
	}
	var d *v3ScanDeferred
	for errors.As(err, &d) {
		if d.cause != nil {
			err = d.cause
			continue
		}
		switch d.stage {
		case "consumer_budget_yield", "publication_budget_yield", "retry_backoff", "shared_batch_bytes", "audit_capacity", "full_audit_expired", "audit_grant_expired", "priority_budget_yield", "audit_ready_yield":
			return d.stage
		}
		break
	}
	if err == nil {
		return "none"
	}
	return "source_or_integrity"
}
func v3SchedulingYield(err error) bool {
	reason := v3SchedulingReason(err)
	return reason != "none" && reason != "source_or_integrity"
}
func (s *v3RetryState) blocked(id uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Now().Before(s.projects[id].after)
}

func (s *v3RetryState) attempt(ctx context.Context, id uint64, work func() error) error {
	if ctx == nil {
		return errV3Stream
	}
	if e := ctx.Err(); e != nil {
		return &v3ScanDeferred{"dispatch_canceled", e}
	}
	now := time.Now()
	s.mu.Lock()
	entry := s.projects[id]
	s.mu.Unlock()
	if now.Before(entry.after) {
		return &v3ScanDeferred{"retry_backoff", nil}
	}
	e := work()
	if ctx.Value(v3PriorityTurnBudgetKey{}) == true && errors.Is(ctx.Err(), context.DeadlineExceeded) && v3Deferred(e) {
		e = &v3ScanDeferred{stage: "priority_budget_yield"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projects == nil {
		s.projects = make(map[uint64]v3RetryEntry)
	}
	if v3SchedulingYield(e) {
		// Scheduling pressure is not evidence of an unavailable source. Keep
		// an existing genuine failure backoff, but never add/increment one.
		return e
	}
	if !v3Deferred(e) {
		delete(s.projects, id)
	} else {
		if entry.failures < 5 {
			entry.failures++
		}
		delay := time.Second * time.Duration(1<<entry.failures)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		// Stable per-project jitter avoids lockstep requests without secret/PRNG state.
		entry.after = time.Now().Add(delay + time.Duration(id%1000)*time.Millisecond)
		s.projects[id] = entry
	}
	if e != nil {
		if logger := log.GetLogger(); logger != nil {
			logger.WithFields(map[string]any{"project_id": id, "failure_class": v3FailureClass(e), "retry_count": entry.failures}).Warn("V3 project scan incomplete")
		}
	}
	return e
}
