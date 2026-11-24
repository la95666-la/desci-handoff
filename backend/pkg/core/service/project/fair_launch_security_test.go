package project

import (
	"encoding/json/v2"
	"testing"
	"time"

	"joincare_server/pkg/common/entity"
)

func TestPreviewBoundDigestRejectsWrongAdminExpiryAndBaseDigest(t *testing.T) {
	now := time.Date(2026, 9, 7, 1, 2, 3, 0, time.UTC)
	row := entity.FairLaunchPreview{ID: 9, PreviewKind: "action", ProjectID: 8, Action: "end", Operation: "fair_launch.action.end", AdminID: 10, BaseDigest: hex64('a'), IssuedAt: now, ExpiresAt: now.Add(FairLaunchPreviewTTL)}
	payload := previewBoundPayload(row, map[string]any{"action": "end"})
	encoded, _ := json.Marshal(payload)
	row.CanonicalPayload = string(encoded)
	digest, err := previewBoundDigest(row)
	if err != nil || digest == "" {
		t.Fatal(err)
	}
	row.AdminID++
	payload = previewBoundPayload(row, map[string]any{"action": "end"})
	encoded, _ = json.Marshal(payload)
	row.CanonicalPayload = string(encoded)
	changed, _ := previewBoundDigest(row)
	if changed == digest {
		t.Fatal("wrong admin did not change preview authorization digest")
	}
	row.AdminID--
	row.ExpiresAt = row.ExpiresAt.Add(time.Second)
	payload = previewBoundPayload(row, map[string]any{"action": "end"})
	encoded, _ = json.Marshal(payload)
	row.CanonicalPayload = string(encoded)
	changed, _ = previewBoundDigest(row)
	if changed == digest {
		t.Fatal("changed expiry did not change preview authorization digest")
	}
	row.ExpiresAt = now.Add(FairLaunchPreviewTTL)
	row.BaseDigest = hex64('b')
	payload = previewBoundPayload(row, map[string]any{"action": "end"})
	encoded, _ = json.Marshal(payload)
	row.CanonicalPayload = string(encoded)
	changed, _ = previewBoundDigest(row)
	if changed == digest {
		t.Fatal("changed payload digest did not change preview authorization digest")
	}
}

func TestMFAOptionalIdentityRequiresExactBinding(t *testing.T) {
	a, b, c := uint64(1), uint64(1), uint64(2)
	if !sameOptionalID(&a, &b) || sameOptionalID(&a, &c) || sameOptionalID(&a, nil) || !sameOptionalID(nil, nil) {
		t.Fatal("optional binding comparison is not exact")
	}
}

func TestOpaqueAuthorizationRejectsShortToken(t *testing.T) {
	if _, err := opaqueDigest("client-value"); err == nil {
		t.Fatal("low entropy client token accepted")
	}
	if digest, err := opaqueDigest("0123456789abcdef0123456789abcdef"); err != nil || len(digest) != 64 {
		t.Fatal(digest, err)
	}
}

func TestProductionMFAProviderVerifierFailsClosedAndFreshnessIsBounded(t *testing.T) {
	SetFairLaunchMFAProviderAdmissionVerifier(nil)
	if err := verifyFairLaunchMFAProviderAdmission(entity.FairLaunchMFAProvider{}, time.Now()); err == nil || err.Error() != "mfa_provider_unavailable" {
		t.Fatal(err)
	}
	if fairLaunchMFAMaxFreshness != 15*time.Minute {
		t.Fatal(fairLaunchMFAMaxFreshness)
	}
}

func TestMFAContextBindsAdminApprovalIdentity(t *testing.T) {
	approvalOne, approvalTwo := uint64(21), uint64(22)
	left := FairLaunchMFAContext{AdminID: 3, Operation: "fair_launch.set_onchain_params", Stage: FairLaunchMFAStageSetApproval, ProjectID: 7, AdminApprovalID: &approvalOne}
	right := left
	right.AdminApprovalID = &approvalTwo
	if sameOptionalID(left.AdminApprovalID, right.AdminApprovalID) {
		t.Fatal("MFA approval assertion can be replayed across approval work orders")
	}
}

func TestV3MFAReviewIDIsNotFairLaunchActionID(t *testing.T) {
	first, second := uint64(11), uint64(12)
	old := FairLaunchMFAContext{ActionRequestID: &first}
	v3 := FairLaunchMFAContext{V3PolicyReviewID: &first}
	if sameOptionalID(old.ActionRequestID, v3.ActionRequestID) ||
		sameOptionalID(old.V3PolicyReviewID, v3.V3PolicyReviewID) ||
		sameOptionalID(&first, &second) {
		t.Fatal("M3 MFA reused a Fair Launch action identity")
	}
}
