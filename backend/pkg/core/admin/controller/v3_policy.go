package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/service/project"
)

func v3PolicyProjectNotCreated(current *project.OnchainParamsCurrent) bool {
	return current != nil && current.Version == constant.ContractVersionIDOV3Stablecoin &&
		current.FundingMode == constant.FundingModeCappedPresale &&
		current.Status == constant.ProjectStatusSet && !current.Locked
}

// PreviewV3InitialPolicy is a read-only versioned diagnostic behind admin permission.
// Its transient digest must be recomputed before a separately permission-gated
// review can be inserted. It never approves or executes a Safe transaction.
func PreviewV3InitialPolicy(c *gin.Context) {
	// Only finite phase names and timings; never log wallets, RPC URLs or
	// provider error strings. In particular, log even if the browser disconnects.
	started := time.Now()
	phase := "input"
	phaseStarted := started
	advance := func(next string) { phase, phaseStarted = next, time.Now() }
	defer func() {
		if logger := log.GetLogger(); logger != nil && (phase != "complete" || time.Since(started) >= 10*time.Second) {
			logger.WithFields(map[string]interface{}{
				"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(),
				"phase_ms":        time.Since(phaseStarted).Milliseconds(),
				"client_canceled": c.Request.Context().Err() != nil,
			}).Warn("V3 policy preview request trace")
		}
	}()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var req struct {
		Wallets []string `json:"wallets"`
		Reason  string   `json:"reason" binding:"required"`
		Enabled *bool    `json:"enabled"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Wallets) > 512 || req.Reason == "" || req.Enabled == nil ||
		(*req.Enabled && len(req.Wallets) == 0) || (!*req.Enabled && len(req.Wallets) != 0) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_INPUT_INVALID"})
		return
	}
	var preview project.V3PolicyPreview
	advance("mode_ready")
	if !project.ApprovedV3ModeReady() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_PREVIEW_UNAVAILABLE", nil, "mode_ready")
		return
	}
	// Policy getters do not exist before the original ProjectCreated receipt
	// has been applied. Explain this read-only prerequisite without relaxing
	// the canonical receipt, cursor, Safe or scanner checks below.
	advance("project_params")
	current, readErr := project.GetOnchainParams(int64(id))
	if readErr != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_POLICY_PREVIEW_UNAVAILABLE"})
		return
	}
	if v3PolicyProjectNotCreated(current) {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_POLICY_PROJECT_NOT_CREATED",
			"message": "V3 project requires confirmed creation and Job reconciliation before policy preview"})
		return
	}
	advance("chain_preview")
	preview, err = project.PreviewApprovedV3PolicyChangeContext(c.Request.Context(), id, req.Wallets, *req.Enabled, req.Reason)
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_PREVIEW_UNAVAILABLE", err, "chain_preview")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": preview})
	advance("complete")
}

// GetV3PolicyStatus reports only pinned on-chain state, never an effective
// membership decision. A candidate root or an observed event is insufficient.
func GetV3PolicyStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var status project.V3PolicyStatus
	if !project.ApprovedV3ModeConfigured() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_STATUS_UNAVAILABLE", nil, "mode_ready")
		return
	}
	status, err = project.ReadApprovedV3PolicyDisplayStatusContext(c.Request.Context(), id)
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_STATUS_UNAVAILABLE", err, "status")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": status})
}

// StartV3PolicyReview recomputes the fresh signed-policy preview rather than
// trusting a posted root/member set. Authentication and live project write
// permission are required; this does not authorize Safe execution.
func StartV3PolicyReview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var req struct {
		Wallets       []string `json:"wallets"`
		Enabled       *bool    `json:"enabled"`
		Reason        string   `json:"reason"`
		PreviewDigest string   `json:"preview_digest"`
		BaseBlock     string   `json:"base_block"`
		BaseHash      string   `json:"base_hash"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_INPUT_INVALID"})
		return
	}
	baseBlock, parseErr := strconv.ParseUint(req.BaseBlock, 10, 64)
	if len(req.Wallets) > 512 || req.Enabled == nil ||
		(*req.Enabled && len(req.Wallets) == 0) || (!*req.Enabled && len(req.Wallets) != 0) ||
		req.Reason == "" || req.PreviewDigest == "" || parseErr != nil || baseBlock == 0 || req.BaseHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_INPUT_INVALID"})
		return
	}
	input := project.V3PolicyReviewInput{ProjectID: id, Wallets: req.Wallets, Reason: req.Reason,
		Enabled: *req.Enabled, AdminID: currentAdminID(c), PreviewDigest: req.PreviewDigest,
		BaseBlock: baseBlock, BaseHash: req.BaseHash}
	var reviewID uint64
	if !project.ApprovedV3ModeReady() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_REVIEW_UNAVAILABLE", nil, "mode_ready")
		return
	}
	reviewID, err = project.StartApprovedV3PolicyReviewContext(c.Request.Context(), input)
	if err != nil {
		code := "V3_POLICY_REVIEW_UNAVAILABLE"
		if errors.Is(err, project.ErrV3PolicyReviewSchemaMigrationRequired) {
			code = "V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED"
		}
		policyDiagnostic(c, http.StatusConflict, code, err, "review_commit")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"review_id": strconv.FormatUint(reviewID, 10), "state": "pending_second_admin"}})
}

