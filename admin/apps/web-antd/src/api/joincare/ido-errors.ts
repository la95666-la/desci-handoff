import { diagnosticCode, formatV3Diagnostic, v3ErrorBody } from './v3-errors';

/** Exact code -> meaning. Never guess a failed guard from an English message. */
const codes: Record<string, string> = {
  ADMIN_PERM_DENIED: '当前账号缺少本操作所需权限；请求在业务校验前被拒绝。请联系权限管理员核对角色授权，不是链上参数校验失败。',
  ADMIN_AUTH_REQUIRED: '需要有效的管理员登录，请重新登录后只读核对原记录。',
  ADMIN_SESSION_REVOKED: '当前管理员会话已失效，请重新登录；先核对原操作记录。',
  ADMIN_TOKEN_INVALID: '管理员登录凭证无效或已过期，请重新登录。',
  ADMIN_TOKEN_KIND_INVALID: '当前凭证不适用于管理接口，请重新登录。',
  ADMIN_ROLE_UNAVAILABLE: '管理员角色无法读取，请联系权限管理员；不能跳过权限检查。',
  PROJECT_BAD_ID: '项目编号无效，请从项目列表重新进入。',
  BAD_PROJECT_ID: '项目编号无效，请从项目列表重新进入。',
  PROJECT_NOT_FOUND: '项目不存在或当前无法读取，请核对项目编号和权限。',
  PROJECT_BAD_REQUEST: '请求参数格式不正确，请核对必填字段和时间格式。',
  PROJECT_EXECUTE_REVOKED: '当前账号已无项目执行权限，请联系权限管理员。',
  PROJECT_ONCHAIN_PARAMS_INVALID: 'Set 参数未通过校验；请核对代币、地址、时间、额度、精度和解锁计划。具体失败项尚需核验，不要提交旧参数。',
  PROJECT_ONCHAIN_PARAMS_FAILED: 'Set 审批请求未获得成功确认；请先刷新工单，确认是否已创建，避免重复提交。',
  PROJECT_CREATE_ONCHAIN_FAILED: '创建上链未获得成功确认；请先查原交易记录，不能据此认定未签名或未广播。',
  PROJECT_ACTIVATE_ONCHAIN_FAILED: '激活未获得成功确认；请核对项目状态及原交易记录，不要重复发送。',
  PROJECT_ACTIVATE_RELAYER_UNAVAILABLE: '激活服务暂不可用，尚不能确认操作结果；请检查服务并核对原记录。',
  PROJECT_END_ONCHAIN_FAILED: '结束项目未获得成功确认；请核对生命周期及原交易记录，不要重复发送。',
  PROJECT_CANCEL_ONCHAIN_FAILED: '取消项目未获得成功确认；请核对原交易和版本支持范围，不要重复发送。',
  PLATFORM_FEE_WITHDRAW_FAILED: '平台费提取未获得成功确认；请核对原回执及实际收款地址，不要重复发送。',
  IDO_PAUSE_FAILED: '全局暂停未获得成功确认；先核对链上状态和原记录。',
  IDO_UNPAUSE_FAILED: '解除全局暂停未获得成功确认；先核对链上状态和原记录。',
  IDO_PAUSE_STATE_FAILED: '无法读取全局暂停状态，当前显示不能作为执行依据。',
  V3_POLICY_SCOPE_INVALID: '白名单项目或工单范围无效，请核对项目编号与工单编号。',
  V3_POLICY_INPUT_INVALID: '白名单输入格式不正确，请核对钱包、去重结果、原因及候选摘要。',
  V3_POLICY_PROJECT_NOT_CREATED: '项目尚未完成创建上链，请先确认创建交易已正式应用。',
  V3_POLICY_PREVIEW_UNAVAILABLE: '白名单候选预览未通过；不能据此发起审批或执行。',
  V3_POLICY_STATUS_UNAVAILABLE: '当前白名单状态读取不可用；旧状态不能作为执行依据。',
  V3_POLICY_APPROVAL_UNAVAILABLE: '白名单二审未获得成功确认；请先刷新工单核对，不要重复批准。',
  V3_POLICY_SAFE_HANDOFF_UNAVAILABLE: 'Safe 执行参数不可用；不要使用旧参数签名，请核对已批准工单。',
  V3_POLICY_REVIEW_UNAVAILABLE: '审批工单不可用，请刷新并核对工单内容、权限和状态。',
  V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED: '白名单审批所需数据库迁移未完成，请联系运维。',
  V3_POLICY_REVIEW_START_UNAVAILABLE: '白名单发起审批未获得成功确认，请先查询已有工单。',
  V3_POLICY_REVIEW_APPROVE_UNAVAILABLE: '白名单二审未获得成功确认，请先查询工单状态及审批人。',
  V3_POLICY_EXECUTION_UNAVAILABLE: 'Safe 执行凭据未核验通过；请查原 SafeTxHash 与链上回执，不要重新执行。',
  SAFE_SIGNATURE_REQUIRED: '缺少符合格式要求的 Safe Owner 签名，请重新核对待签提案。',
  SAFE_PROPOSAL_UNAVAILABLE: 'Safe 提案未获得成功确认；按原 SafeTxHash 查询提案服务，不要直接重新签名或提交。',
  V3_TOKEN_INVALID: '代币地址或项目范围无效，请核对当前入口支持的代币与编号。',
  V3_TOKEN_GATE_UNAVAILABLE: '代币准入核验不可用，请联系运维，不能跳过准入检查。',
  V3_PROJECT_TOKEN_GATE_UNAVAILABLE: '项目币准入核验不可用，请核对代币登记及链上状态。',
  V3_PROJECT_TOKEN_RECEIPT_UNVERIFIED: '项目币准入回执尚未核验通过，请核对原交易。',
  V3_TOKEN_RECEIPT_UNVERIFIED: '代币准入回执尚未核验通过，请核对原交易。',
  V3_RUNTIME_UNAVAILABLE: '合约运行身份核验不可用，请联系运维。',
  V3_RUNTIME_REVIEW_UNAVAILABLE: '合约运行身份登记审批不可用，请核对原工单。',
  TRACKING_UNAVAILABLE: '结清／跟踪状态不可用，不能据此认定项目已结清。',
  TRACKING_DISABLED: '结清／跟踪功能未启用或当前身份不可用，请联系管理员。',
  TRACKING_REASON_REQUIRED: '请填写 3–500 字的审计原因。',
  TRACKING_CHANGE_REJECTED: '跟踪变更未通过；请刷新核对版本、生命周期、来源健康与未决交易。不能据此认定已归档。',
  BAD_SUBMISSION: '原交易编号、哈希或请求格式无效，请从交易详情重新核对。',
  AUDIT_REASON_REQUIRED: '请填写符合要求的操作原因。',
  V3_ORIGINAL_CONTINUATION_REJECTED: '原交易首次广播未获确认；只核对原编号和哈希，切勿另建交易或重发。',
  ADMIN_APPRV_LIST_FAILED: '审批列表读取失败，请刷新后核对。',
  ADMIN_APPRV_BAD_ID: '审批单编号无效。',
  ADMIN_APPRV_NOT_FOUND: '审批单不存在或不可读取。',
  ADMIN_APPRV_APPROVE_FAILED: '审批／执行未获成功确认；先查询工单实际状态，不能直接重复批准。',
  ADMIN_APPRV_REJECT_FAILED: '驳回结果未获确认，请先刷新工单。',
  ADMIN_APPRV_CANCEL_FAILED: '撤单结果未获确认，请先刷新工单。',
  CHAIN_TX_BAD_ID: '交易记录编号无效。',
  CHAIN_TX_BAD_PROJECT_ID: '交易查询的项目编号无效。',
  CHAIN_TX_BAD_STATE: '交易筛选状态无效。',
  CHAIN_TX_BAD_ACTION: '交易操作类型无效。',
  CHAIN_TX_LIST_FAILED: '交易列表读取失败，不能据此认定没有交易。',
  CHAIN_TX_SUMMARY_FAILED: '交易汇总读取失败，请核对原记录。',
  CHAIN_TX_NOT_FOUND: '交易记录不存在或不可读取。',
  CHAIN_TX_NOT_CANCELLABLE: '当前记录不允许人工取消；已签名或广播的交易必须走原交易对账。',
  CHAIN_TX_CANCEL_FAILED: '取消记录未获成功确认，请刷新核对，不能据此解除交易序号限制。',
  CHAIN_TX_ADMIN_REQUIRED: '需要有效管理员登录。',
  CHAIN_TX_ADMIN_REVOKED: '当前管理员执行权限已撤销。',
  CHAIN_TX_CANCEL_REASON_REQUIRED: '人工取消必须填写审计原因。',
  CHAIN_TX_CANCEL_REASON_INVALID: '人工取消原因须为 3–500 字。',
  ONCHAIN_RECONCILIATION_RPC_UNAVAILABLE: '链上对账节点未配置或不可用，请联系运维。',
  ONCHAIN_RECONCILIATION_FAILED: '链上对账读取未完成，不能据此认定业务已完成。',
  ONCHAIN_RECONCILIATION_DB_CHANGED: '对账期间数据发生变化，请刷新并重新核验原记录。',
  ONCHAIN_RECONCILIATION_NOT_READY: '对账检查尚未就绪，请核对来源、配置及原记录。',
  ONCHAIN_RECONCILIATION_PROJECT_NOT_FOUND: '未找到对账项目，请核对项目编号。',
  ONCHAIN_RECONCILIATION_PROJECT_NOT_READY: '项目当前不满足对账条件，请核对版本及链上创建状态。',
};

