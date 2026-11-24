package project

import (
	jsonv1 "encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

func immutableActionRow(t *testing.T) *entity.FairLaunchActionRequest {
	t.Helper()
	payload := map[string]any{"action": "cancel", "reason_code": "0x" + hex64('1'), "evidence_hash": "0x" + hex64('2'), "manifest_version": uint32(0)}
	encoded, err := jsonv1.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := canonicalDigest(payload)
	if err != nil {
		t.Fatal(err)
	}
	return &entity.FairLaunchActionRequest{Action: "cancel", CanonicalPayload: string(encoded), CanonicalDigest: stripActionDigest(digest)}
}
func TestImmutableActionEvidenceComesOnlyFromCanonicalPayload(t *testing.T) {
	row := immutableActionRow(t)
	e, err := immutableActionEvidence(row)
	if err != nil || e.ReasonCode != "0x"+hex64('1') {
		t.Fatal(e, err)
	}
	row.CanonicalDigest = hex64('f')
	if _, err := immutableActionEvidence(row); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatal("changed approved payload accepted", err)
	}
}
func TestDispatchRejectsBeforeSubmissionCreation(t *testing.T) {
	if err := DispatchFairLaunchActionTx(nil, nil, projectruntime.Runtime{Enabled: true}, time.Now()); err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("dispatch escaped materialization gate: %v", err)
	}
	if _, err := PrepareFairLaunchOperatorIntentTx(nil, 1, "activate", common.Hash{}, common.Hash{}); err == nil || err.Error() != "fair_launch_materialization_unavailable" {
		t.Fatalf("operator intent escaped materialization gate: %v", err)
	}
}

func TestCreateDispatchIsExplicitlyUnavailable(t *testing.T) {
	if _, _, err := fairLaunchOperatorCalldata(1, "create", FairLaunchActionEvidence{}); err == nil || err.Error() != "create_dispatch_unavailable" {
		t.Fatal(err)
	}
}

func TestCancelAndReleaseRequireCanonicalNonZeroBytes32(t *testing.T) {
	valid := "0x" + hex64('1')
	for _, bad := range []string{"", "0x1", "0x" + hex64('0'), "0X" + hex64('1')} {
		if _, _, err := fairLaunchOperatorCalldata(1, "cancel", FairLaunchActionEvidence{ReasonCode: bad, EvidenceHash: valid}); err == nil {
			t.Fatalf("cancel accepted invalid bytes32 %q", bad)
		}
	}
	if _, _, err := fairLaunchOperatorCalldata(1, "release", FairLaunchActionEvidence{SettlementBlock: "1", AnchorBlock: "1", AnchorHash: valid, ManifestDigest: valid, PolicyDigest: valid, ManifestVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fairLaunchOperatorCalldata(1, "release", FairLaunchActionEvidence{SettlementBlock: "1", AnchorBlock: "1", AnchorHash: valid, ManifestDigest: valid, PolicyDigest: valid}); err == nil {
		t.Fatal("release accepted zero manifest version")
	}
}
