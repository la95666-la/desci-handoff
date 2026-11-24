import type { ProjectListRow } from '#/api/joincare/project_content';

import { getFundingContractPair } from './funding-mode';

/**
 * 项目列表操作可用性判定（纯函数，便于单测）。
 */

/**
 * Fair Launch, mismatched and unknown identities stay closed. V3 actions
 * are admitted per method and lifecycle state below, never as V2 identity.
 */
export function isLifecycleMutationBlocked(row: ProjectListRow): boolean {
  const pair = getFundingContractPair(row);
  return pair === 'fair_launch_v1' || pair === 'mismatch' || pair === 'unknown';
}

/** Marketing editors remain content-only for Phase 1. */
export function isMarketingContentEditAllowed(_row: ProjectListRow): boolean {
  return true;
}

/** 链上参数的查看权限与修改权限分离：锁定后仍可查看已保存的 V2 参数。 */
export function isOnchainParamsViewAllowed(row: ProjectListRow): boolean {
  const pair = getFundingContractPair(row);
  return (
    pair === 'capped_presale_v2' ||
    pair === 'capped_presale_v3' ||
    pair === 'historical_unassigned'
  );
}

/** Exact V2 or V3 capped-presale Set only; Backend rechecks chain and outbox. */
export function isCreateOnchainAllowed(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    (getFundingContractPair(row) === 'capped_presale_v2' ||
      getFundingContractPair(row) === 'capped_presale_v3') &&
    row.status === 'Set' &&
    !row.onchain_locked &&
    !row.active_submission
  );
}

/** Exact V2 or V3 Pending project; no V2 fallback for a V3 row. */
export function isActivateOnchainAllowed(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    (getFundingContractPair(row) === 'capped_presale_v2' ||
      getFundingContractPair(row) === 'capped_presale_v3') &&
    row.status === 'Pending' &&
    row.onchain_locked &&
    !row.active_submission
  );
}

/** Exact V2 or V3 Active, locked, no pending attempt and past end. */
export function isEndOnchainAllowed(row: ProjectListRow): boolean {
  if (
    isLifecycleMutationBlocked(row) ||
    !['capped_presale_v2', 'capped_presale_v3'].includes(
      getFundingContractPair(row),
    ) ||
    row.status !== 'Active' ||
    !row.onchain_locked ||
    row.active_submission
  ) {
    return false;
  }
  if (!row.end_time) {
    return false;
  }
  return Date.parse(row.end_time) <= Date.now();
}

/** Operator cancellation remains V2-only; V3 cancellation requires Owner Safe. */
export function isCancelOnchainAllowed(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    getFundingContractPair(row) === 'capped_presale_v2' &&
    (row.status === 'Pending' || row.status === 'Active') &&
    row.onchain_locked &&
    !row.active_submission
  );
}

/** Exact V2 or V3 successful project; Backend verifies payout authority. */
export function isPlatformFeeWithdrawable(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    (getFundingContractPair(row) === 'capped_presale_v2' ||
      getFundingContractPair(row) === 'capped_presale_v3') &&
    row.status === 'Successful' &&
    row.onchain_locked &&
    !row.platform_fee_withdrawn &&
    !row.active_submission
  );
}

/**
 * 恢复被拒的原因（归一化后的业务码）。
 * 后端 restore_blocker 字段透传时按规则归一化；前端推断时自行产生。
 */
export type RestoreBlockerCode =
  | 'ACTIVE_SUBMISSION'
  | 'FAIR_LAUNCH'
  | 'LEGACY_ID2_NO_SOURCE_STATUS'
  | 'MODE_CONTRACT_MISMATCH'
  | 'NOT_ARCHIVED'
  | 'ONCHAIN_LOCKED'
  | 'UNKNOWN'
  | 'V2_STABLECOIN'
  | 'V3_UNAVAILABLE';

/** 恢复能力判定结果 */
export interface RestoreCapability {
  /** 是否允许恢复 */
  allowed: boolean;
  /** 不允许时的原因码（允许时为 null） */
  blocker: null | RestoreBlockerCode;
  /** 后端透传的原始原因（未归一化时可能为任意文本） */
  blockerRaw: null | string;
  /** 归档前的状态（后端透传；legacy ID2 可能缺失） */
  archivedFromStatus: null | string;
  /** 能力是否来自后端显式字段（否 = 前端保守推断） */
  fromBackend: boolean;
}

