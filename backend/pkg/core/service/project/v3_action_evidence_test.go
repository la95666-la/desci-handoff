package project

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestContinuationCannotCreateOrSignAnAttempt(t *testing.T) {
	if _, e := ContinueApprovedV3Original(context.Background(), 74, "0x"+strings.Repeat("a", 64), true, nil); e == nil {
		t.Fatal("missing execute guard")
	}
	raw, e := os.ReadFile("v3_original_continuation.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	for _, forbidden := range []string{"PrepareV3Once(", "PrepareApprovedV3Once(", "ReserveV3", "PrivateKey", "SignTx(", "SendTransaction("} {
		if strings.Contains(s, forbidden) {
			t.Fatal("continuation acquired signing authority", forbidden)
		}
	}
	for _, required := range []string{"PreviewV3PersistedFirstBroadcast", "checkOriginalWithIndependentRPC", "BroadcastV3PersistedOnce", "guard(tx)", "checkApprovedV3SignedContinuation", "RecheckPolicyObservationCandidate"} {
		if !strings.Contains(s, required) {
			t.Fatal("continuation fence missing", required)
		}
	}
}

func TestPolicySpecificEvidenceAndSettlementFencesRetained(t *testing.T) {
	for name, checks := range map[string][]string{
		"v3_policy_preview.go":            {"verifyApprovedV3PriorPolicy", "v3PolicyPriorMembersTx", "latestPolicy.Policy != policy.Policy", "PackPolicyChangeCall"},
		"v3_policy_review.go":             {"v3VerifyPriorReceipt", "verifyApprovedV3PriorPolicy", "v3PolicyMemberDiff", "HasLivePermissionTx", "LockNoActiveIncidentTx"},
		"v3_approved_policy_execution.go": {"ReconcilePolicyEvidence", "CheckSafePolicyExecution", "checkApprovedV3ObservationCurrentPolicy", "repeat_receipt", "LockNoActiveIncidentTx"},
		"v3_original_continuation.go":     {"ReadApprovedSetRecord", "previewApprovedV3Lifecycle", "RecheckApprovedLifecycleCreation"},
	} {
		raw, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		for _, check := range checks {
			if !strings.Contains(string(raw), check) {
				t.Fatal(name, check)
			}
		}
	}
	// We did not relax amount obligations or replace complete evidence by a
	// method-specific policy/transport admission.
	raw, e := os.ReadFile("../../../common/projecttracking/closure.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, check := range []string{"user_claims_pending", "user_refunds_pending", "platform_fee_pending", "project_tokens_pending"} {
		if !strings.Contains(string(raw), check) {
			t.Fatal(check)
		}
	}
}
