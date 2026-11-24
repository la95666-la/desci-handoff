// Package v3diagnostic defines a finite, secret-free API diagnostic contract.
// Raw RPC/SQL errors are retained for errors.Is/As, never serialized.
package v3diagnostic

import "errors"

type Error struct {
	Reason, Stage string
	Cause         error
}

func (e *Error) Error() string { return "V3 operation rejected: " + e.Reason }
func (e *Error) Unwrap() error { return e.Cause }

var reasons = map[string]string{
	"CREATE_RUNWAY_INSUFFICIENT":        "距离已批准的创建截止时间过近，无法安全完成签名后复查与广播；本次检查拒绝继续，请重新准备留有充分余量的未来时间并审批。不要反复点击创建。",
	"SENDER_EVIDENCE_PENDING":           "发送钱包历史证据待维护核验或剩余有效时间不足；请查看维护状态，不要反复提交。具体签名／广播结果以本次提交状态和原交易记录为准。",
	"SENDER_EVIDENCE_UNAVAILABLE":       "发送钱包证据存储暂不可用，本次核验未通过；请检查维护状态并核对原交易记录，不能据此判断是否已签名。",
	"SENDER_RETIREMENT_REVIEW_REQUIRED": "存在未登记或已撤销的历史退休记录，自动维护不会授予豁免；请按原交易核验处理。",
	"UNAVAILABLE":                       "本次操作未通过校验；请提供错误码和失败阶段供管理员排查。",
	"CREATE_DEADLINE_EXPIRED":           "已批准的创建截止时间已过，无法创建上链；请按受控流程重新准备未来时间参数，不要继续点击创建。",
	"APPROVAL_INVALID":                  "未找到与项目参数一致的已执行审批，或审批内容已变化；请核对工单及项目参数。",
	"CONFIG_UNAVAILABLE":                "当前 V3 服务配置或运行身份不完整，请联系运维；未因此获得任何执行授权。",
	"PROJECT_SCAN_HALTED":               "项目扫描已保护性暂停，需管理员核验原回执并恢复；不要重复发送交易。",
	"SCANNER_NOT_FRESH":                 "扫描核验尚未完成或已过期；请查看扫描进度，不能把旧结果作为本次操作依据。",
	"SENDER_SUSPENDED":                  "共享发送钱包存在尚未完成对账的暂停记录，禁止新签名；请先核验相关原交易，不要清理 nonce。",
	"SENDER_RECONCILIATION_REQUIRED":    "共享发送钱包的历史签名对账未通过；可能是暂停记录、证据过期或节点读取失败，请管理员核对原记录，不要清理 nonce。",
	"NONCE_UNRESOLVED":                  "发送钱包 nonce 或活动交易槽尚未对齐；请按原交易记录对账，不要重复提交。",
	"RPC_TIMEOUT":                       "链上只读查询超时，尚未完成本次核验。",
	"CHECK_TIMEOUT":                     "本次操作的核验时间预算已耗尽，结果尚未确认；请先刷新记录，不要重复提交写操作。",
	"REQUEST_CANCELLED":                 "请求已取消或连接中断；不能据此判断写操作是否完成，请先刷新并核对原记录。",
	"RPC_UNAVAILABLE":                   "链上节点暂不可用或请求受限，尚未完成本次核验。",
	"PREFLIGHT_REJECTED":                "当前参数、链上身份或模拟未通过校验；请核对参数及失败阶段，不能据此认定交易已上链。",
	"STATE_CHANGED":                     "项目、名单版本或审批内容在核验期间发生变化，请刷新并重新核对。",
	"REVIEW_INVALID":                    "工单不存在、内容不完整或摘要不匹配，请刷新并核对指定工单。",
	"REVIEW_EXPIRED":                    "工单已超过有效审批时间，请重新准备候选；不要继续执行旧提案。",
	"DISTINCT_APPROVER_REQUIRED":        "二审必须由不同于发起人的有效管理员完成。",
	"PERMISSION_DENIED":                 "当前操作人的有效权限不足或已被撤销。",
	"REVIEW_CONFLICT":                   "该版本已有审批或执行记录，或本工单已被接替；请刷新工单列表后核对，不要重复提交。",
	"SAFE_CHECK_FAILED":                 "Safe 身份、提案或执行证据未通过核验；请核对原 SafeTxHash 和链上交易，不要重新执行交易。",
	"RECEIPT_UNVERIFIED":                "原交易回执、事件或确认深度尚未通过核验；不能仅凭交易哈希判断成功。",
}

