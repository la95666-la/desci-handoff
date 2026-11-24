import type { FairLaunchAdminAction } from './fair-launch-contract';

import { requestClient } from '#/api/request';

import {
  FAIR_LAUNCH_CONTRACT_VERSION,
  FAIR_LAUNCH_FUNDING_MODE,
  fairLaunchApiPaths,
} from './fair-launch-contract';

export {
  FAIR_LAUNCH_CONTRACT_VERSION,
  FAIR_LAUNCH_FUNDING_MODE,
  type FairLaunchAdminAction,
  fairLaunchApiPaths,
  type FairLaunchGovernanceAction,
  fairLaunchHttpStatus,
  type FairLaunchLifecycleAction,
  type FairLaunchPauseAction,
  isFairLaunchCapabilityUnavailable,
} from './fair-launch-contract';

/**
 * Fair Launch Admin API seam.
 *
 * Backend integration is intentionally isolated in this module. Every route is
 * below the trusted project identity; callers never provide a contract target,
 * signer, nonce, or raw transaction. A missing/unsupported capability must be
 * treated as disabled by the UI.
 */

export type FairLaunchExecutionDomain =
  | 'external_safe_observation'
  | 'governance_timelock'
  | 'operator_durable_outbox'
  | 'pause_guardian_safe'
  | 'permissionless_observation'
  | 'unpause_timelock';

export interface FairLaunchActionDescriptor {
  action: FairLaunchAdminAction;
  approval_operation: string;
  execute_route?: string;
  enabled: boolean;
  execution_domain: FairLaunchExecutionDomain;
  reason_code?: string;
  requires_four_eyes: boolean;
  requires_preview: boolean;
  requires_step_up_mfa: boolean;
  requires_three_party_separation: boolean;
  step_up_mfa_verified?: boolean;
  /** Backend-provided identities which may not execute this approved action. */
  forbidden_admin_ids?: number[];
}

export interface FairLaunchRuntimeIdentity {
  chain_id: number;
  contract_version: string | typeof FAIR_LAUNCH_CONTRACT_VERSION;
  proxy_address?: string;
  implementation_address?: string;
  implementation_code_hash?: string;
  runtime_verified: boolean;
  source_registry_revision?: number;
  source_registry_digest?: string;
  observed_at?: string;
  observed_block_hash?: string;
  /** Decimal uint64 string; never coerce this field to a JS number. */
  observed_block_number?: string;
}

export interface FairLaunchPauseState {
  claims_pause_until?: string;
  claims_paused: boolean;
  contributions_paused: boolean;
  payouts_paused: boolean;
}

export interface FairLaunchFinancialState {
  deposited_sale_tokens?: string;
  deposited_sale_tokens_raw?: string;
  final_total_raised?: string;
  final_total_raised_raw?: string;
  platform_fee?: string;
  platform_fee_raw?: string;
  platform_fee_withdrawn: boolean;
  project_proceeds?: string;
  project_proceeds_raw?: string;
  project_proceeds_withdrawn: boolean;
  reconciliation_state?: string;
  required_deposit?: string;
  required_deposit_raw?: string;
  sale_token_amount?: string;
  sale_token_amount_raw?: string;
  soft_cap?: string;
  soft_cap_raw?: string;
}

export interface FairLaunchCapabilityEnvelope {
  actions: Partial<Record<FairLaunchAdminAction, FairLaunchActionDescriptor>>;
  available: boolean;
  contract_version: string | typeof FAIR_LAUNCH_CONTRACT_VERSION;
  financial_state?: FairLaunchFinancialState;
  funding_mode: string | typeof FAIR_LAUNCH_FUNDING_MODE;
  observed_at?: string;
  pause_state?: FairLaunchPauseState;
  project_id: number | string;
  reason_code?: string;
  runtime?: FairLaunchRuntimeIdentity;
  runtime_enabled: boolean;
}

export interface FairLaunchVestingRow {
  bps: number;
  unlock_time: string;
}

/**
 * Discriminated Set payload. Deliberately has no token_price or hard_cap.
 * Chain/contract/fee/policy identities are trusted backend snapshots.
 */
