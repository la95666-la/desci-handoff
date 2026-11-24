package approval

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
	adminEntity "joincare_server/pkg/core/admin/entity"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
	projectService "joincare_server/pkg/core/service/project"
)

const (
	OpUserDisable         = "user.disable"
	OpUserEnable          = "user.enable"
	OpUserAdjustPoint     = "user.adjust_point"
	OpAdminCreate         = "admin.create"
	OpAdminDelete         = "admin.delete"
	OpAdminUpdateAccess   = "admin.update_access"
	OpAdminSetRoles       = "admin.set_roles"
	OpAdminChangePassword = "admin.change_password"
	OpAdminRestore        = "admin.restore"
	OpRBACSetPerms        = "rbac.set_permissions"
	OpAnnouncementDel     = "announcement.delete"
)

const HighRiskPointThreshold = 1000

func RegisterAll() {
	Register(OpUserDisable, middleware.ModUser, middleware.ActApprove, handleUserDisable)
	Register(OpUserEnable, middleware.ModUser, middleware.ActApprove, handleUserEnable)
	Register(OpUserAdjustPoint, middleware.ModUser, middleware.ActApprove, handleUserAdjustPoint)
	Register(OpAdminCreate, "rbac", middleware.ActApprove, handleAdminCreate)
	Register(OpAdminDelete, "rbac", middleware.ActApprove, handleAdminDelete)
	Register(OpAdminUpdateAccess, "rbac", middleware.ActApprove, handleAdminUpdateAccess)
	Register(OpAdminSetRoles, "rbac", middleware.ActApprove, handleAdminSetRoles)
	Register(OpAdminChangePassword, "rbac", middleware.ActApprove, handleAdminChangePassword)
	Register(OpAdminRestore, "rbac", middleware.ActApprove, handleAdminRestore)
	Register(OpRBACSetPerms, "rbac", middleware.ActApprove, handleRBACSetPerms)
	Register(OpAnnouncementDel, middleware.ModNews, middleware.ActApprove, handleAnnouncementDelete)
	Register(OpProjectSetOnchainParams, middleware.ModProject, middleware.ActApprove, handleProjectSetOnchainParams)
	Register(OpV3RuntimeRegistration, middleware.ModProject, middleware.ActApprove, handleV3RuntimeRegistration)
	Register(OpFairLaunchSet, middleware.ModProject, middleware.ActApprove, handleFairLaunchSet)
	registerFairLaunchActionHandlers()
}

