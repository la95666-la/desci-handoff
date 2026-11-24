// Package repository 的 N:M 角色查询辅助函数
package repository

import (
	"strings"

	"joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/db"
)

// GetRoleCodesByAdmin 查询某 admin 的所有角色 code（union role_code + admin_user_roles）
//   - admin_users.role_code（主角色）
//   - admin_user_roles.role_code 中的所有行
//
// 去重后返回。用于登录时生成 token / 中间件权限校验
func GetRoleCodesByAdmin(adminID int64) ([]string, error) {
	seen := make(map[string]struct{}, 4)
	// 主角色
	var mainRole string
	db.Mysql().Model(&entity.AdminUser{}).Where("id = ?", adminID).Select("role_code").Scan(&mainRole)
	if mainRole != "" {
		seen[mainRole] = struct{}{}
	}
	// N:M 表
	var joinRoles []string
	db.Mysql().Model(&entity.AdminUserRole{}).Where("admin_id = ?", adminID).Pluck("role_code", &joinRoles)
	for _, r := range joinRoles {
		if r != "" {
			seen[r] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	return out, nil
}

// GetRoleCodesCsvByAdmin 逗号分隔版本；空则返 ""
func GetRoleCodesCsvByAdmin(adminID int64) string {
	codes, err := GetRoleCodesByAdmin(adminID)
	if err != nil || len(codes) == 0 {
		return ""
	}
	return strings.Join(codes, ",")
}
