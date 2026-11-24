package project

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	jsonv1 "encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchMFAStageSetInitiation    = "set_initiation"
	FairLaunchMFAStageSetApproval      = "set_approval"
	FairLaunchMFAStageActionInitiation = "action_initiation"
	FairLaunchMFAStageActionApproval   = "action_approval"
	FairLaunchMFAStageActionExecution  = "action_execution"
	fairLaunchMFAMaxFreshness          = 15 * time.Minute
)

// FairLaunchMFAProviderAdmissionVerifier authenticates the provider's admission
// against an independently distributed manifest/trust root. Production defaults
// to fail closed until an approved adapter installs this seam during bootstrap.
type FairLaunchMFAProviderAdmissionVerifier func(entity.FairLaunchMFAProvider, time.Time) error

var verifyFairLaunchMFAProviderAdmission FairLaunchMFAProviderAdmissionVerifier = func(entity.FairLaunchMFAProvider, time.Time) error {
	return errors.New("mfa_provider_unavailable")
}

// SetFairLaunchMFAProviderAdmissionVerifier is an application-bootstrap seam,
// not an HTTP issuance path. Passing nil restores the production fail-closed verifier.
func SetFairLaunchMFAProviderAdmissionVerifier(verifier FairLaunchMFAProviderAdmissionVerifier) {
	if verifier == nil {
		verifyFairLaunchMFAProviderAdmission = func(entity.FairLaunchMFAProvider, time.Time) error { return errors.New("mfa_provider_unavailable") }
		return
	}
	verifyFairLaunchMFAProviderAdmission = verifier
}

type FairLaunchOpaqueAuthorization struct {
	PreviewToken      string `json:"preview_token,omitempty"`
	MFAAssertionToken string `json:"mfa_assertion_token,omitempty"`
}

type FairLaunchMFAContext struct {
	AdminID          uint64
	Operation        string
	Stage            string
	ProjectID        uint64
	PreviewID        *uint64
	ActionRequestID  *uint64
	V3PolicyReviewID *uint64
	AdminApprovalID  *uint64
}

func opaqueDigest(token string) (string, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 512 {
		return "", errors.New("opaque authorization token is invalid")
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}

func newOpaqueToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(b)
	digest, _ := opaqueDigest(token)
	return token, digest, nil
}

func IssueFairLaunchPreviewTx(tx *gorm.DB, kind string, projectID, adminID uint64, action, operation, baseDigest string, basePayload []byte, now time.Time) (entity.FairLaunchPreview, string, error) {
	if tx == nil || (kind != "set" && kind != "action") || projectID == 0 || adminID == 0 || len(basePayload) == 0 {
		return entity.FairLaunchPreview{}, "", errors.New("fair launch preview identity is invalid")
	}
	token, tokenDigest, err := newOpaqueToken()
	if err != nil {
		return entity.FairLaunchPreview{}, "", err
	}
	idBytes := make([]byte, 8)
	if _, err = rand.Read(idBytes); err != nil {
		return entity.FairLaunchPreview{}, "", err
	}
	id := binary.BigEndian.Uint64(idBytes)
	if id == 0 {
		id = 1
	}
	var payload any
	decoder := jsonv1.NewDecoder(strings.NewReader(string(basePayload)))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err != nil {
		return entity.FairLaunchPreview{}, "", errors.New("preview payload is invalid")
	}
	expires := now.Add(FairLaunchPreviewTTL)
	row := entity.FairLaunchPreview{ID: id, TokenDigest: tokenDigest, PreviewKind: kind, ProjectID: projectID, Action: action, Operation: operation, AdminID: adminID, BaseDigest: stripActionDigest(baseDigest), IssuedAt: now, ExpiresAt: expires, CreatedAt: now}
	wrapper := previewBoundPayload(row, payload)
	encoded, err := jsonv1.Marshal(wrapper)
	if err != nil {
		return entity.FairLaunchPreview{}, "", err
	}
	row.CanonicalPayload = string(encoded)
	row.CanonicalDigest, err = canonicalDigest(wrapper)
	if err != nil {
		return entity.FairLaunchPreview{}, "", err
	}
	row.CanonicalDigest = stripActionDigest(row.CanonicalDigest)
	if err := tx.Create(&row).Error; err != nil {
		return entity.FairLaunchPreview{}, "", err
	}
	return row, token, nil
}

func previewBoundPayload(row entity.FairLaunchPreview, payload any) map[string]any {
	return map[string]any{
		"action": row.Action, "admin_id": row.AdminID, "base_digest": "0x" + stripActionDigest(row.BaseDigest), "immutable_payload": payload,
		"expires_at": row.ExpiresAt.UTC().Format(time.RFC3339Nano), "issued_at": row.IssuedAt.UTC().Format(time.RFC3339Nano),
		"operation": row.Operation, "preview_id": row.ID, "preview_kind": row.PreviewKind, "project_id": row.ProjectID,
	}
}
func previewBoundDigest(row entity.FairLaunchPreview) (string, error) {
	var wrapper map[string]any
	decoder := jsonv1.NewDecoder(strings.NewReader(row.CanonicalPayload))
	decoder.UseNumber()
	if err := decoder.Decode(&wrapper); err != nil {
		return "", err
	}
	return canonicalDigest(wrapper)
}