type UserActionPayload struct {
	UID int64 `json:"uid"`
}
type UserAdjustPayload struct {
	UID    int64  `json:"uid"`
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}
type AdminCreatePayload struct {
	Username     string   `json:"username"`
	PasswordHash string   `json:"password_hash"`
	Email        string   `json:"email"`
	DisplayName  string   `json:"display_name"`
	RoleCode     string   `json:"role_code"`
	Roles        []string `json:"roles"`
	IsEnable     int      `json:"is_enable"`
}
type AdminDeletePayload struct {
	AdminID int64 `json:"admin_id"`
}
type AdminUpdateAccessPayload struct {
	AdminID     int64    `json:"admin_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	RoleCode    string   `json:"role_code"`
	Roles       []string `json:"roles"`
	IsEnable    *int     `json:"is_enable"`
}
type AdminSetRolesPayload struct {
	AdminID int64    `json:"admin_id"`
	Roles   []string `json:"roles"`
}
type AdminPasswordPayload struct {
	AdminID      int64  `json:"admin_id"`
	PasswordHash string `json:"password_hash"`
}
type RBACSetPermsPayload struct {
	RoleCode    string                   `json:"role_code"`
	Permissions []RBACSetPermsPermission `json:"permissions"`
}
type RBACSetPermsPermission struct {
	Module string `json:"module"`
	Action string `json:"action"`
	Scope  string `json:"scope"`
}
type AnnouncementDelPayload struct {
	ID uint64 `json:"id"`
}

func handleUserDisable(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	return handleUserEnablement(tx, payload, 0)
}

func handleUserEnable(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	return handleUserEnablement(tx, payload, 1)
}

func handleUserEnablement(tx *gorm.DB, payload string, enabled int) (string, func(), error) {
	var p UserActionPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	res := tx.Model(&entity.User{}).Where("id = ?", p.UID).Updates(map[string]interface{}{
		"is_enable": enabled, "session_version": gorm.Expr("session_version + 1"),
	})
	if res.Error != nil {
		return "", nil, res.Error
	}
	if res.RowsAffected != 1 {
		return "", nil, errors.New("user not found or enablement unchanged")
	}
	return fmt.Sprintf(`{"uid":%d,"is_enable":%d}`, p.UID, enabled), userCacheInvalidator(p.UID), nil
}

func handleUserAdjustPoint(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p UserAdjustPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if p.UID <= 0 || p.Delta == 0 {
		return "", nil, errors.New("invalid point adjustment")
	}
	var current entity.UserPoint
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", p.UID).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		current = entity.UserPoint{UID: int(p.UID), Point: 0, UpdatedAt: time.Now()}
		if err := tx.Create(&current).Error; err != nil {
			return "", nil, err
		}
	} else if err != nil {
		return "", nil, err
	}
	remain := current.Point + p.Delta
	if remain < 0 {
		return "", nil, errors.New("user points are insufficient")
	}
	if err := tx.Model(&entity.UserPoint{}).Where("uid = ?", p.UID).Updates(map[string]interface{}{"point": remain, "updated_at": time.Now()}).Error; err != nil {
		return "", nil, err
	}
	desc := p.Reason
	if desc == "" {
		desc = "Admin manual adjust (4-eye approved)"
	}
	flow := entity.UserPointFlow{UID: int(p.UID), ChangePoint: p.Delta, Remain: remain, Type: constant.PointFlowTypeAdjust, Desc: desc}
	if err := tx.Create(&flow).Error; err != nil {
		return "", nil, err
	}
	return fmt.Sprintf(`{"uid":%d,"delta":%d}`, p.UID, p.Delta), pointCacheInvalidator(p.UID), nil
}

func handleAdminCreate(tx *gorm.DB, payload string, approverID uint64) (string, func(), error) {
	var p AdminCreatePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if len(p.PasswordHash) < 50 || !strings.HasPrefix(p.PasswordHash, "$2") {
		return "", nil, errors.New("approved password hash is invalid")
	}
	now := time.Now()
	if p.IsEnable != 0 && p.IsEnable != 1 {
		return "", nil, errors.New("admin enablement must be 0 or 1")
	}
	row := adminEntity.AdminUser{Username: p.Username, PasswordHash: p.PasswordHash, PasswordChangedAt: &now, Email: p.Email, DisplayName: p.DisplayName, RoleCode: p.RoleCode, IsEnable: p.IsEnable, SessionVersion: 1}
	if err := tx.Create(&row).Error; err != nil {
		return "", nil, err
	}
	roles := append(append([]string{}, p.Roles...), p.RoleCode)
	if err := writeUserRolesTx(tx, row.ID, uniqueStrings(roles), int64(approverID)); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf(`{"admin_id":%d}`, row.ID), nil, nil
}

func handleAdminDelete(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p AdminDeletePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if err := tx.Model(&adminEntity.AdminUser{}).Where("id = ?", p.AdminID).Update("session_version", gorm.Expr("session_version + 1")).Error; err != nil {
		return "", nil, err
	}
	if result := tx.Delete(&adminEntity.AdminUser{}, p.AdminID); result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("admin not found")
	}
	return fmt.Sprintf(`{"admin_id":%d}`, p.AdminID), middleware.InvalidatePermissionCache, nil
}

func handleAdminUpdateAccess(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p AdminUpdateAccessPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	updates := map[string]interface{}{"session_version": gorm.Expr("session_version + 1")}
	if p.Email != "" {
		updates["email"] = p.Email
	}
	if p.DisplayName != "" {
		updates["display_name"] = p.DisplayName
	}
	if p.RoleCode != "" {
		if err := requireAdminRolesTx(tx, []string{p.RoleCode}); err != nil {
			return "", nil, err
		}
		updates["role_code"] = p.RoleCode
	}
	if p.RoleCode != "" || len(p.Roles) > 0 {
		roles := uniqueStrings(append(append([]string{}, p.Roles...), p.RoleCode))
		if err := writeUserRolesTx(tx, p.AdminID, roles, 0); err != nil {
			return "", nil, err
		}
	}
	if p.IsEnable != nil {
		if *p.IsEnable != 0 && *p.IsEnable != 1 {
			return "", nil, errors.New("admin enablement must be 0 or 1")
		}
		updates["is_enable"] = *p.IsEnable
	}
	if len(updates) == 1 {
		return "", nil, errors.New("admin access update is empty")
	}
	if result := tx.Model(&adminEntity.AdminUser{}).Where("id = ?", p.AdminID).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("admin not found")
	}
	return fmt.Sprintf(`{"admin_id":%d}`, p.AdminID), middleware.InvalidatePermissionCache, nil
}

func handleAdminSetRoles(tx *gorm.DB, payload string, approverID uint64) (string, func(), error) {
	var p AdminSetRolesPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	roles := uniqueStrings(p.Roles)
	if len(roles) == 0 {
		return "", nil, errors.New("admin must have at least one role")
	}
	if err := writeUserRolesTx(tx, p.AdminID, roles, int64(approverID)); err != nil {
		return "", nil, err
	}
	if result := tx.Model(&adminEntity.AdminUser{}).Where("id = ?", p.AdminID).Updates(map[string]interface{}{"role_code": roles[0], "session_version": gorm.Expr("session_version + 1")}); result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("admin not found")
	}
	return fmt.Sprintf(`{"admin_id":%d,"role_count":%d}`, p.AdminID, len(roles)), middleware.InvalidatePermissionCache, nil
}

func handleAdminChangePassword(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p AdminPasswordPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if len(p.PasswordHash) < 50 || !strings.HasPrefix(p.PasswordHash, "$2") {
		return "", nil, errors.New("approved password hash is invalid")
	}
	now := time.Now()
	result := tx.Model(&adminEntity.AdminUser{}).Where("id = ?", p.AdminID).Updates(map[string]interface{}{
		"password_hash": p.PasswordHash, "password_changed_at": &now, "failed_login_count": 0,
		"locked_until": nil, "session_version": gorm.Expr("session_version + 1"),
	})
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("admin not found")
	}
	return fmt.Sprintf(`{"admin_id":%d}`, p.AdminID), nil, nil
}

func handleAdminRestore(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p AdminDeletePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	result := tx.Unscoped().Model(&adminEntity.AdminUser{}).Where("id = ? AND deleted_at IS NOT NULL", p.AdminID).Updates(map[string]interface{}{"deleted_at": nil, "session_version": gorm.Expr("session_version + 1")})
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("deleted admin not found")
	}
	return fmt.Sprintf(`{"admin_id":%d}`, p.AdminID), middleware.InvalidatePermissionCache, nil
}

func handleRBACSetPerms(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p RBACSetPermsPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if err := tx.Where("role_code = ?", p.RoleCode).Delete(&adminEntity.AdminRolePermission{}).Error; err != nil {
		return "", nil, err
	}
	rows := make([]adminEntity.AdminRolePermission, 0, len(p.Permissions))
	for _, pp := range p.Permissions {
		if pp.Module == "" || pp.Action == "" {
			continue
		}
		scope := pp.Scope
		if scope == "" {
			scope = "all"
		}
		rows = append(rows, adminEntity.AdminRolePermission{RoleCode: p.RoleCode, Module: pp.Module, Action: pp.Action, Scope: scope})
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			return "", nil, err
		}
	}
	if err := tx.Model(&adminEntity.AdminUser{}).Where("role_code = ? OR id IN (SELECT admin_id FROM admin_user_roles WHERE role_code = ?)", p.RoleCode, p.RoleCode).Update("session_version", gorm.Expr("session_version + 1")).Error; err != nil {
		return "", nil, err
	}
	return fmt.Sprintf(`{"role_code":%q,"count":%d}`, p.RoleCode, len(rows)), middleware.InvalidatePermissionCache, nil
}

func handleAnnouncementDelete(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p AnnouncementDelPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if result := tx.Delete(&entity.Announcement{}, p.ID); result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return "", nil, result.Error
		}
		return "", nil, errors.New("announcement not found")
	}
	return fmt.Sprintf(`{"id":%d}`, p.ID), nil, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func writeUserRolesTx(tx *gorm.DB, adminID int64, roles []string, grantedBy int64) error {
	if err := requireAdminRolesTx(tx, roles); err != nil {
		return err
	}
	if err := tx.Where("admin_id = ?", adminID).Delete(&adminEntity.AdminUserRole{}).Error; err != nil {
		return err
	}
	rows := make([]adminEntity.AdminUserRole, 0, len(roles))
	for _, role := range roles {
		row := adminEntity.AdminUserRole{AdminID: adminID, RoleCode: role, GrantedAt: time.Now()}
		if grantedBy != 0 {
			row.GrantedBy = &grantedBy
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return errors.New("admin must have at least one role")
	}
	return tx.Create(&rows).Error
}

func requireAdminRolesTx(tx *gorm.DB, roles []string) error {
	roles = uniqueStrings(roles)
	if len(roles) == 0 {
		return errors.New("admin must have at least one role")
	}
	var count int64
	if err := tx.Model(&adminEntity.AdminRole{}).Where("code IN ?", roles).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(roles)) {
		return errors.New("one or more admin roles do not exist")
	}
	return nil
}

func userCacheInvalidator(uid int64) func() {
	return func() {
		_ = db.Redis().Del(context.Background(), fmt.Sprintf(constant.KEY_USER_INFO, uid), fmt.Sprintf(constant.KEY_USER_WALLET, uid))
	}
}

func pointCacheInvalidator(uid int64) func() {
	return func() { _ = db.Redis().Del(context.Background(), fmt.Sprintf(constant.KEY_USER_POINT, uid)) }
}

// —— 项目链上参数（Set 阶段，专用表单提交 + 四眼审批）——

const OpProjectSetOnchainParams = "project.set_onchain_params"

// Registration is a separate, immutable deployment identity approval. It is
// not a Set approval and does not authorize creating a project or transaction.
const OpV3RuntimeRegistration = "project.register_ido_v3_runtime"

func handleV3RuntimeRegistration(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	if payload != "{}" {
		return "", nil, errors.New("V3 runtime approval payload invalid")
	}
	if err := projectService.ApplyReviewedV3RuntimeTx(tx); err != nil {
		return "", nil, err
	}
	return `{"runtime":"registered","set_authorized":false}`, nil, nil
}

type ProjectSetOnchainParamsPayload struct {
	ProjectID int64                               `json:"project_id"`
	Params    projectService.OnchainParamsRequest `json:"params"`
}

const OpFairLaunchSet = "fair_launch.set_onchain_params"

type FairLaunchSetApprovalPayload struct {
	ProjectID         uint64                              `json:"project_id"`
	Params            projectService.FairLaunchSetRequest `json:"params"`
	PreviewID         uint64                              `json:"preview_id"`
	AdminApprovalID   uint64                              `json:"admin_approval_id,omitempty"`
	MFAAssertionToken string                              `json:"mfa_assertion_token,omitempty"`
}

var FairLaunchSetInspector projectService.FairLaunchSetInspector

func handleFairLaunchSet(tx *gorm.DB, payload string, approverID uint64) (string, func(), error) {
	if !constant.FairLaunchMaterializationEnabled {
		return "", nil, errors.New(constant.FairLaunchMaterializationUnavailable)
	}
	var p FairLaunchSetApprovalPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	runtime, err := projectruntime.FairLaunchV1()
	if err != nil {
		return "", nil, err
	}
	if FairLaunchSetInspector == nil {
		return "", nil, errors.New("fair launch trusted inspector unavailable")
	}
	if p.AdminApprovalID == 0 {
		return "", nil, errors.New("fair launch Set approval ID is required")
	}
	if _, err := projectService.ConsumeFairLaunchMFAAssertionTx(tx, p.MFAAssertionToken, projectService.FairLaunchMFAContext{AdminID: approverID, Operation: OpFairLaunchSet, Stage: projectService.FairLaunchMFAStageSetApproval, ProjectID: p.ProjectID, PreviewID: &p.PreviewID, AdminApprovalID: &p.AdminApprovalID}, time.Now().UTC()); err != nil {
		return "", nil, err
	}
	summary, err := projectService.ApplyFairLaunchSetTx(tx, p.ProjectID, p.Params, runtime, FairLaunchSetInspector, time.Now().UTC())
	return summary, nil, err
}

func handleProjectSetOnchainParams(tx *gorm.DB, payload string, _ uint64) (string, func(), error) {
	var p ProjectSetOnchainParamsPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", nil, err
	}
	if p.ProjectID <= 0 {
		return "", nil, errors.New("project_id is required")
	}
	// This legacy handler has no locked approval identity or distinct approver.
	// V3 must enter only through handleApprovedProjectSetOnchainParams.
	if p.Params.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		return "", nil, errors.New("V3 project Set requires four-eyes approval")
	}
	summary, err := projectService.ApplyOnchainParamsTx(tx, p.ProjectID, p.Params)
	if err != nil {
		return "", nil, err
	}
	return summary, nil, nil
}
