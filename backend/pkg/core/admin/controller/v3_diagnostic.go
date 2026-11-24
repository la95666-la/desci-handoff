package controller

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/service/project"
)

// Keep legacy top-level codes for API clients. New finite diagnostics are
// additive; underlying SQL/RPC error.Error() is NEVER used for this response.
func respondV3Diagnostic(c *gin.Context, status int, code string, err error) bool {
	f, ok := project.V3Failure(err)
	if !ok {
		return false
	}
	sendV3Diagnostic(c, status, code, f)
	return true
}
func sendV3Diagnostic(c *gin.Context, status int, code string, f project.V3OperationFailure) {
	token := make([]byte, 12)
	requestID := ""
	if _, e := rand.Read(token); e == nil {
		requestID = hex.EncodeToString(token)
	}
	body := gin.H{"status": false, "code": code, "reason_code": f.Reason, "stage": f.Stage, "message": f.Message, "next_action": f.NextAction, "submission_outcome": f.Outcome, "request_id": requestID}
	if f.SubmissionID != "" {
		body["submission_id"] = f.SubmissionID
	}
	if f.TxHash != "" {
		body["tx_hash"] = f.TxHash
	}
	if f.Check != "" {
		body["check"] = f.Check
	}
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(map[string]any{"request_id": requestID, "reason_code": f.Reason, "stage": f.Stage, "code": code, "path": c.FullPath()}).Warn("V3 API rejected")
	}
	c.Header("Cache-Control", "no-store, private")
	c.Header("X-Request-ID", requestID)
	c.JSON(status, body)
}
func policyDiagnostic(c *gin.Context, status int, code string, err error, stage string) {
	if !respondV3Diagnostic(c, status, code, err) {
		reason := "UNAVAILABLE"
		if stage == "mode_ready" {
			reason = "CONFIG_UNAVAILABLE"
		}
		sendV3Diagnostic(c, status, code, project.V3OperationFailure{Reason: reason, Stage: v3diagnostic.Stage(v3diagnostic.New(reason, stage, nil)), Message: v3diagnostic.Message(reason), NextAction: "refresh_and_check", Outcome: "unknown"})
	}
}
