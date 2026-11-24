package project

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedV3PreviewBudgetAllowsSlowReadWithoutRelaxingReview(t *testing.T) {
	for _, tc := range []struct {
		approved, requireFreshScan bool
		want                       time.Duration
	}{
		{true, false, 120 * time.Second},
		{true, true, 75 * time.Second},
		{false, false, 12 * time.Second},
		{false, true, 12 * time.Second},
	} {
		if got := v3PolicyPreviewRPCBudget(tc.approved, tc.requireFreshScan); got != tc.want {
			t.Fatalf("preview budget (%v,%v)=%s want %s", tc.approved, tc.requireFreshScan, got, tc.want)
		}
	}
}

func TestApprovedV3CandidatePreviewDoesNotGrantReviewFreshness(t *testing.T) {
	// Computing a read-only candidate is allowed while a global scan is
	// checking; review-only and strict freshness still reject missing evidence.
	if err := requireV3PolicyPreviewScan(context.Background(), nil, false); err != nil {
		t.Fatalf("read-only candidate requires global freshness: %v", err)
	}
	if err := requireV3PolicyPreviewScan(context.Background(), nil, true); err == nil {
		t.Fatal("review recomputation bypassed scanner freshness")
	}
}

func TestApprovedFirstEnableReviewBaseWindowIsBounded(t *testing.T) {
	for _, tc := range []struct {
		approved, review, enabled bool
		want                      uint64
	}{
		{true, true, true, 1800},
		{true, false, true, 120},
		{true, true, false, 120},
		{false, true, true, 120},
	} {
		if got := v3PolicyReviewBaseMaxAge(tc.approved, tc.review, tc.enabled); got != tc.want {
			t.Fatalf("base max age=%d want=%d", got, tc.want)
		}
	}
}

func TestFirstEnableReviewScanFenceCannotServeProof(t *testing.T) {
	if requireV3ReviewOrStrictScan(context.Background(), nil, true) == nil ||
		requireV3ReviewOrStrictScan(context.Background(), nil, false) == nil {
		t.Fatal("missing scanner evidence admitted")
	}
	if !approvedV3UnscannedInitialCandidate(idov3whitelist.ProjectPending, idov3whitelist.Policy{}, true) ||
		approvedV3UnscannedInitialCandidate(idov3whitelist.ProjectActive, idov3whitelist.Policy{}, true) {
		t.Fatal("review scope broadened beyond first Pending/public/v0 enable")
	}
}

func TestApprovedV3PreviewRejectsCanceledRequestBeforeAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PreviewApprovedV3PolicyChangeContext(ctx, 19,
		[]string{"0x1111111111111111111111111111111111111111"}, true, "review"); err == nil {
		t.Fatal("canceled Admin preview reached admission")
	}
}

func TestApprovedV3CandidateCannotReuseReviewScope(t *testing.T) {
	// The initial candidate has no base block. A submitted review supplies the
	// exact pinned block/hash and must take the fresh-scan path, never the
	// relaxed read-only entry point.
	if !approvedV3PolicyPreviewScope(false, 0) || !approvedV3PolicyPreviewScope(true, 128805696) ||
		approvedV3PolicyPreviewScope(false, 128805696) || approvedV3PolicyPreviewScope(true, 0) {
		t.Fatal("approved V3 candidate/review scope fence changed")
	}
}

func TestApprovedV3UnscannedCandidateOnlyFirstPendingEnable(t *testing.T) {
	initial := idov3whitelist.Policy{}
	if !approvedV3UnscannedInitialCandidate(idov3whitelist.ProjectPending, initial, true) {
		t.Fatal("first Pending/public/v0 enable refused")
	}
	for _, tc := range []struct {
		state   idov3whitelist.ProjectState
		policy  idov3whitelist.Policy
		enabled bool
	}{
		{idov3whitelist.ProjectPending, initial, false},
		{idov3whitelist.ProjectActive, initial, true},
		{idov3whitelist.ProjectPending, idov3whitelist.Policy{Version: 1}, true},
		{idov3whitelist.ProjectPending, idov3whitelist.Policy{Enabled: true, Root: common.HexToHash("0x01"), Version: 1}, true},
		{idov3whitelist.ProjectPending, idov3whitelist.Policy{Root: common.HexToHash("0x01")}, true},
	} {
		if approvedV3UnscannedInitialCandidate(tc.state, tc.policy, tc.enabled) {
			t.Fatalf("noninitial or disabled candidate bypassed full scan: %+v", tc)
		}
	}
}

func TestApprovedV3UnscannedCandidateMayObserveAdvancingHead(t *testing.T) {
	for _, tc := range []struct {
		verified, observed uint64
		candidate          bool
		want               bool
	}{
		{100, 100, true, true}, {100, 102, true, true},
		{100, 99, true, false}, {100, 102, false, true},
		{100, 100, false, true},
	} {
		if got := v3PolicyPreviewHeadAdmitted(tc.verified, tc.observed, tc.candidate); got != tc.want {
			t.Fatalf("head check (%d,%d,%v)=%v want %v", tc.verified, tc.observed, tc.candidate, got, tc.want)
		}
	}
}

// Candidate policy entry points must remain closed when their independent
// review/execution gates are off. A four-eyes Set is not a Safe execution or
// wallet proof grant by itself.
func TestApprovedV3PolicyCandidatesFailClosedWithoutIndependentGates(t *testing.T) {
	keys := []string{"idoV3PolicyPreview.enabled", "idoV3PolicyReview.enabled", "idoV3PolicyExecution.enabled"}
	for _, key := range keys {
		name, old := key, viper.Get(key)
		t.Cleanup(func() { viper.Set(name, old) })
		viper.Set(key, false)
	}
	wallets := []string{"0x1111111111111111111111111111111111111111"}
	if _, err := PreviewApprovedV3PolicyChange(19, wallets, true, "review"); err == nil {
		t.Fatal("disabled policy preview accepted")
	}
	if id, err := StartApprovedV3PolicyReview(V3PolicyReviewInput{ProjectID: 19, AdminID: 7,
		Wallets: wallets, Enabled: true, Reason: "review", PreviewDigest: "0x" + strings.Repeat("a", 64)}); err == nil || id != 0 {
		t.Fatal("disabled policy review created")
	}
	if err := ApproveApprovedV3PolicyReview(19, 1, 8, "0x"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("disabled policy review approved")
	}
	if err := RecordApprovedV3PolicyExecution(19, 1, "0x"+strings.Repeat("b", 64),
		"0x"+strings.Repeat("c", 64)); err == nil {
		t.Fatal("disabled policy receipt recorded")
	}
	if _, err := ReadApprovedV3PolicySafeHandoff(19, 1, 8); err == nil {
		t.Fatal("disabled policy handoff served")
	}
}
