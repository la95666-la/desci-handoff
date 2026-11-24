package chainevent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	FairLaunchMainnetScannerUnavailable = "fair_launch_mainnet_scanner_unavailable"
	FairLaunchMainnetScannerDisabled    = "fair_launch_mainnet_scanner_disabled"
	FairLaunchMainnetScannerRunning     = "fair_launch_mainnet_scanner_running"
)

type FairLaunchMainnetScannerConfig struct {
	Enabled      bool
	PollInterval time.Duration
	RetryInitial time.Duration
	RetryMaximum time.Duration
	RunTimeout   time.Duration
	MaxLagBlocks uint64
}

func DefaultFairLaunchMainnetScannerConfig() FairLaunchMainnetScannerConfig {
	return FairLaunchMainnetScannerConfig{Enabled: false, PollInterval: 15 * time.Second, RetryInitial: 2 * time.Second, RetryMaximum: 2 * time.Minute, RunTimeout: 90 * time.Second, MaxLagBlocks: 64}
}

type FairLaunchMainnetScannerSourceStatus struct {
	AuthorityRowID        uint64
	ChainID               uint64
	SourceID              uint64
	SourceRevisionID      uint64
	Lifecycle             string
	Attempts              uint64
	ConsecutiveRetries    uint32
	NextAttemptAt         *time.Time
	LastStartedAt         *time.Time
	LastCompletedAt       *time.Time
	LastOutcome           string
	LastReasonCode        string
	LastErrorCode         string
	ArchivedThroughBlock  uint64
	ObservedHeadBlock     uint64
	FinalizedThroughBlock uint64
	LagBlocks             uint64
	CursorGeneration      uint64
}

type FairLaunchMainnetScannerStatus struct {
	Enabled   bool
	Running   bool
	StartedAt *time.Time
	StoppedAt *time.Time
	Sources   []FairLaunchMainnetScannerSourceStatus
}

type fairLaunchMainnetScannerSource struct {
	registry FairLaunchMainnetSourceRegistry
	status   FairLaunchMainnetScannerSourceStatus
}

type fairLaunchMainnetScannerRunner func(context.Context, FairLaunchMainnetSourceRegistry, *gorm.DB, time.Time) (FairLaunchMainnetArchiveWorkerResult, error)

type fairLaunchMainnetScannerObserver interface {
	Observe(FairLaunchMainnetScannerObservation)
}

type fairLaunchMainnetScannerClock interface {
	Now() time.Time
	NewTimer(time.Duration) fairLaunchMainnetScannerTimer
}

type fairLaunchMainnetScannerTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type FairLaunchMainnetScanner struct {
	mu       sync.Mutex
	config   FairLaunchMainnetScannerConfig
	database *gorm.DB
	sources  []fairLaunchMainnetScannerSource
	runner   fairLaunchMainnetScannerRunner
	clock    fairLaunchMainnetScannerClock
	observer fairLaunchMainnetScannerObserver
	running  bool
	cancel   context.CancelFunc
	done     chan struct{}
	started  *time.Time
	stopped  *time.Time
	claims   []string
}

var fairLaunchMainnetScannerClaims = struct {
	sync.Mutex
	owners map[string]*FairLaunchMainnetScanner
}{owners: make(map[string]*FairLaunchMainnetScanner)}

// NewFairLaunchMainnetScanner constructs an inert lifecycle controller. It
// starts no goroutine and registers no cron entry. The default config is
// disabled; callers must explicitly enable it and supply opaque source
// registries that all pass their current activation/lease admission.
func NewFairLaunchMainnetScanner(config FairLaunchMainnetScannerConfig, database *gorm.DB, registries []FairLaunchMainnetSourceRegistry, now time.Time) (*FairLaunchMainnetScanner, error) {
	return newFairLaunchMainnetScanner(config, database, registries, now, runFairLaunchMainnetArchiveOnce, fairLaunchMainnetRealScannerClock{}, fairLaunchMainnetNoopScannerObserver{})
}

// NewFairLaunchMainnetScannerWithMetrics attaches the in-memory typed metrics
// collector without opening an exporter or changing the default-disabled
// lifecycle. Metrics collection is synchronous, bounded and secret-free.
func NewFairLaunchMainnetScannerWithMetrics(config FairLaunchMainnetScannerConfig, database *gorm.DB, registries []FairLaunchMainnetSourceRegistry, metrics *FairLaunchMainnetMetrics, now time.Time) (*FairLaunchMainnetScanner, error) {
	if metrics == nil {
		return nil, errors.New(FairLaunchMainnetScannerUnavailable)
	}
	return newFairLaunchMainnetScanner(config, database, registries, now, runFairLaunchMainnetArchiveOnce, fairLaunchMainnetRealScannerClock{}, metrics)
}

