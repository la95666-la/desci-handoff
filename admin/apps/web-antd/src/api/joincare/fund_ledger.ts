import { requestClient } from '#/api/request';

export type FundLedgerEntryType =
  | 'payment_refunded'
  | 'platform_fee_paid'
  | 'project_proceeds_paid'
  | 'project_token_claimed'
  | 'project_token_deposited'
  | 'project_tokens_withdrawn'
  | 'purchase_payment';

export type FundLedgerStatus = 'canonical' | 'reconciliation' | 'reversed';

export interface FundLedgerRow {
  id: number;
  chain_id: number;
  contract_address: string;
  contract_version: string;
  project_id: number | string;
  wallet_address: string;
  counterparty_address: string;
  entry_type: FundLedgerEntryType;
  asset_role: 'payment' | 'project';
  token_symbol: string;
  token_address: string;
  token_decimals: number;
  amount_raw: string;
  amount_display: string;
  tx_hash: string;
  log_index: number;
  block_number: number;
  block_hash: string;
  status: FundLedgerStatus;
  created_at: string;
  updated_at: string;
  twitter_handle: string;
  twitter_name: string;
  twitter_avatar: string;
  telegram_username: string;
  telegram_name: string;
  telegram_avatar: string;
}

export interface FundLedgerAggregate {
  entry_type: FundLedgerEntryType;
  asset_role: 'payment' | 'project';
  token_symbol: string;
  token_address: string;
  token_decimals: number;
  count: number;
  amount_raw: string;
  amount_display: string;
}

export interface FundReconciliationCheck {
  code: string;
  ok: boolean;
  expected?: string;
  actual?: string;
  message: string;
}

export interface FundProjectSnapshot {
  project_id: number | string;
  name_en: string;
  status: string;
  chain_id: number;
  contract_address: string;
  contract_version: string;
  payment_token: string;
  payment_token_address: string;
  payment_token_decimals: number;
  project_token: string;
  project_token_address: string;
  project_token_decimals: number;
  token_price_raw: string;
  token_price_display: string;
  total_raised_raw: string;
  total_raised_display: string;
  total_deposited_raw: string;
  total_deposited_display: string;
  participant_count: number;
  fee_receiver: string;
  success_fee_rate: number;
  fund_withdrawn: boolean;
  platform_fee_withdrawn: boolean;
  token_withdrawn: boolean;
}

export interface FundReconciliation {
  scope: 'database_ledger_only';
  evidence: 'project_aggregate+fund_ledger+purchase_entitlement';
  balanced: boolean;
  project: FundProjectSnapshot;
  aggregates: FundLedgerAggregate[];
  purchase_entitlements: PurchaseEntitlementSummary;
  checks: FundReconciliationCheck[];
}

export interface PurchaseEntitlementSummary {
  count: number;
  payment_amount_raw: string;
  payment_amount_display: string;
  total_tokens_sold_raw: string;
  total_tokens_sold_display: string;
}

export interface PurchaseEntitlementRow {
  id: number;
  fund_ledger_id: number;
  chain_id: number;
  contract_address: string;
  contract_version: string;
  project_id: number | string;
  wallet_address: string;
  payment_token_symbol: string;
  payment_token_address: string;
  payment_token_decimals: number;
  payment_amount_raw: string;
  payment_amount_display: string;
  project_token_symbol: string;
  project_token_address: string;
  project_token_decimals: number;
  project_token_amount_raw: string;
  project_token_amount_display: string;
  tx_hash: string;
  log_index: number;
  block_number: number;
  block_hash: string;
  status: 'canonical';
  created_at: string;
  twitter_handle: string;
  twitter_name: string;
  twitter_avatar: string;
  telegram_username: string;
  telegram_name: string;
  telegram_avatar: string;
}

export interface OnchainReconciliationCheck {
  code: string;
  ok: boolean;
  expected?: string;
  actual?: string;
  message: string;
}

export interface OnchainWalletSnapshot {
  wallet: string;
  payment_amount_raw: string;
  tokens_bought_raw: string;
  claimed_tokens_raw: string;
  refunded: boolean;
}

export interface OnchainTokenReserveSnapshot {
  role: 'payment' | 'project';
  symbol: string;
  token_address: string;
  allowed: boolean;
  decimals: number;
  erc20_decimals?: number;
  governed_decimals?: number;
  project_decimals?: number;
  proxy_balance_raw: string;
  global_reserved_raw: string;
  project_liability_raw: string;
}

export type IDOReconciliationState =
  | 'cancel_requested'
  | 'cancelled'
  | 'completed'
  | 'failed'
  | 'finalizing'
  | 'invalidated'
  | 'queued'
  | 'retrying'
  | 'running';

export interface IDOReconciliationSummary {
  attempt_count: number;
  balanced: boolean | null;
  completed_at?: null | string;
  evidence_block_hash: string;
  evidence_block_number: number;
  expected_wallet_count: number;
  invalidated_by_reorg: boolean;
  last_error_code: string;
  last_error_message: string;
  manifest_digest?: string;
  outcome?: 'BALANCED' | 'UNBALANCED' | null;
  processed_wallet_count: number;
  project_id: number | string;
  reason: string;
  requested_at: string;
  requested_by_admin_id: number;
  run_id: number;
  run_uuid: string;
  source_digest: string;
  started_at?: null | string;
  state: IDOReconciliationState;
}

export interface IDOReconciliationDetail {
  database_balanced: boolean | null;
  database_checks: FundReconciliationCheck[];
  onchain: null | {
    balanced: boolean | null;
    block_hash: string;
    block_number: number;
    checks: OnchainReconciliationCheck[];
    latest_block: number;
    project: OnchainProjectSnapshot;
    scope: 'onchain_snapshot';
    token_reserves: OnchainTokenReserveSnapshot[];
  };
  summary: IDOReconciliationSummary;
}

