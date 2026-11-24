package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/jwt"
	"joincare_server/pkg/core/db"
)

const (
	CtxAdminID  = "admin_id"
	CtxUsername = "admin_username"
	CtxRoleCode = "admin_role"
)

func adminSecret() string { return strings.TrimSpace(viper.GetString("admin.jwtSecret")) }

func ValidateJWTSecrets() error {
	admin := adminSecret()
	client := strings.TrimSpace(viper.GetString("jwt.secret"))
	if len(admin) < 32 {
		return errors.New("admin.jwtSecret must be an independent secret of at least 32 bytes")
	}
	if len(client) < 32 {
		return errors.New("jwt.secret must be at least 32 bytes")
	}
	if admin == client || admin == client+"::admin" {
		return errors.New("admin.jwtSecret must be independent from jwt.secret")
	}
	return nil
}

// Secret 暴露给 service 层签发 token 时复用
func Secret() string { return adminSecret() }

func Auth(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"code":    "ADMIN_AUTH_REQUIRED",
			"message": "missing bearer token",
		})
		return
	}
	tokenStr := strings.TrimSpace(header[7:])
	claims, err := jwt.Parse(tokenStr, adminSecret())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"code":    "ADMIN_TOKEN_INVALID",
			"message": err.Error(),
		})
		return
	}
	// refresh token 不允许直接走 API（仅供 /refresh 用 body 校验）
	// 兼容旧 access token（Kind=""）：空 kind 视为 access
	if claims.Kind != "" && claims.Kind != "access" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"code":    "ADMIN_TOKEN_KIND_INVALID",
			"message": "access token required",
		})
		return
	}
	var admin adminEntity.AdminUser
	if err := db.Mysql().Select("id,username,role_code,is_enable,session_version").Where("id = ? AND is_enable = 1", claims.AdminID).Take(&admin).Error; err != nil || admin.SessionVersion != claims.SessionVersion {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"status": false, "code": "ADMIN_SESSION_REVOKED", "message": "admin session is no longer valid",
		})
		return
	}
	roles, err := liveRoleCodes(db.Mysql(), admin)
	if err != nil || len(roles) == 0 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"status": false, "code": "ADMIN_ROLE_UNAVAILABLE", "message": "admin roles are unavailable",
		})
		return
	}
	c.Set(CtxAdminID, admin.ID)
	c.Set(CtxUsername, admin.Username)
	c.Set(CtxRoleCode, admin.RoleCode)
	c.Set(CtxAllRoles, strings.Join(roles, ","))
	c.Next()
}
