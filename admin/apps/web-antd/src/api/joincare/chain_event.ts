import { requestClient } from '#/api/request';

export type ChainEventFailureStatus = 'dead_letter' | 'resolved' | 'retrying';

export interface ChainEventSyncState {
  id: number;
  initialized: boolean;
  cursor_block: number;
  cursor_block_hash: string;
  v2_registered: boolean;
  v2_contract_address: string;
  fair_launch_registered?: boolean;
  fair_launch_contract_address?: string;
  fair_launch_start_block?: number;
  fair_launch_implementation_address?: string;
  fair_launch_implementation_code_hash?: string;
  v2_start_block: number;
  v2_implementation_address: string;
  v2_implementation_code_hash: string;
  latest_block: number;
  safe_block: number;
  lag_blocks: number;
  last_started_at?: string;
  last_success_at?: string;
  last_failure_at?: string;
  last_error_code: string;
  created_at: string;
  updated_at: string;
}

export interface ChainEventHealth {
  active_reorg_generation?: ChainEventReorgGeneration;
  active_reorg_assessment?: ChainEventReorgAssessment;
  active_reorg_incident?: ChainEventReorgIncident;
  active_reorg_incidents: number;
  active_reorg_materialization?: ChainEventReorgMaterialization;
  active_reorg_prerequisite?: ChainEventReorgPrerequisite;
  active_reorg_recovery?: ChainEventReorgRecovery;
  blocked_chain_transactions: number;
  sync: ChainEventSyncState;
  failure_counts: Record<ChainEventFailureStatus, number>;
  healthy: boolean;
  lagging: boolean;
  max_healthy_lag_blocks: number;
  stale: boolean;
  stale_after_seconds: number;
}

export interface ChainEventReorgGenerationRequirement {
  created_at: string;
  evidence_status: 'unmet' | 'verified';
  id: number;
  ordinal: number;
  requirement_code: string;
  row_digest: string;
  run_id: number;
  safe_detail: string;
}

export interface ChainEventReorgGeneration {
  algorithm_version: string;
  assessment_manifest_digest: string;
  assessment_run_id: number;
  attempt_no: number;
  boundary_block: number;
  boundary_block_hash: string;
  candidate_ready: false;
  chain_id: number;
  completed_at?: string;
  created_at: string;
  descriptor_digest: string;
  generation_kind: 'candidate';
  generation_uuid: string;
  id: number;
  incident_id: number;
  last_error_code: string;
  legacy_contract_address: string;
  manifest_digest?: string;
  materialization_manifest_digest: string;
  materialization_run_id: number;
  nonce_count: number;
  nonce_root: string;
  old_effect_root: string;
  outcome?: 'BLOCKED';
  prerequisite_manifest_digest: string;
  prerequisite_run_id: number;
  replacement_effect_root: string;
  requirement_count: number;
  requirement_root: string;
  requirements?: ChainEventReorgGenerationRequirement[];
  schema_version: string;
  sender_count: number;
  sender_root: string;
  stage: 'done' | 'finalize' | 'requirements' | 'snapshot';
  state: 'completed' | 'invalidated' | 'preparing';
  submission_count: number;
  submission_payload_bytes: number;
  submission_root: string;
  target_safe_block: number;
  target_safe_block_hash: string;
  unmet_count: number;
  updated_at: string;
  v2_contract_address: string;
  verified_count: number;
  worker_attempt_count: number;
}

export interface ChainEventReorgPrerequisiteRequirement {
  artifact_id?: number;
  created_at: string;
  evidence_status: 'deferred' | 'unmet';
  id: number;
  ordinal: number;
  requirement_code: string;
  row_digest: string;
  run_id: number;
  safe_detail: string;
}

export interface ChainEventReorgPrerequisite {
  algorithm_version: string;
  artifact_count: 0;
  artifact_digest: string;
  assessment_manifest_digest: string;
  assessment_run_id: number;
  boundary_block: number;
  boundary_block_hash: string;
  candidate_ready: false;
  chain_id: number;
  completed_at?: string;
  created_at: string;
  deferred_count: number;
  event_digest?: string;
  expected_requirement_count: 7;
  id: number;
  incident_id: number;
  last_error_code: string;
  manifest_digest?: string;
  next_requirement_ordinal: number;
  outcome?: 'BLOCKED';
  requirement_count: number;
  requirement_digest: string;
  requirements?: ChainEventReorgPrerequisiteRequirement[];
  run_uuid: string;
  satisfied_count: 0;
  state: 'completed' | 'invalidated' | 'preparing';
  unmet_count: number;
  untrusted_count: number;
  updated_at: string;
  worker_attempt_count: number;
}

export interface ChainEventReorgAssessmentBlocker {
  id: number;
  run_id: number;
  ordinal: number;
  disposition_id?: number;
  scope: 'effect' | 'run';
  blocker_code: string;
  safe_detail: string;
  row_digest: string;
  created_at: string;
}

