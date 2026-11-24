package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

type tgActivityReq struct {
	ChatID     int64  `json:"chat_id" binding:"required"`
	ChatTitle  string `json:"chat_title"`
	TgUserID   int64  `json:"tg_user_id" binding:"required"`
	TgUsername string `json:"tg_username"`
}

// ReportTgActivity POST /admin/v1/tg/activity  (由 tgbot 每条消息上报)
// 同 (chat_id, tg_user_id, day) 三元组只 UPSERT 一行；message_count += 1
// 尝试查关联 joincare uid（telegram_id 匹配 → 一次性 SET）
func ReportTgActivity(c *gin.Context) {
	var req tgActivityReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_ACT_BAD_REQUEST", "message": err.Error()})
		return
	}
	day := time.Now().Format("2006-01-02")

	// 关联 joincare uid（若 user 表 telegram_id 匹配）
	var uidPtr *int64
	var uid int64
	if err := db.Mysql().
		Model(&entity.User{}).
		Where("telegram_id = ?", strconv.FormatInt(req.TgUserID, 10)).
		Select("id").
		Take(&uid).Error; err == nil && uid > 0 {
		uidPtr = &uid
	}

	// UPSERT via raw SQL；gorm Clauses OnConflict 对复合唯一键 + 递增更方便
	res := db.Mysql().Exec(`
		INSERT INTO tg_daily_activity
		  (chat_id, chat_title, tg_user_id, tg_username, uid, day, message_count, first_msg_at, last_msg_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		  message_count = message_count + 1,
		  last_msg_at   = NOW(),
		  chat_title    = VALUES(chat_title),
		  tg_username   = VALUES(tg_username),
		  uid           = COALESCE(uid, VALUES(uid))
	`, req.ChatID, req.ChatTitle, req.TgUserID, req.TgUsername, uidPtr, day)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "TG_ACT_WRITE_FAILED", "message": res.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"day": day}})
}

type dailyStatRow struct {
	Day           string `json:"day" gorm:"column:day"`
	ChatID        int64  `json:"chat_id" gorm:"column:chat_id"`
	ChatTitle     string `json:"chat_title" gorm:"column:chat_title"`
	ActiveUsers   int    `json:"active_users" gorm:"column:active_users"`
	TotalMessages int    `json:"total_messages" gorm:"column:total_messages"`
	BindedUsers   int    `json:"binded_users" gorm:"column:binded_users"`
}

// TgDailyStats GET /admin/v1/tg-activity/daily?from=&to=&chat_id=
// 按天 + chat 聚合：活跃用户数 / 总消息数 / 已绑定 joincare 的用户数
func TgDailyStats(c *gin.Context) {
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))

	q := db.Mysql().Table("tg_daily_activity").
		Select("day, chat_id, MAX(chat_title) AS chat_title, COUNT(*) AS active_users, SUM(message_count) AS total_messages, SUM(CASE WHEN uid IS NOT NULL THEN 1 ELSE 0 END) AS binded_users").
		Where("day >= ? AND day <= ?", from, to)
	if v := c.Query("chat_id"); v != "" {
		q = q.Where("chat_id = ?", v)
	}
	var rows []dailyStatRow
	if err := q.Group("day, chat_id").Order("day DESC, chat_id ASC").Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "TG_ACT_STATS_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "from": from, "to": to,
	}})
}

type tgChatOption struct {
	ChatID    int64  `json:"chat_id" gorm:"column:chat_id"`
	ChatTitle string `json:"chat_title" gorm:"column:chat_title"`
	Days      int    `json:"days" gorm:"column:days"`
}

// TgChats GET /admin/v1/tg-activity/chats
// 返所有出现过的 chat（下拉用）
func TgChats(c *gin.Context) {
	var rows []tgChatOption
	err := db.Mysql().Table("tg_daily_activity").
		Select("chat_id, MAX(chat_title) AS chat_title, COUNT(DISTINCT day) AS days").
		Group("chat_id").
		Order("days DESC").
		Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "TG_CHATS_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows}})
}