export interface IDOReconciliationWallet {
  actual_claimed_raw: string;
  actual_payment_raw: string;
  actual_refunded: boolean;
  actual_tokens_raw: string;
  expected_claimed_raw: string;
  expected_payment_raw: string;
  expected_refunded: boolean;
  expected_tokens_raw: string;
  id: number;
  observed_at: string;
  ordinal: number;
  row_digest: string;
  run_id: number;
  wallet_address: string;
}

export interface OnchainProjectSnapshot {
  project_id: string;
  status: number;
  project_token: string;
  payment_token: string;
  project_owner: string;
  fee_receiver: string;
  project_token_decimals: number;
  payment_token_decimals: number;
  success_fee_rate: number;
  token_price_raw: string;
  hard_cap_raw: string;
  soft_cap_raw: string;
  max_per_user_raw: string;
  start_time: string;
  end_time: string;
  total_raised_raw: string;
  total_tokens_sold_raw: string;
  total_tokens_deposited_raw: string;
  funds_withdrawn: boolean;
  platform_fee_withdrawn: boolean;
  project_tokens_withdrawn: boolean;
  vesting_times: string[];
  vesting_percents: string[];
}

export interface ProjectOnchainReconciliation {
  balanced: boolean;
  database: {
    balanced: boolean;
    checks: FundReconciliationCheck[];
    scope: 'database_ledger_only';
  };
  onchain_snapshot: {
    balanced: boolean;
    block_hash: string;
    block_number: number;
    checks: OnchainReconciliationCheck[];
    latest_block: number;
    project: OnchainProjectSnapshot;
    scope: 'onchain_snapshot';
    token_reserves: OnchainTokenReserveSnapshot[];
    wallets: OnchainWalletSnapshot[];
  };
  scope: 'database_ledger_plus_onchain_snapshot';
}

export function listProjectFundLedger(
  projectId: number | string,
  params: {
    entry_type?: FundLedgerEntryType;
    page?: number;
    page_size?: number;
    status?: FundLedgerStatus;
  },
) {
  return requestClient.get<{
    items: FundLedgerRow[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/projects/${projectId}/fund-ledger`, { params });
}

export function getProjectFundReconciliation(projectId: number | string) {
  return requestClient.get<FundReconciliation>(
    `/v1/projects/${projectId}/fund-reconciliation`,
  );
}

export function listProjectPurchaseEntitlements(
  projectId: number | string,
  params: { page?: number; page_size?: number },
) {
  return requestClient.get<{
    items: PurchaseEntitlementRow[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/projects/${projectId}/purchase-entitlements`, { params });
}

export interface WalletPurchaseDetail {
  wallet_address: string;
  twitter_handle: string;
  twitter_name: string;
  twitter_avatar: string;
  telegram_username: string;
  telegram_name: string;
  telegram_avatar: string;
  payment_raw: string;
  payment_display: string;
  tokens_bought_raw: string;
  tokens_bought_display: string;
  entitlement_count: number;
  project_status: string;
  refund_expected_now: boolean;
  refunded_now: boolean;
  refunded_now_raw: string;
  refunded_now_display: string;
  ledger: FundLedgerRow[];
  onchain_run_id?: number;
  onchain?: null | {
    actual_claimed_display: string;
    actual_claimed_raw: string;
    actual_payment_display: string;
    actual_payment_raw: string;
    actual_refunded: boolean;
    actual_tokens_display: string;
    actual_tokens_raw: string;
    expected_claimed_display: string;
    expected_claimed_raw: string;
    expected_payment_display: string;
    expected_payment_raw: string;
    expected_refunded: boolean;
    expected_tokens_display: string;
    expected_tokens_raw: string;
    id?: number;
    observed_at?: string;
    ordinal?: number;
    run_id?: number;
    wallet_address?: string;
  };
}

export function getWalletPurchaseDetail(
  projectId: number | string,
  wallet: string,
) {
  return requestClient.get<WalletPurchaseDetail>(
    `/v1/projects/${projectId}/wallet-purchase`,
    { params: { wallet } },
  );
}

export function getProjectOnchainReconciliation(projectId: number | string) {
  return requestClient.get<ProjectOnchainReconciliation>(
    `/v1/projects/${projectId}/onchain-reconciliation`,
  );
}

export function createProjectIDOReconciliation(
  projectId: number | string,
  data: { idempotency_key: string; reason: string },
) {
  return requestClient.post<{
    created: boolean;
    run_id: number;
    run_uuid: string;
    state: IDOReconciliationState;
  }>(`/v1/projects/${projectId}/onchain-reconciliations`, data);
}

export function listProjectIDOReconciliations(
  projectId: number | string,
  params: { page?: number; page_size?: number },
) {
  return requestClient.get<{
    items: IDOReconciliationSummary[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/projects/${projectId}/onchain-reconciliations`, { params });
}

export function getIDOReconciliation(runId: number) {
  return requestClient.get<IDOReconciliationDetail>(
    `/v1/onchain-reconciliations/${runId}`,
  );
}

export function listIDOReconciliationWallets(
  runId: number,
  params: { page?: number; page_size?: number },
) {
  return requestClient.get<{
    items: IDOReconciliationWallet[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/onchain-reconciliations/${runId}/wallets`, { params });
}

export function cancelIDOReconciliation(runId: number, reason: string) {
  return requestClient.post<{
    run_id: number;
    state: IDOReconciliationState;
  }>(`/v1/onchain-reconciliations/${runId}/cancel`, { reason });
}
