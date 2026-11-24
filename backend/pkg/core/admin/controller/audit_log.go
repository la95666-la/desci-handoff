package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// ListAuditLogs GET /admin/v1/audit-logs?page=&page_size=&admin_id=&method=&path_like=&from=&to=
func ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}

	q := db.Mysql().Model(&adminEntity.AdminAuditLog{})
	if v := c.Query("admin_id"); v != "" {
		q = q.Where("admin_id = ?", v)
	}
	if v := c.Query("method"); v != "" {
		q = q.Where("method = ?", v)
	}
	if v := c.Query("path_like"); v != "" {
		q = q.Where("path LIKE ?", "%"+v+"%")
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("created_at < ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	q.Count(&total)

	var rows []adminEntity.AdminAuditLog
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_AUDIT_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

// VerifyAuditChain GET /admin/v1/audit-logs/verify-chain
// 全表哈希链核对；O(N) 扫描。只有 super_admin/auditor 才能调
func VerifyAuditChain(c *gin.Context) {
	ok, brokenID, err := middleware.VerifyChain()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "AUDIT_VERIFY_FAILED", "message": err.Error()})
		return
	}
	if ok {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"ok": true, "message": "chain intact"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"ok": false, "broken_at_id": brokenID,
		"message": "hash chain broken at row " + strconv.FormatUint(brokenID, 10),
	}})
}
