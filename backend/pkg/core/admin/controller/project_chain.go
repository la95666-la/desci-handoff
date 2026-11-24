package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/log"
	projectService "joincare_server/pkg/core/service/project"
)

// CreateProjectOnChain POST /admin/v1/projects/:id/create-onchain
func CreateProjectOnChain(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid project id"})
		return
	}
	adminID := currentAdminID(c)
	submission, err := projectService.CreateWithGuard(projectID, func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		if respondV3Diagnostic(c, http.StatusBadRequest, "PROJECT_CREATE_ONCHAIN_FAILED", err) {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_CREATE_ONCHAIN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"project_id": projectID, "status": submission.Status, "submitted": submission.Submitted, "tx_hash": submission.TxHash}})
}

// ActivateProjectOnChain POST /admin/v1/projects/:id/activate-onchain
func ActivateProjectOnChain(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid project id"})
		return
	}
	adminID := currentAdminID(c)
	submission, err := projectService.ActiveWithGuard(projectID, func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		if errors.Is(err, projectService.ErrV3ActivationTokenReserveInsufficient) {
			respondV3InsufficientTokens(c)
			return
		}
		var insufficient *projectService.InsufficientProjectTokensError
		if errors.As(err, &insufficient) {
			respondV2InsufficientTokens(c, insufficient)
			return
		}
		// A relayer revert surfaced during gas estimation (for example a node
		// that did not expose revert data) is mapped back to the stable
		// business error instead of echoing raw RPC internals.
		if projectService.IsInsufficientProjectTokensRPCError(err) {
			shortfall, recomputeErr := projectService.ActivationTokenShortfallFromProject(int64(projectID))
			if recomputeErr == nil && shortfall != nil {
				respondV2InsufficientTokens(c, shortfall)
				return
			}
			c.JSON(http.StatusConflict, gin.H{"status": false, "code": "PROJECT_TOKEN_RESERVE_INSUFFICIENT", "message": projectService.V2ActivationInsufficientUnknownMessage()})
			return
		}
		if projectService.IsRelayerInternalError(err) {
			c.JSON(http.StatusBadGateway, gin.H{"status": false, "code": "PROJECT_ACTIVATE_RELAYER_UNAVAILABLE", "message": projectService.V2ActivationRelayerUnavailableMessage()})
			return
		}
		if respondV3Diagnostic(c, http.StatusBadRequest, "PROJECT_ACTIVATE_ONCHAIN_FAILED", err) {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_ACTIVATE_ONCHAIN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"project_id": projectID, "status": submission.Status, "submitted": submission.Submitted, "tx_hash": submission.TxHash}})
}

func respondV3InsufficientTokens(c *gin.Context) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"status": false, "code": "PROJECT_TOKEN_RESERVE_INSUFFICIENT",
		"message": "项目代币储备不足。请项目方使用项目 Owner 钱包通过 depositProjectTokens 存入足额项目代币，确认链上入账后再激活；不要直接转账到合约地址。"})
}

// EndProjectOnChain POST /admin/v1/projects/:id/end
func EndProjectOnChain(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid project id"})
		return
	}
	adminID := currentAdminID(c)
	submission, err := projectService.EndWithGuard(projectID, func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		if respondV3Diagnostic(c, http.StatusBadRequest, "PROJECT_END_ONCHAIN_FAILED", err) {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_END_ONCHAIN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"project_id": projectID, "status": submission.Status, "submitted": submission.Submitted, "tx_hash": submission.TxHash}})
}

// CancelProjectOnChain POST /admin/v1/projects/:id/cancel
func CancelProjectOnChain(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid project id"})
		return
	}
	adminID := currentAdminID(c)
	submission, err := projectService.CancelWithGuard(projectID, func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		log.GetLogger().WithError(err).WithField("project_id", projectID).Error("project cancel on-chain failed")
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_CANCEL_ONCHAIN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"project_id": projectID, "status": submission.Status, "submitted": submission.Submitted, "tx_hash": submission.TxHash}})
}

// WithdrawPlatformFee POST /admin/v1/projects/:id/withdraw-platform-fee
// 平台侧提取成功项目的平台费（合约 owner-only，入 feeReceiver）；写操作自动入审计日志。
func WithdrawPlatformFee(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PROJECT_BAD_ID", "message": "invalid project id"})
		return
	}
	adminID := currentAdminID(c)
	submission, err := projectService.WithdrawPlatformFeeWithGuard(projectID, func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		if respondV3Diagnostic(c, http.StatusBadRequest, "PLATFORM_FEE_WITHDRAW_FAILED", err) {
			return
		}
		log.GetLogger().WithError(err).WithField("project_id", projectID).Error("platform fee withdraw failed")
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "PLATFORM_FEE_WITHDRAW_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"project_id": projectID, "status": submission.Status, "submitted": submission.Submitted, "tx_hash": submission.TxHash}})
}

// PauseIDOV2 POST /admin/v1/ido/pause（幂等：已暂停直接返回当前状态）
// 安全审计 HIGH-1 修复：走 durable outbox（global_pause intent），Paused 事件推进 applied；
// LOW-1 修复：与其它链上写端点一致，事务内复核管理员项目执行权限，POST 经 Audit 中间件落 admin_audit_log。
func PauseIDOV2(c *gin.Context) {
	adminID := currentAdminID(c)
	paused, txHash, err := projectService.PauseIDOV2WithGuard(func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		log.GetLogger().WithError(err).Error("IDO V2 pause failed")
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_PAUSE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"paused": paused, "tx_hash": txHash}})
}

// UnpauseIDOV2 POST /admin/v1/ido/unpause（幂等：未暂停直接返回当前状态）
// 同 PauseIDOV2：durable outbox + 管理员权限复核 + 审计日志。
func UnpauseIDOV2(c *gin.Context) {
	adminID := currentAdminID(c)
	paused, txHash, err := projectService.UnpauseIDOV2WithGuard(func(tx *gorm.DB) error {
		return requireCurrentProjectExecuteAdmin(tx, adminID)
	})
	if err != nil {
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "PROJECT_EXECUTE_REVOKED", "message": err.Error()})
			return
		}
		log.GetLogger().WithError(err).Error("IDO V2 unpause failed")
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_UNPAUSE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"paused": paused, "tx_hash": txHash}})
}

// IDOPausedState GET /admin/v1/ido/pause-state
func IDOPausedState(c *gin.Context) {
	paused, err := projectService.IDOPausedState()
	if err != nil {
		log.GetLogger().WithError(err).Error("IDO V2 pause state read failed")
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_PAUSE_STATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"paused": paused}})
}

// respondV2InsufficientTokens renders the stable insufficient-token business
// error with exact raw amounts, symbol and decimals. RPC internals are never
// echoed by this path.
func respondV2InsufficientTokens(c *gin.Context, shortfall *projectService.InsufficientProjectTokensError) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"status":  false,
		"code":    "PROJECT_TOKEN_RESERVE_INSUFFICIENT",
		"message": projectService.V2ActivationInsufficientMessage(),
		"data": gin.H{
			"current":   shortfall.CurrentRaw,
			"required":  shortfall.RequiredRaw,
			"shortfall": shortfall.ShortfallRaw,
			"symbol":    shortfall.Symbol,
			"decimals":  shortfall.Decimals,
		},
	})
}