export interface FairLaunchSetRequest {
  contract_version: typeof FAIR_LAUNCH_CONTRACT_VERSION;
  end_time: string;
  funding_mode: typeof FAIR_LAUNCH_FUNDING_MODE;
  max_contribution: string;
  min_contribution: string;
  payment_token: string;
  project_owner: string;
  project_proceeds_receiver: string;
  project_token: string;
  project_token_symbol?: string;
  sale_token_amount: string;
  soft_cap: string;
  start_time: string;
  vesting: FairLaunchVestingRow[];
}

export type FairLaunchSetCurrent = Omit<
  Partial<FairLaunchSetRequest>,
  'contract_version' | 'funding_mode'
> & {
  configured: boolean;
  contract_version: string | typeof FAIR_LAUNCH_CONTRACT_VERSION;
  funding_mode: string | typeof FAIR_LAUNCH_FUNDING_MODE;
  locked: boolean;
  status: string;
};

export interface FairLaunchActionEvidence {
  canonical_digest?: string;
  evidence_hash?: string;
  governance_class?: string;
  notes?: string;
  operation_id?: string;
  reason_code?: string;
  requested_until?: string;
  settlement_block?: string;
  anchor_block?: string;
  anchor_hash?: string;
  manifest_digest?: string;
  manifest_version?: number;
  policy_digest?: string;
}

export interface FairLaunchActionPreview {
  action: FairLaunchAdminAction;
  approval_operation: string;
  canonical_digest: string;
  preview_id: number;
  preview_token: string;
  execution_domain: FairLaunchExecutionDomain;
  expires_at: string;
  immutable_payload: Record<string, unknown>;
  preconditions: Array<{ code: string; message: string; ok: boolean }>;
  requires_four_eyes: boolean;
  requires_step_up_mfa: boolean;
  requires_three_party_separation: boolean;
  runtime: FairLaunchRuntimeIdentity;
}

export interface FairLaunchApprovalSubmission {
  action?: FairLaunchAdminAction;
  action_request_id?: number;
  approval_id: number;
  expires_at?: string;
  status: string;
  submission_id?: number;
  submitted: boolean;
}

function localFairLaunchAuthority() {
  try {
    if (import.meta.env.PROD || import.meta.env.VITE_ENVIRONMENT !== 'local')
      return null;
    const baseUrl = new URL(
      String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''),
    );
    const runId = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_RUN_ID || '',
    );
    const token = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN || '',
    );
    const expiresAt = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT || '',
    );
    if (
      baseUrl.protocol !== 'http:' ||
      baseUrl.hostname !== '127.0.0.1' ||
      !/^\d+$/.test(baseUrl.port) ||
      baseUrl.pathname !== '/' ||
      !/^[0-9a-f-]{36}$/.test(runId) ||
      !/^[0-9a-f]{64}$/.test(token) ||
      Date.now() >= Date.parse(expiresAt)
    )
      return null;
    return { baseUrl: baseUrl.origin, runId, token };
  } catch {
    return null;
  }
}

async function localFairLaunchGet<T>(path: string): Promise<T> {
  const authority = localFairLaunchAuthority();
  if (!authority) throw new Error('materialization_unavailable');
  const response = await fetch(new URL(`/admin${path}`, authority.baseUrl), {
    headers: {
      'X-JC-FL-Run-ID': authority.runId,
      'X-JC-FL-Capability': authority.token,
      'X-JC-FL-Origin': String(
        import.meta.env.VITE_FAIR_LAUNCH_LOCAL_ADMIN_ORIGIN || '',
      ),
    },
  });
  const result = await response.json();
  if (!response.ok || result?.status !== true)
    throw new Error(result?.message || 'materialization_unavailable');
  return result.data as T;
}

export function getFairLaunchCapabilities(projectId: number | string) {
  if (localFairLaunchAuthority())
    return localFairLaunchGet<FairLaunchCapabilityEnvelope>(
      fairLaunchApiPaths.capability(projectId),
    );
  return requestClient.get<FairLaunchCapabilityEnvelope>(
    fairLaunchApiPaths.capability(projectId),
  );
}

