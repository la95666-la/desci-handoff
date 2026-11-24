package chainevent

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestFairLaunchMainnetMetricsScannerObservationAlerts(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	completed := now
	source := FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "backoff", Attempts: 1, ConsecutiveRetries: 1, LastCompletedAt: &completed, LastErrorCode: FairLaunchMainnetWorkerRetryable, ArchivedThroughBlock: 100, ObservedHeadBlock: 200, FinalizedThroughBlock: 180, LagBlocks: 80, CursorGeneration: 4}
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now, LifecycleStartedAt: now.Add(-time.Second), MaxLagBlocks: 64, Source: source})
	// Exact replay of the same immutable completion is idempotent.
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now, LifecycleStartedAt: now.Add(-time.Second), MaxLagBlocks: 64, Source: source})
	snapshot := metrics.Snapshot(now)
	if len(snapshot.Sources) != 1 {
		t.Fatalf("unexpected source count: %+v", snapshot)
	}
	got := snapshot.Sources[0]
	if got.AttemptsTotal != 1 || got.RetryableFailuresTotal != 1 || got.LagBlocks != 80 || got.ConfirmationDepthBlocks != 20 || got.ConsecutiveRetries != 1 {
		t.Fatalf("unexpected retry metrics: %+v", got)
	}
	if !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_scanner_lag") || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_provider_retry") {
		t.Fatalf("retry/lag alerts missing: %+v", got.Alerts)
	}
}

func TestFairLaunchMainnetMetricsProviderInconsistencyAndReorg(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	completed := now
	source := FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "halted", Attempts: 1, LastCompletedAt: &completed, LastOutcome: "reorg_detected", LastReasonCode: "rpc_inconsistent", ArchivedThroughBlock: 100, CursorGeneration: 3}
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now, LifecycleStartedAt: now.Add(-time.Second), MaxLagBlocks: 64, Source: source})
	got := metrics.Snapshot(now).Sources[0]
	if got.ProviderInconsistencyTotal != 1 || got.ReorgTransitionsTotal != 1 || got.BlockedTransitionsTotal != 0 {
		t.Fatalf("unexpected inconsistency metrics: %+v", got)
	}
	if !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_provider_inconsistent") || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_cursor_halted") {
		t.Fatalf("inconsistency alerts missing: %+v", got.Alerts)
	}
}

func TestFairLaunchMainnetMetricsArchiveStatusAndRecoveryDedup(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	lastBatch, lastRecovery := uint64(10), uint64(11)
	reason := "canonical_conflict"
	status := FairLaunchMainnetArchiveStatus{ActivationRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, CursorStatus: "reorg_detected", CursorCanonical: false, ArchivedThroughBlock: 100, ObservedHeadBlock: 119, FinalizedThroughBlock: 99, LagBlocks: 0, CursorGeneration: 5, LastBatchReceiptID: &lastBatch, LastRecoveryReceiptID: &lastRecovery, LastErrorCode: &reason, DeadLetterCount: 2}
	metrics.ObserveArchiveStatus(status, 64, now)
	metrics.ObserveArchiveStatus(status, 64, now.Add(time.Second))
	got := metrics.Snapshot(now.Add(time.Second)).Sources[0]
	if got.RecoveryReceiptsObserved != 1 || got.DeadLetterCount != 2 || got.CursorStatus != "reorg_detected" || got.CursorCanonical || got.ConfirmationDepthBlocks != 20 {
		t.Fatalf("unexpected durable metrics: %+v", got)
	}
	if !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_dead_letter") || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_cursor_noncanonical") || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_cursor_halted") || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_recovery_receipt") {
		t.Fatalf("durable alerts missing: %+v", got.Alerts)
	}
	// Snapshot pointers/slices are defensive copies.
	*got.LastRecoveryReceiptID = 999
	got.Alerts[0].Code = "mutated"
	again := metrics.Snapshot(now.Add(2 * time.Second)).Sources[0]
	if again.LastRecoveryReceiptID == nil || *again.LastRecoveryReceiptID != 11 || again.Alerts[0].Code == "mutated" {
		t.Fatal("metrics snapshot leaked mutable state")
	}
}

