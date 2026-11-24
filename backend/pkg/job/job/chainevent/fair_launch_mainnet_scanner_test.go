package chainevent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestFairLaunchMainnetScannerDefaultsDisabled(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	config := DefaultFairLaunchMainnetScannerConfig()
	if config.Enabled {
		t.Fatal("mainnet scanner defaulted enabled")
	}
	scanner, err := NewFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{mustFairLaunchMainnetSourceRegistry(t, capability, rpc)}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := scanner.Start(context.Background()); err == nil || err.Error() != FairLaunchMainnetScannerDisabled {
		t.Fatalf("disabled scanner started: %v", err)
	}
	status := scanner.Status()
	if status.Enabled || status.Running || len(status.Sources) != 1 || status.Sources[0].Lifecycle != "disabled" || status.Sources[0].Attempts != 0 {
		t.Fatalf("unexpected disabled status: %+v", status)
	}
}

func TestFairLaunchMainnetScannerSingleFlightAndGracefulStop(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clock := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	started := make(chan struct{}, 2)
	var active atomic.Int32
	var maximum atomic.Int32
	runner := func(ctx context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
		}
		started <- struct{}{}
		<-ctx.Done()
		return FairLaunchMainnetArchiveWorkerResult{}, ctx.Err()
	}
	scanner := mustNewFairLaunchMainnetScannerForTest(t, database, capability, rpc, runner, clock)
	if err := scanner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scanner run did not start")
	}
	if err := scanner.Start(context.Background()); err == nil || err.Error() != FairLaunchMainnetScannerRunning {
		t.Fatalf("overlapping scanner start accepted: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scanner.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() != 1 || active.Load() != 0 {
		t.Fatalf("scanner overlapped work: active=%d maximum=%d", active.Load(), maximum.Load())
	}
	status := scanner.Status()
	if status.Running || status.StoppedAt == nil || status.Sources[0].Lifecycle != "stopped" || status.Sources[0].Attempts != 1 {
		t.Fatalf("unexpected stopped status: %+v", status)
	}
	if err := scanner.Start(context.Background()); err == nil || err.Error() != FairLaunchMainnetScannerUnavailable {
		t.Fatalf("single-use scanner restarted with stale registry: %v", err)
	}
}

