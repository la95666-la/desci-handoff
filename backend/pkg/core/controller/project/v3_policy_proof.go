package project

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/auth/sign"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/http/tools"
	projectService "joincare_server/pkg/core/service/project"
)

// V3WalletProof is authenticated by the existing signed user route. The
// wallet must be verified and bound to that uid. Neither a posted buyer nor
// Warmup registration is an eligibility authority. No full list is returned.
func V3WalletProof(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	uid := tools.UID(c)
	claims, claimErr := sign.ParseToken(c.GetHeader(constant.HEADER_JOINCARE_SIGN), viper.GetString("jwt.secret"))
	if claimErr != nil || claims.UID != uid || claims.SessionVersion == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "V3_PROOF_AUTH_UNAVAILABLE"})
		return
	}
	projectID, err := strconv.ParseUint(c.Param("projectID"), 10, 64)
	if uid <= 0 || err != nil || projectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "V3_PROOF_SCOPE_INVALID"})
		return
	}
	var proof projectService.V3WalletProof
	// Proof is a read-only membership operation; it does not need the Core
	// signer, original-create admission or scanner release readiness.
	if !projectService.ApprovedV3ModeConfigured() {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_PROOF_UNAVAILABLE"})
		return
	}
	proof, err = projectService.ProveApprovedV3Wallet(uid, projectID, c.Query("wallet"), claims.SessionVersion)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_PROOF_UNAVAILABLE"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": proof})
}
