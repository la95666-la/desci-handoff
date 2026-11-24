package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

// ListUserActivity GET /admin/v1/users/:id/activity?page=1&page_size=20&type=
// 直接列 user_point_flow —— 平台里所有涨/扣分动作（TASK / CHECK_IN / INVITE / ADJUST）
// 都统一走 points.ChangePoints 落库，type + desc 已经把动作类型说清了
func ListUserActivity(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	q := db.Mysql().Model(&entity.UserPointFlow{}).Where("uid = ?", id)
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	var total int64
	q.Count(&total)
	var rows []entity.UserPointFlow
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_ACT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

// UserSummary GET /admin/v1/users/:id/summary
// 用户详情页顶部一次性拉：user 基本信息 + 备注条数 + 登录次数 + 积分余额 + 任务完成数 + 连续签到
func UserSummary(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var u entity.User
	if err := db.Mysql().Where("id = ?", id).Take(&u).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_USR_NOT_FOUND", "message": err.Error()})
		return
	}
	var noteCount, loginCount, taskCount int64
	db.Mysql().Model(&entity.UserNote{}).Where("uid = ?", id).Count(&noteCount)
	db.Mysql().Model(&entity.UserLoginLog{}).Where("uid = ?", id).Count(&loginCount)
	db.Mysql().Model(&entity.TaskRecord{}).Where("uid = ?", id).Count(&taskCount)

	var totalPoint int64
	db.Mysql().Model(&entity.UserPoint{}).Where("uid = ?", id).Select("COALESCE(point, 0)").Scan(&totalPoint)

	// 最近一次签到，取连续天数
	var lastCheckin entity.UserCheckInRecord
	db.Mysql().Where("uid = ?", id).Order("id DESC").Limit(1).Take(&lastCheckin)

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"user":                    u,
		"note_count":              noteCount,
		"login_count":             loginCount,
		"task_completed":          taskCount,
		"total_point":             totalPoint,
		"continuous_checkin_days": lastCheckin.ContinuousCheckInDays,
		"total_checkin_days":      lastCheckin.TotalCheckInDays,
		"last_checkin_at":         lastCheckin.CheckInAt,
	}})
}
