// Package approval 4 眼审批引擎。
//
// 高风险操作走"发起 → 二审 → 系统重放执行"三段式：
//
//  1. 发起人调"业务端点"（如 POST /users/:id/disable）
//     → controller 里 approval.Submit(op, target, payload, initiator)
//     → 返 { approval_id, status: "pending" }（**不立刻执行**）
//
//  2. 另一位 admin 调 POST /admin/v1/approvals/:id/approve
//     → approval.Approve(id, approver) 验发起人 ≠ 审批人
//     → 调 handler 用当时的 payload 重放执行
//     → 结果落 approval.Result，状态 executed / failed
//
//  3. 或 POST /admin/v1/approvals/:id/reject（可给理由）
//
// 通过 Register(op, handler) 注册操作；重启时集中在 init.go 里注册即可。
//
// 4 眼安全：
//   - initiator_id != approver_id 硬约束
//   - approval TTL（默认 72h）过期自动 expired
//   - super_admin 也不能自审自批
//   - 每次状态跃迁写审计（audit middleware 已覆盖 POST /approvals/:id/*）
package approval

import (
	"errors"
	"sync"

	"gorm.io/gorm"
)

// Handler 从 payload JSON 重放业务动作
// 返回 result JSON（存 approval.result 供审计），err 决定 executed / failed
type Handler func(tx *gorm.DB, payload string, approverID uint64) (result string, afterCommit func(), err error)

type operationSpec struct {
	handler         Handler
	module          string
	action          string
	initiatorAction string
}

var (
	regMu sync.RWMutex
	reg   = make(map[string]operationSpec, 16)
)

// Register 注册操作码 → 执行器；重复注册会 panic（防漏）
func Register(op, module, action string, h Handler) {
	regMu.Lock()
	defer regMu.Unlock()
	if op == "" || module == "" || action == "" || h == nil {
		panic("approval: operation registration is incomplete")
	}
	if _, exists := reg[op]; exists {
		panic("approval: op already registered: " + op)
	}
	initiatorAction := "write"
	if action == "execute" {
		initiatorAction = "execute"
	}
	reg[op] = operationSpec{handler: h, module: module, action: action, initiatorAction: initiatorAction}
}

// Get 拿注册好的 handler
func Get(op string) (operationSpec, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	spec, ok := reg[op]
	if !ok {
		return operationSpec{}, errors.New("approval: unknown op " + op)
	}
	return spec, nil
}

// AllOps 列出注册过的所有操作码（用于 admin 端 dropdown 选筛选）
func AllOps() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for k := range reg {
		out = append(out, k)
	}
	return out
}
