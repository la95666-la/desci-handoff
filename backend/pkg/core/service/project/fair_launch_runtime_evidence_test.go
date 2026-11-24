package project

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

func registryFixture() (fairLaunchRegistryBinding, entity.ChainContractSource, entity.ChainContractSourceRevision, projectruntime.Runtime) {
	r := projectruntime.Runtime{Enabled: true, ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ImplementationAddress: "0x2000000000000000000000000000000000000002", ImplementationCodeHash: "0x" + hex64('a')}
	s := entity.ChainContractSource{ID: 7, ChainID: 56, ContractAddress: r.ContractAddress}
	rev := entity.ChainContractSourceRevision{ID: 8, SourceID: 7, RevisionNo: 9, ImplementationAddress: r.ImplementationAddress, ImplementationCodeHash: r.ImplementationCodeHash, ABIDigest: "0x" + hex64('b')}
	b := fairLaunchRegistryBinding{Environment: "test", Revision: 10, ChainID: 56, ProxyAddress: r.ContractAddress, ImplementationAddress: r.ImplementationAddress, ImplementationCodeHash: r.ImplementationCodeHash, ABIDigest: rev.ABIDigest, SourceID: 7, SourceRevisionID: 8, SourceRevisionNo: 9, RuntimeGeneration: 11, PolicyGeneration: 12, SourceRegistryGeneration: 13, ConfigGeneration: 14}
	canonical, _ := canonicalRegistryPayload(b)
	digest := sha256.Sum256(canonical)
	b.CanonicalPayload = string(canonical)
	b.PayloadDigest = hex.EncodeToString(digest[:])
	return b, s, rev, r
}

func TestPositionTokenIdentityRejectsAddressOrDecimalDrift(t *testing.T) {
	project := entity.Project{PaymentTokenAddress: "0x1000000000000000000000000000000000000001", PaymentTokenDecimals: 18, TokenAddress: "0x2000000000000000000000000000000000000002", TokenDecimals: 6}
	position := entity.FairLaunchPosition{PaymentTokenAddress: project.PaymentTokenAddress, PaymentTokenDecimals: 18, ProjectTokenAddress: project.TokenAddress, ProjectTokenDecimals: 6}
	if !fairLaunchPositionTokenIdentityMatches(position, project) {
		t.Fatal("exact identity rejected")
	}
	position.PaymentTokenDecimals = 6
	if fairLaunchPositionTokenIdentityMatches(position, project) {
		t.Fatal("decimal mismatch accepted")
	}
	position.PaymentTokenDecimals = 18
	position.ProjectTokenAddress = "0x3000000000000000000000000000000000000003"
	if fairLaunchPositionTokenIdentityMatches(position, project) {
		t.Fatal("address mismatch accepted")
	}
}

func TestRegistryBindingAcceptsPrefixedSnapshotForUnprefixedRevision(t *testing.T) {
	b, s, rev, r := registryFixture()
	rev.ImplementationCodeHash = strings.TrimPrefix(rev.ImplementationCodeHash, "0x")
	rev.ABIDigest = strings.TrimPrefix(rev.ABIDigest, "0x")
	if err := validateRegistryBinding(b, s, rev, r, "test"); err != nil {
		t.Fatalf("prefixed snapshot did not match unprefixed revision: %v", err)
	}
	proxy, implementation, codeHash, abi := registrySnapshotLookupIdentity(r, rev)
	if proxy != strings.ToLower(r.ContractAddress) || implementation != strings.ToLower(rev.ImplementationAddress) || codeHash != b.ImplementationCodeHash || abi != b.ABIDigest {
		t.Fatalf("unexpected snapshot lookup normalization: %q %q %q %q", proxy, implementation, codeHash, abi)
	}
}

func TestRegistryBindingRejectsPrefixedSnapshotHashMismatch(t *testing.T) {
	b, s, rev, r := registryFixture()
	rev.ImplementationCodeHash = strings.TrimPrefix(rev.ImplementationCodeHash, "0x")
	rev.ABIDigest = strings.TrimPrefix(rev.ABIDigest, "0x")
	b.ImplementationCodeHash = "0x" + hex64('c')
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "runtime/ABI") {
		t.Fatalf("prefixed snapshot mismatch accepted: %v", err)
	}
}