type tgUserSummaryRow struct {
	UID            *int64 `json:"uid" gorm:"column:uid"`
	TgUserID       int64  `json:"tg_user_id" gorm:"column:tg_user_id"`
	TgUsername     string `json:"tg_username" gorm:"column:tg_username"`
	WalletAddress  string `json:"wallet_address" gorm:"column:wallet_address"`
	TwitterHandle  string `json:"twitter_handle" gorm:"column:twitter_handle"`
	TotalMessages  int    `json:"total_messages" gorm:"column:total_messages"`
	ActiveDays     int    `json:"active_days" gorm:"column:active_days"`
	FirstActiveDay string `json:"first_active_day" gorm:"column:first_active_day"`
	LastActiveDay  string `json:"last_active_day" gorm:"column:last_active_day"`
}

// TgUserActivity GET /admin/v1/tg-activity/user-summary?chat_id=&from=&to=&bound_only=&page=&page_size=&sort=
// 用户维度聚合：某群 + 某时段，每个 (tg_user_id) 一行
// sort: msgs_desc(默认) / days_desc / recent_desc
// bound_only: 1(默认，仅注册用户) / 0(含未绑定 TG 用户)
func TgUserActivity(c *gin.Context) {
	chatID := c.Query("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "TG_USER_ACT_NEED_CHAT", "message": "chat_id required"})
		return
	}
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}

	order := "total_messages DESC, active_days DESC"
	switch c.DefaultQuery("sort", "msgs_desc") {
	case "days_desc":
		order = "active_days DESC, total_messages DESC"
	case "recent_desc":
		order = "last_active_day DESC, total_messages DESC"
	}

	q := db.Mysql().Table("tg_daily_activity AS a").
		Joins("LEFT JOIN user u ON u.id = a.uid").
		Where("a.chat_id = ? AND a.day >= ? AND a.day <= ?", chatID, from, to)
	if c.DefaultQuery("bound_only", "1") == "1" {
		q = q.Where("a.uid IS NOT NULL") // 仅注册用户
	}

	// count = 独立 tg_user_id 的数量
	type cntRow struct {
		C int64 `gorm:"column:c"`
	}
	var cnt cntRow
	q.Session(&gorm.Session{}).Select("COUNT(DISTINCT a.tg_user_id) AS c").Scan(&cnt)

	var rows []tgUserSummaryRow
	if err := q.
		Select(`a.uid,
			a.tg_user_id,
			MAX(a.tg_username) AS tg_username,
			MAX(u.wallet_address) AS wallet_address,
			MAX(u.twitter_handle) AS twitter_handle,
			SUM(a.message_count) AS total_messages,
			COUNT(DISTINCT a.day) AS active_days,
			MIN(a.day) AS first_active_day,
			MAX(a.day) AS last_active_day`).
		Group("a.tg_user_id, a.uid").
		Order(order).
		Offset((page - 1) * size).Limit(size).
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "TG_USER_ACT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": cnt.C, "page": page, "page_size": size,
		"from": from, "to": to, "chat_id": chatID,
	}})
}

// TgActivityDetail GET /admin/v1/tg-activity/detail?day=&chat_id=&page=&page_size=
// 某天某群的详细用户列表
func TgActivityDetail(c *gin.Context) {
	day := c.Query("day")
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if size < 1 || size > 500 {
		size = 50
	}

	q := db.Mysql().Model(&entity.TgDailyActivity{}).Where("day = ?", day)
	if v := c.Query("chat_id"); v != "" {
		q = q.Where("chat_id = ?", v)
	}
	if v := c.Query("bound_only"); v == "1" {
		q = q.Where("uid IS NOT NULL")
	}
	var total int64
	q.Count(&total)
	var rows []entity.TgDailyActivity
	if err := q.Order("message_count DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "TG_ACT_DETAIL_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}
