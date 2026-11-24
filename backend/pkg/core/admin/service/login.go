package service

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	adminEntity "joincare_server/pkg/core/admin/entity"
	adminJwt "joincare_server/pkg/core/admin/jwt"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/admin/repository"
	"joincare_server/pkg/core/db"
)

type LoginResult struct {
	Token           string `json:"token"`
	ExpireAt        int64  `json:"expire_at"`
	RefreshToken    string `json:"refresh_token"`
	RefreshExpireAt int64  `json:"refresh_expire_at"`
	AdminID         int64  `json:"admin_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	RoleCode        string `json:"role_code"`
}

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked, try later")
)

// FailedLoginAuditCap bounds the persisted observability counter. Automated
// blocking is enforced by the distributed limiter before password validation;
// a remote attacker must not be able to set locked_until for another admin.
const FailedLoginAuditCap = 1_000

func Login(username, password, clientIP string) (*LoginResult, error) {
	u, err := repository.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}
	if u.IsEnable != 1 {
		return nil, ErrAccountDisabled
	}
	// 锁定检查
	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return nil, ErrAccountLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		// Keep an audit counter, but never let an unauthenticated caller lock the account.
		recordFailedLogin(u.ID)
		return nil, ErrInvalidCredentials
	}
	// 成功 → 清零
	if u.FailedLoginCount > 0 || u.LockedUntil != nil {
		db.Mysql().Model(&adminEntity.AdminUser{}).Where("id = ?", u.ID).
			Updates(map[string]interface{}{"failed_login_count": 0, "locked_until": nil})
	}

	secret := middleware.Secret()
	token, exp, err := adminJwt.Generate(u.ID, u.Username, u.RoleCode, u.SessionVersion, secret)
	if err != nil {
		return nil, err
	}
	refresh, refreshExp, err := adminJwt.GenerateRefresh(u.ID, u.Username, u.RoleCode, u.SessionVersion, secret)
	if err != nil {
		return nil, err
	}

	_ = repository.UpdateLoginInfo(u.ID, clientIP)

	return &LoginResult{
		Token:           token,
		ExpireAt:        exp,
		RefreshToken:    refresh,
		RefreshExpireAt: refreshExp,
		AdminID:         u.ID,
		Username:        u.Username,
		DisplayName:     u.DisplayName,
		RoleCode:        u.RoleCode,
	}, nil
}

// Refresh 用 refresh token 换一对新的 access + refresh
// （滑动续期：每次刷新都生成新的 refresh，相当于把过期时间往后推 7 天）
func Refresh(refreshToken string) (*LoginResult, error) {
	secret := middleware.Secret()
	claims, err := adminJwt.Parse(refreshToken, secret)
	if err != nil {
		return nil, err
	}
	if claims.Kind != "refresh" {
		return nil, errors.New("not a refresh token")
	}
	u, err := repository.GetByID(claims.AdminID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("admin not found")
	}
	if u.IsEnable != 1 {
		return nil, ErrAccountDisabled
	}
	if u.SessionVersion == 0 || claims.SessionVersion != u.SessionVersion {
		return nil, errors.New("admin session has been revoked")
	}
	access, accessExp, err := adminJwt.Generate(u.ID, u.Username, u.RoleCode, u.SessionVersion, secret)
	if err != nil {
		return nil, err
	}
	newRefresh, newRefreshExp, err := adminJwt.GenerateRefresh(u.ID, u.Username, u.RoleCode, u.SessionVersion, secret)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:           access,
		ExpireAt:        accessExp,
		RefreshToken:    newRefresh,
		RefreshExpireAt: newRefreshExp,
		AdminID:         u.ID,
		Username:        u.Username,
		DisplayName:     u.DisplayName,
		RoleCode:        u.RoleCode,
	}, nil
}

type MeResult struct {
	AdminID     int64  `json:"admin_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	RoleCode    string `json:"role_code"`
}

func Me(adminID int64) (*MeResult, error) {
	u, err := repository.GetByID(adminID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("admin not found")
	}
	return &MeResult{
		AdminID:     u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		RoleCode:    u.RoleCode,
	}, nil
}

// ResolvedUser is the value stored on Gin context by Auth middleware.
type ResolvedUser struct {
	AdminID  int64  `json:"admin_id"`
	Username string `json:"username"`
	RoleCode string `json:"role_code"`
}

var _ = adminEntity.AdminUser{}

func recordFailedLogin(adminID int64) {
	db.Mysql().Model(&adminEntity.AdminUser{}).Where("id = ?", adminID).
		Update("failed_login_count", gorm.Expr("LEAST(failed_login_count + 1, ?)", FailedLoginAuditCap))
}
