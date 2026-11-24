package controller

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	project "joincare_server/pkg/core/service/project"
)

// Both routes are audited by the existing authenticated Admin middleware.
// preflight never marks/sends; broadcast requires explicit original hash and
// project execute permission rechecked inside the atomic one-shot marker.
func V3OriginalPreflight(c *gin.Context) { v3OriginalContinuation(c, false) }
func V3OriginalBroadcast(c *gin.Context) { v3OriginalContinuation(c, true) }
func v3OriginalContinuation(c *gin.Context, execute bool) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil || id == 0 {
		c.JSON(400, gin.H{"status": false, "code": "BAD_SUBMISSION"})
		return
	}
	var req struct {
		TxHash string `json:"tx_hash"`
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(strings.TrimSpace(req.Reason)) < 3 || utf8.RuneCountInString(req.Reason) > 500 {
		c.JSON(400, gin.H{"status": false, "code": "AUDIT_REASON_REQUIRED"})
		return
	}
	admin := currentAdminID(c)
	result, e := project.ContinueApprovedV3Original(c.Request.Context(), id, req.TxHash, execute, func(tx *gorm.DB) error { return requireCurrentProjectExecuteAdmin(tx, admin) })
	if e != nil {
		if respondV3Diagnostic(c, 409, "V3_ORIGINAL_CONTINUATION_REJECTED", e) {
			return
		}
		c.JSON(409, gin.H{"status": false, "code": "V3_ORIGINAL_CONTINUATION_REJECTED", "message": "Inspect original attempt and hash; do not create or resend a transaction."})
		return
	}
	c.JSON(200, gin.H{"status": true, "data": result})
}
