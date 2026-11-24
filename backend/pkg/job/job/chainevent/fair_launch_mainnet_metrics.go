package chainevent

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type FairLaunchMainnetScannerObservation struct {
	ObservedAt         time.Time
	LifecycleStartedAt time.Time
	MaxLagBlocks       uint64
	Source             FairLaunchMainnetScannerSourceStatus
}

type FairLaunchMainnetOperationalAlert struct {
	Code             string
	Severity         string
	AuthorityRowID   uint64
	ChainID          uint64
	SourceID         uint64
	SourceRevisionID uint64
	ObservedAt       time.Time
	Value            uint64
	Threshold        uint64
	ReasonCode       string
}

type FairLaunchMainnetSourceMetrics struct {
	AuthorityRowID             uint64
	ChainID                    uint64
	SourceID                   uint64
	SourceRevisionID           uint64
	Lifecycle                  string
	AttemptsTotal              uint64
	ArchivedRunsTotal          uint64
	NoWorkRunsTotal            uint64
	RetryableFailuresTotal     uint64
	ProviderInconsistencyTotal uint64
	BlockedTransitionsTotal    uint64
	ReorgTransitionsTotal      uint64
	RecoveryReceiptsObserved   uint64
	DeadLetterCount            uint64
	ConsecutiveRetries         uint32
	ArchivedThroughBlock       uint64
	ObservedHeadBlock          uint64
	FinalizedThroughBlock      uint64
	LagBlocks                  uint64
	ConfirmationDepthBlocks    uint64
	CursorGeneration           uint64
	CursorCanonical            bool
	CursorStatus               string
	LastReasonCode             string
	LastErrorCode              string
	LastBatchReceiptID         *uint64
	LastRecoveryReceiptID      *uint64
	LastObservedAt             time.Time
	Alerts                     []FairLaunchMainnetOperationalAlert
}

type FairLaunchMainnetMetricsSnapshot struct {
	ObservedAt time.Time
	Sources    []FairLaunchMainnetSourceMetrics
	Alerts     []FairLaunchMainnetOperationalAlert
}

type fairLaunchMainnetMetricsState struct {
	metrics                FairLaunchMainnetSourceMetrics
	lastLifecycleStartedAt time.Time
	lastAttemptCount       uint64
	lastCompletedAttempt   uint64
	lastRecoveryReceiptID  *uint64
}

// FairLaunchMainnetMetrics is an in-memory, secret-free operational collector.
// It opens no listener and sends no alert. A future admitted host may export
// Snapshot() or forward its deterministic alert records through an existing
// operations channel.
type FairLaunchMainnetMetrics struct {
	mu      sync.Mutex
	sources map[string]*fairLaunchMainnetMetricsState
}

func NewFairLaunchMainnetMetrics() *FairLaunchMainnetMetrics {
	return &FairLaunchMainnetMetrics{sources: make(map[string]*fairLaunchMainnetMetricsState)}
}

