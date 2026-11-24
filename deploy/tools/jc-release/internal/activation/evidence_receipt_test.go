package activation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvidenceReceiptRejectsForgedMetadataAndStaleEvidence(t *testing.T) {
	claim, want, _, _ := readinessFixture(t)
	base := EvidenceReceipt{Domain: EvidenceReceiptDomain, Kind: "schema", Result: "verified",
		Environment: claim.Environment, AgentID: claim.AgentID, HostKeySHA256: claim.HostKeySHA256,
		ReleaseID: claim.ReleaseID, Generation: claim.Generation, DatabaseServerUUID: claim.DatabaseServerUUID,
		MigrationSHA256: claim.MigrationSHA256, ChainID: claim.ChainID,
		EvidenceReferenceSHA256: strings.Repeat("a", 64), ObservedAt: want.Now.Add(-time.Minute)}
	encode := func(r EvidenceReceipt) []byte {
		t.Helper()
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	if err := VerifyEvidenceReceipt(encode(base), "schema", claim, want.Now); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*EvidenceReceipt){
		"wrong kind":                 func(r *EvidenceReceipt) { r.Kind = "backup" },
		"not verified":               func(r *EvidenceReceipt) { r.Result = "unchecked" },
		"wrong host":                 func(r *EvidenceReceipt) { r.HostKeySHA256 = "SHA256:other" },
		"wrong release":              func(r *EvidenceReceipt) { r.ReleaseID = "other" },
		"wrong generation":           func(r *EvidenceReceipt) { r.Generation++ },
		"wrong db":                   func(r *EvidenceReceipt) { r.DatabaseServerUUID = "00000000-0000-0000-0000-000000000000" },
		"wrong migration":            func(r *EvidenceReceipt) { r.MigrationSHA256 = strings.Repeat("0", 64) },
		"wrong chain":                func(r *EvidenceReceipt) { r.ChainID = 31337 },
		"missing evidence reference": func(r *EvidenceReceipt) { r.EvidenceReferenceSHA256 = "" },
		"stale observation":          func(r *EvidenceReceipt) { r.ObservedAt = want.Now.Add(-25 * time.Hour) },
		"future observation":         func(r *EvidenceReceipt) { r.ObservedAt = want.Now.Add(time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			change(&r)
			if VerifyEvidenceReceipt(encode(r), "schema", claim, want.Now) == nil {
				t.Fatal("ineligible receipt accepted")
			}
		})
	}
	if VerifyEvidenceReceipt(append([]byte(" "), encode(base)...), "schema", claim, want.Now) == nil {
		t.Fatal("noncanonical receipt accepted")
	}
	if VerifyEvidenceReceipt(append(encode(base)[:len(encode(base))-1], []byte(`,"unknown":1}`)...), "schema", claim, want.Now) == nil {
		t.Fatal("unknown field accepted")
	}
}
