package activation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVerifyArtifactEnvelopeBindsIdentityTimeGenerationAndThreshold(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	want := ArtifactExpectation{
		Environment: "joincare-mainnet", Component: "job",
		ArtifactSHA256: strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("b", 64),
		MinGeneration: 12, Now: now,
	}
	declaration := ArtifactDeclaration{
		Domain: artifactDomain, Environment: want.Environment, Component: want.Component,
		Generation: 12, ArtifactSHA256: want.ArtifactSHA256, ImageID: want.ImageID,
		IssuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	}
	keys := map[string]ed25519.PublicKey{}
	private := map[string]ed25519.PrivateKey{}
	for _, id := range []string{"one", "two", "three"} {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys[id], private[id] = pub, priv
	}
	sign := func(d ArtifactDeclaration) []byte {
		t.Helper()
		body, _ := json.Marshal(d)
		digest := sha256.Sum256(append([]byte(artifactDomain+"\n"), body...))
		envelope := ArtifactEnvelope{Declaration: body}
		for _, id := range []string{"one", "two"} {
			envelope.Signatures = append(envelope.Signatures, ArtifactSignature{
				KeyID: id, Signature: hex.EncodeToString(ed25519.Sign(private[id], digest[:])),
			})
		}
		data, _ := json.Marshal(envelope)
		return data
	}
	valid := sign(declaration)
	if err := VerifyArtifactEnvelope(valid, keys, want); err != nil {
		t.Fatalf("two independent signatures rejected: %v", err)
	}
	if err := VerifyArtifactEnvelope(append([]byte(" "), valid...), keys, want); err == nil {
		t.Fatal("noncanonical envelope admitted")
	}
	duplicate := append(bytes.TrimSuffix(valid, []byte("}")), []byte(`,"declaration":`)...)
	duplicate = append(duplicate, envelopeDeclaration(t, valid)...)
	duplicate = append(duplicate, '}')
	if err := VerifyArtifactEnvelope(duplicate, keys, want); err == nil {
		t.Fatal("duplicate declaration key admitted")
	}
	for name, mutate := range map[string]func(*ArtifactDeclaration){
		"wrong component":   func(d *ArtifactDeclaration) { d.Component = "backend" },
		"wrong digest":      func(d *ArtifactDeclaration) { d.ArtifactSHA256 = strings.Repeat("c", 64) },
		"wrong image":       func(d *ArtifactDeclaration) { d.ImageID = "sha256:" + strings.Repeat("d", 64) },
		"old generation":    func(d *ArtifactDeclaration) { d.Generation-- },
		"expired":           func(d *ArtifactDeclaration) { d.ExpiresAt = now },
		"future issuance":   func(d *ArtifactDeclaration) { d.IssuedAt = now.Add(time.Second) },
		"wrong environment": func(d *ArtifactDeclaration) { d.Environment = "test" },
	} {
		t.Run(name, func(t *testing.T) {
			d := declaration
			mutate(&d)
			if err := VerifyArtifactEnvelope(sign(d), keys, want); err == nil {
				t.Fatal("ineligible signed artifact admitted")
			}
		})
	}
	var envelope ArtifactEnvelope
	if err := json.Unmarshal(valid, &envelope); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ArtifactEnvelope){
		"one signature":  func(e *ArtifactEnvelope) { e.Signatures = e.Signatures[:1] },
		"duplicate key":  func(e *ArtifactEnvelope) { e.Signatures[1] = e.Signatures[0] },
		"unknown key":    func(e *ArtifactEnvelope) { e.Signatures[1].KeyID = "unknown" },
		"tampered bytes": func(e *ArtifactEnvelope) { e.Signatures[0].Signature = strings.Repeat("0", 128) },
		"unknown field": func(e *ArtifactEnvelope) {
			e.Declaration = append(bytes.TrimSuffix(e.Declaration, []byte("}")), []byte(",\"extra\":true}")...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := ArtifactEnvelope{Declaration: append([]byte(nil), envelope.Declaration...), Signatures: append([]ArtifactSignature(nil), envelope.Signatures...)}
			mutate(&e)
			data, _ := json.Marshal(e)
			if err := VerifyArtifactEnvelope(data, keys, want); err == nil {
				t.Fatal("invalid signature or declaration admitted")
			}
		})
	}
	if err := VerifyArtifactEnvelope(valid, nil, want); err == nil {
		t.Fatal("missing pinned keys admitted")
	}
}

func envelopeDeclaration(t *testing.T, encoded []byte) []byte {
	t.Helper()
	var env ArtifactEnvelope
	if err := json.Unmarshal(encoded, &env); err != nil {
		t.Fatal(err)
	}
	return env.Declaration
}
