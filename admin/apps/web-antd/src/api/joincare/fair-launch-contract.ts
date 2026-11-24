export const FAIR_LAUNCH_FUNDING_MODE = 'fair_launch' as const;
export const FAIR_LAUNCH_CONTRACT_VERSION = 'fair_launch_v1' as const;

export type FairLaunchLifecycleAction =
  | 'activate'
  | 'cancel'
  | 'create'
  | 'end'
  | 'release'
  | 'withdraw_failed_project_tokens'
  | 'withdraw_platform_fee'
  | 'withdraw_project_proceeds';

export type FairLaunchGovernanceAction =
  | 'configure_beneficiary_safe'
  | 'configure_project_token'
  | 'configure_release_policy'
  | 'invalidate_payment_token'
  | 'invalidate_project'
  | 'recover_excess_tokens'
  | 'register_payment_token_policy'
  | 'set_operator'
  | 'set_payment_token_admission'
  | 'set_payment_token_emergency_blocked'
  | 'set_platform_fee'
  | 'set_project_token_emergency_blocked'
  | 'upgrade_proxy';

export type FairLaunchPauseAction =
  | 'pause_claims'
  | 'pause_contributions'
  | 'pause_payouts'
  | 'unpause_claims'
  | 'unpause_contributions'
  | 'unpause_payouts';

export type FairLaunchAdminAction =
  | FairLaunchGovernanceAction
  | FairLaunchLifecycleAction
  | FairLaunchPauseAction;

export const fairLaunchApiPaths = {
  capability: (projectId: number | string) =>
    `/v1/projects/${projectId}/fair-launch/capabilities`,
  currentSet: (projectId: number | string) =>
    `/v1/projects/${projectId}/fair-launch/onchain-params`,
  lifecycle: (
    projectId: number | string,
    action: FairLaunchLifecycleAction,
  ) => {
    const suffix: Record<FairLaunchLifecycleAction, string> = {
      activate: 'activate-onchain',
      cancel: 'cancel',
      create: 'create-onchain',
      end: 'end',
      release: 'release',
      withdraw_failed_project_tokens: 'withdraw-failed-project-tokens',
      withdraw_platform_fee: 'withdraw-platform-fee',
      withdraw_project_proceeds: 'withdraw-project-proceeds',
    };
    return `/v1/projects/${projectId}/fair-launch/${suffix[action]}`;
  },
  governance: (
    projectId: number | string,
    action: FairLaunchGovernanceAction,
  ) =>
    `/v1/projects/${projectId}/fair-launch/governance/${action.replaceAll('_', '-')}`,
  pause: (projectId: number | string, action: FairLaunchPauseAction) =>
    `/v1/projects/${projectId}/fair-launch/pause/${action.replaceAll('_', '-')}`,
  executionQueue: (projectId: number | string) =>
    `/v1/projects/${projectId}/fair-launch/actions/execution-queue`,
  executeAction: (projectId: number | string, requestId: number) =>
    `/v1/projects/${projectId}/fair-launch/actions/${requestId}/execute`,
} as const;

export function fairLaunchHttpStatus(error: unknown): number | undefined {
  const candidate = error as { response?: { status?: number } };
  return candidate?.response?.status;
}

export function isFairLaunchCapabilityUnavailable(error: unknown): boolean {
  const status = fairLaunchHttpStatus(error);
  return status === 404 || status === 405 || status === 501;
}
