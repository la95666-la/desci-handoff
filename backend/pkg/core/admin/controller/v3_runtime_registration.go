package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/core/admin/service/approval"
	projectService "joincare_server/pkg/core/service/project"
)

// SubmitV3RuntimeRegistration performs read-only chain admission before
// asking a separate administrator to approve one immutable runtime identity.
// The Admin cannot choose a proxy, hash, owner, fee recipient or signer.
func SubmitV3RuntimeRegistration(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	if err := projectService.PreviewV3RuntimeRegistration(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "V3_RUNTIME_UNAVAILABLE"})
		return
	}
	adminID, name := currentInitiator(c)
	row, err := approval.Submit(approval.SubmitReq{
		Operation:  approval.OpV3RuntimeRegistration,
		TargetKind: "ido_v3_runtime", TargetID: "56", Payload: "{}",
		Reason: "BSC V3 runtime identity registration", InitiatorID: adminID, InitiatorName: name,
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_RUNTIME_REVIEW_UNAVAILABLE"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": row.Status}})
}
