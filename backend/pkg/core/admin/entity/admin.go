package entity

import (
	"time"

	"gorm.io/gorm"
)

type AdminUser struct {
	ID                int64          `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Username          string         `gorm:"column:username;size:64;not null" json:"username"`
	PasswordHash      string         `gorm:"column:password_hash;size:120;not null" json:"-"`
	PasswordChangedAt *time.Time     `gorm:"column:password_changed_at" json:"password_changed_at"`
	LockedUntil       *time.Time     `gorm:"column:locked_until" json:"locked_until"`
	FailedLoginCount  int            `gorm:"column:failed_login_count;not null;default:0" json:"failed_login_count"`
	Email             string         `gorm:"column:email;size:120;not null" json:"email"`
	DisplayName       string         `gorm:"column:display_name;size:64;not null" json:"display_name"`
	RoleCode          string         `gorm:"column:role_code;size:32;not null" json:"role_code"` // 主角色，实际权限走 admin_user_roles
	IsEnable          int            `gorm:"column:is_enable;not null" json:"is_enable"`
	SessionVersion    uint64         `gorm:"column:session_version;not null;default:1" json:"-"`
	LastLoginAt       *time.Time     `gorm:"column:last_login_at" json:"last_login_at"`
	LastLoginIP       string         `gorm:"column:last_login_ip;size:45;not null" json:"last_login_ip"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (*AdminUser) TableName() string { return "admin_users" }

type AdminRole struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Code        string    `gorm:"column:code;size:32;not null" json:"code"`
	Name        string    `gorm:"column:name;size:64;not null" json:"name"`
	Description string    `gorm:"column:description;size:255;not null" json:"description"`
	IsBuiltIn   int       `gorm:"column:is_built_in;not null" json:"is_built_in"`
	Sort        int       `gorm:"column:sort;not null" json:"sort"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"-"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"-"`
}

func (*AdminRole) TableName() string { return "admin_roles" }

type AdminRolePermission struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	RoleCode  string    `gorm:"column:role_code;size:32;not null" json:"role_code"`
	Module    string    `gorm:"column:module;size:32;not null" json:"module"`
	Action    string    `gorm:"column:action;size:16;not null" json:"action"`
	Scope     string    `gorm:"column:scope;size:16;not null;default:'all'" json:"scope"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*AdminRolePermission) TableName() string { return "admin_role_permissions" }

// AdminUserRole 管理员 ↔ 角色 N:M
type AdminUserRole struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	AdminID   int64     `gorm:"column:admin_id;not null" json:"admin_id"`
	RoleCode  string    `gorm:"column:role_code;size:32;not null" json:"role_code"`
	GrantedBy *int64    `gorm:"column:granted_by" json:"granted_by"`
	GrantedAt time.Time `gorm:"column:granted_at;not null;default:CURRENT_TIMESTAMP" json:"granted_at"`
}

func (*AdminUserRole) TableName() string { return "admin_user_roles" }

// AdminAuditLog 操作审计（仅追加 + 哈希链）
type AdminAuditLog struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	AdminID    uint64    `gorm:"column:admin_id;not null" json:"admin_id"`
	Username   string    `gorm:"column:username;size:64;not null;default:''" json:"username"`
	RoleCodes  string    `gorm:"column:role_codes;size:255;not null;default:''" json:"role_codes"`
	IP         string    `gorm:"column:ip;size:45;not null;default:''" json:"ip"`
	UserAgent  string    `gorm:"column:user_agent;size:500;not null;default:''" json:"user_agent"`
	Method     string    `gorm:"column:method;size:10;not null" json:"method"`
	Path       string    `gorm:"column:path;size:255;not null" json:"path"`
	Route      string    `gorm:"column:route;size:255;not null;default:''" json:"route"`
	TargetID   string    `gorm:"column:target_id;size:64;not null;default:''" json:"target_id"`
	ReqBody    string    `gorm:"column:req_body;type:mediumtext" json:"req_body"`
	RespStatus int       `gorm:"column:resp_status;not null;default:0" json:"resp_status"`
	RespHash   string    `gorm:"column:resp_hash;size:64;not null;default:''" json:"resp_hash"`
	PrevHash   string    `gorm:"column:prev_hash;size:64;not null;default:''" json:"prev_hash"`
	Hash       string    `gorm:"column:hash;size:64;not null" json:"hash"`
	DurationMS int       `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	Note       string    `gorm:"column:note;size:500;not null;default:''" json:"note"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (*AdminAuditLog) TableName() string { return "admin_audit_log" }

// AdminApproval 4 眼审批工单
type AdminApproval struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Operation     string     `gorm:"column:operation;size:64;not null" json:"operation"`
	TargetKind    string     `gorm:"column:target_kind;size:32;not null;default:''" json:"target_kind"`
	TargetID      string     `gorm:"column:target_id;size:64;not null;default:''" json:"target_id"`
	Payload       string     `gorm:"column:payload;type:mediumtext;not null" json:"-"`
	Reason        string     `gorm:"column:reason;size:500;not null;default:''" json:"reason"`
	InitiatorID   uint64     `gorm:"column:initiator_id;not null" json:"initiator_id"`
	InitiatorName string     `gorm:"column:initiator_name;size:64;not null;default:''" json:"initiator_name"`
	ApproverID    *uint64    `gorm:"column:approver_id" json:"approver_id"`
	ApproverName  string     `gorm:"column:approver_name;size:64;not null;default:''" json:"approver_name"`
	Status        string     `gorm:"column:status;size:16;not null;default:'pending'" json:"status"`
	RejectReason  string     `gorm:"column:reject_reason;size:500;not null;default:''" json:"reject_reason"`
	Result        *string    `gorm:"column:result;type:text" json:"result"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	ReviewedAt    *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	ExecutedAt    *time.Time `gorm:"column:executed_at" json:"executed_at"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expires_at"`
}

func (*AdminApproval) TableName() string { return "admin_approval" }

// 状态常量
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
	ApprovalExpired  = "expired"
	ApprovalExecuted = "executed"
	ApprovalFailed   = "failed"
)
