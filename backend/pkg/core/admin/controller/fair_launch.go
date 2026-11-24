package controller

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/service/approval"
	coredb "joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
)

func fairLaunchProjectID(c *gin.Context) (uint64, bool) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID"})
		return 0, false
	}
	return id, true
}
func fairLaunchRuntime(c *gin.Context) (projectruntime.Runtime, bool) {
	r, e := projectruntime.FairLaunchV1()
	if e != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_RUNTIME_INVALID", "message": e.Error()})
		return r, false
	}
	return r, true
}
func fairLaunchSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"status": true, "data": data})
}

func FairLaunchCapabilities(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	fairLaunchSuccess(c, fairLaunchCapabilitiesForAdmin(id, r, projectService.NowUTC(), currentAdminID(c)))
}

func fairLaunchCapabilitiesForAdmin(id uint64, r projectruntime.Runtime, now time.Time, adminID uint64) projectService.FairLaunchCapabilityEnvelope {
	evidence, err := projectService.ReadFairLaunchRuntimeEvidence(coredb.Mysql(), r, now)
	caps := projectService.BuildFairLaunchCapabilities(id, r, evidence, err)
	return projectService.DecorateFairLaunchCapabilities(coredb.Mysql(), caps, id, adminID, now)
}

type DisabledFairLaunchSetInspector struct{}

func (DisabledFairLaunchSetInspector) ProjectToken(string) (projectService.FairLaunchProjectInspection, error) {
	return projectService.FairLaunchProjectInspection{}, errors.New("fair launch trusted inspection unavailable")
}
func (DisabledFairLaunchSetInspector) BeneficiarySafe(string) (projectService.FairLaunchBeneficiaryInspection, error) {
	return projectService.FairLaunchBeneficiaryInspection{}, errors.New("fair launch trusted inspection unavailable")
}
func (DisabledFairLaunchSetInspector) PlatformFeeReceiver(string) (projectService.FairLaunchBeneficiaryInspection, error) {
	return projectService.FairLaunchBeneficiaryInspection{}, errors.New("fair launch trusted inspection unavailable")
}

var FairLaunchSetInspector projectService.FairLaunchSetInspector = DisabledFairLaunchSetInspector{}

func PreviewFairLaunchSet(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	var req projectService.FairLaunchSetRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST", "message": e.Error()})
		return
	}
	now := projectService.NowUTC()
	evidence, evidenceErr := projectService.ReadFairLaunchRuntimeEvidence(coredb.Mysql(), r, now)
	preview, e := projectService.PrepareFairLaunchSetPreview(id, req, r, evidence, evidenceErr, FairLaunchSetInspector, now)
	if e == nil {
		adminID, _ := currentInitiator(c)
		payload, marshalErr := json.Marshal(map[string]any{"request": req, "base_digest": preview.CanonicalDigest})
		if marshalErr != nil {
			e = marshalErr
		} else {
			e = coredb.Mysql().Transaction(func(tx *gorm.DB) error {
				row, token, issueErr := projectService.IssueFairLaunchPreviewTx(tx, "set", id, adminID, "", approval.OpFairLaunchSet, preview.CanonicalDigest, payload, now)
				if issueErr == nil {
					preview.PreviewID, preview.PreviewToken, preview.CanonicalDigest = row.ID, token, "0x"+row.CanonicalDigest
				}
				return issueErr
			})
		}
	}
	if e != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "FAIR_LAUNCH_SET_INVALID", "message": e.Error()})
		return
	}
	fairLaunchSuccess(c, preview)
}
func GetFairLaunchSet(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	now := projectService.NowUTC()
	evidence, evidenceErr := projectService.ReadFairLaunchRuntimeEvidence(coredb.Mysql(), r, now)
	if reason := projectService.FairLaunchRuntimeReason(r, evidence, evidenceErr); reason != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_RUNTIME_UNAVAILABLE", "reason_code": reason})
		return
	}
	fairLaunchSuccess(c, gin.H{"configured": false, "locked": false, "status": "", "funding_mode": "fair_launch", "contract_version": "fair_launch_v1", "project_id": id, "runtime_enabled": r.Enabled, "runtime": projectService.FairLaunchAdminRuntimeIdentity(evidence.Runtime, r.Version)})
}
func SubmitFairLaunchSet(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	if !constant.FairLaunchMaterializationEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_SET_DISABLED", "reason_code": constant.FairLaunchMaterializationUnavailable})
		return
	}
	if _, ok := fairLaunchRuntime(c); !ok {
		return
	}
	var req projectService.FairLaunchSetRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST", "message": e.Error()})
		return
	}
	now := projectService.NowUTC()
	initiatorID, initiatorName := currentInitiator(c)
	var row *adminEntity.AdminApproval
	e := coredb.Mysql().Transaction(func(tx *gorm.DB) error {
		persistedPreview, consumeErr := projectService.ConsumeFairLaunchPreviewTx(tx, req.PreviewToken, initiatorID, id, "set", "", approval.OpFairLaunchSet, req.CanonicalDigest, now)
		if consumeErr != nil {
			return consumeErr
		}
		if _, consumeErr = projectService.ConsumeFairLaunchMFAAssertionTx(tx, req.MFAAssertionToken, projectService.FairLaunchMFAContext{AdminID: initiatorID, Operation: approval.OpFairLaunchSet, Stage: projectService.FairLaunchMFAStageSetInitiation, ProjectID: id, PreviewID: &persistedPreview.ID}, now); consumeErr != nil {
			return consumeErr
		}
		var stored struct {
			BaseDigest       string `json:"base_digest"`
			ImmutablePayload struct {
				Request projectService.FairLaunchSetRequest `json:"request"`
			} `json:"immutable_payload"`
		}
		if err := json.Unmarshal([]byte(persistedPreview.CanonicalPayload), &stored); err != nil {
			return errors.New("stored preview malformed")
		}
		stored.ImmutablePayload.Request.CanonicalDigest = stored.BaseDigest
		approval.FairLaunchSetInspector = FairLaunchSetInspector
		payload, err := json.Marshal(approval.FairLaunchSetApprovalPayload{ProjectID: id, Params: stored.ImmutablePayload.Request, PreviewID: persistedPreview.ID})
		if err != nil {
			return err
		}
		row, err = approval.SubmitTx(tx, approval.SubmitReq{Operation: approval.OpFairLaunchSet, TargetKind: "project", TargetID: strconv.FormatUint(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initiatorID, InitiatorName: initiatorName, TTL: 2 * time.Hour})
		return err
	})
	if e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_SET_FAILED", "message": e.Error()})
		return
	}
	fairLaunchSuccess(c, gin.H{"submitted": true, "approval_id": row.ID, "status": row.Status, "expires_at": row.ExpiresAt})
}