// ListV3PolicyReviews is a read-only work-item list. Unlike the generic
// approval executor, approval here never executes the on-chain Safe call.
func ListV3PolicyReviews(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	if !project.ApprovedV3ModeReady() {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_POLICY_REVIEW_UNAVAILABLE"})
		return
	}
	items, err := project.ListV3PolicyReviews(c.Request.Context(), id, currentAdminID(c))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_POLICY_REVIEW_UNAVAILABLE"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items}})
}

// ReadV3PolicyReview lets the second administrator inspect the immutable
// full snapshot and digest before making their independent decision.
func ReadV3PolicyReview(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	id, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	reviewID, e2 := strconv.ParseUint(c.Param("reviewID"), 10, 64)
	if e1 != nil || e2 != nil || id == 0 || reviewID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	preview, err := project.V3PolicyReviewRead(id, reviewID, currentAdminID(c))
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_REVIEW_UNAVAILABLE", err, "review_read")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": preview})
}

// ApproveV3PolicyReview checks live distinct Admin identities and permissions.
// Approval creates only an immutable decision; governance remains separate.
func ApproveV3PolicyReview(c *gin.Context) {
	id, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	reviewID, e2 := strconv.ParseUint(c.Param("reviewID"), 10, 64)
	if e1 != nil || e2 != nil || id == 0 || reviewID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var req struct {
		PreviewDigest string `json:"preview_digest"`
	}
	if c.ShouldBindJSON(&req) != nil || req.PreviewDigest == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_INPUT_INVALID"})
		return
	}
	var approvalErr error
	if !project.ApprovedV3ModeReady() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_APPROVAL_UNAVAILABLE", nil, "mode_ready")
		return
	}
	approvalErr = project.ApproveApprovedV3PolicyReviewContext(c.Request.Context(), id, reviewID, currentAdminID(c), req.PreviewDigest)
	if approvalErr != nil {
		code := "V3_POLICY_APPROVAL_UNAVAILABLE"
		if errors.Is(approvalErr, project.ErrV3PolicyReviewSchemaMigrationRequired) {
			code = "V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED"
		}
		policyDiagnostic(c, http.StatusConflict, code, approvalErr, "approval_commit")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"review_id": strconv.FormatUint(reviewID, 10), "state": "approved_pending_governance"}})
}

// ObserveV3PolicyExecution only audits an externally executed Safe original
// hash pinned by a separately signed release attestation. No request body or
// Admin hash may authorize a transaction, broadcast or financial effect.
func ObserveV3PolicyExecution(c *gin.Context) {
	id, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	reviewID, e2 := strconv.ParseUint(c.Param("reviewID"), 10, 64)
	if e1 != nil || e2 != nil || id == 0 || reviewID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var observationErr error
	if !project.ApprovedV3ModeReady() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_EXECUTION_UNAVAILABLE", nil, "mode_ready")
		return
	}
	var req struct {
		TxHash     string `json:"tx_hash"`
		SafeTxHash string `json:"safe_tx_hash"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_INPUT_INVALID"})
		return
	}
	observationErr = project.RecordApprovedV3PolicyExecutionContext(c.Request.Context(), id, reviewID, req.TxHash, req.SafeTxHash)
	if observationErr != nil {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_EXECUTION_UNAVAILABLE", observationErr, project.V3PolicyObservationFailureStage(observationErr))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"review_id": strconv.FormatUint(reviewID, 10), "state": "confirmed_observed_not_proof"}})
}

// GetV3PolicySafeHandoff returns the exact calldata for an independently
// approved immutable review. It does not propose, sign or broadcast via Safe.
func GetV3PolicySafeHandoff(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	id, e1 := strconv.ParseUint(c.Param("id"), 10, 64)
	reviewID, e2 := strconv.ParseUint(c.Param("reviewID"), 10, 64)
	if e1 != nil || e2 != nil || id == 0 || reviewID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_POLICY_SCOPE_INVALID"})
		return
	}
	var handoff project.V3PolicySafeHandoff
	var err error
	if !project.ApprovedV3ModeConfigured() {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_SAFE_HANDOFF_UNAVAILABLE", nil, "mode_ready")
		return
	}
	handoff, err = project.ReadApprovedV3PolicySafeHandoffContext(c.Request.Context(), id, reviewID, currentAdminID(c))
	if err != nil {
		policyDiagnostic(c, http.StatusConflict, "V3_POLICY_SAFE_HANDOFF_UNAVAILABLE", err, "safe_handoff")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": handoff})
}