func TestFairLaunchMainnetMetricsIgnoresStaleObservation(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	completed := now
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now, LifecycleStartedAt: now.Add(-time.Second), MaxLagBlocks: 64, Source: FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "scheduled", Attempts: 2, LastCompletedAt: &completed, LastOutcome: "no_work"}})
	staleCompleted := now.Add(-time.Minute)
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now.Add(time.Second), LifecycleStartedAt: now.Add(-time.Second), MaxLagBlocks: 64, Source: FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "backoff", Attempts: 1, LastCompletedAt: &staleCompleted, LastErrorCode: FairLaunchMainnetWorkerRetryable}})
	got := metrics.Snapshot(now.Add(time.Second)).Sources[0]
	if got.AttemptsTotal != 2 || got.NoWorkRunsTotal != 1 || got.RetryableFailuresTotal != 0 || got.Lifecycle != "scheduled" {
		t.Fatalf("stale observation mutated metrics: %+v", got)
	}
}

func TestFairLaunchMainnetMetricsAcceptsNewScannerLifecycle(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	completedOne := now
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: now, LifecycleStartedAt: now.Add(-time.Minute), MaxLagBlocks: 64, Source: FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "scheduled", Attempts: 2, LastCompletedAt: &completedOne, LastOutcome: "no_work"}})
	completedTwo := now.Add(2 * time.Second)
	metrics.Observe(FairLaunchMainnetScannerObservation{ObservedAt: completedTwo, LifecycleStartedAt: now.Add(time.Second), MaxLagBlocks: 64, Source: FairLaunchMainnetScannerSourceStatus{AuthorityRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, Lifecycle: "scheduled", Attempts: 1, LastCompletedAt: &completedTwo, LastOutcome: "archived"}})
	got := metrics.Snapshot(completedTwo).Sources[0]
	if got.AttemptsTotal != 3 || got.NoWorkRunsTotal != 1 || got.ArchivedRunsTotal != 1 || got.Lifecycle != "scheduled" {
		t.Fatalf("new scanner lifecycle not accumulated: %+v", got)
	}
}

func TestFairLaunchMainnetMetricsConfirmationDepthAlert(t *testing.T) {
	metrics := NewFairLaunchMainnetMetrics()
	now := time.Now().UTC()
	status := FairLaunchMainnetArchiveStatus{ActivationRowID: 1, ChainID: 56, SourceID: 2, SourceRevisionID: 3, CursorStatus: "active", CursorCanonical: true, ObservedHeadBlock: 110, FinalizedThroughBlock: 100}
	metrics.ObserveArchiveStatus(status, 64, now)
	got := metrics.Snapshot(now).Sources[0]
	if got.ConfirmationDepthBlocks != 10 || !hasFairLaunchMainnetAlert(got.Alerts, "fair_launch_mainnet_confirmation_depth") {
		t.Fatalf("confirmation depth alert missing: %+v", got)
	}
}

func TestFairLaunchMainnetScannerPublishesTypedMetrics(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	capability.seal.admission.SourceID = 2
	capability.seal.admission.SourceRevisionID = 3
	_, database := fairLaunchMainnetWorkerMockDB(t)
	clock := newFairLaunchMainnetScannerTestClock(time.Now().UTC())
	metrics := NewFairLaunchMainnetMetrics()
	runner := func(_ context.Context, _ FairLaunchMainnetSourceRegistry, _ *gorm.DB, _ time.Time) (FairLaunchMainnetArchiveWorkerResult, error) {
		return FairLaunchMainnetArchiveWorkerResult{}, errors.New(FairLaunchMainnetWorkerRetryable)
	}
	config := DefaultFairLaunchMainnetScannerConfig()
	config.Enabled = true
	scanner, err := newFairLaunchMainnetScanner(config, database, []FairLaunchMainnetSourceRegistry{mustFairLaunchMainnetSourceRegistry(t, capability, rpc)}, clock.Now(), runner, clock, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanner.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = scanner.Stop(stopCtx)
	})
	waitFairLaunchMainnetScannerStatus(t, scanner, func(status FairLaunchMainnetScannerStatus) bool { return status.Sources[0].Lifecycle == "backoff" })
	snapshot := metrics.Snapshot(clock.Now())
	if len(snapshot.Sources) != 1 || snapshot.Sources[0].RetryableFailuresTotal != 1 || !hasFairLaunchMainnetAlert(snapshot.Alerts, "fair_launch_mainnet_provider_retry") {
		t.Fatalf("scanner metrics not published: %+v", snapshot)
	}
}

func hasFairLaunchMainnetAlert(alerts []FairLaunchMainnetOperationalAlert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}