// No arbitrary phase text from a provider/caller is allowed across HTTP.
var stages = map[string]bool{}

func init() {
	for _, s := range []string{"retired_marker_lookup", "retired_marker_membership", "sender_maintenance_deadline"} {
		stages[s] = true
	}
	stages["sender_readiness"] = true
	for _, s := range []string{"sender_maintenance_selection", "sender_maintenance_enrollment", "sender_maintenance_provider_agreement", "sender_maintenance_final_nonce", "sender_maintenance_commit", "retired_original_signature", "retired_chain_identity", "retired_original_transaction", "retired_confirmed_anchor", "retired_consumed_nonce", "retired_original_receipt", "retired_receipt_anchor", "retired_journal_match", "retired_repeat_receipt", "retired_final_anchor", "retired_final_nonce"} {
		stages[s] = true
	}
	for _, stage := range []string{"original_signature", "original_rpc_chain", "original_nonce", "original_hash_absence", "original_balance", "original_exact_simulation"} {
		stages[stage] = true
	}
	stages["policy_evidence_store"] = true
	stages["final_policy_evidence_store"] = true
	for _, s := range []string{
		"unavailable", "request_gate", "approved_set", "operator_identity", "safe_identity", "create_preflight", "reserve_intent", "unsigned_intent", "prepare_signature", "signed_admission", "signed_preflight", "broadcast_original", "rpc_setup", "creation_admission", "project_scan_canonical", "lifecycle_preflight", "signed_project_scan_canonical", "transaction_guard", "signature_persist", "nonce_check", "sender_check", "gas_estimate",
		"input", "mode_ready", "project_params", "chain_preview", "project_admission", "confirmed_snapshot", "governance", "cursor_canonical", "policy_simulation", "latest_chain_state", "cursor_transaction", "final_project_admission", "prior_policy", "candidate", "review_recompute", "review_digest", "review_read", "status", "live_state", "prior_receipt", "final_admission", "review_commit", "approval_commit", "safe_handoff", "safe_proposal", "safe_service", "confirmed_head", "project", "policy", "final_anchor", "repeat_policy", "admission",
		"scanner", "project_policy", "final_recheck", "scanner_freshness", "immutable_review", "review_commitment", "canonical_policy_receipt", "safe_execution", "governance_identity", "current_policy", "repeat_receipt", "final_scanner_freshness", "atomic_cursor_incident", "atomic_project_lock", "atomic_review_identity", "atomic_initiator_permission", "atomic_approver_permission", "atomic_execution_insert", "receipt_lookup", "confirmations", "receipt_header", "receipt_policy_match", "head_policy_match",
	} {
		stages[s] = true
	}
}
func Reason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		if _, ok := reasons[e.Reason]; ok {
			return e.Reason
		}
	}
	return "UNAVAILABLE"
}
func Stage(err error) string {
	var e *Error
	if errors.As(err, &e) && stages[e.Stage] {
		return e.Stage
	}
	return "unavailable"
}
func Message(reason string) string {
	if s, ok := reasons[reason]; ok {
		return s
	}
	return reasons["UNAVAILABLE"]
}
func New(reason, stage string, cause error) error {
	if _, ok := reasons[reason]; !ok {
		reason = "UNAVAILABLE"
	}
	if !stages[stage] {
		stage = "unavailable"
	}
	return &Error{reason, stage, cause}
}

// Annotate preserves a more specific lower-layer reason and its exact stage.
func Annotate(err error, reason, stage string) error {
	if err == nil {
		return nil
	}
	var known *Error
	if errors.As(err, &known) {
		return err
	}
	return New(reason, stage, err)
}