func (metrics *FairLaunchMainnetMetrics) Observe(observation FairLaunchMainnetScannerObservation) {
	if metrics == nil || observation.ObservedAt.IsZero() || observation.LifecycleStartedAt.IsZero() || observation.LifecycleStartedAt.After(observation.ObservedAt) || observation.Source.AuthorityRowID == 0 || observation.Source.ChainID != fairLaunchMainnetChainID || observation.Source.SourceID == 0 || observation.Source.SourceRevisionID == 0 || observation.MaxLagBlocks < fairLaunchMainnetArchiveMaxBlocks {
		return
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	state := metrics.source(observation.Source.AuthorityRowID, observation.Source.ChainID, observation.Source.SourceID, observation.Source.SourceRevisionID)
	if !state.lastLifecycleStartedAt.IsZero() && observation.LifecycleStartedAt.Before(state.lastLifecycleStartedAt) {
		return
	}
	if observation.LifecycleStartedAt.After(state.lastLifecycleStartedAt) {
		state.lastLifecycleStartedAt = observation.LifecycleStartedAt
		state.lastAttemptCount = 0
		state.lastCompletedAttempt = 0
	}
	if observation.Source.Attempts < state.lastAttemptCount {
		return
	}
	state.metrics.AttemptsTotal += observation.Source.Attempts - state.lastAttemptCount
	state.lastAttemptCount = observation.Source.Attempts
	newCompletion := observation.Source.LastCompletedAt != nil && observation.Source.Attempts > state.lastCompletedAttempt
	if newCompletion {
		switch observation.Source.LastOutcome {
		case "archived":
			state.metrics.ArchivedRunsTotal++
		case "no_work":
			state.metrics.NoWorkRunsTotal++
		}
		if observation.Source.LastErrorCode == FairLaunchMainnetWorkerRetryable {
			state.metrics.RetryableFailuresTotal++
		}
		if observation.Source.LastReasonCode == "rpc_inconsistent" {
			state.metrics.ProviderInconsistencyTotal++
		}
		if observation.Source.LastOutcome == "blocked" {
			state.metrics.BlockedTransitionsTotal++
		}
		if observation.Source.LastOutcome == "reorg_detected" {
			state.metrics.ReorgTransitionsTotal++
		}
		state.lastCompletedAttempt = observation.Source.Attempts
	}
	state.metrics.Lifecycle = observation.Source.Lifecycle
	state.metrics.ConsecutiveRetries = observation.Source.ConsecutiveRetries
	state.metrics.ArchivedThroughBlock = observation.Source.ArchivedThroughBlock
	state.metrics.ObservedHeadBlock = observation.Source.ObservedHeadBlock
	state.metrics.FinalizedThroughBlock = observation.Source.FinalizedThroughBlock
	state.metrics.LagBlocks = observation.Source.LagBlocks
	state.metrics.ConfirmationDepthBlocks = fairLaunchMainnetConfirmationDepth(observation.Source.ObservedHeadBlock, observation.Source.FinalizedThroughBlock)
	state.metrics.CursorGeneration = observation.Source.CursorGeneration
	state.metrics.LastReasonCode = observation.Source.LastReasonCode
	state.metrics.LastErrorCode = observation.Source.LastErrorCode
	state.metrics.LastObservedAt = observation.ObservedAt.UTC()
	state.metrics.Alerts = evaluateFairLaunchMainnetOperationalAlerts(state.metrics, observation.MaxLagBlocks, observation.ObservedAt)
}

// ObserveArchiveStatus merges durable read-only repository evidence: canonical
// state, dead letters, cursor status and receipt IDs. It performs no DB/RPC IO.
func (metrics *FairLaunchMainnetMetrics) ObserveArchiveStatus(status FairLaunchMainnetArchiveStatus, maxLagBlocks uint64, observedAt time.Time) {
	if metrics == nil || observedAt.IsZero() || status.ActivationRowID == 0 || status.ChainID != fairLaunchMainnetChainID || status.SourceID == 0 || status.SourceRevisionID == 0 || maxLagBlocks < fairLaunchMainnetArchiveMaxBlocks {
		return
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	state := metrics.source(status.ActivationRowID, status.ChainID, status.SourceID, status.SourceRevisionID)
	state.metrics.ArchivedThroughBlock = status.ArchivedThroughBlock
	state.metrics.ObservedHeadBlock = status.ObservedHeadBlock
	state.metrics.FinalizedThroughBlock = status.FinalizedThroughBlock
	state.metrics.LagBlocks = status.LagBlocks
	state.metrics.ConfirmationDepthBlocks = fairLaunchMainnetConfirmationDepth(status.ObservedHeadBlock, status.FinalizedThroughBlock)
	state.metrics.CursorGeneration = status.CursorGeneration
	state.metrics.CursorCanonical = status.CursorCanonical
	state.metrics.CursorStatus = status.CursorStatus
	state.metrics.DeadLetterCount = status.DeadLetterCount
	state.metrics.LastBatchReceiptID = copyUint64Pointer(status.LastBatchReceiptID)
	state.metrics.LastRecoveryReceiptID = copyUint64Pointer(status.LastRecoveryReceiptID)
	state.metrics.LastReasonCode = ""
	if status.LastErrorCode != nil {
		state.metrics.LastReasonCode = *status.LastErrorCode
	}
	if status.LastRecoveryReceiptID != nil && (state.lastRecoveryReceiptID == nil || *status.LastRecoveryReceiptID > *state.lastRecoveryReceiptID) {
		state.metrics.RecoveryReceiptsObserved++
		state.lastRecoveryReceiptID = copyUint64Pointer(status.LastRecoveryReceiptID)
	}
	state.metrics.LastObservedAt = observedAt.UTC()
	state.metrics.Alerts = evaluateFairLaunchMainnetOperationalAlerts(state.metrics, maxLagBlocks, observedAt)
}

func (metrics *FairLaunchMainnetMetrics) Snapshot(observedAt time.Time) FairLaunchMainnetMetricsSnapshot {
	if metrics == nil {
		return FairLaunchMainnetMetricsSnapshot{}
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	snapshot := FairLaunchMainnetMetricsSnapshot{ObservedAt: observedAt.UTC(), Sources: make([]FairLaunchMainnetSourceMetrics, 0, len(metrics.sources))}
	keys := make([]string, 0, len(metrics.sources))
	for key := range metrics.sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		source := copyFairLaunchMainnetSourceMetrics(metrics.sources[key].metrics)
		snapshot.Sources = append(snapshot.Sources, source)
		snapshot.Alerts = append(snapshot.Alerts, source.Alerts...)
	}
	return snapshot
}

func (metrics *FairLaunchMainnetMetrics) source(authorityRowID, chainID, sourceID, sourceRevisionID uint64) *fairLaunchMainnetMetricsState {
	key := fairLaunchMainnetMetricsKey(authorityRowID, chainID, sourceID, sourceRevisionID)
	state := metrics.sources[key]
	if state == nil {
		state = &fairLaunchMainnetMetricsState{metrics: FairLaunchMainnetSourceMetrics{AuthorityRowID: authorityRowID, ChainID: chainID, SourceID: sourceID, SourceRevisionID: sourceRevisionID}}
		metrics.sources[key] = state
	}
	return state
}

func fairLaunchMainnetMetricsKey(authorityRowID, chainID, sourceID, sourceRevisionID uint64) string {
	return fmt.Sprintf("%d/%d/%d/%d", authorityRowID, chainID, sourceID, sourceRevisionID)
}

func fairLaunchMainnetConfirmationDepth(observedHead, finalizedThrough uint64) uint64 {
	if observedHead <= finalizedThrough {
		return 0
	}
	return observedHead - finalizedThrough
}

func evaluateFairLaunchMainnetOperationalAlerts(source FairLaunchMainnetSourceMetrics, maxLagBlocks uint64, observedAt time.Time) []FairLaunchMainnetOperationalAlert {
	alerts := make([]FairLaunchMainnetOperationalAlert, 0, 6)
	appendAlert := func(code, severity string, value, threshold uint64, reason string) {
		alerts = append(alerts, FairLaunchMainnetOperationalAlert{Code: code, Severity: severity, AuthorityRowID: source.AuthorityRowID, ChainID: source.ChainID, SourceID: source.SourceID, SourceRevisionID: source.SourceRevisionID, ObservedAt: observedAt.UTC(), Value: value, Threshold: threshold, ReasonCode: reason})
	}
	if source.LagBlocks > maxLagBlocks {
		appendAlert("fair_launch_mainnet_scanner_lag", "warning", source.LagBlocks, maxLagBlocks, source.LastReasonCode)
	}
	if source.ObservedHeadBlock > 0 && source.ConfirmationDepthBlocks < fairLaunchMainnetConfirmations {
		appendAlert("fair_launch_mainnet_confirmation_depth", "critical", source.ConfirmationDepthBlocks, fairLaunchMainnetConfirmations, source.LastReasonCode)
	}
	if source.LastErrorCode == FairLaunchMainnetWorkerRetryable || source.ConsecutiveRetries > 0 {
		appendAlert("fair_launch_mainnet_provider_retry", "warning", uint64(source.ConsecutiveRetries), 1, source.LastReasonCode)
	}
	if source.LastReasonCode == "rpc_inconsistent" {
		appendAlert("fair_launch_mainnet_provider_inconsistent", "critical", 1, 0, source.LastReasonCode)
	}
	if source.DeadLetterCount > 0 && (source.CursorStatus == "blocked" || source.CursorStatus == "reorg_detected" || source.LastReasonCode != "") {
		appendAlert("fair_launch_mainnet_dead_letter", "critical", source.DeadLetterCount, 0, source.LastReasonCode)
	}
	if source.CursorStatus != "" && !source.CursorCanonical {
		appendAlert("fair_launch_mainnet_cursor_noncanonical", "critical", 1, 0, source.LastReasonCode)
	}
	if source.CursorStatus == "blocked" || source.CursorStatus == "reorg_detected" || source.Lifecycle == "halted" && (source.LastReasonCode == "canonical_conflict" || source.LastReasonCode == "rpc_inconsistent") {
		appendAlert("fair_launch_mainnet_cursor_halted", "critical", 1, 0, source.LastReasonCode)
	}
	if source.LastRecoveryReceiptID != nil {
		appendAlert("fair_launch_mainnet_recovery_receipt", "info", *source.LastRecoveryReceiptID, 0, "")
	}
	return alerts
}

func copyUint64Pointer(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func copyFairLaunchMainnetSourceMetrics(source FairLaunchMainnetSourceMetrics) FairLaunchMainnetSourceMetrics {
	source.LastBatchReceiptID = copyUint64Pointer(source.LastBatchReceiptID)
	source.LastRecoveryReceiptID = copyUint64Pointer(source.LastRecoveryReceiptID)
	source.Alerts = append([]FairLaunchMainnetOperationalAlert(nil), source.Alerts...)
	return source
}
