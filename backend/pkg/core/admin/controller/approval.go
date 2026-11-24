// Package controller — 4 眼审批端点
//
//	GET    /admin/v1/approvals?status=&operation=&page=
//	GET    /admin/v1/approvals/:id                    详情
//	POST   /admin/v1/approvals/:id/approve            二审通过 → 执行
//	POST   /admin/v1/approvals/:id/reject             二审驳回
//	POST   /admin/v1/approvals/:id/cancel             发起人自己撤单
//	GET    /admin/v1/approvals/ops                    支持的操作码列表（用于筛选下拉）
package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
)

// ListApprovals GET /admin/v1/approvals?status=pending&operation=user.disable&page=1
func ListApprovals(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	q := db.Mysql().Model(&adminEntity.AdminApproval{})
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("operation"); v != "" {
		q = q.Where("operation = ?", v)
	}
	if v := c.Query("initiator_id"); v != "" {
		q = q.Where("initiator_id = ?", v)
	}
	var total int64
	q.Count(&total)
	var rows []adminEntity.AdminApproval
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_APPRV_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": size,
	}})
}

// GetApproval GET /admin/v1/approvals/:id
func GetApproval(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_BAD_ID", "message": "invalid id"})
		return
	}
	var row adminEntity.AdminApproval
	if err := db.Mysql().Where("id = ?", id).Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "ADMIN_APPRV_NOT_FOUND", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// ApproveApproval POST /admin/v1/approvals/:id/approve
func ApproveApproval(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_BAD_ID", "message": "invalid id"})
		return
	}
	adminIDAny, _ := c.Get(middleware.CtxAdminID)
	usernameAny, _ := c.Get(middleware.CtxUsername)
	adminID, _ := adminIDAny.(int64)
	username, _ := usernameAny.(string)

	var decision struct {
		MFAAssertionToken string `json:"mfa_assertion_token"`
	}
	_ = c.ShouldBindJSON(&decision)
	row, err := approval.ApproveWithMFA(id, uint64(adminID), username, decision.MFAAssertionToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_APPROVE_FAILED", "message": err.Error(), "data": row})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

type approvalDecisionReq struct {
	Reason string `json:"reason"`
}

// RejectApproval POST /admin/v1/approvals/:id/reject
func RejectApproval(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_BAD_ID", "message": "invalid id"})
		return
	}
	var req approvalDecisionReq
	_ = c.ShouldBindJSON(&req) // reason optional
	adminIDAny, _ := c.Get(middleware.CtxAdminID)
	usernameAny, _ := c.Get(middleware.CtxUsername)
	adminID, _ := adminIDAny.(int64)
	username, _ := usernameAny.(string)

	row, err := approval.Reject(id, uint64(adminID), username, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_REJECT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

// CancelApproval POST /admin/v1/approvals/:id/cancel
func CancelApproval(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_BAD_ID", "message": "invalid id"})
		return
	}
	adminIDAny, _ := c.Get(middleware.CtxAdminID)
	adminID, _ := adminIDAny.(int64)
	if err := approval.Cancel(id, uint64(adminID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_APPRV_CANCEL_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

// ListApprovalOps GET /admin/v1/approvals/ops
func ListApprovalOps(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"ops": approval.AllOps()}})
}
