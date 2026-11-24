package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/core/db"
)

// PointFlowWithWallet — user_point_flow LEFT JOIN user 拿钱包 + Twitter 简称
type PointFlowWithWallet struct {
	ID            int64     `json:"id" gorm:"column:id"`
	UID           int64     `json:"uid" gorm:"column:uid"`
	ChangePoint   int       `json:"change_point" gorm:"column:change_point"`
	Remain        int       `json:"remain" gorm:"column:remain"`
	Type          string    `json:"type" gorm:"column:type"`
	Desc          string    `json:"desc" gorm:"column:desc"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at"` // 原表列名 typo 保留
	WalletAddress string    `json:"wallet_address" gorm:"column:wallet_address"`
	TwitterHandle string    `json:"twitter_handle" gorm:"column:twitter_handle"`
}

// ListPointFlows GET /admin/v1/point-flows?uid=&wallet=&type=&from=&to=&page=&page_size=
// 全用户视角。type: TASK / CHECK_IN / INVITE / ADJUST
func ListPointFlows(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}

	q := db.Mysql().Table("user_point_flow AS f").
		Joins("LEFT JOIN user u ON u.id = f.uid")
	if v := c.Query("uid"); v != "" {
		q = q.Where("f.uid = ?", v)
	}
	if v := c.Query("wallet"); v != "" {
		q = q.Where("u.wallet_address LIKE ?", "%"+v+"%")
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("f.type = ?", v)
	}
	if v := c.Query("min_change"); v != "" {
		q = q.Where("f.change_point >= ?", v)
	}
	if v := c.Query("max_change"); v != "" {
		q = q.Where("f.change_point <= ?", v)
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("f.created_at >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("f.created_at < ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	q.Count(&total)

	var rows []PointFlowWithWallet
	if err := q.Select("f.id, f.uid, f.change_point, f.remain, f.type, f.desc, f.created_at, u.wallet_address, u.twitter_handle").
		Order("f.id DESC").
		Offset((page - 1) * size).Limit(size).
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_PF_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}
