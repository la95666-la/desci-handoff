// Package controller — 软删除恢复 + 回收站
//
//	POST /admin/v1/{news,announcements,banners,tasks,admin-users}/:id/restore
//	GET  /admin/v1/trash?kind=news&page=1
//
// 30 天硬删由 cron 触发（未实现 → Phase 4）
package controller

import (
	"encoding/json/v2"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
)

// generic restore helper：清空 deleted_at
func restoreByID(c *gin.Context, model any) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_RESTORE_BAD_ID", "message": "invalid id"})
		return
	}
	// gorm.Unscoped() 才能 UPDATE 已软删的行
	res := db.Mysql().Unscoped().Model(model).Where("id = ?", id).Update("deleted_at", nil)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_RESTORE_FAILED", "message": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_RESTORE_NOT_FOUND", "message": "not found or not deleted"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "restored": true}})
}

func RestoreNews(c *gin.Context)         { restoreByID(c, &entity.News{}) }
func RestoreAnnouncement(c *gin.Context) { restoreByID(c, &entity.Announcement{}) }
func RestoreBanner(c *gin.Context)       { restoreByID(c, &entity.HomeBanner{}) }
func RestoreTask(c *gin.Context)         { restoreByID(c, &entity.Task{}) }
func RestoreAdminUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_RESTORE_BAD_ID", "message": "invalid id"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.AdminDeletePayload{AdminID: int64(id)})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminRestore, TargetKind: "admin", TargetID: strconv.FormatUint(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_RESTORE_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}

// TrashList GET /admin/v1/trash?kind=news
// 返回该 kind 已软删的行；30 天内可恢复
func TrashList(c *gin.Context) {
	kind := c.DefaultQuery("kind", "news")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}

	var total int64
	var items any
	offset := (page - 1) * size

	switch kind {
	case "news":
		var rows []entity.News
		q := db.Mysql().Unscoped().Where("deleted_at IS NOT NULL")
		q.Model(&entity.News{}).Count(&total)
		q.Order("deleted_at DESC").Offset(offset).Limit(size).Find(&rows)
		items = rows
	case "announcement", "announcements":
		var rows []entity.Announcement
		q := db.Mysql().Unscoped().Where("deleted_at IS NOT NULL")
		q.Model(&entity.Announcement{}).Count(&total)
		q.Order("deleted_at DESC").Offset(offset).Limit(size).Find(&rows)
		items = rows
	case "banner", "banners":
		var rows []entity.HomeBanner
		q := db.Mysql().Unscoped().Where("deleted_at IS NOT NULL")
		q.Model(&entity.HomeBanner{}).Count(&total)
		q.Order("deleted_at DESC").Offset(offset).Limit(size).Find(&rows)
		items = rows
	case "task", "tasks":
		var rows []entity.Task
		q := db.Mysql().Unscoped().Where("deleted_at IS NOT NULL")
		q.Model(&entity.Task{}).Count(&total)
		q.Order("deleted_at DESC").Offset(offset).Limit(size).Find(&rows)
		items = rows
	case "admin_user", "admin-users", "admin_users":
		var rows []adminEntity.AdminUser
		q := db.Mysql().Unscoped().Where("deleted_at IS NOT NULL")
		q.Model(&adminEntity.AdminUser{}).Count(&total)
		q.Order("deleted_at DESC").Offset(offset).Limit(size).Find(&rows)
		items = rows
	default:
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_TRASH_BAD_KIND", "message": "kind must be news/announcement/banner/task/admin-users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"kind": kind, "items": items, "total": total, "page": page, "page_size": size,
	}})
}
