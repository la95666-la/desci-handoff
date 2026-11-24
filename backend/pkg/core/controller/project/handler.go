package project

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/common/i18n"
	coredb "joincare_server/pkg/core/db"
	"joincare_server/pkg/core/service/project"
)

// List 获取项目列表
func List(c *gin.Context) {
	var req ListRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.List(req.Cursor, req.Status, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// Detail 获取项目详情
func Detail(c *gin.Context) {
	var req DetailRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.Detail(tools.UID(c), req.ProjectID, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// PendingClaimList 获取待领取列表
func PendingClaimList(c *gin.Context) {
	var req PendingClaimListRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.PendingClaimList(tools.UID(c), req.Cursor, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// PurchaseRecord 获取认购记录
func PurchaseRecord(c *gin.Context) {
	var req PurchaseRecordRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.PurchaseRecord(tools.UID(c), req.Cursor, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// ClaimRecord 获取领取记录
func ClaimRecord(c *gin.Context) {
	var req ClaimRecordRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.ClaimRecord(tools.UID(c), req.Cursor, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

func MyProject(c *gin.Context) {
	res, err := project.MyProject(tools.UID(c), tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// Investors 项目方分页查看投资记录；V2 ledger / V3 canonical effects 独立读取。
func Investors(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("projectID"))
	if err != nil || projectID <= 0 {
		ret.Error(c, ret.CusError(i18n.ERR_000_00003))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	res, err := project.ProjectInvestors(tools.UID(c), projectID, page, pageSize)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// RefundRecord 获取退款记录
func RefundRecord(c *gin.Context) {
	var req ClaimRecordRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	res, err := project.RefundRecord(tools.UID(c), req.Cursor, tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// V2Portfolio returns stablecoin-era wallet positions and immutable history.
func V2Portfolio(c *gin.Context) {
	res, err := project.V2Portfolio(tools.UID(c), tools.Lang(c))
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// IDOV3Portfolio returns the authenticated, verified wallet's canonical V3
// positions. The explicit wallet is cross-checked against the signed session;
// it can never be used to inspect another account.
func IDOV3Portfolio(c *gin.Context) {
	wallet := strings.ToLower(strings.TrimSpace(c.Query("wallet")))
	if !common.IsHexAddress(wallet) || common.HexToAddress(wallet) == (common.Address{}) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "WALLET_BAD_ADDRESS"})
		return
	}
	wallet = strings.ToLower(common.HexToAddress(wallet).Hex())
	uid := tools.UID(c)
	var count int64
	if uid <= 0 || coredb.Mysql().Table("user_wallet w").
		Joins("JOIN user_wallet_identity i ON i.wallet_address=w.wallet_address AND i.uid=w.uid AND i.chain=w.chain").
		Where("w.uid=? AND w.wallet_address=? AND w.chain='evm' AND w.verified_at IS NOT NULL", uid, wallet).
		Count(&count).Error != nil || count != 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": "AUTHENTICATED_WALLET_MISMATCH"})
		return
	}
	res, err := project.IDOV3Portfolio(wallet, tools.Lang(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "IDO_V3_PORTFOLIO_UNAVAILABLE", "message": err.Error()})
		return
	}
	ret.Success(c, res, nil)
}

// V2FundingState 服务端按登录钱包地址解析指定项目的余额/授权/认购数据，
// 不依赖浏览器钱包 provider，避免多钱包扩展注入冲突导致读不到链上数据。
func V2FundingState(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("projectID"))
	if err != nil || projectID <= 0 {
		ret.Error(c, ret.CusError(i18n.ERR_000_00003))
		return
	}
	res, err := project.FundingState(tools.UID(c), projectID)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, res, nil)
}

// NewProject 创建项目审核
func NewProject(c *gin.Context) {
	var req project.NewProjectRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	err = project.NewProject(tools.UID(c), req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, nil, ret.CusError(i18n.SCC_000_00001))
}

func SubmitProjectReview(c *gin.Context) {
	var req project.SubmitProjectReviewRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	err = project.SubmitProjectReview(tools.UID(c), req)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, nil, ret.CusError(i18n.SCC_000_00001))
}
