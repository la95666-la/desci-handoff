package idov3whitelist

import (
	"strings"
	"testing"
)

func TestApprovedSetDigestBindsProjectApprovalAndCall(t *testing.T) {
	addr := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	identity := ApprovedSetIdentity{ProjectID: 19, ChainID: 56, Proxy: addr("1"), ProjectToken: addr("2"),
		TokenDecimals: 6, PaymentToken: addr("3"), PaymentDecimals: 18, ProjectOwner: addr("4"),
		FeeReceiver: addr("5"), FeeRate: 100, Operator: addr("6"), RequestDigest: hash("a"),
		CreateCallHash: hash("b"), CreateDeadline: 2000000000, ApprovalID: 7, InitiatorID: 8, ApproverID: 9}
	baseline, err := DigestApprovedSetIdentity(identity)
	if err != nil || baseline == ([32]byte{}) {
		t.Fatalf("identity rejected: %v", err)
	}
	evidence := ApprovedSetEvidence{ApprovalID: identity.ApprovalID, InitiatorID: identity.InitiatorID,
		ApproverID: identity.ApproverID, ApprovalStatus: "executed", RequestDigest: identity.RequestDigest,
		IdentityDigest: baseline.Hex(), CreateCallHash: identity.CreateCallHash}
	if err := VerifyApprovedSetEvidence(identity, evidence); err != nil {
		t.Fatal(err)
	}
	// This alone does not read the DB or authorize Set. A different project's
	// approval must not be accepted even when the request digest matches.
	other := identity
	other.ProjectID++
	if err := VerifyApprovedSetEvidence(other, evidence); err == nil {
		t.Fatal("foreign project borrowed approval evidence")
	}
	for _, change := range []func(*ApprovedSetEvidence){
		func(v *ApprovedSetEvidence) { v.ApprovalStatus = "pending" },
		func(v *ApprovedSetEvidence) { v.ApproverID = v.InitiatorID },
		func(v *ApprovedSetEvidence) { v.IdentityDigest = hash("c") },
		func(v *ApprovedSetEvidence) { v.CreateCallHash = hash("c") },
	} {
		item := evidence
		change(&item)
		if err := VerifyApprovedSetEvidence(identity, item); err == nil {
			t.Fatal("unexecuted or altered evidence accepted")
		}
	}
	for _, mutate := range []func(*ApprovedSetIdentity){
		func(i *ApprovedSetIdentity) { i.ProjectID++ },
		func(i *ApprovedSetIdentity) { i.ApprovalID++ },
		func(i *ApprovedSetIdentity) { i.RequestDigest = hash("c") },
		func(i *ApprovedSetIdentity) { i.CreateCallHash = hash("c") },
		func(i *ApprovedSetIdentity) { i.TokenDecimals = 18 },
		func(i *ApprovedSetIdentity) { i.CreateDeadline++ },
	} {
		candidate := identity
		mutate(&candidate)
		got, err := DigestApprovedSetIdentity(candidate)
		if err != nil || got == baseline {
			t.Fatal("project/approval/terms not bound")
		}
	}
	for _, mutate := range []func(*ApprovedSetIdentity){
		func(i *ApprovedSetIdentity) { i.ApproverID = i.InitiatorID },
		func(i *ApprovedSetIdentity) { i.ProjectToken = i.PaymentToken },
		func(i *ApprovedSetIdentity) { i.RequestDigest = "0x" + strings.Repeat("g", 64) },
		func(i *ApprovedSetIdentity) { i.Proxy = strings.ToUpper(i.Proxy) },
		func(i *ApprovedSetIdentity) { i.CreateDeadline = 0 },
	} {
		candidate := identity
		mutate(&candidate)
		if _, err := DigestApprovedSetIdentity(candidate); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}
