package controller

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"strconv"

	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/v3diagnostic"
	"joincare_server/pkg/core/db"
)

// Read-only operational gate, not approval or a token reusable for signing.
// It returns no RPC URLs, raw signatures, credentials, or arbitrary SQL text.
func V3SenderReadiness(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	e := chaintx.RequireV3SenderReady(c.Request.Context(), db.Mysql(), 56, common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"))
	data := gin.H{"ready": e == nil, "signing_authority": false, "minimum_evidence_runway_seconds": int64(chaintx.SenderPreflightReserve.Seconds())}
	window, windowErr := chaintx.ReadV3SenderMaintenanceWindow(c.Request.Context(), db.Mysql(), 56, common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"))
	if windowErr == nil {
		if window.RemainingSeconds != nil {
			data["evidence_remaining_seconds"] = *window.RemainingSeconds
		}
		if window.ActiveSubmission != nil {
			// An active slot takes diagnostic precedence even when old retired
			// evidence also lacks runway. No hash or resend authority exposed.
			data["active_submission_id"] = strconv.FormatUint(*window.ActiveSubmission, 10)
			data["ready"] = false
			e = v3diagnostic.New("NONCE_UNRESOLVED", "nonce_check", nil)
		}
	}
	if e != nil {
		data["reason_code"], data["stage"], data["next_action"] = v3diagnostic.Reason(e), v3diagnostic.Stage(e), "check_sender_maintenance"
		if v3diagnostic.Reason(e) == "NONCE_UNRESOLVED" {
			data["next_action"] = "reconcile_original_do_not_resend"
		}
	}
	c.JSON(200, gin.H{"status": true, "data": data})
}
