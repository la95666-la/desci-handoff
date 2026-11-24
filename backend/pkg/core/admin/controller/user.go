package controller

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/third/ops"
)

const userRankQuery = `
	SELECT up.uid AS uid, ranked.rk AS rk
	FROM user_point up
	JOIN (
		SELECT point, 1 + COALESCE(SUM(n) OVER (
			ORDER BY point DESC ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
		), 0) AS rk
		FROM (SELECT point, COUNT(*) AS n FROM user_point GROUP BY point) grouped
	) ranked ON ranked.point = up.point
	WHERE up.uid IN ?`

// loadUserRanks calculates competition ranks for one displayed page in one
// bounded query. Scores without a user_point row retain the UI's 0 rank.
func loadUserRanks(ctx context.Context, database *gorm.DB, uids []int64) (map[int64]int, error) {
	ranksByUID := make(map[int64]int, len(uids))
	if len(uids) == 0 {
		return ranksByUID, nil
	}
	var ranks []struct {
		UID  int64 `gorm:"column:uid"`
		Rank int   `gorm:"column:rk"`
	}
	if err := database.WithContext(ctx).Raw(userRankQuery, uids).Scan(&ranks).Error; err != nil {
		return nil, err
	}
	for _, rank := range ranks {
		ranksByUID[rank.UID] = rank.Rank
	}
	return ranksByUID, nil
}

type userListItem struct {
	ID               int64  `json:"id"`
	WalletAddress    string `json:"wallet_address"`
	TwitterID        string `json:"twitter_id"`
	TwitterHandle    string `json:"twitter_handle"`
	TwitterDisplay   string `json:"twitter_display_name"`
	TwitterAvatar    string `json:"twitter_avatar"`
	TelegramID       string `json:"telegram_id"`
	TelegramUsername string `json:"telegram_username"`
	Type             string `json:"type"`
	IsEnable         int8   `json:"is_enable"`
	Point            int    `json:"point"`
	Rank             int    `json:"rank"` // 全站积分排名（并列名次算 ties）
	LastLoginAt      string `json:"last_login_at"`
	LastLoginIP      string `json:"last_login_ip"`
	CreatedAt        string `json:"created_at"`
}