type fairLaunchActionRequest = projectService.FairLaunchActionMutationRequest

func fairLaunchActionName(c *gin.Context) string {
	if value := c.Param("action"); value != "" {
		return strings.ReplaceAll(value, "-", "_")
	}
	path := c.FullPath()
	for route, action := range map[string]string{"create-onchain": "create", "activate-onchain": "activate", "withdraw-project-proceeds": "withdraw_project_proceeds", "withdraw-platform-fee": "withdraw_platform_fee", "withdraw-failed-project-tokens": "withdraw_failed_project_tokens", "release": "release", "cancel": "cancel", "end": "end"} {
		if strings.Contains(path, "/"+route) {
			return action
		}
	}
	return ""
}
func FairLaunchActionPreview(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	now := projectService.NowUTC()
	evidence, evidenceErr := projectService.ReadFairLaunchRuntimeEvidence(coredb.Mysql(), r, now)
	caps := projectService.BuildFairLaunchCapabilities(id, r, evidence, evidenceErr)
	action := fairLaunchActionName(c)
	desc, exists := caps.Actions[action]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_UNKNOWN"})
		return
	}
	if !desc.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_DISABLED", "reason_code": desc.ReasonCode})
		return
	}
	var request fairLaunchActionRequest
	if err := c.ShouldBindJSON(&request); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST", "message": err.Error()})
		return
	}
	preview, err := projectService.PrepareFairLaunchActionPreview(id, action, request.Evidence, r, evidence, evidenceErr, now)
	if err == nil {
		adminID, _ := currentInitiator(c)
		payload, marshalErr := json.Marshal(preview.ImmutablePayload)
		if marshalErr != nil {
			err = marshalErr
		} else {
			err = coredb.Mysql().Transaction(func(tx *gorm.DB) error {
				row, token, issueErr := projectService.IssueFairLaunchPreviewTx(tx, "action", id, adminID, action, preview.ApprovalOperation, preview.CanonicalDigest, payload, now)
				if issueErr == nil {
					preview.PreviewID, preview.PreviewToken, preview.CanonicalDigest = row.ID, token, "0x"+row.CanonicalDigest
				}
				return issueErr
			})
		}
	}
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_INVALID", "message": err.Error()})
		return
	}
	fairLaunchSuccess(c, preview)
}
func SubmitFairLaunchAction(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	now := projectService.NowUTC()
	runtimeEvidence, runtimeEvidenceErr := projectService.ReadFairLaunchRuntimeEvidence(coredb.Mysql(), r, now)
	caps := projectService.BuildFairLaunchCapabilities(id, r, runtimeEvidence, runtimeEvidenceErr)
	action := fairLaunchActionName(c)
	desc, exists := caps.Actions[action]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_UNKNOWN"})
		return
	}
	if !desc.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_DISABLED", "reason_code": desc.ReasonCode})
		return
	}
	var request fairLaunchActionRequest
	if err := c.ShouldBindJSON(&request); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST", "message": err.Error()})
		return
	}
	preview, err := projectService.PrepareFairLaunchActionPreview(id, action, request.Evidence, r, runtimeEvidence, runtimeEvidenceErr, now)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_INVALID", "message": err.Error()})
		return
	}
	initiatorID, initiatorName := currentInitiator(c)
	var actionRequest entity.FairLaunchActionRequest
	var row *adminEntity.AdminApproval
	err = coredb.Mysql().Transaction(func(tx *gorm.DB) error {
		created, e := projectService.CreateFairLaunchActionRequestTx(tx, projectService.CreateFairLaunchActionRequestInput{ProjectID: id, Action: action, Evidence: request.Evidence, CanonicalDigest: request.CanonicalDigest, PreviewToken: request.PreviewToken, MFAAssertionToken: request.MFAAssertionToken, InitiatorAdminID: initiatorID, Runtime: r, Now: now})
		if e != nil {
			return e
		}
		actionRequest = created
		payload, e := json.Marshal(approval.FairLaunchActionApprovalPayload{ActionRequestID: actionRequest.ID})
		if e != nil {
			return e
		}
		row, e = approval.SubmitTx(tx, approval.SubmitReq{Operation: approval.FairLaunchActionOperation(action), TargetKind: "project", TargetID: strconv.FormatUint(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initiatorID, InitiatorName: initiatorName, TTL: 2 * time.Hour})
		if e != nil {
			return e
		}
		actionRequest.AdminApprovalID = &row.ID
		return tx.Save(&actionRequest).Error
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_FAILED", "message": err.Error()})
		return
	}
	fairLaunchSuccess(c, gin.H{"submitted": true, "action_request_id": actionRequest.ID, "approval_id": row.ID, "status": row.Status, "expires_at": row.ExpiresAt, "preview": preview})
}

type fairLaunchExecuteRequest struct {
	MFAAssertionToken string `json:"mfa_assertion_token"`
}

func FairLaunchExecutionQueue(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	items, err := projectService.ListFairLaunchExecutionQueue(coredb.Mysql(), id, currentAdminID(c), projectService.NowUTC())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_EXECUTION_QUEUE_UNAVAILABLE", "message": err.Error()})
		return
	}
	fairLaunchSuccess(c, gin.H{"items": items})
}

