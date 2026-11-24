// Package controller — 角色 + 权限矩阵编辑
//
// 端点：
//
//	GET  /admin/v1/roles                           全部角色（含内置）
//	GET  /admin/v1/roles/:code/permissions         某角色的 (module, action, scope) 矩阵
//	PUT  /admin/v1/roles/:code/permissions         覆盖式设置某角色的权限矩阵
//	GET  /admin/v1/my/permissions                  当前登录 admin 的 union 权限矩阵
package controller

import (
	"encoding/json/v2"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/repository"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/db"
)

// ListRoles GET /admin/v1/roles
func ListRoles(c *gin.Context) {
	var rows []adminEntity.AdminRole
	if err := db.Mysql().Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ROLE_LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows}})
}

// GetRolePermissions GET /admin/v1/roles/:code/permissions
func GetRolePermissions(c *gin.Context) {
	code := c.Param("code")
	var rows []adminEntity.AdminRolePermission
	if err := db.Mysql().Where("role_code = ?", code).Order("module ASC, action ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ROLE_PERM_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"role_code": code, "items": rows}})
}

type permissionEntry struct {
	Module string `json:"module" binding:"required"`
	Action string `json:"action" binding:"required"`
	Scope  string `json:"scope"`
}

type rolePermReq struct {
	Permissions []permissionEntry `json:"permissions" binding:"required"`
}

// SetRolePermissions PUT /admin/v1/roles/:code/permissions → 高风险，走 4 眼审批
// 设计 §3.8.2 高风险清单："管理员账号创建 / 权限变更"
func SetRolePermissions(c *gin.Context) {
	code := c.Param("code")
	var req rolePermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_ROLE_PERM_BAD_REQUEST", "message": err.Error()})
		return
	}
	// 转成审批 payload
	permsPayload := make([]approval.RBACSetPermsPermission, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		if p.Module == "" || p.Action == "" {
			continue
		}
		scope := p.Scope
		if scope == "" {
			scope = "all"
		}
		permsPayload = append(permsPayload, approval.RBACSetPermsPermission{
			Module: p.Module, Action: p.Action, Scope: scope,
		})
	}
	payloadJSON, _ := json.Marshal(approval.RBACSetPermsPayload{
		RoleCode: code, Permissions: permsPayload,
	})
	initID, initName := currentApprover(c)
	row, err := approval.Submit(approval.SubmitReq{
		Operation: approval.OpRBACSetPerms, TargetKind: "role", TargetID: code,
		Payload: string(payloadJSON), Reason: c.Query("reason"),
		InitiatorID: initID, InitiatorName: initName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_ROLE_PERM_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"approval_id": row.ID, "status": "pending",
		"message": "role permissions change requires 4-eye approval",
	}})
}

// currentApprover 拿 admin_id + username（复用 user.go 里 currentInitiator 名不冲突）
func currentApprover(c *gin.Context) (uint64, string) {
	var uid uint64
	var name string
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
	return uid, name
}

// 兼容旧引用（strconv 只在这里用于 currentApprover 之外，避免 unused import）
var _ = strconv.Itoa
var _ = adminEntity.AdminUserRole{}
var _ = db.Mysql
var _ = repository.GetRoleCodesByAdmin

// MyPermissions GET /admin/v1/my/permissions
// 返回当前登录 admin 的所有角色 + 聚合权限矩阵（union）
func MyPermissions(c *gin.Context) {
	adminID, _ := c.Get(middleware.CtxAdminID)
	id, _ := adminID.(int64)
	if id == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "ADMIN_AUTH_REQUIRED", "message": "no admin on context"})
		return
	}
	roles, _ := repository.GetRoleCodesByAdmin(id)
	if len(roles) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"roles": []string{}, "permissions": []string{}}})
		return
	}
	// 拉这些角色的所有权限；super_admin 特殊处理 —— 返一个通配位
	isSuper := false
	for _, r := range roles {
		if r == "super_admin" {
			isSuper = true
			break
		}
	}
	if isSuper {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
			"roles": roles, "permissions": []string{"*"}, "is_super": true,
		}})
		return
	}
	var perms []adminEntity.AdminRolePermission
	db.Mysql().Where("role_code IN ?", roles).Find(&perms)
	// 去重成 "module:action" 字符串数组
	seen := make(map[string]struct{}, len(perms))
	list := make([]string, 0, len(perms))
	for _, p := range perms {
		k := p.Module + ":" + p.Action
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		list = append(list, k)
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"roles": roles, "permissions": list, "is_super": false,
	}})
}