type userListResp struct {
	Items    []userListItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// ListUsers GET /admin/v1/users?page=1&page_size=20&keyword=xxx&sort=point_desc
// sort: point_desc(默认) / point_asc / id_desc / id_asc
func ListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	keyword := c.Query("keyword")

	// 排序：默认按积分降序（前端"排名"体验的主用场景）
	orderClause := "COALESCE(up.point, 0) DESC, u.id ASC"
	switch c.DefaultQuery("sort", "point_desc") {
	case "point_asc":
		orderClause = "COALESCE(up.point, 0) ASC, u.id ASC"
	case "id_desc":
		orderClause = "u.id DESC"
	case "id_asc":
		orderClause = "u.id ASC"
	}

	// LEFT JOIN user_point 拉积分；用 alias 避免和 gorm 反射自动加 SELECT 冲突
	q := db.Mysql().WithContext(ctx).Table("user AS u").
		Joins("LEFT JOIN user_point up ON up.uid = u.id")
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where(
			"u.wallet_address LIKE ? OR u.twitter_handle LIKE ? OR u.twitter_display_name LIKE ? OR u.telegram_username LIKE ?",
			like, like, like, like,
		)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_LIST_FAILED", "message": err.Error()})
		return
	}

	type joinedRow struct {
		entity.User
		Point int `gorm:"column:point"`
	}
	var rows []joinedRow
	if err := q.
		Select("u.*, COALESCE(up.point, 0) AS point").
		Order(orderClause).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_LIST_FAILED", "message": err.Error()})
		return
	}

	// 计算每行的全站排名（同分并列）
	// 一次批量取：SELECT uid, RANK() OVER (ORDER BY point DESC) rk FROM user_point WHERE uid IN (...)
	uids := make([]int64, 0, len(rows))
	for _, r := range rows {
		uids = append(uids, r.User.ID)
	}
	// Compute competition ranks once per distinct score. A correlated
	// COUNT(*) per displayed user repeatedly scans the entire points table.
	rankMap, err := loadUserRanks(ctx, db.Mysql(), uids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_RANK_FAILED"})
		return
	}

	items := make([]userListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, userListItem{
			ID:               r.User.ID,
			WalletAddress:    r.User.WalletAddress,
			TwitterID:        r.User.TwitterID,
			TwitterHandle:    r.User.TwitterHandle,
			TwitterDisplay:   r.User.TwitterDisplayName,
			TwitterAvatar:    r.User.TwitterAvatar,
			TelegramID:       r.User.TelegramID,
			TelegramUsername: r.User.TelegramUsername,
			Type:             r.User.Type,
			IsEnable:         r.User.IsEnable,
			Point:            r.Point,
			Rank:             rankMap[r.User.ID], // 0 表示没积分记录（未上榜）
			LastLoginAt:      r.User.LastLoginAt.Format("2006-01-02 15:04:05"),
			LastLoginIP:      r.User.LastLoginIP,
			CreatedAt:        r.User.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": userListResp{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// helper：读取当前发起人身份
func currentInitiator(c *gin.Context) (uid uint64, name string) {
	if v, ok := c.Get(middleware.CtxAdminID); ok {
		if i, ok := v.(int64); ok {
			uid = uint64(i)
		}
	}
	if v, ok := c.Get(middleware.CtxUsername); ok {
		if s, ok := v.(string); ok {
			name = s
		}
	}
	return
}

// EnableUser POST /admin/v1/users/:id/enable → 高风险，走 4 眼审批
func EnableUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.UserActionPayload{UID: id})
	row, err := approval.Submit(approval.SubmitReq{
		Operation: approval.OpUserEnable, TargetKind: "user",
		TargetID: strconv.FormatInt(id, 10),
		Payload:  string(payload), Reason: c.Query("reason"),
		InitiatorID: initID, InitiatorName: initName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	ops.PostAlert(c.Request.Context(), "INFO", "用户启用申请已提交（待 4 眼审批）",
		fmt.Sprintf("uid=%d approval_id=%d ip=%s", id, row.ID, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"approval_id": row.ID, "status": "pending",
		"message": "submitted, waiting for approval",
	}})
}

// DisableUser POST /admin/v1/users/:id/disable → 高风险，走 4 眼审批
func DisableUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.UserActionPayload{UID: id})
	row, err := approval.Submit(approval.SubmitReq{
		Operation: approval.OpUserDisable, TargetKind: "user",
		TargetID: strconv.FormatInt(id, 10),
		Payload:  string(payload), Reason: c.Query("reason"),
		InitiatorID: initID, InitiatorName: initName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	ops.PostAlert(c.Request.Context(), "WARN", "用户停用申请已提交（待 4 眼审批）",
		fmt.Sprintf("uid=%d approval_id=%d ip=%s", id, row.ID, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"approval_id": row.ID, "status": "pending",
	}})
}

type adjustPointReq struct {
	Delta  int    `json:"delta" binding:"required"`
	Reason string `json:"reason"`
}

// AdjustUserPoint POST /admin/v1/users/:id/adjust-point
// |delta| ≥ HighRiskPointThreshold（1000） → 4 眼审批
// 小额（1000 以内）走当前直发
func AdjustUserPoint(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}
	var req adjustPointReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_ADJUST_BAD_REQUEST", "message": err.Error()})
		return
	}
	abs := req.Delta
	if abs < 0 {
		abs = -abs
	}
	initID, initName := currentInitiator(c)

	if abs >= approval.HighRiskPointThreshold {
		// 提审
		payload, _ := json.Marshal(approval.UserAdjustPayload{UID: id, Delta: req.Delta, Reason: req.Reason})
		row, err := approval.Submit(approval.SubmitReq{
			Operation: approval.OpUserAdjustPoint, TargetKind: "user",
			TargetID: strconv.FormatInt(id, 10),
			Payload:  string(payload), Reason: req.Reason,
			InitiatorID: initID, InitiatorName: initName,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_SUBMIT_FAILED", "message": err.Error()})
			return
		}
		ops.PostAlert(c.Request.Context(), "WARN", "大额积分调整申请（待 4 眼审批）",
			fmt.Sprintf("uid=%d delta=%d reason=%q approval_id=%d ip=%s", id, req.Delta, req.Reason, row.ID, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
			"approval_id": row.ID, "status": "pending",
			"message": fmt.Sprintf("|delta|>=%d requires 4-eye approval", approval.HighRiskPointThreshold),
		}})
		return
	}
	// 小额直发
	result, err := approval.RunHandlerDirect(approval.OpUserAdjustPoint, approval.UserAdjustPayload{
		UID: id, Delta: req.Delta, Reason: req.Reason,
	}, initID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_ADJUST_FAILED", "message": err.Error()})
		return
	}
	ops.PostAlert(c.Request.Context(), "INFO", "用户积分被手动调整（小额直发）",
		fmt.Sprintf("uid=%d delta=%d reason=%q ip=%s", id, req.Delta, req.Reason, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"id": id, "delta": req.Delta, "result": result,
	}})
}