func ExecuteFairLaunchAction(c *gin.Context) {
	id, ok := fairLaunchProjectID(c)
	if !ok {
		return
	}
	if !constant.FairLaunchMaterializationEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_DISABLED", "reason_code": constant.FairLaunchMaterializationUnavailable})
		return
	}
	requestID, err := strconv.ParseUint(c.Param("requestID"), 10, 64)
	if err != nil || requestID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST_ID"})
		return
	}
	r, ok := fairLaunchRuntime(c)
	if !ok {
		return
	}
	var req fairLaunchExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FAIR_LAUNCH_BAD_REQUEST", "message": err.Error()})
		return
	}
	executorID, _ := currentInitiator(c)
	now := projectService.NowUTC()
	var row entity.FairLaunchActionRequest
	err = coredb.Mysql().Transaction(func(tx *gorm.DB) error {
		executing, e := projectService.ExecuteFairLaunchActionRequestTx(tx, projectService.ExecuteFairLaunchActionRequestInput{RequestID: requestID, ProjectID: id, ExecutorAdminID: executorID, MFAAssertionToken: req.MFAAssertionToken, Runtime: r, Now: now})
		if e != nil {
			return e
		}
		row = executing
		return projectService.DispatchFairLaunchActionTx(tx, &row, r, now)
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "FAIR_LAUNCH_ACTION_EXECUTION_FAILED", "message": err.Error()})
		return
	}
	fairLaunchSuccess(c, gin.H{"executed": true, "action_request_id": row.ID, "status": row.Status, "submission_id": row.SubmissionID, "timelock_operation_id": row.TimelockOperationRowID, "safe_tx_id": row.SafeTxID, "external_observation_ref": row.ExternalObservationRef})
}
