package chainevent

import (
	"context"
	"time"
)

// Leave 10s for final header/depth fences and caller receipt queries, plus
// twice the last completed window's cost for another unit. This estimate
// schedules work only; authority still requires live canonical evidence.
func yieldV3RangeBetweenWindows(ctx context.Context, last time.Duration) bool {
	if evidence, ok := ctx.Value(v3PreparedBudgetKey{}).(*v3PreparedRPC); ok {
		// Replay the SAME prepared prefix even though publication CPU/DB time
		// differs. The real context deadline is still enforced by SQL/RPC replay.
		plain := context.WithValue(ctx, v3PreparedBudgetKey{}, nil)
		yield, e := preparedRead(ctx, evidence, "schedule-yield", nil, func() (bool, error) { return yieldV3RangeBetweenWindows(plain, last), nil }, func(v bool) bool { return v })
		return e != nil || yield
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return false
	}
	return time.Until(deadline) <= v3NextWindowBudget(last)
}

func v3NextWindowBudget(last time.Duration) time.Duration {
	if last < 2500*time.Millisecond {
		last = 2500 * time.Millisecond
	}
	// Saturate before arithmetic to avoid an overflow admitting more work.
	if last >= time.Minute {
		return 3 * time.Minute
	}
	return 10*time.Second + 2*last
}
