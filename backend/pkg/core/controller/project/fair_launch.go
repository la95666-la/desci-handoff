package project

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/common/projectruntime"
	coredb "joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
)

// Fair Launch financial endpoints are wired now but intentionally fail closed
// until the disabled runtime, CE-V2 materialization and reconciliation reader
// are all available. They never fall back to legacy project data.
func FairLaunchPosition(c *gin.Context) {
	id, e := strconv.ParseUint(c.Param("projectID"), 10, 64)
	if e != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "PROJECT_BAD_ID"})
		return
	}
	r, e := projectruntime.FairLaunchV1()
	if e != nil || !r.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIR_LAUNCH_RUNTIME_DISABLED", "projectId": strconv.FormatUint(id, 10), "actions": gin.H{"claim": gin.H{"available": false, "reason": "runtime_disabled"}, "refund": gin.H{"available": false, "reason": "runtime_disabled"}, "contribute": gin.H{"available": false, "reason": "runtime_disabled"}, "release": gin.H{"available": false, "reason": "operator_only"}}})
		return
	}
	wallet, ok := authenticatedFairLaunchWallet(c)
	if !ok {
		return
	}
	result, err := projectService.ReadFairLaunchPosition(coredb.Mysql(), id, wallet, r)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIR_LAUNCH_POSITION_UNAVAILABLE", "message": err.Error()})
		return
	}
	ret.Success(c, result, nil)
}
func FairLaunchPortfolio(c *gin.Context) {
	r, e := projectruntime.FairLaunchV1()
	if e != nil || !r.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIR_LAUNCH_RUNTIME_DISABLED", "positions": []any{}, "claims": []any{}, "refunds": []any{}})
		return
	}
	wallet, ok := authenticatedFairLaunchWallet(c)
	if !ok {
		return
	}
	positions, err := projectService.ReadFairLaunchPortfolio(coredb.Mysql(), wallet, r)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIR_LAUNCH_PORTFOLIO_UNAVAILABLE", "message": err.Error()})
		return
	}
	ret.Success(c, gin.H{"positions": positions, "claims": []any{}, "refunds": []any{}}, nil)
}

func authenticatedFairLaunchWallet(c *gin.Context) (string, bool) {
	wallet := strings.ToLower(strings.TrimSpace(c.Query("wallet")))
	if wallet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "WALLET_REQUIRED"})
		return "", false
	}
	if !common.IsHexAddress(wallet) || common.HexToAddress(wallet) == (common.Address{}) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "WALLET_BAD_ADDRESS"})
		return "", false
	}
	wallet = strings.ToLower(common.HexToAddress(wallet).Hex())
	uid := tools.UID(c)
	if uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "AUTHENTICATED_WALLET_REQUIRED"})
		return "", false
	}
	var count int64
	if err := coredb.Mysql().Table("user_wallet w").Joins("JOIN user_wallet_identity i ON i.wallet_address=w.wallet_address AND i.uid=w.uid AND i.chain=w.chain").Where("w.uid=? AND w.wallet_address=? AND w.chain='evm' AND w.verified_at IS NOT NULL", uid, wallet).Count(&count).Error; err != nil || count != 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": "AUTHENTICATED_WALLET_MISMATCH"})
		return "", false
	}
	return wallet, true
}
