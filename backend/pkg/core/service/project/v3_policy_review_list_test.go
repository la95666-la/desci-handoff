package project

import (
	"testing"
	"time"
)

func TestV3PolicyReviewListStateDoesNotConflateApprovalAndExecution(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		approved, observed bool
		expiry             time.Time
		want               string
	}{
		{false, false, now.Add(time.Minute), "pending_second_admin"},
		{false, false, now, "expired"},
		{true, false, now.Add(time.Minute), "approved_pending_safe"},
		{true, false, now, "approved_expired"},
		{true, true, now, "observed_not_proof"},
	} {
		if got := v3PolicyReviewListState(tc.approved, tc.observed, tc.expiry, now); got != tc.want {
			t.Fatalf("state(%v,%v,%v) = %s, want %s", tc.approved, tc.observed, tc.expiry, got, tc.want)
		}
	}
}

func TestV3PolicyReviewListMatchesSecondAdminDeadline(t *testing.T) {
	created := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	expires := created.Add(v3PolicyReviewTTL)
	if got := v3PolicyReviewListStateAt(false, false, created, expires, created.Add(23*time.Hour)); got != "pending_second_admin" {
		t.Fatalf("review pending within approval window: %s", got)
	}
	if got := v3PolicyReviewListStateAt(false, false, created, expires, created.Add(24*time.Hour)); got != "expired" {
		t.Fatalf("review remained approvable after 24h: %s", got)
	}
	if got := v3PolicyReviewListStateAt(true, false, created, expires, created.Add(47*time.Hour)); got != "approved_pending_safe" {
		t.Fatalf("approved Safe window closed early: %s", got)
	}
	if got := v3PolicyReviewListStateAt(true, false, created, expires, expires); got != "approved_expired" {
		t.Fatalf("expired Safe window accepted: %s", got)
	}
}

func TestV3PolicyReviewSupersessionIsNotAnExecution(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	created, expires := now.Add(-time.Hour), now.Add(-time.Minute)
	if got := v3PolicyReviewListStateWithSuccessor(true, false, true, created, expires, now); got != "superseded" {
		t.Fatalf("unexecuted old approval: %s", got)
	}
	if got := v3PolicyReviewListStateWithSuccessor(true, true, true, created, expires, now); got != "observed_not_proof" {
		t.Fatalf("execution must not be erased by successor display: %s", got)
	}
	if got := v3PolicyReviewListStateWithSuccessor(false, false, true, created, expires, now); got != "expired" {
		t.Fatalf("unapproved review must not become approved: %s", got)
	}
}
