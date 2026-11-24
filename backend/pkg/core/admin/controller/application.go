package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
	"joincare_server/pkg/core/third/ops"
)

// ListApplications GET /admin/v1/applications?page=1&page_size=20&status=0
func ListApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	q := db.Mysql().Model(&entity.ApplyProject{})
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			q = q.Where("status = ?", v)
		}
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR project_name LIKE ? OR email LIKE ? OR wallet_address LIKE ?", like, like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_APP_LIST_FAILED", "message": err.Error()})
		return
	}

	var rows []entity.ApplyProject
	if err := q.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_APP_LIST_FAILED", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": pageSize,
	}})
}

// GetApplication GET /admin/v1/applications/:id
func GetApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_BAD_ID", "message": "invalid id"})
		return
	}
	var row entity.ApplyProject
	if err := db.Mysql().Where("id = ?", id).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_APP_NOT_FOUND", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// CloneApprovedApplication POST /admin/v1/applications/:id/clone
// Admin-authenticated, audited proxy submission of a new pending application
// from a prior approved application. It is not a client-wallet submission or
// an approval, and it does not copy Set/on-chain parameters.
func CloneApprovedApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_BAD_ID", "message": "invalid source application id"})
		return
	}
	adminID, ok := c.Get(middleware.CtxAdminID)
	actor, valid := adminID.(int64)
	if !ok || !valid || actor <= 0 {
		c.JSON(http.StatusForbidden, gin.H{"status": false, "code": "ADMIN_APP_ACTOR_REQUIRED", "message": "admin actor unavailable"})
		return
	}
	newID, err := projectService.CloneApprovedApplication(id, actor)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "ADMIN_APP_CLONE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": true, "data": gin.H{"id": newID, "status": 0, "source_application_id": id}})
}

type applicationDecisionReq struct {
	Reason string `json:"reason"`
}

// ApproveApplication POST /admin/v1/applications/:id/approve  (Phase 1: 单人通过)
func ApproveApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_BAD_ID", "message": "invalid id"})
		return
	}
	if err := projectService.ApproveApplication(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_APPROVE_FAILED", "message": err.Error()})
		return
	}
	ops.PostAlert(c.Request.Context(), "INFO", "项目申请已通过",
		fmt.Sprintf("app_id=%d ip=%s", id, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "status": 1}})
}

// RejectApplication POST /admin/v1/applications/:id/reject
func RejectApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_BAD_ID", "message": "invalid id"})
		return
	}
	var req applicationDecisionReq
	_ = c.ShouldBindJSON(&req) // reason optional
	if err := projectService.RejectApplication(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APP_REJECT_FAILED", "message": err.Error()})
		return
	}
	ops.PostAlert(c.Request.Context(), "INFO", "项目申请已驳回",
		fmt.Sprintf("app_id=%d reason=%s ip=%s", id, req.Reason, c.ClientIP()))
	// Phase 1: reason 暂时只回显，未持久化（apply_project 表无 reject_reason 列）
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "status": 2, "reason": req.Reason}})
}