export function formatIdoCode(body: unknown): null | string {
  const r = v3ErrorBody(body);
  const code = diagnosticCode(r.code);
  if(code==='ADMIN_PERM_DENIED'){
    const labels:Record<string,string>={
      'm2_project:read':'项目查看', 'm2_project:write':'项目编辑／Set 参数预览与提交审批',
      'm2_project:approve':'项目二审', 'm2_project:execute':'项目执行／链上操作',
      'approval:read':'审批查看', 'approval:approve':'审批复核',
    };
    const permission=typeof r.required_permission==='string'?r.required_permission:'';
    return `${codes[code]}${Object.hasOwn(labels,permission)?` 所需权限：${labels[permission]}（${permission}）。`:''} 错误码：${code}`;
  }
  return code && Object.hasOwn(codes, code) ? `${codes[code]} 错误码：${code}` : null;
}

/** Unknown server codes remain honest: generic Chinese + validated code only. */
export function idoErrorMessage(error: unknown, fallback = '操作未获得成功确认，请先刷新并核对原工单或交易记录。'): string {
  const r = v3ErrorBody(error);
  const known = formatV3Diagnostic(r) ?? formatIdoCode(r);
  if (known) return known;
  const code = diagnosticCode(r.code) || diagnosticCode(r.reason_code) || 'IDO_RESULT_UNCONFIRMED';
  return `${fallback} 错误码：${code}`;
}

export function isIdoRequest(url: unknown): boolean {
  return /\/(?:admin\/)?v1\/(?:projects(?:\/|\?|$)|approvals(?:\/|\?|$)|chain-transactions(?:\/|\?|$)|ido-v[23](?:\/|\?|$)|ido(?:\/|\?|$))/.test(String(url ?? ''));
}

/** Stored free-text failures have no trustworthy machine classification. */
export function storedIdoFailure(value: unknown): string {
  if (!value) return '—';
  if (typeof value === 'object') return idoErrorMessage(value);
  return formatIdoCode({ code: value }) ?? '历史错误未提供结构化诊断；请由运维按原工单／交易编号核验日志。错误码：IDO_STORED_ERROR_UNCLASSIFIED';
}
