import type { V3SenderReadiness } from '#/api/joincare/project_content';

import { formatV3Diagnostic } from '#/api/joincare/v3-errors';

// Diagnostic UI snapshot is never signing authority. Backend repeats its live
// gate and nonce checks on EVERY submission. No cached-ready auto submission.
export function senderReadinessLabel(
  value: null | V3SenderReadiness,
  checking = false,
): string {
  if (checking) return '正在读取发送钱包状态… [SENDER_READINESS_CHECKING]';
  if (!value) return '发送钱包状态未知，请刷新核验 [SENDER_READINESS_UNKNOWN]';
  if (value.active_submission_id || value.reason_code === 'NONCE_UNRESOLVED')
    return '存在未决交易，请按原交易核验，切勿重发 [NONCE_UNRESOLVED]';
  if (value.ready) return '发送钱包可操作（提交时仍会实时核验） [SENDER_READY]';
  if (value.reason_code === 'SENDER_EVIDENCE_PENDING')
    return '历史证据待维护核验或有效时间不足，页面只读检查状态，不会自动重提交易 [SENDER_EVIDENCE_PENDING]';
  return formatV3Diagnostic(value) ?? '发送钱包核验异常，请联系管理员查看维护日志 [SENDER_READINESS_UNAVAILABLE]';
}

export function senderReadinessAllowsSubmit(value: null | V3SenderReadiness) {
  return value?.ready === true && !value.active_submission_id;
}
