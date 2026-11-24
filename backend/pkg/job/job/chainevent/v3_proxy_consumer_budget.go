package chainevent

import (
	"context"
	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
	"time"
)

type v3ScheduledConsumerBudget struct{}

// Prepare one bounded public-evidence window without tentative SQL probes.
// Shared project-quiet coverage advances up to 512 heights in one checkpoint INSERT;
// eventful preparation yields between complete units, retaining fence budget.
// A started unit still fails closed on timeout; no RPC error is masked.
func consumeV3StreamWindows(ctx context.Context, db *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, rt idov3whitelist.RuntimeIdentity) error {
	// The prepared consumer reads the durable producer/project tips before
	// deciding whether an atomic work window needs a reservation. An idle
	// project still verifies its checkpoint and canonical anchors; it does not
	// need to reserve a write window which it will never enter.
	limit := v3ConsumerBlockLimit(ctx)
	ctx = context.WithValue(ctx, v3ScheduledConsumerBudget{}, true)
	return runV3ConsumerWindows(ctx, func() error { return consumeV3Stream(ctx, db, rpc, s, b, rt, limit) })
}

func runV3ConsumerWindows(ctx context.Context, consume func() error) error {
	// At most three separately committed windows in the SAME bounded turn.
	// Reuse only this turn's audited grant; consumeV3Stream rechecks it and
	// the epoch/cursor/anchors under every writer lock. No parallel writers.
	for batch := 0; batch < 3; batch++ {
		if err := consume(); err != nil {
			return err
		}
		trace, ok := ctx.Value(v3ScanTraceKey{}).(*v3ScanTrace)
		if !ok {
			return nil
		} // reference/non-scheduled callers keep one window
		trace.mu.Lock()
		behind := trace.consumerObserved && trace.consumerTo < trace.producerTip
		trace.mu.Unlock()
		if !behind || !v3HasProjectBudget(ctx, 15*time.Second) {
			return nil
		}
	}
	return nil
}
func requireV3ConsumerWindowBudget(ctx context.Context, hasWork bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hasWork && !v3HasProjectBudget(ctx, 15*time.Second) {
		v3TracePhase(ctx, "consumer_budget_yield")
		return &v3ScanDeferred{stage: "consumer_budget_yield"}
	}
	return nil
}

func requireV3PublicationBudget(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !v3HasProjectBudget(ctx, 8*time.Second) {
		v3TracePhase(ctx, "publication_budget_yield")
		return &v3ScanDeferred{stage: "publication_budget_yield"}
	}
	return nil
}