func ConsumeFairLaunchPreviewTx(tx *gorm.DB, token string, adminID, projectID uint64, kind, action, operation, suppliedDigest string, now time.Time) (entity.FairLaunchPreview, error) {
	digest, err := opaqueDigest(token)
	if err != nil {
		return entity.FairLaunchPreview{}, errors.New("preview_required")
	}
	var row entity.FairLaunchPreview
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_digest=?", digest).Take(&row).Error; err != nil {
		return row, errors.New("preview_invalid")
	}
	if row.ConsumedAt != nil || !now.Before(row.ExpiresAt) || row.AdminID != adminID || row.ProjectID != projectID || row.PreviewKind != kind || row.Action != action || row.Operation != operation || !strings.EqualFold(stripActionDigest(suppliedDigest), row.CanonicalDigest) {
		return row, errors.New("preview_expired_consumed_or_mismatched")
	}
	recomputed, err := previewBoundDigest(row)
	if err != nil || !strings.EqualFold(stripActionDigest(recomputed), row.CanonicalDigest) {
		return row, errors.New("preview_canonical_digest_mismatch")
	}
	row.ConsumedAt, row.ConsumedByAdminID = &now, &adminID
	if err := tx.Save(&row).Error; err != nil {
		return row, err
	}
	return row, nil
}

// ConsumeFairLaunchMFAAssertionTx only consumes assertions already verified and
// provisioned by an approved external provider adapter. No production HTTP
// route issues these records.
func ConsumeFairLaunchMFAAssertionTx(tx *gorm.DB, token string, expected FairLaunchMFAContext, now time.Time) (entity.FairLaunchMFAAssertion, error) {
	if tx == nil || expected.AdminID == 0 || expected.Operation == "" || expected.Stage == "" || expected.ProjectID == 0 {
		return entity.FairLaunchMFAAssertion{}, errors.New("mfa_provider_unavailable")
	}
	// M3 has a separate database review FK; a coincidentally equal Fair
	// Launch action ID must never be interchangeable with a V3 decision.
	if expected.Operation == v3PolicyOperation {
		if expected.PreviewID != nil || expected.ActionRequestID != nil || expected.AdminApprovalID != nil ||
			(expected.Stage == FairLaunchMFAStageActionApproval) != (expected.V3PolicyReviewID != nil) ||
			(expected.Stage != FairLaunchMFAStageActionApproval && expected.Stage != FairLaunchMFAStageActionInitiation) {
			return entity.FairLaunchMFAAssertion{}, errors.New("mfa_assertion_binding_mismatch")
		}
	} else if expected.V3PolicyReviewID != nil {
		return entity.FairLaunchMFAAssertion{}, errors.New("mfa_assertion_binding_mismatch")
	}
	digest, err := opaqueDigest(token)
	if err != nil {
		return entity.FairLaunchMFAAssertion{}, errors.New("mfa_assertion_required")
	}
	var row entity.FairLaunchMFAAssertion
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_digest=?", digest).Take(&row).Error; err != nil {
		return row, errors.New("mfa_assertion_required")
	}
	var provider entity.FairLaunchMFAProvider
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("provider_id=? AND enabled=1 AND revoked_at IS NULL AND valid_from<=? AND (valid_until IS NULL OR valid_until>?)", row.ProviderID, now, now).Take(&provider).Error; err != nil {
		return row, errors.New("mfa_provider_unavailable")
	}
	if err := verifyFairLaunchMFAProviderAdmission(provider, now); err != nil {
		return row, errors.New("mfa_provider_unavailable")
	}
	if !row.ExternallyVerified || row.ProviderID == "" || strings.EqualFold(row.ProviderID, "test") || row.ProviderEvidenceDigest == "" || row.ProviderAdmissionDigest == "" || row.ProviderGeneration == 0 || row.ProviderGeneration != provider.Generation || !strings.EqualFold(row.ProviderAdmissionDigest, provider.AdmissionDigest) || row.ConsumedAt != nil || now.Before(row.IssuedAt) || !now.Before(row.ExpiresAt) || row.ExpiresAt.Sub(row.IssuedAt) > fairLaunchMFAMaxFreshness {
		return row, errors.New("mfa_assertion_stale_or_unverified")
	}
	if row.AdminID != expected.AdminID || row.Operation != expected.Operation || row.Stage != expected.Stage || row.ProjectID != expected.ProjectID || !sameOptionalID(row.PreviewID, expected.PreviewID) || !sameOptionalID(row.ActionRequestID, expected.ActionRequestID) || !sameOptionalID(row.V3PolicyReviewID, expected.V3PolicyReviewID) || !sameOptionalID(row.AdminApprovalID, expected.AdminApprovalID) {
		return row, errors.New("mfa_assertion_binding_mismatch")
	}
	row.ConsumedAt = &now
	if err := tx.Save(&row).Error; err != nil {
		return row, err
	}
	return row, nil
}

func sameOptionalID(a, b *uint64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
