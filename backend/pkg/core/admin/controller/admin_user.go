// Package controller — admin_users CRUD + N:M 角色管理
//
// 端点：
//
//	GET    /admin/v1/admin-users              列表（含每人的角色 CSV）
//	GET    /admin/v1/admin-users/:id          详情
//	POST   /admin/v1/admin-users              创建（自动 bcrypt 密码）
//	PUT    /admin/v1/admin-users/:id          改基本信息（email/display_name/主 role_code）
//	POST   /admin/v1/admin-users/:id/password 改密码
//	POST   /admin/v1/admin-users/:id/roles    覆盖式设置角色列表
//	DELETE /admin/v1/admin-users/:id          软删除
package controller

import (
	"encoding/json/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/repository"
	"joincare_server/pkg/core/admin/service/approval"
	"joincare_server/pkg/core/admin/service/password"
	"joincare_server/pkg/core/db"
)

type adminUserListItem struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	RoleCode    string     `json:"role_code"` // 主角色
	Roles       []string   `json:"roles"`     // 全部角色
	IsEnable    int        `json:"is_enable"`
	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListAdminUsers GET /admin/v1/admin-users?page=&page_size=&search=
func ListAdminUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 200 {
		size = 20
	}
	q := db.Mysql().Model(&adminEntity.AdminUser{})
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?", like, like, like)
	}
	var total int64
	q.Count(&total)
	var rows []adminEntity.AdminUser
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_LIST_FAILED", "message": err.Error()})
		return
	}
	items := make([]adminUserListItem, 0, len(rows))
	for _, r := range rows {
		roles, _ := repository.GetRoleCodesByAdmin(r.ID)
		items = append(items, adminUserListItem{
			ID: r.ID, Username: r.Username, Email: r.Email,
			DisplayName: r.DisplayName, RoleCode: r.RoleCode,
			Roles: roles, IsEnable: r.IsEnable,
			LastLoginAt: r.LastLoginAt, LastLoginIP: r.LastLoginIP,
			CreatedAt: r.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"items": items, "total": total, "page": page, "page_size": size,
	}})
}

