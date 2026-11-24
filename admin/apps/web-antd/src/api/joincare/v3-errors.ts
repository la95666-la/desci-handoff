/** Finite V3 diagnostics: never display arbitrary provider/SQL messages. */
const reasons: Record<string, string> = {
  CREATE_RUNWAY_INSUFFICIENT:
    '距离已批准的创建截止时间过近，无法安全完成签名后复查和广播。请重新准备留有充分余量的未来时间并审批，不要反复点击创建。',
  SENDER_EVIDENCE_PENDING:
    '发送钱包历史证据待维护核验或剩余有效时间不足；请查看维护状态，不要反复提交。',
  SENDER_EVIDENCE_UNAVAILABLE:
    '发送钱包证据存储暂不可用，请管理员检查维护状态。',
  SENDER_RETIREMENT_REVIEW_REQUIRED:
    '存在未登记或已撤销的退休记录，需要管理员核验，自动维护不会授予豁免。',
  UNAVAILABLE: '操作未通过校验，具体原因尚未确认，请按失败阶段联系管理员。',
  CREATE_DEADLINE_EXPIRED:
    '已批准的创建截止时间已过；请按受控流程重新准备未来时间参数，不要继续点击创建。',
  APPROVAL_INVALID: '项目审批与参数未通过一致性校验，请核对已执行工单。',
  CONFIG_UNAVAILABLE: 'V3 服务配置或运行身份不完整，请联系运维。',
  PROJECT_SCAN_HALTED:
    '项目扫描已保护性暂停，请管理员核验原回执并恢复；不要重复发送交易。',
  SCANNER_NOT_FRESH: '扫描核验未完成或已过期，请查看扫描进度后再核对。',
  SENDER_SUSPENDED:
    '共享发送钱包存在暂停签名记录，请先完成原交易对账；不要清理 nonce。',
  SENDER_RECONCILIATION_REQUIRED:
    '共享发送钱包历史签名对账未通过，请管理员核验原记录及节点连接。',
  NONCE_UNRESOLVED: '发送钱包 nonce 或交易槽未对齐，请核对原交易，勿重复提交。',
  RPC_TIMEOUT: '链上只读核验超时，当前未完成核验。',
  CHECK_TIMEOUT:
    '本次核验时间预算已耗尽，结果尚未确认；请先刷新记录，不要重复提交写操作。',
  REQUEST_CANCELLED:
    '请求已取消或连接中断；请先核对原记录，不能据此判断写操作未发生。',
  RPC_UNAVAILABLE: '链上节点暂不可用或请求受限，当前未完成核验。',
  PREFLIGHT_REJECTED:
    '参数、链上身份或模拟未通过，请核对失败阶段；不代表交易已上链。',
  STATE_CHANGED: '项目、策略版本或审批内容在核验期间变化，请刷新核对。',
  REVIEW_INVALID: '工单内容或摘要不匹配，请刷新并核对指定工单。',
  REVIEW_EXPIRED: '工单已过审批有效期，请重新准备候选，不要执行旧提案。',
  DISTINCT_APPROVER_REQUIRED: '二审必须由不同于发起人的管理员完成。',
  PERMISSION_DENIED: '操作人权限不足或已被撤销。',
  REVIEW_CONFLICT: '该版本已有审批／执行记录或工单已接替，请先刷新核对。',
  SAFE_CHECK_FAILED:
    'Safe 提案或执行证据未通过，请核对原交易和 SafeTxHash，勿重新执行。',
  RECEIPT_UNVERIFIED:
    '原交易回执、事件或确认深度未通过核验，不能仅凭交易哈希认定成功。',
};
type RecordValue = Record<string, unknown>;
const stages = new Set(
  `unavailable request_gate approved_set operator_identity safe_identity create_preflight reserve_intent unsigned_intent prepare_signature signed_admission signed_preflight broadcast_original rpc_setup creation_admission project_scan_canonical lifecycle_preflight signed_project_scan_canonical transaction_guard signature_persist nonce_check sender_check gas_estimate input mode_ready project_params chain_preview project_admission confirmed_snapshot governance cursor_canonical policy_simulation latest_chain_state cursor_transaction final_project_admission prior_policy candidate review_recompute review_digest review_read status live_state prior_receipt final_admission review_commit approval_commit safe_handoff safe_proposal safe_service confirmed_head project policy final_anchor repeat_policy admission scanner project_policy final_recheck scanner_freshness immutable_review review_commitment canonical_policy_receipt safe_execution governance_identity current_policy repeat_receipt final_scanner_freshness atomic_cursor_incident atomic_project_lock atomic_review_identity atomic_initiator_permission atomic_approver_permission atomic_execution_insert receipt_lookup confirmations receipt_header receipt_policy_match head_policy_match`.split(
    ' ',
  ),
);
stages.add('sender_readiness');
const stageLabels: Record<string, string> = {
  request_gate: '入口检查', approved_set: '已批准参数核验',
  operator_identity: '操作钱包身份核验', safe_identity: 'Safe 身份核验',
  create_preflight: '创建前核验', reserve_intent: '建立交易记录',
  unsigned_intent: '未签名记录检查', prepare_signature: '准备签名',
  signed_admission: '签名后准入复查', signed_preflight: '原签名交易复查',
  broadcast_original: '原交易广播', rpc_setup: '节点连接初始化',
  lifecycle_preflight: '生命周期操作预检', signature_persist: '签名持久化',
  sender_readiness: '发送钱包就绪检查', nonce_check: '交易序号与活动槽核验',
  project_scan_canonical: '项目扫描区块核验', signed_project_scan_canonical: '签名后项目区块核验',
  gas_estimate: '交易费用估算', project_params: '项目参数校验',
  chain_preview: '链上参数预览', policy_simulation: '白名单调用模拟',
  review_recompute: '重新核验审批候选', review_digest: '审批摘要核验',
  review_read: '读取审批工单', review_commit: '保存审批工单',
  approval_commit: '保存二审结果', final_recheck: '最终提交前复查',
  final_admission: '最终准入检查', status: '读取白名单状态',
  safe_handoff: '导出 Safe 执行参数', safe_proposal: 'Safe 提案核验',
  safe_service: 'Safe 提案服务', safe_execution: 'Safe 执行凭据核验',
  receipt_lookup: '查询原交易回执', confirmations: '确认区块深度',
  canonical_policy_receipt: '白名单规范回执核验',
  policy_evidence_store: '白名单证据存储', final_policy_evidence_store: '最终白名单证据存储',
  sender_maintenance_final_nonce: '维护结束序号复查',
  retired_chain_identity: '历史交易链身份核验',
  original_signature: '原始签名核验', original_rpc_chain: '原交易节点链身份核验',
  original_nonce: '原交易序号核验', original_hash_absence: '原交易未广播核验',
  original_balance: '原发送钱包余额核验', original_exact_simulation: '原交易精确模拟',
};
for (const stage of Object.keys(stageLabels)) stages.add(stage);
const stageGroups: Array<[string,string]> = [
  ['项目链上身份及准入核验','creation_admission transaction_guard input mode_ready project_admission final_project_admission admission project atomic_project_lock'],
  ['规范区块及来源核验','confirmed_snapshot cursor_canonical latest_chain_state cursor_transaction confirmed_head final_anchor atomic_cursor_incident receipt_header head_policy_match'],
  ['扫描证据有效性核验','scanner scanner_freshness final_scanner_freshness'],
  ['白名单版本及历史凭据核验','project_policy prior_policy candidate live_state prior_receipt policy repeat_policy immutable_review review_commitment governance_identity current_policy repeat_receipt receipt_policy_match'],
  ['管理员权限与工单一致性核验','atomic_review_identity atomic_initiator_permission atomic_approver_permission atomic_execution_insert'],
  ['历史交易证据及维护核验','retired_marker_lookup retired_marker_membership sender_maintenance_deadline sender_maintenance_selection sender_maintenance_enrollment sender_maintenance_provider_agreement sender_maintenance_commit retired_original_signature retired_original_transaction retired_confirmed_anchor retired_consumed_nonce retired_original_receipt retired_receipt_anchor retired_journal_match retired_repeat_receipt retired_final_anchor retired_final_nonce'],
  ['发送钱包核验','sender_check'],
  ['未确定具体阶段','unavailable'],
];
for (const [label,list] of stageGroups) for(const stage of list.split(' ')) {
  stageLabels[stage] ??= label;
  stages.add(stage);
}
for (const stage of `retired_marker_lookup retired_marker_membership sender_maintenance_deadline sender_maintenance_selection sender_maintenance_enrollment sender_maintenance_provider_agreement sender_maintenance_commit retired_original_signature retired_original_transaction retired_confirmed_anchor retired_consumed_nonce retired_original_receipt retired_receipt_anchor retired_journal_match retired_repeat_receipt retired_final_anchor retired_final_nonce`.split(' ')) stages.add(stage);