// Start launches one explicitly controlled lifecycle goroutine. Sources are
// executed serially, so one scanner cannot overlap a source with itself or
// another source. Nothing calls Start from the production starter today.
func (scanner *FairLaunchMainnetScanner) Start(parent context.Context) error {
	if scanner == nil || parent == nil {
		return errors.New(FairLaunchMainnetScannerUnavailable)
	}
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	if !scanner.config.Enabled {
		return errors.New(FairLaunchMainnetScannerDisabled)
	}
	if scanner.running {
		return errors.New(FairLaunchMainnetScannerRunning)
	}
	if scanner.started != nil {
		return errors.New(FairLaunchMainnetScannerUnavailable)
	}
	now := scanner.clock.Now().UTC()
	claims, err := scanner.claimSources()
	if err != nil {
		return err
	}
	for index := range scanner.sources {
		if _, _, _, err := scanner.sources[index].registry.route(now); err != nil {
			scanner.releaseSources(claims)
			return errors.New(FairLaunchMainnetScannerUnavailable)
		}
		scanner.sources[index].status.Lifecycle = "scheduled"
		scanner.sources[index].status.NextAttemptAt = timePointer(now)
		scanner.sources[index].status.ConsecutiveRetries = 0
		scanner.sources[index].status.LastErrorCode = ""
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	scanner.cancel = cancel
	scanner.done = done
	scanner.running = true
	scanner.claims = claims
	scanner.started = timePointer(now)
	scanner.stopped = nil
	go scanner.run(ctx, done)
	return nil
}

func (scanner *FairLaunchMainnetScanner) Stop(ctx context.Context) error {
	if scanner == nil || ctx == nil {
		return errors.New(FairLaunchMainnetScannerUnavailable)
	}
	scanner.mu.Lock()
	if !scanner.running {
		scanner.mu.Unlock()
		return nil
	}
	cancel, done := scanner.cancel, scanner.done
	scanner.mu.Unlock()
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (scanner *FairLaunchMainnetScanner) Status() FairLaunchMainnetScannerStatus {
	if scanner == nil {
		return FairLaunchMainnetScannerStatus{}
	}
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	status := FairLaunchMainnetScannerStatus{Enabled: scanner.config.Enabled, Running: scanner.running, StartedAt: copyTimePointer(scanner.started), StoppedAt: copyTimePointer(scanner.stopped), Sources: make([]FairLaunchMainnetScannerSourceStatus, len(scanner.sources))}
	for index := range scanner.sources {
		status.Sources[index] = copyFairLaunchMainnetScannerSourceStatus(scanner.sources[index].status)
	}
	return status
}

func newFairLaunchMainnetScanner(config FairLaunchMainnetScannerConfig, database *gorm.DB, registries []FairLaunchMainnetSourceRegistry, now time.Time, runner fairLaunchMainnetScannerRunner, clock fairLaunchMainnetScannerClock, observer fairLaunchMainnetScannerObserver) (*FairLaunchMainnetScanner, error) {
	if database == nil || runner == nil || clock == nil || observer == nil || len(registries) == 0 || len(registries) > 64 || !validFairLaunchMainnetScannerConfig(config) {
		return nil, errors.New(FairLaunchMainnetScannerUnavailable)
	}
	sources := make([]fairLaunchMainnetScannerSource, len(registries))
	seen := make(map[string]bool, len(registries))
	for index, registry := range registries {
		_, _, admission, err := registry.route(now)
		if err != nil {
			return nil, errors.New(FairLaunchMainnetScannerUnavailable)
		}
		key := fmt.Sprintf("%d/%d/%d/%d", admission.AuthorityRowID, admission.ChainID, admission.SourceID, admission.SourceRevisionID)
		if seen[key] {
			return nil, errors.New(FairLaunchMainnetScannerUnavailable)
		}
		seen[key] = true
		sources[index] = fairLaunchMainnetScannerSource{registry: registry, status: FairLaunchMainnetScannerSourceStatus{AuthorityRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, Lifecycle: "disabled"}}
	}
	return &FairLaunchMainnetScanner{config: config, database: database, sources: sources, runner: runner, clock: clock, observer: observer}, nil
}

func validFairLaunchMainnetScannerConfig(config FairLaunchMainnetScannerConfig) bool {
	return config.PollInterval >= 5*time.Second && config.PollInterval <= 15*time.Minute && config.RetryInitial >= time.Second && config.RetryInitial <= config.RetryMaximum && config.RetryMaximum <= 15*time.Minute && config.RunTimeout >= 5*time.Second && config.RunTimeout <= 5*time.Minute && config.MaxLagBlocks >= fairLaunchMainnetArchiveMaxBlocks && config.MaxLagBlocks <= 100000
}

func (scanner *FairLaunchMainnetScanner) run(ctx context.Context, done chan struct{}) {
	defer func() {
		now := scanner.clock.Now().UTC()
		scanner.mu.Lock()
		for index := range scanner.sources {
			if scanner.sources[index].status.Lifecycle == "running" || scanner.sources[index].status.Lifecycle == "scheduled" || scanner.sources[index].status.Lifecycle == "backoff" {
				scanner.sources[index].status.Lifecycle = "stopped"
				scanner.sources[index].status.NextAttemptAt = nil
			}
		}
		scanner.running = false
		scanner.cancel = nil
		scanner.stopped = timePointer(now)
		claims := append([]string(nil), scanner.claims...)
		scanner.claims = nil
		scanner.releaseSources(claims)
		close(done)
		scanner.mu.Unlock()
	}()
	for {
		now := scanner.clock.Now().UTC()
		index, wait := scanner.nextSource(now)
		if index >= 0 {
			scanner.runSource(ctx, index, now)
			continue
		}
		if wait < 0 {
			<-ctx.Done()
			return
		}
		timer := scanner.clock.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C():
		}
	}
}

func (scanner *FairLaunchMainnetScanner) claimSources() ([]string, error) {
	claims := make([]string, len(scanner.sources))
	for index := range scanner.sources {
		status := scanner.sources[index].status
		claims[index] = fmt.Sprintf("%d/%d/%d/%d", status.AuthorityRowID, status.ChainID, status.SourceID, status.SourceRevisionID)
	}
	fairLaunchMainnetScannerClaims.Lock()
	defer fairLaunchMainnetScannerClaims.Unlock()
	for _, claim := range claims {
		if fairLaunchMainnetScannerClaims.owners[claim] != nil {
			return nil, errors.New(FairLaunchMainnetScannerRunning)
		}
	}
	for _, claim := range claims {
		fairLaunchMainnetScannerClaims.owners[claim] = scanner
	}
	return claims, nil
}

func (scanner *FairLaunchMainnetScanner) releaseSources(claims []string) {
	fairLaunchMainnetScannerClaims.Lock()
	defer fairLaunchMainnetScannerClaims.Unlock()
	for _, claim := range claims {
		if fairLaunchMainnetScannerClaims.owners[claim] == scanner {
			delete(fairLaunchMainnetScannerClaims.owners, claim)
		}
	}
}

func (scanner *FairLaunchMainnetScanner) nextSource(now time.Time) (int, time.Duration) {
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	selected := -1
	var earliest time.Time
	for index := range scanner.sources {
		next := scanner.sources[index].status.NextAttemptAt
		if next == nil {
			continue
		}
		if !next.After(now) {
			return index, 0
		}
		if selected < 0 || next.Before(earliest) {
			selected, earliest = index, *next
		}
	}
	if selected < 0 {
		return -1, -1
	}
	return -1, earliest.Sub(now)
}

func (scanner *FairLaunchMainnetScanner) runSource(parent context.Context, index int, startedAt time.Time) {
	scanner.mu.Lock()
	source := &scanner.sources[index]
	source.status.Lifecycle = "running"
	source.status.Attempts++
	source.status.LastStartedAt = timePointer(startedAt)
	source.status.NextAttemptAt = nil
	source.status.LastOutcome = ""
	source.status.LastReasonCode = ""
	source.status.LastErrorCode = ""
	registry := source.registry
	scanner.mu.Unlock()

	runCtx, cancel := context.WithTimeout(parent, scanner.config.RunTimeout)
	result, err := scanner.runner(runCtx, registry, scanner.database, startedAt)
	cancel()
	completedAt := scanner.clock.Now().UTC()

	scanner.mu.Lock()
	source = &scanner.sources[index]
	source.status.LastCompletedAt = timePointer(completedAt)
	if parent.Err() != nil {
		source.status.Lifecycle = "stopped"
		source.status.NextAttemptAt = nil
		scanner.observeAndUnlock(index, completedAt)
		return
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), FairLaunchMainnetWorkerRetryable) {
			source.status.ConsecutiveRetries++
			source.status.Lifecycle = "backoff"
			source.status.LastErrorCode = FairLaunchMainnetWorkerRetryable
			next := completedAt.Add(fairLaunchMainnetScannerBackoff(scanner.config, source.status.ConsecutiveRetries))
			source.status.NextAttemptAt = timePointer(next)
			scanner.observeAndUnlock(index, completedAt)
			return
		}
		source.status.Lifecycle = "halted"
		source.status.LastErrorCode = FairLaunchMainnetWorkerUnavailable
		source.status.NextAttemptAt = nil
		scanner.observeAndUnlock(index, completedAt)
		return
	}
	source.status.ConsecutiveRetries = 0
	source.status.LastErrorCode = ""
	source.status.LastOutcome = result.Outcome
	source.status.LastReasonCode = result.ReasonCode
	source.status.ArchivedThroughBlock = result.ArchivedThroughBlock
	source.status.ObservedHeadBlock = result.ObservedHeadBlock
	source.status.FinalizedThroughBlock = result.FinalizedThroughBlock
	source.status.LagBlocks = result.LagBlocks
	source.status.CursorGeneration = result.CursorGeneration
	if result.Outcome == "blocked" || result.Outcome == "reorg_detected" || result.Outcome == "revoked" {
		source.status.Lifecycle = "halted"
		source.status.NextAttemptAt = nil
		scanner.observeAndUnlock(index, completedAt)
		return
	}
	source.status.Lifecycle = "scheduled"
	next := completedAt.Add(scanner.config.PollInterval)
	source.status.NextAttemptAt = timePointer(next)
	scanner.observeAndUnlock(index, completedAt)
}

