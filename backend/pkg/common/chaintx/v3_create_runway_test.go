package chaintx

import (
	"context"
	"joincare_server/pkg/common/v3diagnostic"
	"testing"
	"time"
)

func TestCreateRunwayBoundaries(t *testing.T) {
	now := time.Unix(1800000000, 500000000)
	for _, tc := range []struct {
		name      string
		remaining int64
		reserve   time.Duration
		reason    string
	}{
		{"expired", 0, V3CreateRequestReserve, "CREATE_DEADLINE_EXPIRED"},
		{"incident seventeen seconds", 17, V3CreateSigningReserve, "CREATE_RUNWAY_INSUFFICIENT"},
		{"request too late", 180, V3CreateRequestReserve, "CREATE_RUNWAY_INSUFFICIENT"},
		{"request enough", 181, V3CreateRequestReserve, ""},
		{"signing too late", 90, V3CreateSigningReserve, "CREATE_RUNWAY_INSUFFICIENT"},
		{"signing enough", 91, V3CreateSigningReserve, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireV3CreateRunway(now, now.Unix()+tc.remaining, tc.reserve, "prepare_signature")
			if tc.reason == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || v3diagnostic.Reason(err) != tc.reason {
				t.Fatal(err)
			}
		})
	}
}

func TestCreateCannotSignAtEndOfRequestBudget(t *testing.T) {
	for _, remaining := range []time.Duration{17 * time.Second, 60 * time.Second, 61 * time.Second} {
		now := time.Now()
		ctx, cancel := context.WithDeadline(context.Background(), now.Add(remaining))
		err := requireV3CreateSigningBudget(ctx, now)
		cancel()
		if (err == nil) != (remaining > V3CreatePostSignBudget) {
			t.Fatal(remaining, err)
		}
	}
	if requireV3CreateSigningBudget(context.Background(), time.Now()) == nil {
		t.Fatal("unbounded caller accepted")
	}
}
