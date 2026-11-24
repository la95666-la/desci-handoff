package project

import (
	"context"
	"testing"

	"github.com/spf13/viper"
)

func TestApprovedPolicySafeProposalRequiresReviewedHandoff(t *testing.T) {
	old := viper.Get("idoV3PolicyReview.enabled")
	t.Cleanup(func() { viper.Set("idoV3PolicyReview.enabled", old) })
	viper.Set("idoV3PolicyReview.enabled", false)
	if _, err := ApprovedV3PolicySafeProposal(19, 7, 8, nil); err == nil {
		t.Fatal("unreviewed policy reached Safe proposal")
	}
}

func TestApprovedPolicySafeProposalRejectsCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ApprovedV3PolicySafeProposalContext(ctx, 19, 7, 8, nil); err == nil {
		t.Fatal("canceled request reached Safe proposal")
	}
	if _, err := ReadApprovedV3PolicySafeHandoffContext(ctx, 19, 7, 8); err == nil {
		t.Fatal("canceled request reached Safe handoff")
	}
}