func (scanner *FairLaunchMainnetScanner) observeAndUnlock(index int, observedAt time.Time) {
	source := copyFairLaunchMainnetScannerSourceStatus(scanner.sources[index].status)
	startedAt := time.Time{}
	if scanner.started != nil {
		startedAt = *scanner.started
	}
	observation := FairLaunchMainnetScannerObservation{ObservedAt: observedAt, LifecycleStartedAt: startedAt, MaxLagBlocks: scanner.config.MaxLagBlocks, Source: source}
	scanner.observer.Observe(observation)
	scanner.mu.Unlock()
}

func fairLaunchMainnetScannerBackoff(config FairLaunchMainnetScannerConfig, retries uint32) time.Duration {
	delay := config.RetryInitial
	for count := uint32(1); count < retries && delay < config.RetryMaximum; count++ {
		if delay > config.RetryMaximum/2 {
			return config.RetryMaximum
		}
		delay *= 2
	}
	if delay > config.RetryMaximum {
		return config.RetryMaximum
	}
	return delay
}

func timePointer(value time.Time) *time.Time {
	copy := value
	return &copy
}

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	return timePointer(*value)
}

func copyFairLaunchMainnetScannerSourceStatus(source FairLaunchMainnetScannerSourceStatus) FairLaunchMainnetScannerSourceStatus {
	source.NextAttemptAt = copyTimePointer(source.NextAttemptAt)
	source.LastStartedAt = copyTimePointer(source.LastStartedAt)
	source.LastCompletedAt = copyTimePointer(source.LastCompletedAt)
	return source
}

type fairLaunchMainnetRealScannerClock struct{}

func (fairLaunchMainnetRealScannerClock) Now() time.Time { return time.Now().UTC() }

func (fairLaunchMainnetRealScannerClock) NewTimer(delay time.Duration) fairLaunchMainnetScannerTimer {
	return fairLaunchMainnetRealScannerTimer{timer: time.NewTimer(delay)}
}

type fairLaunchMainnetRealScannerTimer struct{ timer *time.Timer }

func (timer fairLaunchMainnetRealScannerTimer) C() <-chan time.Time { return timer.timer.C }
func (timer fairLaunchMainnetRealScannerTimer) Stop() bool          { return timer.timer.Stop() }

type fairLaunchMainnetNoopScannerObserver struct{}

func (fairLaunchMainnetNoopScannerObserver) Observe(FairLaunchMainnetScannerObservation) {}