export function getFairLaunchSet(projectId: number | string) {
  return requestClient.get<FairLaunchSetCurrent>(
    fairLaunchApiPaths.currentSet(projectId),
  );
}

function actionPath(
  projectId: number | string,
  action: FairLaunchAdminAction,
): string {
  if (
    action === 'pause_claims' ||
    action === 'pause_contributions' ||
    action === 'pause_payouts' ||
    action === 'unpause_claims' ||
    action === 'unpause_contributions' ||
    action === 'unpause_payouts'
  ) {
    return fairLaunchApiPaths.pause(projectId, action);
  }
  if (
    action === 'activate' ||
    action === 'cancel' ||
    action === 'create' ||
    action === 'end' ||
    action === 'release' ||
    action === 'withdraw_failed_project_tokens' ||
    action === 'withdraw_platform_fee' ||
    action === 'withdraw_project_proceeds'
  ) {
    return fairLaunchApiPaths.lifecycle(projectId, action);
  }
  return fairLaunchApiPaths.governance(projectId, action);
}

export function previewFairLaunchAction(
  projectId: number | string,
  action: FairLaunchAdminAction,
  evidence: FairLaunchActionEvidence = {},
) {
  return requestClient.post<FairLaunchActionPreview>(
    `${actionPath(projectId, action)}/preview`,
    { evidence },
  );
}

export interface FairLaunchActionMutationRequest {
  canonical_digest?: string;
  preview_token?: string;
  evidence?: FairLaunchActionEvidence;
  mfa_assertion_token: string;
}

export function submitFairLaunchAction(
  projectId: number | string,
  action: FairLaunchAdminAction,
  payload: FairLaunchActionMutationRequest & { canonical_digest: string },
) {
  return requestClient.post<FairLaunchApprovalSubmission>(
    actionPath(projectId, action),
    payload,
  );
}

export interface FairLaunchExecuteActionRequest {
  mfa_assertion_token: string;
}

export type FairLaunchActionStatus =
  | 'applied'
  | 'approval_bound'
  | 'approved'
  | 'cancelled'
  | 'dispatched'
  | 'executing'
  | 'execution_bound'
  | 'expired'
  | 'failed'
  | 'observing'
  | 'pending_approval';

export interface FairLaunchActionTransition {
  sequence_no: number;
  from_status: FairLaunchActionStatus;
  to_status: FairLaunchActionStatus;
  actor_admin_id: number;
  mfa_evidence_ref: string;
  evidence_digest: string;
  created_at: string;
}

export interface FairLaunchExecutionQueueItem {
  action: FairLaunchAdminAction;
  approver_admin_id: number;
  config_generation: number;
  transitions: FairLaunchActionTransition[];
  execution_domain: FairLaunchExecutionDomain;
  execution_expires_at: string;
  executor_admin_id?: number;
  forbidden_admin_ids: number[];
  id: number;
  initiator_admin_id: number;
  policy_generation: number;
  project_id: number | string;
  runtime_generation: number;
  source_registry_generation: number;
  status: FairLaunchActionStatus;
  step_up_mfa_fresh_until?: string;
  step_up_mfa_verified: boolean;
}

export function getFairLaunchExecutionQueue(projectId: number | string) {
  return requestClient.get<{ items: FairLaunchExecutionQueueItem[] }>(
    fairLaunchApiPaths.executionQueue(projectId),
  );
}

export interface FairLaunchExecuteActionResponse {
  action_request_id: number;
  executed: boolean;
  external_observation_ref?: string;
  safe_tx_id?: number;
  status: string;
  submission_id?: number;
  timelock_operation_id?: number;
}

export function executeFairLaunchAction(
  projectId: number | string,
  requestId: number,
  payload: FairLaunchExecuteActionRequest,
) {
  return requestClient.post<FairLaunchExecuteActionResponse>(
    fairLaunchApiPaths.executeAction(projectId, requestId),
    payload,
  );
}
