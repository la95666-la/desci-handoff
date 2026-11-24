package middleware

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/db"
)

// 模块 code 常量；与 admin_role_permissions.module 对齐
const (
	ModIdentity = "m1_identity"
	ModProject  = "m2_project"
	ModTask     = "m5_task"
	ModUser     = "m7_user"
	ModI18n     = "m8_i18n"
	ModNews     = "news"
	ModService  = "service"
	ModApproval = "approval"
)

// 动作常量
const (
	ActRead    = "read"
	ActWrite   = "write"
	ActApprove = "approve"
	ActExecute = "execute"
)

func checkPerm(roleCode, module, action string) bool {
	// super_admin 直通
	if roleCode == "super_admin" {
		return true
	}
	var cnt int64
	result := db.Mysql().Model(&entity.AdminRolePermission{}).
		Where("role_code = ? AND module = ? AND action = ?", roleCode, module, action).
		Count(&cnt)
	allow := result.Error == nil && cnt > 0

	return allow
}

func InvalidatePermissionCache() {}

func liveRoleCodes(database *gorm.DB, admin entity.AdminUser) ([]string, error) {
	seen := make(map[string]struct{}, 4)
	if role := strings.TrimSpace(admin.RoleCode); role != "" {
		seen[role] = struct{}{}
	}
	var joined []string
	if err := database.Model(&entity.AdminUserRole{}).Where("admin_id = ?", admin.ID).Pluck("role_code", &joined).Error; err != nil {
		return nil, err
	}
	for _, role := range joined {
		if role = strings.TrimSpace(role); role != "" {
			seen[role] = struct{}{}
		}
	}
	roles := make([]string, 0, len(seen))
	for role := range seen {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles, nil
}

func HasLivePermissionTx(tx *gorm.DB, adminID int64, module, action string) (entity.AdminUser, error) {
	var admin entity.AdminUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,username,role_code,is_enable,session_version").Where("id = ? AND is_enable = 1", adminID).Take(&admin).Error; err != nil {
		return admin, err
	}
	roles, err := liveRoleCodes(tx, admin)
	if err != nil || len(roles) == 0 {
		return admin, errors.New("admin has no live roles")
	}
	for _, role := range roles {
		if role == "super_admin" {
			return admin, nil
		}
	}
	var count int64
	if err := tx.Model(&entity.AdminRolePermission{}).Where("role_code IN ? AND module = ? AND action = ?", roles, module, action).Count(&count).Error; err != nil {
		return admin, err
	}
	if count == 0 {
		return admin, errors.New("missing live permission " + module + ":" + action)
	}
	return admin, nil
}

// checkPermUnion 任意一个角色有权限即通过（N:M 场景）
func checkPermUnion(roleCodesCsv, module, action string) bool {
	if roleCodesCsv == "" {
		return false
	}
	for _, code := range splitCsv(roleCodesCsv) {
		if checkPerm(code, module, action) {
			return true
		}
	}
	return false
}

func splitCsv(s string) []string {
	if s == "" {
		return nil
	}
	// 手写拆分避免 strings 包引入
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// RequirePerm 返回一个 gin handler：校验当前 admin 是否对 (module, action) 有权限。
// N:M 场景下取 CtxAllRoles（Auth 中间件写入 CSV），任一角色有权限即通过。
//
//	authed.GET("/news", middleware.RequirePerm(middleware.ModNews, middleware.ActRead), controller.ListNews)
func RequirePerm(module, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先用 aggregated roles（B3 加）
		if v, ok := c.Get(CtxAllRoles); ok {
			if csv, _ := v.(string); csv != "" {
				if checkPermUnion(csv, module, action) {
					c.Next()
					return
				}
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"status": false, "code": "ADMIN_PERM_DENIED",
					"message":             "missing permission " + module + ":" + action,
					"required_permission": module + ":" + action,
				})
				return
			}
		}
		// 兼容旧 token（只有主角色）
		roleAny, ok := c.Get(CtxRoleCode)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status": false, "code": "ADMIN_AUTH_REQUIRED",
				"message": "no role on context",
			})
			return
		}
		role, _ := roleAny.(string)
		if !checkPerm(role, module, action) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"status": false, "code": "ADMIN_PERM_DENIED",
				"message":             "missing permission " + module + ":" + action,
				"required_permission": module + ":" + action,
			})
			return
		}
		c.Next()
	}
}

// CtxAllRoles 是 Auth 中间件把当前 admin 所有角色（CSV）写到 ctx 的 key
const CtxAllRoles = "admin_all_roles"
