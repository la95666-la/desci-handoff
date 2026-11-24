package activation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Readiness is an independently signed, short-lived *evidence assertion*.
// It does not perform a live DB/RPC probe, authorize a migration or decide
// whether a release should be made. The operator must obtain the evidence
// from separate, reviewed production checks, not from test fixtures.
type Readiness struct {
	Domain                    string    `json:"domain"`
	Environment               string    `json:"environment"`
	AgentID                   string    `json:"agent_id"`
	Root                      string    `json:"root"`
	HostKeySHA256             string    `json:"host_key_sha256"`
	ArtifactImageID           string    `json:"artifact_image_id"`
	ReleaseSHA256             string    `json:"release_sha256"`
	ReleaseID                 string    `json:"release_id"`
	Generation                uint64    `json:"generation"`
	MigrationSHA256           string    `json:"migration_sha256"`
	DatabaseServerUUID        string    `json:"database_server_uuid"`
	SchemaEvidenceSHA256      string    `json:"schema_evidence_sha256"`
	BackupReceiptSHA256       string    `json:"backup_receipt_sha256"`
	RestoreEvidenceSHA256     string    `json:"restore_evidence_sha256"`
	SignerEvidenceSHA256      string    `json:"signer_evidence_sha256"`
	ChainRPCEvidenceSHA256    string    `json:"chain_rpc_evidence_sha256"`
	PermissionsEvidenceSHA256 string    `json:"permissions_evidence_sha256"`
	CoreOperatorAddress       string    `json:"core_operator_address"`
	JobSignerAddress          string    `json:"job_signer_address"`
	V2ContractAddress         string    `json:"v2_contract_address"`
	ChainID                   uint64    `json:"chain_id"`
	IssuedAt                  time.Time `json:"issued_at"`
	ExpiresAt                 time.Time `json:"expires_at"`
}

type ReadinessExpectation struct {
	Environment     string
	AgentID         string
	Root            string
	HostKeySHA256   string
	ArtifactImageID string
	ReleaseSHA256   string
	ReleaseID       string
	Generation      uint64
	Now             time.Time
}

// EvidenceReceipt is a bounded, identity-bound index entry for independently
// reviewed evidence. The referenced underlying material remains external;
// this structure does NOT assert that DB/RPC checks actually ran.
type EvidenceReceipt struct {
	Domain                  string    `json:"domain"`
	Kind                    string    `json:"kind"`
	Result                  string    `json:"result"`
	Environment             string    `json:"environment"`
	AgentID                 string    `json:"agent_id"`
	HostKeySHA256           string    `json:"host_key_sha256"`
	ReleaseID               string    `json:"release_id"`
	Generation              uint64    `json:"generation"`
	DatabaseServerUUID      string    `json:"database_server_uuid"`
	MigrationSHA256         string    `json:"migration_sha256"`
	ChainID                 uint64    `json:"chain_id"`
	EvidenceReferenceSHA256 string    `json:"evidence_reference_sha256"`
	ObservedAt              time.Time `json:"observed_at"`
}

const EvidenceReceiptDomain = "joincare.ido.job.evidence-receipt.v1"

func VerifyEvidenceReceipt(data []byte, kind string, claim Readiness, now time.Time) error {
	if len(data) == 0 || len(data) > 1024*1024 || now.IsZero() {
		return errors.New("job_evidence_receipt_invalid")
	}
	var receipt EvidenceReceipt
	if decodeExactJSON(data, &receipt) != nil {
		return errors.New("job_evidence_receipt_invalid")
	}
	canonical, err := json.Marshal(receipt)
	if err != nil || !bytes.Equal(data, canonical) {
		return errors.New("job_evidence_receipt_not_canonical")
	}
	if receipt.Domain != EvidenceReceiptDomain || receipt.Kind != kind || receipt.Result != "verified" ||
		receipt.Environment != claim.Environment || receipt.AgentID != claim.AgentID || receipt.HostKeySHA256 != claim.HostKeySHA256 ||
		receipt.ReleaseID != claim.ReleaseID || receipt.Generation != claim.Generation ||
		receipt.DatabaseServerUUID != claim.DatabaseServerUUID || receipt.MigrationSHA256 != claim.MigrationSHA256 ||
		receipt.ChainID != claim.ChainID || !lowerSHA256(receipt.EvidenceReferenceSHA256) ||
		receipt.ObservedAt.IsZero() || receipt.ObservedAt.After(now) || receipt.ObservedAt.Before(now.Add(-24*time.Hour)) {
		return errors.New("job_evidence_receipt_not_eligible")
	}
	return nil
}

const ReadinessDomain = "joincare.ido.job.readiness.v1"
const SigningMetadataMigrationSHA256 = "0a0d4a20a36065175aadf17111a7730b473615628d3e88c9eadcd3e9fc9d8bb2"