function normalizeRestoreBlocker(
  raw: null | string,
): null | RestoreBlockerCode {
  if (!raw) {
    return null;
  }
  const text = String(raw)
    .toUpperCase()
    .replaceAll(/[^A-Z0-9_]/g, '');
  if (text.includes('V3') && text.includes('UNAVAILABLE')) {
    return 'V3_UNAVAILABLE';
  }
  if (text.includes('V2') && text.includes('STABLECOIN')) {
    return 'V2_STABLECOIN';
  }
  if (text.includes('FAIR') && text.includes('LAUNCH')) {
    return 'FAIR_LAUNCH';
  }
  if (text.includes('MODE') && text.includes('CONTRACT')) {
    return 'MODE_CONTRACT_MISMATCH';
  }
  if (
    text.includes('ONCHAIN_LOCKED') ||
    (text.includes('ONCHAIN') && text.includes('LOCKED'))
  ) {
    return 'ONCHAIN_LOCKED';
  }
  if (text.includes('SUBMISSION')) {
    return 'ACTIVE_SUBMISSION';
  }
  if (
    text.includes('LEGACY') ||
    text.includes('SOURCE_STATUS') ||
    text.includes('ARCHIVED_FROM')
  ) {
    return 'LEGACY_ID2_NO_SOURCE_STATUS';
  }
  if (text.includes('NOT_ARCHIVED') || text.includes('NOTDELETED')) {
    return 'NOT_ARCHIVED';
  }
  return 'UNKNOWN';
}

/**
 * 恢复能力判定：
 * 1) 后端返回 restore_allowed 时以它为准（restore_blocker / archived_from_status 一并透传）；
 * 2) 旧后端未返回时，前端保守推断（与后端 disable 约束对称）：
 *    仅 archived + 非 v2_stablecoin + 非 onchain_locked + 无在途提交 才允许。
 */
export function getRestoreCapability(row: ProjectListRow): RestoreCapability {
  const archivedFromStatus = row.pre_archive_status ?? null;
  const rawAllowed = row.restore_allowed;
  const fundingPair = getFundingContractPair(row);

  // Backend capability can narrow permissions, but it cannot override an
  // immutable identity rejection. Fair Launch and capped V2 lifecycle rows
  // are never restored through content management; mismatch/unknown fail closed.
  if (
    isLifecycleMutationBlocked(row) ||
    fundingPair === 'capped_presale_v2' ||
    fundingPair === 'capped_presale_v3'
  ) {
    let blocker: RestoreBlockerCode = 'MODE_CONTRACT_MISMATCH';
    if (fundingPair === 'fair_launch_v1') {
      blocker = 'FAIR_LAUNCH';
    } else if (fundingPair === 'capped_presale_v3') {
      blocker = 'V3_UNAVAILABLE';
    } else if (fundingPair === 'capped_presale_v2') {
      blocker = 'V2_STABLECOIN';
    }
    return {
      allowed: false,
      blocker,
      blockerRaw: row.restore_blocker ?? null,
      archivedFromStatus,
      fromBackend: rawAllowed !== undefined && rawAllowed !== null,
    };
  }

  if (rawAllowed !== undefined && rawAllowed !== null) {
    return {
      allowed: rawAllowed === true || rawAllowed === 1,
      blocker: normalizeRestoreBlocker(row.restore_blocker ?? null),
      blockerRaw: row.restore_blocker ?? null,
      archivedFromStatus,
      fromBackend: true,
    };
  }

  // 前端保守推断（旧后端）：没有可信归档前状态时一律拒绝，不能猜目标状态。
  if (row.status !== 'archived') {
    return {
      allowed: false,
      blocker: 'NOT_ARCHIVED',
      blockerRaw: null,
      archivedFromStatus,
      fromBackend: false,
    };
  }
  if (row.onchain_locked) {
    return {
      allowed: false,
      blocker: 'ONCHAIN_LOCKED',
      blockerRaw: null,
      archivedFromStatus,
      fromBackend: false,
    };
  }
  if (row.active_submission) {
    return {
      allowed: false,
      blocker: 'ACTIVE_SUBMISSION',
      blockerRaw: null,
      archivedFromStatus,
      fromBackend: false,
    };
  }
  const allowedSourceStatuses = ['Setup', 'Set', 'pre_launch', 'warmup'];
  if (
    !archivedFromStatus ||
    !allowedSourceStatuses.includes(archivedFromStatus)
  ) {
    return {
      allowed: false,
      blocker: 'LEGACY_ID2_NO_SOURCE_STATUS',
      blockerRaw: 'PROJECT_RESTORE_STATE_UNKNOWN',
      archivedFromStatus,
      fromBackend: false,
    };
  }
  return {
    allowed: true,
    blocker: null,
    blockerRaw: null,
    archivedFromStatus,
    fromBackend: false,
  };
}

/** 是否可「停用/归档」；Phase 1 Fair/mismatch/unknown 均不可触发生命周期写入。 */
export function isDisableAllowed(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    getFundingContractPair(row) === 'historical_unassigned' &&
    row.status !== 'archived' &&
    !row.onchain_locked &&
    !row.active_submission
  );
}

/** 是否可「删除」；仅允许尚未分配链身份且未锁定的历史内容项目。 */
export function isDeleteAllowed(row: ProjectListRow): boolean {
  return (
    !isLifecycleMutationBlocked(row) &&
    getFundingContractPair(row) === 'historical_unassigned' &&
    !row.onchain_locked &&
    !row.active_submission
  );
}

/** 是否可「启用/恢复」（能力判定主入口） */
export function isRestorable(row: ProjectListRow): boolean {
  return getRestoreCapability(row).allowed;
}
