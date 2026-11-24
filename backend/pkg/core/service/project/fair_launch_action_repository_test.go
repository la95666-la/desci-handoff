package project

import (
	"joincare_server/pkg/common/projectruntime"
	"testing"
	"time"
)

func TestFairLaunchActionSubmissionRejectsBeforeDatabaseOrMFA(t *testing.T) {
	_, err := CreateFairLaunchActionRequestTx(nil, CreateFairLaunchActionRequestInput{ProjectID: 1, Action: "end", InitiatorAdminID: 1, MFAAssertionToken: "would-be-consumed", Runtime: projectruntime.Runtime{Enabled: true}, Now: time.Now()})
	if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("submission escaped materialization gate: %v", err)
	}
}
func TestExecuteFairLaunchActionRequestRejectsBeforeDatabaseOrMFA(t *testing.T) {
	_, err := ExecuteFairLaunchActionRequestTx(nil, ExecuteFairLaunchActionRequestInput{RequestID: 1, ProjectID: 1, ExecutorAdminID: 3, MFAAssertionToken: "would-be-consumed", Runtime: projectruntime.Runtime{Enabled: true}, Now: time.Now()})
	if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("execution escaped materialization gate: %v", err)
	}
}
func TestApproveFairLaunchActionRequestRejectsBeforeDatabaseOrMFA(t *testing.T) {
	err := ApproveFairLaunchActionRequestTx(nil, 1, 2, 3, "would-be-consumed", projectruntime.Runtime{Enabled: true}, time.Now())
	if err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("approval escaped materialization gate: %v", err)
	}
}

func TestPersistedUnsupportedActionIsRejectedBeforeApprovalOrExecutionMFA(t *testing.T) {
	cases := []struct{ action, domain, reason string }{
		{"create", "operator_durable_outbox", "fair_launch_materialization_unavailable"},
		{"upgrade_proxy", "governance_timelock", "fair_launch_materialization_unavailable"},
		{"pause_claims", "pause_guardian_safe", "fair_launch_materialization_unavailable"},
		{"unknown_old_action", "operator_durable_outbox", "fair_launch_materialization_unavailable"},
		{"end", "governance_timelock", "fair_launch_materialization_unavailable"},
	}
	for _, tc := range cases {
		if got := fairLaunchActionDisabledReason(tc.action, tc.domain); got != tc.reason {
			t.Fatalf("%s/%s: expected %s, got %s", tc.action, tc.domain, tc.reason, got)
		}
	}
	if got := fairLaunchActionDisabledReason("end", "operator_durable_outbox"); got != "fair_launch_materialization_unavailable" {
		t.Fatalf("durable operator request escaped materialization gate: %s", got)
	}
}

func TestFairLaunchActionBindingStatesAreExplicitAndNonDispatch(t *testing.T) {
	if fairLaunchStatusPending != "pending_approval" || fairLaunchStatusApprovalBound != "approval_bound" || fairLaunchStatusApproved != "approved" || fairLaunchStatusExecutionBound != "execution_bound" || fairLaunchStatusExecuting != "executing" {
		t.Fatal("durable action binding state machine drifted")
	}
	if fairLaunchStatusApprovalBound == fairLaunchStatusExecuting || fairLaunchStatusApproved == fairLaunchStatusExecuting {
		t.Fatal("approval states must never imply dispatch")
	}
}

func TestVerifiedRuntimeCannotEnableAnyFairLaunchAction(t *testing.T) {
	runtime := projectruntime.Runtime{Enabled: true, Version: "fair_launch_v1", FundingMode: "fair_launch", ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ImplementationAddress: "0x2000000000000000000000000000000000000002", ImplementationCodeHash: "0x" + hex64('5')}
	evidence := FairLaunchRuntimeEvidence{Runtime: FairLaunchPublicRuntimeIdentity{ChainID: runtime.ChainID, ProxyAddress: runtime.ContractAddress, ImplementationAddress: runtime.ImplementationAddress, ImplementationCodeHash: runtime.ImplementationCodeHash, SourceRegistryRevision: 1, SourceRegistryDigest: "digest", ObservedBlockNumber: "1", ObservedBlockHash: "hash", ObservedAt: time.Now().UTC().Format(time.RFC3339), RuntimeVerified: true}}
	for action, descriptor := range BuildFairLaunchCapabilities(1, runtime, evidence, nil).Actions {
		if descriptor.Enabled || descriptor.ReasonCode != "fair_launch_materialization_unavailable" {
			t.Fatalf("verified runtime enabled %s: %+v", action, descriptor)
		}
	}
}
