// engage.go —— Sprint A/E 用户参与行为：Notify me（subscribe）/ Watchlist / Whitelist / UserStatus
// 以及公开 Updates 列表（未登录也可查）。
package project

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/core/db"
)

type projectIDBody struct {
	// json.Number accepts both the legacy JSON number and a decimal string.
	// Browser clients must send Sonyflake IDs as strings to avoid precision loss.
	ProjectID json.Number `json:"project_id" binding:"required"`
}

func (req projectIDBody) projectID() (int64, error) {
	id, err := strconv.ParseInt(string(req.ProjectID), 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

// Subscribe POST /project/v1/subscribe  { project_id }
// idempotent: 已订阅返回 { subscribed: true, changed: false }
func Subscribe(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "AUTH_REQUIRED"})
		return
	}
	var req projectIDBody
	if err := c.ShouldBindJSON(&req); err != nil {
		ret.Error(c, err)
		return
	}
	projectID, err := req.projectID()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "invalid project_id"})
		return
	}

	row := entity.ProjectSubscriber{ProjectID: projectID, UID: uid}
	err = db.Mysql().Where("project_id = ? AND uid = ?", projectID, uid).
		FirstOrCreate(&row).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "SUBSCRIBE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"subscribed": true}})
}

// Unsubscribe DELETE /project/v1/subscribe?id=X
func Unsubscribe(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "AUTH_REQUIRED"})
		return
	}
	pid, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "id required"})
		return
	}
	if err := db.Mysql().Where("project_id = ? AND uid = ?", pid, uid).
		Delete(&entity.ProjectSubscriber{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UNSUBSCRIBE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"subscribed": false}})
}

// Watch POST /project/v1/watch  { project_id }
func Watch(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "AUTH_REQUIRED"})
		return
	}
	var req projectIDBody
	if err := c.ShouldBindJSON(&req); err != nil {
		ret.Error(c, err)
		return
	}
	projectID, err := req.projectID()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "invalid project_id"})
		return
	}
	row := entity.ProjectWatch{ProjectID: projectID, UID: uid}
	if err := db.Mysql().Where("project_id = ? AND uid = ?", projectID, uid).
		FirstOrCreate(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WATCH_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"watched": true}})
}

// Unwatch DELETE /project/v1/watch?id=X
func Unwatch(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "AUTH_REQUIRED"})
		return
	}
	pid, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST"})
		return
	}
	if err := db.Mysql().Where("project_id = ? AND uid = ?", pid, uid).
		Delete(&entity.ProjectWatch{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UNWATCH_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"watched": false}})
}

type joinWhitelistReq struct {
	ProjectID     json.Number `json:"project_id" binding:"required"`
	WalletAddress string      `json:"wallet_address" binding:"required,max=42"`
	Note          string      `json:"note" binding:"max=200"`
}

// JoinWhitelist POST /project/v1/whitelist
// Warmup signup is not an on-chain purchase eligibility assertion.
// A V3 project must never receive the historical auto-approved signup result.
func JoinWhitelist(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "AUTH_REQUIRED"})
		return
	}
	var req joinWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ret.Error(c, err)
		return
	}
	projectID, err := (projectIDBody{ProjectID: req.ProjectID}).projectID()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST", "message": "invalid project_id"})
		return
	}
	version := ""
	if err := db.Mysql().Model(&entity.Project{}).Select("contract_version").Where("project_id = ?", projectID).Scan(&version).Error; err != nil || version == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "PROJECT_IDENTITY_UNAVAILABLE"})
		return
	}
	if version == constant.ContractVersionIDOV3Stablecoin {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "V3_WHITELIST_SIGNUP_UNAVAILABLE", "message": "V3 whitelist eligibility requires a separately governed policy"})
		return
	}
	row := entity.ProjectWhitelist{
		ProjectID:     projectID,
		UID:           uid,
		WalletAddress: req.WalletAddress,
		Note:          req.Note,
		Approved:      1,
	}
	err = db.Mysql().Where("project_id = ? AND uid = ?", projectID, uid).
		FirstOrCreate(&row).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WHITELIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"whitelisted": true, "approved": row.Approved == 1}})
}

// UserStatus GET /project/v1/userStatus?id=X
// 返回当前用户对该 project 的 3 状态标记，供前端一次性渲染 CTA
func UserStatus(c *gin.Context) {
	uid := tools.UID(c)
	pid, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST"})
		return
	}
	res := gin.H{"subscribed": false, "watched": false, "whitelisted": false}
	// Determine identity even for anonymous users; a stale Warmup flag must
	// never masquerade as V3 whitelist eligibility in this response.
	version := ""
	if err := db.Mysql().Model(&entity.Project{}).Select("contract_version").Where("project_id = ?", pid).Scan(&version).Error; err != nil || version == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "PROJECT_IDENTITY_UNAVAILABLE"})
		return
	}
	if uid == 0 {
		if version == constant.ContractVersionIDOV3Stablecoin {
			res["whitelist_eligibility"] = "unavailable"
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "data": res})
		return
	}
	var s entity.ProjectSubscriber
	if err := db.Mysql().Where("project_id = ? AND uid = ?", pid, uid).Take(&s).Error; err == nil {
		res["subscribed"] = true
	} else if err != gorm.ErrRecordNotFound {
		// 非 not-found 的 err 不阻塞返回，log warn 就够（本 endpoint 不 critical）
	}
	var w entity.ProjectWatch
	if err := db.Mysql().Where("project_id = ? AND uid = ?", pid, uid).Take(&w).Error; err == nil {
		res["watched"] = true
	}
	// Historical Warmup rows (including approved=1) can never be presented as
	// V3 policy membership. No V3 proof or eligibility data is served here.
	if version != constant.ContractVersionIDOV3Stablecoin {
		var wl entity.ProjectWhitelist
		if err := db.Mysql().Where("project_id = ? AND uid = ?", pid, uid).Take(&wl).Error; err == nil {
			res["whitelisted"] = true
			res["whitelist_approved"] = wl.Approved == 1
		}
	} else {
		res["whitelist_eligibility"] = "unavailable"
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": res})
}

// PublicUpdates GET /project/v1/updates?id=X&page=1&size=20
// 公开访问：只列 is_published=1
func PublicUpdates(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "BAD_REQUEST"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size < 1 || size > 100 {
		size = 20
	}

	var total int64
	db.Mysql().Model(&entity.ProjectUpdate{}).
		Where("project_id = ? AND is_published = 1", pid).
		Count(&total)

	var rows []entity.ProjectUpdate
	err = db.Mysql().Model(&entity.ProjectUpdate{}).
		Select("id, project_id, author_admin_id, title, content_html, published_at").
		Where("project_id = ? AND is_published = 1", pid).
		Order("published_at DESC, id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "UPDATES_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}
