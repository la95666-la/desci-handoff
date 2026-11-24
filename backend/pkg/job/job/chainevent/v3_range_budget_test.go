package chainevent

import (
	"context"
	"testing"
	"time"
)

func TestApprovedRangeYieldsBetweenCommittedWindowsWithFinalCheckReserve(t *testing.T) {
	for _, tc := range []struct {
		remaining, last time.Duration
		yield           bool
	}{
		{30 * time.Second, time.Second, false},
		{14 * time.Second, time.Second, true},
		{25 * time.Second, 10 * time.Second, true},
		{40 * time.Second, 10 * time.Second, false},
		{time.Second, time.Second, true},
		{40 * time.Second, time.Hour, true},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), tc.remaining)
		got := yieldV3RangeBetweenWindows(ctx, tc.last)
		cancel()
		if got != tc.yield {
			t.Fatalf("remaining=%v last=%v yield=%v", tc.remaining, tc.last, got)
		}
	}
	if yieldV3RangeBetweenWindows(context.Background(), time.Second) {
		t.Fatal("unbounded driver stopped without deadline")
	}
	if v3NextWindowBudget(0) != 15*time.Second {
		t.Fatal("final checks reserve lost")
	}
}
