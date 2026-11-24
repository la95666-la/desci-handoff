package activation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// ArtifactDeclaration is the narrowly scoped signed input to Job admission.
// It is not a publication decision: production trusted keys and runtime
// eligibility must still be supplied and reviewed independently.
type ArtifactDeclaration struct {
	Domain         string    `json:"domain"`
	Environment    string    `json:"environment"`
	Component      string    `json:"component"`
	Generation     uint64    `json:"generation"`
	ArtifactSHA256 string    `json:"artifact_sha256"`
	ImageID        string    `json:"image_id"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type ArtifactSignature struct {
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type ArtifactEnvelope struct {
	Declaration json.RawMessage     `json:"declaration"`
	Signatures  []ArtifactSignature `json:"signatures"`
}

type ArtifactExpectation struct {
	Environment    string
	Component      string
	ArtifactSHA256 string
	ImageID        string
	MinGeneration  uint64
	Now            time.Time
}

const artifactDomain = "joincare.release.artifact.v1"

// VerifyArtifactEnvelope verifies exact canonical declaration bytes and a
// threshold of distinct, explicitly supplied public keys. It has no default
// keys: absent trust configuration fails closed. The caller must also bind
// expected image ID to the *loaded* image before creating a container.
func VerifyArtifactEnvelope(data []byte, pinned map[string]ed25519.PublicKey, want ArtifactExpectation) error {
	if len(data) == 0 || len(data) > 64*1024 || len(pinned) < 3 ||
		want.Now.IsZero() || want.Environment == "" || want.Component == "" ||
		want.ArtifactSHA256 == "" || want.ImageID == "" || want.MinGeneration == 0 {
		return errors.New("artifact_admission_unavailable")
	}
	var env ArtifactEnvelope
	if err := decodeExactJSON(data, &env); err != nil || len(env.Signatures) < 2 || len(env.Signatures) > len(pinned) {
		return errors.New("artifact_envelope_invalid")
	}
	// A duplicate envelope key can make different JSON consumers disagree
	// about which declaration is signed. Require the complete envelope to be
	// the exact canonical encoding, not only its inner declaration.
	canonicalEnvelope, err := json.Marshal(env)
	if err != nil || !bytes.Equal(data, canonicalEnvelope) {
		return errors.New("artifact_envelope_not_canonical")
	}
	var declaration ArtifactDeclaration
	if err := decodeExactJSON(env.Declaration, &declaration); err != nil {
		return errors.New("artifact_declaration_invalid")
	}
	canonical, err := json.Marshal(declaration)
	if err != nil || !bytes.Equal(env.Declaration, canonical) {
		return errors.New("artifact_declaration_not_canonical")
	}
	if declaration.Domain != artifactDomain || declaration.Environment != want.Environment ||
		declaration.Component != want.Component || declaration.Generation < want.MinGeneration ||
		declaration.ArtifactSHA256 != want.ArtifactSHA256 || declaration.ImageID != want.ImageID ||
		!lowerSHA256(declaration.ArtifactSHA256) || !strings.HasPrefix(declaration.ImageID, "sha256:") ||
		!lowerSHA256(strings.TrimPrefix(declaration.ImageID, "sha256:")) ||
		declaration.IssuedAt.IsZero() || declaration.IssuedAt.After(want.Now) ||
		!declaration.ExpiresAt.After(want.Now) || !declaration.ExpiresAt.After(declaration.IssuedAt) {
		return errors.New("artifact_declaration_not_eligible")
	}
	digest := sha256.Sum256(append([]byte(artifactDomain+"\n"), canonical...))
	seen := make(map[string]bool, len(env.Signatures))
	for _, sig := range env.Signatures {
		key, ok := pinned[sig.KeyID]
		if !ok || sig.KeyID == "" || seen[sig.KeyID] || len(key) != ed25519.PublicKeySize {
			return errors.New("artifact_signature_invalid")
		}
		seen[sig.KeyID] = true
		decoded, err := hex.DecodeString(sig.Signature)
		if err != nil || len(decoded) != ed25519.SignatureSize ||
			!ed25519.Verify(key, digest[:], decoded) {
			return errors.New("artifact_signature_invalid")
		}
	}
	return nil
}

// VerifiedDeclaration returns the signed declaration only after verification.
func VerifiedDeclaration(data []byte, pinned map[string]ed25519.PublicKey, want ArtifactExpectation) (ArtifactDeclaration, error) {
	if err := VerifyArtifactEnvelope(data, pinned, want); err != nil {
		return ArtifactDeclaration{}, err
	}
	var envelope ArtifactEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ArtifactDeclaration{}, err
	}
	var declaration ArtifactDeclaration
	if err := json.Unmarshal(envelope.Declaration, &declaration); err != nil {
		return ArtifactDeclaration{}, err
	}
	return declaration, nil
}

func lowerSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func decodeExactJSON(data []byte, into any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