// VerifyReadiness requires independent threshold keys and canonical JSON. All
// evidence digests are non-secret references to externally retained receipts.
func VerifyReadiness(data []byte, keys map[string]ed25519.PublicKey, want ReadinessExpectation) error {
	_, err := ParseAndVerifyReadiness(data, keys, want)
	return err
}

// ParseAndVerifyReadiness returns the verified references for local receipt
// integrity checks; no unverified claims should be used as release authority.
func ParseAndVerifyReadiness(data []byte, keys map[string]ed25519.PublicKey, want ReadinessExpectation) (Readiness, error) {
	reject := func(message string) (Readiness, error) { return Readiness{}, errors.New(message) }
	if len(data) == 0 || len(data) > 64*1024 || len(keys) < 3 || want.Now.IsZero() ||
		want.Environment == "" || want.AgentID == "" || want.Root == "" || want.HostKeySHA256 == "" ||
		want.ReleaseID == "" || want.Generation == 0 || !lowerSHA256(want.ReleaseSHA256) ||
		!validReadinessImageID(want.ArtifactImageID) {
		return reject("job_readiness_trust_unavailable")
	}
	var envelope ArtifactEnvelope
	if decodeExactJSON(data, &envelope) != nil || len(envelope.Signatures) != 2 {
		return reject("job_readiness_envelope_invalid")
	}
	canonicalEnvelope, err := json.Marshal(envelope)
	if err != nil || !bytes.Equal(data, canonicalEnvelope) {
		return reject("job_readiness_envelope_not_canonical")
	}
	var claim Readiness
	if decodeExactJSON(envelope.Declaration, &claim) != nil {
		return reject("job_readiness_declaration_invalid")
	}
	canonical, err := json.Marshal(claim)
	if err != nil || !bytes.Equal(canonical, envelope.Declaration) {
		return reject("job_readiness_declaration_not_canonical")
	}
	if claim.Domain != ReadinessDomain || claim.Environment != want.Environment || claim.AgentID != want.AgentID ||
		claim.Root != want.Root || claim.HostKeySHA256 != want.HostKeySHA256 || claim.ReleaseSHA256 != want.ReleaseSHA256 ||
		claim.ArtifactImageID != want.ArtifactImageID ||
		claim.ReleaseID != want.ReleaseID || claim.Generation != want.Generation ||
		claim.MigrationSHA256 != SigningMetadataMigrationSHA256 || claim.ChainID != 56 ||
		!validEvidenceUUID(claim.DatabaseServerUUID) || !validAddress(claim.CoreOperatorAddress) ||
		!validAddress(claim.V2ContractAddress) ||
		!validAddress(claim.JobSignerAddress) || strings.EqualFold(claim.CoreOperatorAddress, claim.JobSignerAddress) ||
		!lowerSHA256(claim.SchemaEvidenceSHA256) || !lowerSHA256(claim.BackupReceiptSHA256) ||
		!lowerSHA256(claim.RestoreEvidenceSHA256) || !lowerSHA256(claim.SignerEvidenceSHA256) ||
		!lowerSHA256(claim.ChainRPCEvidenceSHA256) || !lowerSHA256(claim.PermissionsEvidenceSHA256) ||
		claim.IssuedAt.IsZero() || claim.IssuedAt.After(want.Now) || claim.IssuedAt.Before(want.Now.Add(-30*time.Minute)) ||
		!claim.ExpiresAt.After(want.Now) || claim.ExpiresAt.After(claim.IssuedAt.Add(30*time.Minute)) {
		return reject("job_readiness_not_eligible")
	}
	digest := sha256.Sum256(append([]byte(ReadinessDomain+"\n"), canonical...))
	seenID := map[string]bool{}
	seenKey := map[string]bool{}
	for _, sig := range envelope.Signatures {
		key, ok := keys[sig.KeyID]
		decoded, err := hex.DecodeString(sig.Signature)
		if !ok || len(key) != ed25519.PublicKeySize || seenID[sig.KeyID] || seenKey[string(key)] || err != nil ||
			len(decoded) != ed25519.SignatureSize || !ed25519.Verify(key, digest[:], decoded) {
			return reject("job_readiness_signature_invalid")
		}
		seenID[sig.KeyID], seenKey[string(key)] = true, true
	}
	return claim, nil
}

func validReadinessImageID(value string) bool {
	return strings.HasPrefix(value, "sha256:") && lowerSHA256(strings.TrimPrefix(value, "sha256:"))
}

func validAddress(s string) bool {
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(s[2:])
	return err == nil && s != "0x"+strings.Repeat("0", 40)
}

func validEvidenceUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
