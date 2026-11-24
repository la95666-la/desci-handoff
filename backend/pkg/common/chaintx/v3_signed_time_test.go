package chaintx

import (
	"testing"
	"time"

	"joincare_server/pkg/common/entity"
)

func TestV3SignedBeforeSetUsesUTCWallClockAndStrictDeadline(t *testing.T) {
	deadline := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC).Unix()
	local := time.FixedZone("local+8", 8*3600)
	for _, tc := range []struct {
		name string
		at   *time.Time
		want bool
	}{
		{"missing", nil, false},
		{"before", func() *time.Time { v := time.Date(2026, 10, 19, 9, 59, 59, 0, local); return &v }(), true},
		{"at deadline", func() *time.Time { v := time.Date(2026, 10, 19, 10, 0, 0, 0, local); return &v }(), false},
		{"after", func() *time.Time { v := time.Date(2026, 10, 19, 10, 0, 1, 0, local); return &v }(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := V3SignedBeforeSet(entity.ChainTxSubmission{V3SignedAt: tc.at}, deadline); got != tc.want {
				t.Fatalf("signed admission=%t want=%t", got, tc.want)
			}
		})
	}
}
