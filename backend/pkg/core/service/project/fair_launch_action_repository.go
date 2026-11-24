package project

import (
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

const (
	fairLaunchStatusPending        = "pending_approval"
	fairLaunchStatusApprovalBound  = "approval_bound"
	fairLaunchStatusApproved       = "approved"
	fairLaunchStatusExecutionBound = "execution_bound"
	fairLaunchStatusExecuting      = "executing"
)

type FairLaunchActionMFAEvidence struct {
	Reference  string
	VerifiedAt time.Time
}

type FairLaunchActionMutationRequest struct {
	CanonicalDigest   string                   `json:"canonical_digest,omitempty"`
	PreviewToken      string                   `json:"preview_token,omitempty"`
	MFAAssertionToken string                   `json:"mfa_assertion_token,omitempty"`
	Evidence          FairLaunchActionEvidence `json:"evidence,omitempty"`
}

type CreateFairLaunchActionRequestInput struct {
	ProjectID, InitiatorAdminID                              uint64
	Action, CanonicalDigest, PreviewToken, MFAAssertionToken string
	Evidence                                                 FairLaunchActionEvidence
	Runtime                                                  projectruntime.Runtime
	Now                                                      time.Time
}

type ExecuteFairLaunchActionRequestInput struct {
	RequestID, ProjectID, ExecutorAdminID uint64
	MFAAssertionToken                     string
	Runtime                               projectruntime.Runtime
	Now                                   time.Time
}

func lockFairLaunchProject(tx *gorm.DB, projectID uint64) error {
	var project entity.Project
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id=? AND funding_mode=? AND contract_version=?", projectID, constant.FundingModeFairLaunch, constant.ContractVersionFairLaunchV1).Take(&project).Error
}
func sameGenerations(a FairLaunchActionGenerations, row *entity.FairLaunchActionRequest) bool {
	return a.Runtime == row.RuntimeGeneration && a.Policy == row.PolicyGeneration && a.SourceRegistry == row.SourceRegistryGeneration && a.Config == row.ConfigGeneration
}

func CreateFairLaunchActionRequestTx(tx *gorm.DB, in CreateFairLaunchActionRequestInput) (entity.FairLaunchActionRequest, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return entity.FairLaunchActionRequest{}, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if tx == nil || in.ProjectID == 0 || in.InitiatorAdminID == 0 {
		return entity.FairLaunchActionRequest{}, errors.New("mfa_assertion_required")
	}
	if err := lockFairLaunchProject(tx, in.ProjectID); err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	evidence, evidenceErr := ReadFairLaunchRuntimeEvidenceTx(tx, in.Runtime, in.Now, true)
	preview, err := PrepareFairLaunchActionPreview(in.ProjectID, in.Action, in.Evidence, in.Runtime, evidence, evidenceErr, in.Now)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	persisted, err := ConsumeFairLaunchPreviewTx(tx, in.PreviewToken, in.InitiatorAdminID, in.ProjectID, "action", in.Action, preview.ApprovalOperation, in.CanonicalDigest, in.Now)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	if !strings.EqualFold(stripActionDigest(preview.CanonicalDigest), persisted.BaseDigest) {
		return entity.FairLaunchActionRequest{}, errors.New("preview payload changed before submission")
	}
	mfaRow, err := ConsumeFairLaunchMFAAssertionTx(tx, in.MFAAssertionToken, FairLaunchMFAContext{AdminID: in.InitiatorAdminID, Operation: preview.ApprovalOperation, Stage: FairLaunchMFAStageActionInitiation, ProjectID: in.ProjectID, PreviewID: &persisted.ID}, in.Now)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	var wrapper struct {
		BaseDigest       string         `json:"base_digest"`
		ImmutablePayload map[string]any `json:"immutable_payload"`
	}
	decoder := jsonv1.NewDecoder(strings.NewReader(persisted.CanonicalPayload))
	decoder.UseNumber()
	if err := decoder.Decode(&wrapper); err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	if digest, err := canonicalDigest(wrapper.ImmutablePayload); err != nil || !strings.EqualFold(stripActionDigest(digest), persisted.BaseDigest) || !strings.EqualFold(stripActionDigest(wrapper.BaseDigest), persisted.BaseDigest) {
		return entity.FairLaunchActionRequest{}, errors.New("stored preview payload is not canonical")
	}
	payload, err := jsonv1.Marshal(wrapper.ImmutablePayload)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	g := evidence.Generations
	row := entity.FairLaunchActionRequest{ProjectID: in.ProjectID, CanonicalDigest: persisted.BaseDigest, CanonicalPayload: string(payload), Action: in.Action, ExecutionDomain: preview.ExecutionDomain, InitiatorAdminID: in.InitiatorAdminID, Status: fairLaunchStatusPending, PreviewExpiresAt: persisted.ExpiresAt, ApprovalExpiresAt: in.Now.Add(2 * time.Hour), ExecutionExpiresAt: in.Now.Add(2 * time.Hour), InitiatorMFAEvidenceRef: mfaRow.ProviderEvidenceDigest, InitiatorMFAVerifiedAt: mfaRow.IssuedAt, RuntimeGeneration: g.Runtime, PolicyGeneration: g.Policy, SourceRegistryGeneration: g.SourceRegistry, ConfigGeneration: g.Config, PreviewID: persisted.ID, CreatedAt: in.Now, UpdatedAt: in.Now}
	if err := tx.Create(&row).Error; err != nil {
		return row, err
	}
	return row, nil
}

func ApproveFairLaunchActionRequestTx(tx *gorm.DB, requestID, adminApprovalID, approverID uint64, mfaToken string, runtime projectruntime.Runtime, now time.Time) error {
	if !constant.FairLaunchMaterializationEnabled {
		return errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	row, err := lockActionRequest(tx, requestID)
	if err != nil {
		return err
	}
	if row.Status != fairLaunchStatusPending || now.After(row.ApprovalExpiresAt) {
		return errors.New("action request is not approval-eligible")
	}
	if adminApprovalID == 0 || approverID == 0 || approverID == row.InitiatorAdminID {
		return errors.New("distinct approver and mfa_assertion_required")
	}
	// Persisted requests may predate a capability shutdown. Re-resolve support
	// before consuming MFA so an unavailable domain can never be represented as approved.
	if reason := fairLaunchActionDisabledReason(row.Action, row.ExecutionDomain); reason != "" {
		return errors.New(reason)
	}
	mfaRow, err := ConsumeFairLaunchMFAAssertionTx(tx, mfaToken, FairLaunchMFAContext{AdminID: approverID, Operation: "fair_launch.action." + row.Action, Stage: FairLaunchMFAStageActionApproval, ProjectID: row.ProjectID, ActionRequestID: &row.ID, AdminApprovalID: &adminApprovalID}, now)
	if err != nil {
		return err
	}
	mfa := FairLaunchActionMFAEvidence{Reference: mfaRow.ProviderEvidenceDigest, VerifiedAt: mfaRow.IssuedAt}
	if err := lockFairLaunchProject(tx, row.ProjectID); err != nil {
		return err
	}
	ev, evErr := ReadFairLaunchRuntimeEvidenceTx(tx, runtime, now, true)
	if reason := FairLaunchRuntimeReason(runtime, ev, evErr); reason != "" {
		return errors.New(reason)
	}
	if !sameGenerations(ev.Generations, row) {
		return errors.New("action trusted generations changed before approval")
	}
	if err := appendActionTransitionTx(tx, row, fairLaunchStatusPending, fairLaunchStatusApprovalBound, approverID, mfa, now); err != nil {
		return err
	}
	row.ApproverAdminID = &approverID
	row.AdminApprovalID = &adminApprovalID
	row.ApproverMFAEvidenceRef = &mfa.Reference
	row.ApproverMFAVerifiedAt = &mfa.VerifiedAt
	row.Status = fairLaunchStatusApprovalBound
	row.UpdatedAt = now
	if err := tx.Save(row).Error; err != nil {
		return err
	}
	if err := appendActionTransitionTx(tx, row, fairLaunchStatusApprovalBound, fairLaunchStatusApproved, approverID, mfa, now); err != nil {
		return err
	}
	row.Status = fairLaunchStatusApproved
	row.UpdatedAt = now
	return tx.Save(row).Error
}

func ExecuteFairLaunchActionRequestTx(tx *gorm.DB, in ExecuteFairLaunchActionRequestInput) (entity.FairLaunchActionRequest, error) {
	if !constant.FairLaunchMaterializationEnabled {
		return entity.FairLaunchActionRequest{}, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	if !in.Runtime.Enabled {
		return entity.FairLaunchActionRequest{}, errors.New("runtime_disabled")
	}
	if tx == nil {
		return entity.FairLaunchActionRequest{}, errors.New("runtime_unverified")
	}
	row, err := lockActionRequest(tx, in.RequestID)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	if row.ProjectID != in.ProjectID || row.Status != fairLaunchStatusApproved || row.ApproverAdminID == nil || in.Now.After(row.ExecutionExpiresAt) {
		return entity.FairLaunchActionRequest{}, errors.New("action request is not execution-eligible")
	}
	if in.ExecutorAdminID == 0 || in.ExecutorAdminID == row.InitiatorAdminID || in.ExecutorAdminID == *row.ApproverAdminID {
		return entity.FairLaunchActionRequest{}, errors.New("distinct executor and mfa_assertion_required")
	}
	// This is independent of the later dispatcher check and precedes MFA consumption.
	if reason := fairLaunchActionDisabledReason(row.Action, row.ExecutionDomain); reason != "" {
		return entity.FairLaunchActionRequest{}, errors.New(reason)
	}
	mfaRow, err := ConsumeFairLaunchMFAAssertionTx(tx, in.MFAAssertionToken, FairLaunchMFAContext{AdminID: in.ExecutorAdminID, Operation: "fair_launch.action." + row.Action, Stage: FairLaunchMFAStageActionExecution, ProjectID: row.ProjectID, ActionRequestID: &row.ID, AdminApprovalID: row.AdminApprovalID}, in.Now)
	if err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	mfa := FairLaunchActionMFAEvidence{Reference: mfaRow.ProviderEvidenceDigest, VerifiedAt: mfaRow.IssuedAt}
	if err := lockFairLaunchProject(tx, in.ProjectID); err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	ev, evErr := ReadFairLaunchRuntimeEvidenceTx(tx, in.Runtime, in.Now, true)
	if reason := FairLaunchRuntimeReason(in.Runtime, ev, evErr); reason != "" {
		return entity.FairLaunchActionRequest{}, errors.New(reason)
	}
	if !sameGenerations(ev.Generations, row) {
		return entity.FairLaunchActionRequest{}, errors.New("action trusted generations changed")
	}
	if err := appendActionTransitionTx(tx, row, fairLaunchStatusApproved, fairLaunchStatusExecutionBound, in.ExecutorAdminID, mfa, in.Now); err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	row.ExecutorAdminID = &in.ExecutorAdminID
	row.ExecutorMFAEvidenceRef = &mfa.Reference
	row.ExecutorMFAVerifiedAt = &mfa.VerifiedAt
	row.Status = fairLaunchStatusExecutionBound
	row.UpdatedAt = in.Now
	if err := tx.Save(row).Error; err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	if err := appendActionTransitionTx(tx, row, fairLaunchStatusExecutionBound, fairLaunchStatusExecuting, in.ExecutorAdminID, mfa, in.Now); err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	row.Status = fairLaunchStatusExecuting
	row.UpdatedAt = in.Now
	if err := tx.Save(row).Error; err != nil {
		return entity.FairLaunchActionRequest{}, err
	}
	return *row, nil
}
func lockActionRequest(tx *gorm.DB, id uint64) (*entity.FairLaunchActionRequest, error) {
	var row entity.FairLaunchActionRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
func appendActionTransitionTx(tx *gorm.DB, row *entity.FairLaunchActionRequest, from, to string, actor uint64, mfa FairLaunchActionMFAEvidence, now time.Time) error {
	var sequence uint32
	if err := tx.Model(&entity.FairLaunchActionTransition{}).Where("action_request_id=?", row.ID).Select("COALESCE(MAX(sequence_no),0)").Scan(&sequence).Error; err != nil {
		return err
	}
	sequence++
	digest := sha256.Sum256([]byte(row.CanonicalDigest + ":" + from + ":" + to + ":" + mfa.Reference))
	return tx.Create(&entity.FairLaunchActionTransition{ActionRequestID: row.ID, SequenceNo: sequence, FromStatus: from, ToStatus: to, ActorAdminID: actor, MFAEvidenceRef: mfa.Reference, EvidenceDigest: hex.EncodeToString(digest[:]), CreatedAt: now}).Error
}
func stripActionDigest(s string) string { return strings.TrimPrefix(strings.ToLower(s), "0x") }
