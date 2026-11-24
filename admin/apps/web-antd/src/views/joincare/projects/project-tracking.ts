import type { ProjectListRow } from '#/api/joincare/project_content';

export function trackingActions(row: ProjectListRow) {
  const t = row.tracking;
  if (!t || row.contract_version !== 'ido_v3_stablecoin' || !row.onchain_locked)
    return [];
  const result = [{ action: 'sync', label: '立即同步' }];
  if (t.mode !== 'normal')
    result.push({ action: 'normal', label: '恢复正常跟踪' });
  if (
    t.mode === 'normal' &&
    ['Canceled', 'Failed', 'Successful'].includes(row.status) &&
    !row.active_submission
  )
    result.push({ action: 'maintenance', label: '低频维护（12小时）' });
  if (
    t.finished &&
    t.evidence_current &&
    !t.wake_block &&
    t.mode !== 'archived'
  )
    result.push({ action: 'archive', label: '归档已结清项目' });
  return result;
}

export function trackingLabel(row: ProjectListRow) {
  const t = row.tracking;
  if (!t) return '';
  const settlement = t.finished
    ? `Finished（原结果 ${row.status}）`
    : `未结清：${t.pending_reason}`;
  return `${settlement} · ${t.mode === 'maintenance' ? '12小时维护' : t.mode === 'archived' ? '已归档' : '正常跟踪'}`;
}

// Presentation only. Never infer archive/maintenance permission from colors.
export function trackingPresentation(row: ProjectListRow) {
  const t = row.tracking;
  if (!t) return null;
  const reasons: Record<string, string> = {
    user_refunds_pending: '等待用户退款',
    user_claims_pending: '等待用户领取',
    project_proceeds_pending: '等待项目方提款',
    platform_fee_pending: '等待平台费提取',
    project_tokens_pending: '等待项目代币取回',
    not_verified: '尚未核验',
    source_recovery_required: '等待同步恢复',
  };
  return {
    settlement: t.finished ? '已结清' : '未结清',
    settlementColor: t.finished ? 'success' : 'warning',
    reason: t.finished
      ? `链上结果：${row.status}`
      : reasons[t.pending_reason] || t.pending_reason || '等待核验',
    mode:
      t.mode === 'maintenance'
        ? '12小时维护'
        : t.mode === 'archived'
          ? '已归档'
          : '正常跟踪',
    modeColor:
      t.mode === 'maintenance'
        ? 'purple'
        : t.mode === 'archived'
          ? 'default'
          : 'blue',
    reasonCode: t.pending_reason,
  };
}
