import { requestClient } from '#/api/request';

export type ChainTransactionAction =
  | 'fair_launch_activate'
  | 'fair_launch_cancel'
  | 'fair_launch_create'
  | 'fair_launch_end'
  | 'fair_launch_release'
  | 'fair_launch_withdraw_failed_project_tokens'
  | 'fair_launch_withdraw_platform_fee'
  | 'fair_launch_withdraw_project_proceeds'
  | 'ido_v3_activate'
  | 'ido_v3_create'
  | 'ido_v3_end'
  | 'ido_v3_withdraw_platform_fee'
  | 'project_end'
  | 'v2_activate'
  | 'v2_create';

export type ChainTransactionState =
  | 'admin_cancelled'
  | 'ambiguous'
  | 'applied'
  | 'broadcast_unknown'
  | 'cancelled'
  | 'final_revert'
  | 'final_success'
  | 'intent'
  | 'mined_revert'
  | 'mined_success'
  | 'ready'
  | 'signed'
  | 'submitted';

export interface ChainTransaction {
  id: number;
  project_id: number;
  action: ChainTransactionAction;
  attempt_no: number;
  chain_id: number;
  sender_address: string;
  contract_address: string;
  value_raw: string;
  state: ChainTransactionState;
  nonce?: number;
  gas_limit?: number;
  gas_price_raw: string;
  tx_hash?: string;
  broadcast_count: number;
  last_broadcast_at?: string;
  last_error: string;
  receipt_status?: number;
  receipt_block_number?: number;
  receipt_block_hash: string;
  receipt_gas_used?: number;
  final_event_record_id?: number;
  finalized_at?: string;
  cancelled_by_admin_id?: number;
  cancel_reason?: string;
  cancelled_at?: string;
  admin_cancellable: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChainTransactionListResponse {
  items: ChainTransaction[];
  page: number;
  page_size: number;
  total: number;
}

export interface ChainTransactionSummary {
  stale_active: number;
  stale_after_seconds: number;
  state_counts: Array<{ count: number; state: ChainTransactionState }>;
}

export function listChainTransactions(params: {
  action?: ChainTransactionAction;
  page?: number;
  page_size?: number;
  project_id?: number;
  state?: ChainTransactionState;
}) {
  return requestClient.get<ChainTransactionListResponse>(
    '/v1/chain-transactions',
    { params },
  );
}

export function getChainTransaction(id: number) {
  return requestClient.get<ChainTransaction>(`/v1/chain-transactions/${id}`);
}

export function getChainTransactionSummary() {
  return requestClient.get<ChainTransactionSummary>(
    '/v1/chain-transactions/summary',
  );
}

export function cancelChainTransaction(id: number, reason: string) {
  return requestClient.post<{
    id: number;
    reason: string;
    state: 'admin_cancelled';
  }>(`/v1/chain-transactions/${id}/cancel`, { reason });
}

export function continueV3Original(
  id: number,
  txHash: string,
  reason: string,
  execute: boolean,
) {
  return requestClient.post<{ submitted: boolean; tx_hash: string; }>(
    `/v1/chain-transactions/${id}/v3-original/${execute ? 'broadcast' : 'preflight'}`,
    { tx_hash: txHash, reason },
    { timeout: 165_000 },
  );
}
