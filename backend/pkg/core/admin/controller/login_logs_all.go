package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// LoginLogWithWallet — 登录日志 + 关联的钱包地址
type LoginLogWithWallet struct {
	entity.UserLoginLog
	WalletAddress string `gorm:"column:wallet_address" json:"wallet_address"`
}

// ListAllLoginLogs GET /admin/v1/login-logs?uid=&ip=&login_type=&from=&to=&page=&page_size=
// 全用户视角；每行 LEFT JOIN user 拿钱包地址
func ListAllLoginLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	q := db.Mysql().Table("user_login_log").
		Select("user_login_log.*, user.wallet_address AS wallet_address").
		Joins("LEFT JOIN user ON user.id = user_login_log.uid")
	if v := c.Query("uid"); v != "" {
		q = q.Where("user_login_log.uid = ?", v)
	}
	if v := c.Query("ip"); v != "" {
		q = q.Where("user_login_log.ip = ?", v)
	}
	if v := c.Query("login_type"); v != "" {
		q = q.Where("user_login_log.login_type = ?", v)
	}
	if v := c.Query("wallet"); v != "" {
		q = q.Where("user.wallet_address LIKE ?", "%"+v+"%")
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("user_login_log.logged_in_at >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("user_login_log.logged_in_at < ?", t.Add(24*time.Hour))
		}
	}
	var total int64
	// count 单独跑，别选那么多列
	db.Mysql().Table("user_login_log").
		Joins("LEFT JOIN user ON user.id = user_login_log.uid").
		Where(cloneWhereFrom(c)).Count(&total)

	var rows []LoginLogWithWallet
	if err := q.Order("user_login_log.id DESC").
		Offset((page - 1) * size).Limit(size).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_LOGIN_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

// 重跑一遍相同 filter 集合，用于 count（避免 SELECT 大列拉全表数据算 total）
// 简单做法：直接拼字符串 map（都是白名单参数）
func cloneWhereFrom(c *gin.Context) map[string]interface{} {
	m := map[string]interface{}{}
	if v := c.Query("uid"); v != "" {
		m["user_login_log.uid"] = v
	}
	if v := c.Query("login_type"); v != "" {
		m["user_login_log.login_type"] = v
	}
	if v := c.Query("ip"); v != "" {
		m["user_login_log.ip"] = v
	}
	return m
}
