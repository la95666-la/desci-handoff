package idov3whitelist

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestApprovedStoredSaleSecondsRequiresExactReviewedDATETIMERounding(t *testing.T) {
	for _, nanos := range []int{420000000, 586000000} {
		start := time.Date(2030, 1, 1, 7, 0, 53, nanos, time.UTC)
		end := start.Add(time.Hour)
		request, err := json.Marshal(struct {
			Start string `json:"start_time"`
			End   string `json:"end_time"`
		}{start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)})
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(request)
		record := ApprovedSetRecord{Request: request, Identity: ApprovedSetIdentity{
			CreateDeadline: start.Unix(), RequestDigest: fmt.Sprintf("0x%x", digest)}}
		local := time.FixedZone("Asia/Shanghai", 8*60*60)
		storedStart, storedEnd := start.Round(time.Second).In(local), end.Round(time.Second).In(local)
		a, b, err := ApprovedStoredSaleSeconds(record, storedStart, storedEnd)
		if err != nil || a != start.Unix() || b != end.Unix() {
			t.Fatalf("reviewed times changed: %d %d %v", a, b, err)
		}
		for _, changed := range []struct {
			name           string
			start, end     time.Time
			requestChanged bool
			deadlineChange bool
		}{
			{name: "start drift", start: storedStart.Add(time.Second), end: storedEnd},
			{name: "end drift", start: storedStart, end: storedEnd.Add(time.Second)},
			{name: "different request", start: storedStart, end: storedEnd, requestChanged: true},
			{name: "different deadline", start: storedStart, end: storedEnd, deadlineChange: true},
		} {
			t.Run(fmt.Sprintf("%d/%s", nanos, changed.name), func(t *testing.T) {
				altered := record
				if changed.requestChanged {
					altered.Request = append(append([]byte(nil), request...), ' ')
				}
				if changed.deadlineChange {
					altered.Identity.CreateDeadline++
				}
				if _, _, err := ApprovedStoredSaleSeconds(altered, changed.start, changed.end); err == nil {
					t.Fatal("unreviewed time accepted")
				}
			})
		}
	}
}
