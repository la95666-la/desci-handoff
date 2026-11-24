package activation

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func readinessFixture(t *testing.T) (Readiness, ReadinessExpectation, map[string]ed25519.PublicKey, map[string]ed25519.PrivateKey) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	claim := Readiness{
		Domain: ReadinessDomain, Environment: "mainnet", AgentID: "app-agent", Root: "/opt/joincare-prod",
		HostKeySHA256: "SHA256:independently-confirmed-host-key", ReleaseSHA256: strings.Repeat("a", 64),
		ArtifactImageID: "sha256:" + strings.Repeat("9", 64),
		ReleaseID:       "release-1", Generation: 8, MigrationSHA256: SigningMetadataMigrationSHA256,
		DatabaseServerUUID:   "78dbed8b-77a9-11f1-ac1c-06eab0296b63",
		SchemaEvidenceSHA256: strings.Repeat("b", 64), BackupReceiptSHA256: strings.Repeat("c", 64),
		RestoreEvidenceSHA256: strings.Repeat("d", 64), SignerEvidenceSHA256: strings.Repeat("e", 64),
		ChainRPCEvidenceSHA256: strings.Repeat("f", 64), PermissionsEvidenceSHA256: strings.Repeat("1", 64),
		CoreOperatorAddress: "0x" + strings.Repeat("2", 40), JobSignerAddress: "0x" + strings.Repeat("3", 40),
		V2ContractAddress: "0x" + strings.Repeat("4", 40),
		ChainID:           56, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	want := ReadinessExpectation{Environment: claim.Environment, AgentID: claim.AgentID, Root: claim.Root,
		HostKeySHA256: claim.HostKeySHA256, ReleaseSHA256: claim.ReleaseSHA256,
		ArtifactImageID: claim.ArtifactImageID,
		ReleaseID:       claim.ReleaseID, Generation: claim.Generation, Now: now}
	keys := map[string]ed25519.PublicKey{}
	private := map[string]ed25519.PrivateKey{}
	for _, name := range []string{"one", "two", "three"} {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys[name], private[name] = pub, priv
	}
	return claim, want, keys, private
}

func signReadinessFixture(t *testing.T, claim Readiness, private map[string]ed25519.PrivateKey) []byte {
	t.Helper()
	declaration, err := json.Marshal(claim)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(append([]byte(ReadinessDomain+"\n"), declaration...))
	envelope := ArtifactEnvelope{Declaration: declaration}
	for _, name := range []string{"one", "two"} {
		envelope.Signatures = append(envelope.Signatures, ArtifactSignature{KeyID: name, Signature: hex.EncodeToString(ed25519.Sign(private[name], digest[:]))})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReadinessBindsProductionEvidenceAndIndependentSignatures(t *testing.T) {
	claim, want, keys, private := readinessFixture(t)
	valid := signReadinessFixture(t, claim, private)
	if err := VerifyReadiness(valid, keys, want); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Readiness){
		"wrong environment":    func(r *Readiness) { r.Environment = "test" },
		"wrong agent":          func(r *Readiness) { r.AgentID = "control-agent" },
		"wrong root":           func(r *Readiness) { r.Root = "/opt/joincare-test" },
		"wrong host":           func(r *Readiness) { r.HostKeySHA256 = "SHA256:other" },
		"wrong release digest": func(r *Readiness) { r.ReleaseSHA256 = strings.Repeat("0", 64) },
		"wrong image":          func(r *Readiness) { r.ArtifactImageID = "sha256:" + strings.Repeat("8", 64) },
		"wrong release ID":     func(r *Readiness) { r.ReleaseID = "another" },
		"wrong generation":     func(r *Readiness) { r.Generation-- },
		"wrong migration":      func(r *Readiness) { r.MigrationSHA256 = strings.Repeat("0", 64) },
		"missing schema":       func(r *Readiness) { r.SchemaEvidenceSHA256 = "" },
		"missing backup":       func(r *Readiness) { r.BackupReceiptSHA256 = "" },
		"missing restore":      func(r *Readiness) { r.RestoreEvidenceSHA256 = "" },
		"missing signer":       func(r *Readiness) { r.SignerEvidenceSHA256 = "" },
		"missing RPC":          func(r *Readiness) { r.ChainRPCEvidenceSHA256 = "" },
		"missing permissions":  func(r *Readiness) { r.PermissionsEvidenceSHA256 = "" },
		"wrong chain":          func(r *Readiness) { r.ChainID = 31337 },
		"invalid UUID":         func(r *Readiness) { r.DatabaseServerUUID = "unknown" },
		"same signer":          func(r *Readiness) { r.JobSignerAddress = r.CoreOperatorAddress },
		"missing contract":     func(r *Readiness) { r.V2ContractAddress = "" },
		"future":               func(r *Readiness) { r.IssuedAt = want.Now.Add(time.Second) },
		"stale":                func(r *Readiness) { r.IssuedAt = want.Now.Add(-time.Hour) },
		"expired":              func(r *Readiness) { r.ExpiresAt = want.Now },
		"overlong validity":    func(r *Readiness) { r.ExpiresAt = r.IssuedAt.Add(time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			r := claim
			change(&r)
			if err := VerifyReadiness(signReadinessFixture(t, r, private), keys, want); err == nil {
				t.Fatal("invalid readiness admitted")
			}
		})
	}
	for name, data := range map[string][]byte{
		"noncanonical outer": append([]byte(" "), valid...),
		"extra outer key":    append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"unknown":1}`)...),
		"tampered signature": bytesReplaceFirstSignature(t, valid),
	} {
		t.Run(name, func(t *testing.T) {
			if err := VerifyReadiness(data, keys, want); err == nil {
				t.Fatal("invalid envelope admitted")
			}
		})
	}
	if err := VerifyReadiness(valid, nil, want); err == nil {
		t.Fatal("absent trust admitted")
	}
	duplicate := map[string]ed25519.PublicKey{"one": keys["one"], "two": keys["one"], "three": keys["three"]}
	if err := VerifyReadiness(valid, duplicate, want); err == nil {
		t.Fatal("duplicate key admitted")
	}
}

func bytesReplaceFirstSignature(t *testing.T, data []byte) []byte {
	t.Helper()
	var env ArtifactEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	env.Signatures[0].Signature = strings.Repeat("0", 128)
	changed, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return changed
}