// 兼容旧 entity 引用
var _ = entity.User{}

// UnbindTwitter POST /admin/v1/users/:id/unbind-twitter
// 解绑用户 X（Twitter）：清 user 表 twitter_* 字段 + 删 Redis token；小额直发（不走审批）。
// 语义：只是切断绑定关系，不撤销已获得的 X 任务积分（那属于人工审核范畴）。
func UnbindTwitter(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}

	var u entity.User
	if err := db.Mysql().Select("id, twitter_id, twitter_handle").Where("id = ?", id).First(&u).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USER_NOT_FOUND", "message": err.Error()})
		return
	}
	if u.TwitterID == "" {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "unbound": false, "message": "not bound"}})
		return
	}
	prevHandle := u.TwitterHandle
	prevTwitterID := u.TwitterID

	// twitter_id 列有 uk_twitter_id 唯一索引，允许 NULL；用 NULL 而不是空串避免以后重绑同号冲突
	err = db.Mysql().Model(&entity.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"twitter_id":           gorm.Expr("NULL"),
		"twitter_handle":       "",
		"twitter_display_name": "",
		"twitter_avatar":       "",
	}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_UNBIND_FAILED", "message": err.Error()})
		return
	}
	// 删 Redis 里的 X access token，避免残留 token 让 completeTask 误以为还绑着
	_ = db.Redis().Del(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, id))

	initID, initName := currentInitiator(c)
	ops.PostAlert(c.Request.Context(), "WARN", "管理员解绑用户 X",
		fmt.Sprintf("uid=%d prev_twitter_id=%s prev_handle=@%s admin=%s(#%d) ip=%s",
			id, prevTwitterID, prevHandle, initName, initID, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"id": id, "unbound": true, "prev_twitter_id": prevTwitterID, "prev_handle": prevHandle,
	}})
}

// UnbindTelegram POST /admin/v1/users/:id/unbind-telegram
// 解绑用户 Telegram：清 user 表 telegram_* 字段；不撤销已发放积分。
func UnbindTelegram(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}

	var u entity.User
	if err := db.Mysql().Select("id, telegram_id, telegram_username").Where("id = ?", id).First(&u).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USER_NOT_FOUND", "message": err.Error()})
		return
	}
	if u.TelegramID == "" {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "unbound": false, "message": "not bound"}})
		return
	}
	prevTgID := u.TelegramID
	prevTgUser := u.TelegramUsername

	// telegram_id 列有 uk_telegram_id 唯一索引，允许 NULL；用 NULL 避免以后重绑冲突
	err = db.Mysql().Model(&entity.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"telegram_id":         gorm.Expr("NULL"),
		"telegram_username":   "",
		"telegram_first_name": "",
		"telegram_last_name":  "",
		"telegram_avatar":     "",
	}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USER_UNBIND_FAILED", "message": err.Error()})
		return
	}

	initID, initName := currentInitiator(c)
	ops.PostAlert(c.Request.Context(), "WARN", "管理员解绑用户 Telegram",
		fmt.Sprintf("uid=%d prev_telegram_id=%s prev_username=@%s admin=%s(#%d) ip=%s",
			id, prevTgID, prevTgUser, initName, initID, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"id": id, "unbound": true, "prev_telegram_id": prevTgID, "prev_username": prevTgUser,
	}})
}

// GetUser GET /admin/v1/users/:id
func GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USER_BAD_ID", "message": "invalid id"})
		return
	}

	var u entity.User
	if err := db.Mysql().Where("id = ?", id).First(&u).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USER_NOT_FOUND", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": userListItem{
			ID:             u.ID,
			WalletAddress:  u.WalletAddress,
			TwitterID:      u.TwitterID,
			TwitterHandle:  u.TwitterHandle,
			TwitterDisplay: u.TwitterDisplayName,
			TwitterAvatar:  u.TwitterAvatar,
			Type:           u.Type,
			IsEnable:       u.IsEnable,
			LastLoginAt:    u.LastLoginAt.Format("2006-01-02 15:04:05"),
			LastLoginIP:    u.LastLoginIP,
			CreatedAt:      u.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}