export function diagnosticCode(value: unknown): string {
  return typeof value === 'string' && /^[A-Z][A-Z0-9_]{1,95}$/.test(value) ? value : '';
}
function object(value: unknown): RecordValue {
  return value !== null && typeof value === 'object'
    ? (value as RecordValue)
    : {};
}
export function v3ErrorBody(error: unknown): RecordValue {
  const outer = object(error);
  return object(object(outer.response).data ?? outer);
}
export function formatV3Diagnostic(body: unknown): null | string {
  const r = object(body);
  const reason = typeof r.reason_code === 'string' ? r.reason_code : '';
  if (!Object.hasOwn(reasons, reason)) return null;
  const parts = [reasons[reason], `原因代码：${reason}`];
  const code = diagnosticCode(r.code);
  if (code && code !== reason) parts.push(`接口错误码：${code}`);
  // Reconciliation instruction wins even if an inconsistent response says
  // not_reserved. Never infer "not signed" from a reason code alone.
  if (r.submission_outcome === 'signed_or_submitted' || r.next_action === 'reconcile_original_do_not_resend') {
    parts.push('已签名或结果未确认，请只核对原交易，切勿重复发送。');
  } else if (r.submission_outcome === 'not_reserved') {
    parts.push(
      '本次尚未建立新的提交记录；此状态不代表其他历史交易已经完成。',
    );
  } else if (r.submission_outcome === 'unsigned_cancelled') {
    parts.push(
      '本次交易记录已确认未签名并取消，未广播；这不代表可以不经重新校验再次提交。',
    );
  } else if (r.submission_outcome === 'unknown') {
    parts.push('本次提交结果尚未确认；先刷新工单或交易记录，勿重复提交。');
  }
  if (typeof r.stage === 'string' && stages.has(r.stage))
    parts.push(`失败阶段：${stageLabels[r.stage] ?? '执行核验'}（${r.stage}）`);
  if (r.next_action === 'check_sender_maintenance' && r.submission_outcome !== 'signed_or_submitted') parts.push('下一步：查看发送钱包维护状态；不要反复点击。');
  if (
    typeof r.submission_id === 'string' &&
    /^[1-9]\d{0,19}$/.test(r.submission_id)
  )
    parts.push(`交易记录：#${r.submission_id}`);
  if (typeof r.tx_hash === 'string' && /^0x[\da-fA-F]{64}$/.test(r.tx_hash))
    parts.push(`原交易：${r.tx_hash}`);
  if (typeof r.request_id === 'string' && /^[\da-f]{24}$/.test(r.request_id))
    parts.push(`排查编号：${r.request_id}`);
  return parts.join(' ');
}
export function v3DiagnosticMessage(error: unknown, fallback: string): string {
  return formatV3Diagnostic(v3ErrorBody(error)) ?? `${fallback} [IDO_CHECK_UNAVAILABLE]`;
}