type adminUserCreateReq struct {
	Username    string   `json:"username" binding:"required,min=3,max=64"`
	Password    string   `json:"password" binding:"required,min=8"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	RoleCode    string   `json:"role_code" binding:"required"` // 主角色
	Roles       []string `json:"roles"`                        // 附加角色（可空）
	IsEnable    *int     `json:"is_enable"`                    // nil = 默认启用；显式 0 = 停用
}

// CreateAdminUser POST /admin/v1/admin-users
func CreateAdminUser(c *gin.Context) {
	var req adminUserCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_REQUEST", "message": err.Error()})
		return
	}
	if err := password.Check(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_WEAK_PW", "message": err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_HASH_FAILED", "message": err.Error()})
		return
	}
	initID, initName := currentInitiator(c)
	isEnable := 1
	if req.IsEnable != nil {
		isEnable = *req.IsEnable
	}
	payload, _ := json.Marshal(approval.AdminCreatePayload{Username: req.Username, PasswordHash: string(hash), Email: req.Email, DisplayName: req.DisplayName, RoleCode: req.RoleCode, Roles: req.Roles, IsEnable: isEnable})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminCreate, TargetKind: "admin", TargetID: req.Username, Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}

type adminUserUpdateReq struct {
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	RoleCode    string   `json:"role_code"`
	Roles       []string `json:"roles"`
	IsEnable    *int     `json:"is_enable"`
}

// UpdateAdminUser PUT /admin/v1/admin-users/:id
// 不改密码；密码走独立端点
func UpdateAdminUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var req adminUserUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_REQUEST", "message": err.Error()})
		return
	}
	if req.RoleCode != "" || req.IsEnable != nil {
		initID, initName := currentInitiator(c)
		payload, _ := json.Marshal(approval.AdminUpdateAccessPayload{AdminID: id, Email: req.Email, DisplayName: req.DisplayName, RoleCode: req.RoleCode, Roles: req.Roles, IsEnable: req.IsEnable})
		row, submitErr := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminUpdateAccess, TargetKind: "admin", TargetID: strconv.FormatInt(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
		if submitErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_ACCESS_SUBMIT_FAILED", "message": submitErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
		return
	}
	updates := map[string]interface{}{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "changed": 0}})
		return
	}
	res := db.Mysql().Model(&adminEntity.AdminUser{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_UPDATE_FAILED", "message": res.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "changed": res.RowsAffected}})
}

type adminUserPwdReq struct {
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangeAdminUserPassword POST /admin/v1/admin-users/:id/password
func ChangeAdminUserPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var req adminUserPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_REQUEST", "message": err.Error()})
		return
	}
	if err := password.Check(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_WEAK_PW", "message": err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_HASH_FAILED", "message": err.Error()})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.AdminPasswordPayload{AdminID: id, PasswordHash: string(hash)})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminChangePassword, TargetKind: "admin", TargetID: strconv.FormatInt(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_PASSWORD_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}

type selfPwdReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangeSelfPassword POST /admin/v1/self/password
// 当前登录管理员自助改自己密码：校验旧密码 → 更新为新密码
// 无需 rbac 写权限，普通管理员也能用（个人中心场景）
func ChangeSelfPassword(c *gin.Context) {
	uidVal, ok := c.Get(middleware.CtxAdminID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "ADMIN_AUTH_REQUIRED", "message": "not logged in"})
		return
	}
	uid, _ := uidVal.(int64)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "ADMIN_AUTH_REQUIRED", "message": "invalid session"})
		return
	}
	var req selfPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_REQUEST", "message": err.Error()})
		return
	}
	// 校验新密码强度
	if err := password.Check(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_WEAK_PW", "message": err.Error()})
		return
	}
	// 拉当前密码 hash 校验旧密码
	var u adminEntity.AdminUser
	if err := db.Mysql().Select("id, password_hash").Where("id = ?", uid).First(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_NOT_FOUND", "message": err.Error()})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_OLD_PW_WRONG", "message": "old password is wrong"})
		return
	}
	// 不允许新密码 = 旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.NewPassword)); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_SAME_PW", "message": "new password must differ from old"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_HASH_FAILED", "message": err.Error()})
		return
	}
	now := time.Now()
	if err := db.Mysql().Model(&adminEntity.AdminUser{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"password_hash":       string(hash),
		"password_changed_at": &now,
		"failed_login_count":  0,
		"locked_until":        nil,
		"session_version":     gorm.Expr("session_version + 1"),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_UPDATE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": uid}})
}

type adminUserRolesReq struct {
	Roles []string `json:"roles" binding:"required"`
}

// SetAdminUserRoles POST /admin/v1/admin-users/:id/roles
// 覆盖式设置：老的行删干净 → 写新的（主角色 + 附加）
func SetAdminUserRoles(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	var req adminUserRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_REQUEST", "message": err.Error()})
		return
	}
	if len(req.Roles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_NO_ROLE", "message": "at least one role required"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.AdminSetRolesPayload{AdminID: id, Roles: req.Roles})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminSetRoles, TargetKind: "admin", TargetID: strconv.FormatInt(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_ROLE_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}

// DeleteAdminUser DELETE /admin/v1/admin-users/:id — 软删（gorm 走 deleted_at）
// 禁止删自己
func DeleteAdminUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_BAD_ID", "message": "invalid id"})
		return
	}
	selfID, _ := c.Get(middleware.CtxAdminID)
	if s, _ := selfID.(int64); s == id {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "ADMIN_USR_SELF_DELETE", "message": "cannot delete self"})
		return
	}
	initID, initName := currentInitiator(c)
	payload, _ := json.Marshal(approval.AdminDeletePayload{AdminID: id})
	row, err := approval.Submit(approval.SubmitReq{Operation: approval.OpAdminDelete, TargetKind: "admin", TargetID: strconv.FormatInt(id, 10), Payload: string(payload), Reason: c.Query("reason"), InitiatorID: initID, InitiatorName: initName})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "ADMIN_USR_DELETE_SUBMIT_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"approval_id": row.ID, "status": "pending"}})
}
