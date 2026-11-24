import type {
  FairLaunchActionDescriptor,
  FairLaunchAdminAction,
  FairLaunchCapabilityEnvelope,
  FairLaunchExecutionQueueItem,
} from '#/api/joincare/fair_launch';
import type {
  ProjectContent,
  ProjectListRow,
} from '#/api/joincare/project_content';

import { getFundingContractPair, isExactFairLaunchPair } from './funding-mode';

export interface FairLaunchActionAccess {
  allowed: boolean;
  descriptor?: FairLaunchActionDescriptor;
  reason:
    | 'action_disabled'
    | 'backend_capability_unavailable'
    | 'identity_mismatch'
    | 'materialization_unavailable'
    | 'missing_permission'
    | 'runtime_disabled'
    | 'runtime_unverified'
    | 'step_up_mfa_required'
    | 'three_party_separation'
    | null;
}

/**
 * Fair Launch mutations are fail-closed. The UI never derives action authority
 * from project status alone; a fresh backend descriptor is mandatory.
 */
export const FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE =
  'fair_launch_materialization_unavailable';

function normalizedOperation(value: string): string {
  return value.trim().toLowerCase();
}

export function isFairLaunchApprovalOperation(operation: string): boolean {
  const normalized = normalizedOperation(operation);
  return (
    normalized.startsWith('fair_launch.') ||
    normalized.startsWith('project.fair_launch.')
  );
}

/** Fair Launch approvals are append-only audit records while materialization is frozen. */
export function isFairLaunchApprovalMutationAvailable(
  operation: string,
): boolean {
  return !isFairLaunchApprovalOperation(operation);
}

/** Backward-compatible spelling for existing callers; all mutations share this gate. */
export const isFairLaunchApprovalExecutionAvailable =
  isFairLaunchApprovalMutationAvailable;

export async function runApprovalMutation<T>(
  operation: string,
  mutate: () => Promise<T>,
): Promise<T> {
  if (!isFairLaunchApprovalMutationAvailable(operation)) {
    throw new Error(FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE);
  }
  return mutate();
}

/** Any present or future Fair Launch durable action remains audit-only during freeze. */
export function isFairLaunchDurableAction(action: string): boolean {
  return normalizedOperation(action).startsWith('fair_launch_');
}

export function isFairLaunchEventContractVersion(
  contractVersion: string,
): boolean {
  return normalizedOperation(contractVersion).startsWith('fair_launch_');
}

export async function loadExactFairLaunchExecutionQueue(input: {
  loadProject: (projectId: number | string) => Promise<ProjectContent>;
  loadQueue: (
    projectId: number | string,
  ) => Promise<{ items: FairLaunchExecutionQueueItem[] }>;
  projectId: number | string;
}): Promise<{ items: FairLaunchExecutionQueueItem[] }> {
  if (
    !/^[1-9]\d*$/.test(String(input.projectId)) ||
    (typeof input.projectId === 'number' &&
      !Number.isSafeInteger(input.projectId))
  ) {
    throw new Error('fair_launch_project_identity_unavailable');
  }
  const project = await input.loadProject(input.projectId);
  if (!isExactFairLaunchPair(project)) {
    throw new Error('fair_launch_project_identity_unavailable');
  }
  return input.loadQueue(input.projectId);
}

export function getFairLaunchActionAccess(input: {
  action: FairLaunchAdminAction;
  capabilities: FairLaunchCapabilityEnvelope | null;
  currentAdminId?: number;
  permissions: readonly string[];
  project: ProjectListRow;
}): FairLaunchActionAccess {
  const { action, capabilities, currentAdminId, permissions, project } = input;
  if (getFundingContractPair(project) !== 'fair_launch_v1') {
    return { allowed: false, reason: 'identity_mismatch' };
  }
  if (
    !capabilities ||
    !capabilities.available ||
    String(capabilities.project_id) !== project.project_id ||
    capabilities.funding_mode !== 'fair_launch' ||
    capabilities.contract_version !== 'fair_launch_v1'
  ) {
    return { allowed: false, reason: 'backend_capability_unavailable' };
  }
  if (!capabilities.runtime_enabled) {
    return { allowed: false, reason: 'runtime_disabled' };
  }
  if (!capabilities.runtime?.runtime_verified) {
    return { allowed: false, reason: 'runtime_unverified' };
  }
  const localAuthority = (() => {
    try {
      const api = new URL(
        String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''),
      );
      return (
        !import.meta.env.PROD &&
        import.meta.env.VITE_ENVIRONMENT === 'local' &&
        api.protocol === 'http:' &&
        api.hostname === '127.0.0.1' &&
        /^\d+$/.test(api.port) &&
        /^[0-9a-f]{64}$/.test(
          String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN || ''),
        ) &&
        Date.now() <
          Date.parse(
            String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT || ''),
          )
      );
    } catch {
      return false;
    }
  })();
  if (localAuthority) {
    const descriptor = capabilities.actions[action];
    if (!descriptor?.enabled)
      return { allowed: false, descriptor, reason: 'action_disabled' };
    if (!permissions.includes('project.fair_launch.local_e2e'))
      return { allowed: false, descriptor, reason: 'missing_permission' };
    return { allowed: true, descriptor, reason: null };
  }
  // Runtime verification proves contract identity only. Until CE-V2 can
  // atomically materialize lifecycle/governance/ledger effects and reverse
  // them on reorg, no verified runtime may expose mutation controls. Keep the
  // action/permission/identity inputs in the signature so re-enabling requires
  // an explicit source change and renewed review rather than a UI bypass.
  void action;
  void currentAdminId;
  void permissions;
  return { allowed: false, reason: 'materialization_unavailable' };
}

export function fairLaunchActionReasonText(
  access: FairLaunchActionAccess,
): string {
  switch (access.reason) {
    case 'action_disabled': {
      return access.descriptor?.reason_code || 'Backend 当前不允许此动作';
    }
    case 'backend_capability_unavailable': {
      return 'Backend capability unavailable';
    }
    case 'identity_mismatch': {
      return 'funding_mode / contract_version 不匹配，已拒绝动作';
    }
    case 'materialization_unavailable': {
      return FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE;
    }
    case 'missing_permission': {
      return '缺少所需项目权限';
    }
    case 'runtime_disabled': {
      return 'Fair Launch runtime 未启用';
    }
    case 'runtime_unverified': {
      return '受信 runtime 身份尚未验证';
    }
    case 'step_up_mfa_required': {
      return '需要完成 step-up MFA 后重新获取 capability';
    }
    case 'three_party_separation': {
      return '当前管理员与发起人/审批人冲突，三权分离拒绝执行';
    }
    default: {
      return '';
    }
  }
}