func TestStoreSourceSnapshotIdentityAcceptsPrefixedSnapshotForUnprefixedRevision(t *testing.T) {
	b, _, rev, r := registryFixture()
	rev.ImplementationCodeHash = strings.TrimPrefix(rev.ImplementationCodeHash, "0x")
	rev.ABIDigest = strings.TrimPrefix(rev.ABIDigest, "0x")
	trusted := projectruntime.TrustedSource{Identity: projectruntime.SourceIdentity{ChainID: r.ChainID, ContractAddress: r.ContractAddress, ContractVersion: "fair_launch_v1", FundingMode: "fair_launch", Enabled: true}, Revision: projectruntime.SourceRevision{RevisionNo: rev.RevisionNo, EffectiveFromBlock: 1, ActivationBlockHash: "0x" + hex64('d'), ImplementationAddress: rev.ImplementationAddress, ImplementationCodeHash: rev.ImplementationCodeHash, ABIVersion: "fair-launch-v1", ABIDigest: rev.ABIDigest}}
	row := entity.FairLaunchSourceRegistrySnapshot{Environment: b.Environment, Revision: b.Revision, PayloadDigest: b.PayloadDigest, KeySetVersion: 1, ChainID: b.ChainID, ProxyAddress: b.ProxyAddress, ImplementationAddress: b.ImplementationAddress, ImplementationCodeHash: b.ImplementationCodeHash, ABIDigest: b.ABIDigest, SourceID: b.SourceID, SourceRevisionID: b.SourceRevisionID, SourceRevisionNo: b.SourceRevisionNo, RuntimeGeneration: b.RuntimeGeneration, PolicyGeneration: b.PolicyGeneration, SourceRegistryGeneration: b.SourceRegistryGeneration, ConfigGeneration: b.ConfigGeneration}
	if !fairLaunchSourceSnapshotIdentityMatches(row, trusted) {
		t.Fatal("prefixed snapshot did not match unprefixed trusted revision")
	}
	row.ImplementationCodeHash = "0x" + hex64('e')
	if fairLaunchSourceSnapshotIdentityMatches(row, trusted) {
		t.Fatal("mismatched snapshot identity accepted")
	}
}

func TestRegistryJCSUsesLexicographicOrderAndRejectsEquivalentNonCanonicalText(t *testing.T) {
	b, s, rev, r := registryFixture()
	canonical, err := canonicalRegistryPayload(b)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := `{"abi_digest":`
	if !strings.HasPrefix(string(canonical), wantPrefix) {
		t.Fatalf("not JCS key order: %s", canonical)
	}
	b.CanonicalPayload = `{"environment":"test","revision":10,"chain_id":56,"proxy_address":"` + b.ProxyAddress + `","implementation_address":"` + b.ImplementationAddress + `","implementation_code_hash":"` + b.ImplementationCodeHash + `","abi_digest":"` + b.ABIDigest + `","source_id":7,"source_revision_id":8,"source_revision_no":9,"runtime_generation":11,"policy_generation":12,"source_registry_generation":13,"config_generation":14}`
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "RFC 8785") {
		t.Fatalf("equivalent noncanonical JSON accepted: %v", err)
	}
}

func TestRegistryJCSRejectsIllegalJSONType(t *testing.T) {
	b, _, _, _ := registryFixture()
	canonical, err := canonicalRegistryPayload(b)
	if err != nil {
		t.Fatal(err)
	}
	illegal := strings.Replace(string(canonical), `"chain_id":56`, `"chain_id":"56"`, 1)
	if _, err := validateCanonicalRegistryText(illegal, b); err == nil {
		t.Fatal("illegal JCS field type accepted")
	}
	if _, err := validateCanonicalRegistryText(string(canonical)+` {}`, b); err == nil {
		t.Fatal("trailing JSON value accepted")
	}
}

func TestRegistryJCSRejectsUnsafeIntegerAndNonCanonicalEnvironment(t *testing.T) {
	b, _, _, _ := registryFixture()
	b.RuntimeGeneration = jcsMaxSafeInteger + 1
	if _, err := canonicalRegistryPayload(b); err == nil {
		t.Fatal("unsafe JCS integer accepted")
	}
	b, _, _, _ = registryFixture()
	b.Environment = "te\nst"
	if _, err := canonicalRegistryPayload(b); err == nil {
		t.Fatal("noncanonical string accepted")
	}
}

func TestRegistryBindingRejectsUnrelatedSnapshot(t *testing.T) {
	b, s, rev, r := registryFixture()
	b.ImplementationAddress = "0x3000000000000000000000000000000000000003"
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "runtime/ABI") {
		t.Fatalf("unrelated snapshot accepted: %v", err)
	}
}

func TestRegistryBindingRejectsDigestMismatch(t *testing.T) {
	b, s, rev, r := registryFixture()
	b.PayloadDigest = hex64('f')
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("digest mismatch accepted: %v", err)
	}
}

func TestRegistryBindingRejectsMissingBindingAndStaleGeneration(t *testing.T) {
	b, s, rev, r := registryFixture()
	b.SourceRevisionID = 0
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("missing binding accepted: %v", err)
	}
	b, s, rev, r = registryFixture()
	b.ConfigGeneration = 0
	if err := validateRegistryBinding(b, s, rev, r, "test"); err == nil || !strings.Contains(err.Error(), "generations") {
		t.Fatalf("stale generation accepted: %v", err)
	}
}