export interface ChainEventReorgAssessment {
  id: number;
  run_uuid: string;
  incident_id: number;
  materialization_run_id: number;
  materialization_manifest_digest: string;
  algorithm_version: string;
  chain_id: number;
  boundary_block: number;
  boundary_block_hash: string;
  target_safe_block: number;
  target_safe_block_hash: string;
  expected_effect_count: number;
  state: 'completed' | 'invalidated' | 'preparing';
  next_effect_ordinal: number;
  disposition_count: number;
  exact_count: number;
  conditional_count: number;
  blocked_count: number;
  noop_count: number;
  blocker_count: number;
  disposition_digest: string;
  blocker_digest: string;
  worker_attempt_count: number;
  last_error_code: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  outcome?: 'BLOCKED';
  manifest_digest?: string;
  switch_authorized: false;
  blockers?: ChainEventReorgAssessmentBlocker[];
}

export interface ChainEventReorgMaterialization {
  id: number;
  run_uuid: string;
  recovery_run_id: number;
  incident_id: number;
  algorithm_version: string;
  recovery_manifest_digest: string;
  chain_id: number;
  contract_address: string;
  legacy_contract_address: string;
  boundary_block: number;
  boundary_block_hash: string;
  old_cursor_block: number;
  old_cursor_block_hash: string;
  target_safe_block: number;
  target_safe_block_hash: string;
  expected_old_record_count: number;
  expected_replacement_log_count: number;
  created_at: string;
  state: 'completed' | 'invalidated' | 'preparing';
  next_branch: 'done' | 'old_committed' | 'replacement';
  old_effect_count: number;
  replacement_effect_count: number;
  ignored_log_count: number;
  old_effect_digest: string;
  replacement_effect_digest: string;
  worker_attempt_count: number;
  last_error_code: string;
  completed_at?: string;
  updated_at: string;
  manifest_digest?: string;
}

export interface ChainEventReorgRecovery {
  id: number;
  run_uuid: string;
  incident_id: number;
  attempt_no: number;
  chain_id: number;
  contract_address: string;
  legacy_contract_address: string;
  contract_version: string;
  algorithm_version: string;
  boundary_kind: 'last_canonical_checkpoint';
  boundary_checkpoint_id: number;
  boundary_block: number;
  boundary_block_hash: string;
  target_safe_block: number;
  target_safe_block_hash: string;
  confirmation_depth: number;
  expected_batch_count: number;
  expected_scanned_blocks: number;
  requested_at: string;
  state: 'completed' | 'invalidated' | 'scanning';
  next_block: number;
  next_batch_ordinal: number;
  next_log_ordinal: number;
  observed_log_count: number;
  batch_evidence_digest: string;
  log_evidence_digest: string;
  worker_attempt_count: number;
  last_error_code: string;
  completed_at?: string;
  updated_at: string;
  manifest_digest?: string;
  completed_batch_count: number;
}

export interface ChainEventReorgIncident {
  id: number;
  incident_key: string;
  chain_id: number;
  contract_address: string;
  contract_version: string;
  cursor_block: number;
  expected_block_hash: string;
  observed_block_hash: string;
  cursor_checkpoint_id: number;
  v2_start_block: number;
  status: 'detected';
  detection_code: 'CURSOR_BLOCK_HASH_MISMATCH';
  detected_at: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ChainEventFailure {
  id: number;
  event: string;
  block_number: number;
  block_hash: string;
  tx_hash: string;
  log_index: number;
  contract_address: string;
  contract_version: string;
  error_message: string;
  status: ChainEventFailureStatus;
  retry_count: number;
  consecutive_count: number;
  replay_count: number;
  first_failed_at: string;
  last_failed_at: string;
  replay_requested_at?: string;
  replay_audits?: ChainEventReplayAudit[];
  admin_replayable: boolean;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ChainEventReplayAudit {
  id: number;
  failure_id: number;
  replay_no: number;
  admin_id: number;
  reason: string;
  requested_at: string;
}

export interface ChainEventFailureListResponse {
  items: ChainEventFailure[];
  page: number;
  page_size: number;
  total: number;
}

export function getChainEventHealth() {
  return requestClient.get<ChainEventHealth>('/v1/chain-events/health');
}

export function listChainEventFailures(params: {
  page?: number;
  page_size?: number;
  status?: ChainEventFailureStatus;
}) {
  return requestClient.get<ChainEventFailureListResponse>(
    '/v1/chain-events/failures',
    { params },
  );
}

export function getChainEventFailure(id: number) {
  return requestClient.get<ChainEventFailure>(
    `/v1/chain-events/failures/${id}`,
  );
}

export function replayChainEventFailure(id: number, reason: string) {
  return requestClient.post<{
    id: number;
    reason: string;
    status: 'retrying';
  }>(`/v1/chain-events/failures/${id}/replay`, { reason });
}