func TestFairLaunchMainnetScannerProcessWideSourceClaim(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clockOne := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	clockTwo := newFairLaunchMainnetScannerTestClock(clockOne.Now())
	started := make(chan struct{}, 1)
	runnerOne := func(ctx context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		return FairLaunchMainnetArchiveWorkerResult{}, ctx.Err()
	}
	runnerTwo := func(context.Context, FairLaunchMainnetSourceRegistry, *gorm.DB, time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		return FairLaunchMainnetArchiveWorkerResult{Outcome: "no_work"}, nil
	}
	first := mustNewFairLaunchMainnetScannerForTest(t, database, capability, rpc, runnerOne, clockOne)
	second := mustNewFairLaunchMainnetScannerForTest(t, database, capability, rpc, runnerTwo, clockTwo)
	if err := first.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first scanner did not acquire source")
	}
	if err := second.Start(context.Background()); err == nil || err.Error() != FairLaunchMainnetScannerRunning {
		t.Fatalf("duplicate process-wide source claim accepted: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := first.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if err := second.Start(context.Background()); err != nil {
		t.Fatalf("source claim not released after graceful stop: %v", err)
	}
	if err := second.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestFairLaunchMainnetScannerRetryBackoffThenSchedules(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clock := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	var calls atomic.Uint32
	runner := func(_ context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		if calls.Add(1) == 1 {
			return FairLaunchMainnetArchiveWorkerResult{}, errors.New(FairLaunchMainnetWorkerRetryable)
		}
		return FairLaunchMainnetArchiveWorkerResult{Outcome: "no_work", ArchivedThroughBlock: 100, ObservedHeadBlock: 120, FinalizedThroughBlock: 100, CursorGeneration: 2}, nil
	}
	scanner := mustNewFairLaunchMainnetScannerForTest(t, database, capability, rpc, runner, clock)
	if err := scanner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFairLaunchMainnetScannerStatus(t, scanner, func(status FairLaunchMainnetScannerStatus) bool {
		return status.Sources[0].Lifecycle == "backoff" && status.Sources[0].Attempts == 1 && status.Sources[0].ConsecutiveRetries == 1 && status.Sources[0].LastErrorCode == FairLaunchMainnetWorkerRetryable
	})
	clock.Advance(time.Second)
	if calls.Load() != 1 {
		t.Fatal("scanner retried before bounded backoff")
	}
	clock.Advance(time.Second)
	waitFairLaunchMainnetScannerStatus(t, scanner, func(status FairLaunchMainnetScannerStatus) bool {
		source := status.Sources[0]
		return source.Lifecycle == "scheduled" && source.Attempts == 2 && source.ConsecutiveRetries == 0 && source.LastOutcome == "no_work" && source.ArchivedThroughBlock == 100 && source.CursorGeneration == 2
	})
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scanner.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestFairLaunchMainnetScannerDeterministicBlockerHalts(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clock := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	var calls atomic.Uint32
	runner := func(_ context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		calls.Add(1)
		return FairLaunchMainnetArchiveWorkerResult{Outcome: "reorg_detected", ReasonCode: "canonical_conflict", ArchivedThroughBlock: 100, CursorGeneration: 3}, nil
	}
	scanner := mustNewFairLaunchMainnetScannerForTest(t, database, capability, rpc, runner, clock)
	if err := scanner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFairLaunchMainnetScannerStatus(t, scanner, func(status FairLaunchMainnetScannerStatus) bool {
		source := status.Sources[0]
		return source.Lifecycle == "halted" && source.LastOutcome == "reorg_detected" && source.LastReasonCode == "canonical_conflict" && source.NextAttemptAt == nil
	})
	clock.Advance(time.Hour)
	if calls.Load() != 1 {
		t.Fatalf("halted scanner retried: %d", calls.Load())
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scanner.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestFairLaunchMainnetScannerGracefulCancellationDoesNotBackoff(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clock := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	runner := func(ctx context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		<-ctx.Done()
		return FairLaunchMainnetArchiveWorkerResult{}, ctx.Err()
	}
	config := DefaultFairLaunchMainnetScannerConfig()
	config.Enabled = true
	config.RunTimeout = 5 * time.Second
	scanner, err := newFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{mustFairLaunchMainnetSourceRegistry(t, capability, rpc)}, clock.Now(), runner, clock, fairLaunchMainnetNoopScannerObserver{})
	if err != nil {
		t.Fatal(err)
	}
	if err := scanner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFairLaunchMainnetScannerStatus(t, scanner, func(status FairLaunchMainnetScannerStatus) bool {
		return status.Sources[0].Lifecycle == "running"
	})
	// Context timeout uses the real monotonic clock. Cancel the run through Stop;
	// graceful cancellation must not be counted as a retry or blocker.
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scanner.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	status := scanner.Status()
	if status.Sources[0].ConsecutiveRetries != 0 || status.Sources[0].LastErrorCode != "" || status.Sources[0].Lifecycle != "stopped" {
		t.Fatalf("shutdown cancellation misclassified: %+v", status.Sources[0])
	}
}

func TestFairLaunchMainnetScannerRejectsDuplicateSource(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	registry := mustFairLaunchMainnetSourceRegistry(t, capability, rpc)
	config := DefaultFairLaunchMainnetScannerConfig()
	if _, err := NewFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{registry, registry}, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetScannerUnavailable {
		t.Fatalf("duplicate source accepted: %v", err)
	}
}

func TestFairLaunchMainnetScannerRejectsInvalidBounds(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	_, database := fairLaunchMainnetWorkerMockDB(t)
	config := DefaultFairLaunchMainnetScannerConfig()
	config.PollInterval = time.Second
	if _, err := NewFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{mustFairLaunchMainnetSourceRegistry(t, capability, rpc)}, time.Now().UTC()); err == nil || err.Error() != FairLaunchMainnetScannerUnavailable {
		t.Fatalf("unsafe scanner timing accepted: %v", err)
	}
}

func TestFairLaunchMainnetScannerBackoffIsBounded(t *testing.T) {
	config := DefaultFairLaunchMainnetScannerConfig()
	if delay := fairLaunchMainnetScannerBackoff(config, 1); delay != 2*time.Second {
		t.Fatalf("first backoff=%s", delay)
	}
	if delay := fairLaunchMainnetScannerBackoff(config, 7); delay != 2*time.Minute {
		t.Fatalf("bounded backoff=%s", delay)
	}
}

func mustNewFairLaunchMainnetScannerForTest(t *testing.T, database *gorm.DB, capability FairLaunchMainnetCapability, rpc FairLaunchMainnetRPC, runner fairLaunchMainnetScannerRunner, clock fairLaunchMainnetScannerClock) *FairLaunchMainnetScanner {
	t.Helper()
	config := DefaultFairLaunchMainnetScannerConfig()
	config.Enabled = true
	scanner, err := newFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{mustFairLaunchMainnetSourceRegistry(t, capability, rpc)}, clock.Now(), runner, clock, fairLaunchMainnetNoopScannerObserver{})
	if err != nil {
		t.Fatal(err)
	}
	return scanner
}

func waitFairLaunchMainnetScannerStatus(t *testing.T, scanner *FairLaunchMainnetScanner, condition func(FairLaunchMainnetScannerStatus) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition(scanner.Status()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("scanner condition unavailable: %+v", scanner.Status())
}

type fairLaunchMainnetScannerTestClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fairLaunchMainnetScannerTestTimer
}

func newFairLaunchMainnetScannerTestClock(now time.Time) *fairLaunchMainnetScannerTestClock {
	return &fairLaunchMainnetScannerTestClock{now: now.UTC()}
}

func (clock *fairLaunchMainnetScannerTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fairLaunchMainnetScannerTestClock) NewTimer(delay time.Duration) fairLaunchMainnetScannerTimer {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := &fairLaunchMainnetScannerTestTimer{clock: clock, due: clock.now.Add(delay), channel: make(chan time.Time, 1)}
	clock.timers = append(clock.timers, timer)
	return timer
}

func (clock *fairLaunchMainnetScannerTestClock) Advance(delay time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(delay)
	now := clock.now
	timers := append([]*fairLaunchMainnetScannerTestTimer(nil), clock.timers...)
	clock.mu.Unlock()
	for _, timer := range timers {
		timer.fire(now)
	}
}

type fairLaunchMainnetScannerTestTimer struct {
	mu      sync.Mutex
	clock   *fairLaunchMainnetScannerTestClock
	due     time.Time
	channel chan time.Time
	stopped bool
	fired   bool
}

func (timer *fairLaunchMainnetScannerTestTimer) C() <-chan time.Time { return timer.channel }

func (timer *fairLaunchMainnetScannerTestTimer) Stop() bool {
	timer.mu.Lock()
	defer timer.mu.Unlock()
	if timer.stopped || timer.fired {
		return false
	}
	timer.stopped = true
	return true
}

func (timer *fairLaunchMainnetScannerTestTimer) fire(now time.Time) {
	timer.mu.Lock()
	defer timer.mu.Unlock()
	if timer.stopped || timer.fired || now.Before(timer.due) {
		return
	}
	timer.fired = true
	timer.channel <- now
}
