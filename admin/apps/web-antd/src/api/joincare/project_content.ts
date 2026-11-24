import { requestClient } from '#/api/request';

export interface V3SenderReadiness {
  ready: boolean;
  signing_authority?: boolean;
  minimum_evidence_runway_seconds?: number;
  evidence_remaining_seconds?: number;
  active_submission_id?: string;
  reason_code?: string;
  stage?: string;
  next_action?: string;
}

export function getV3SenderReadiness() {
  return requestClient.get<V3SenderReadiness>('/v1/ido-v3/sender-readiness');
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

async function localFairLaunchProjectContent(
  projectId: number | string,
): Promise<ProjectContent> {
  const authority = localFairLaunchAuthority();
  if (!authority) throw new Error('materialization_unavailable');
  const response = await fetch(
    new URL(`/admin/v1/projects/${projectId}/content`, authority.baseUrl),
    {
      headers: {
        'X-JC-FL-Run-ID': authority.runId,
        'X-JC-FL-Capability': authority.token,
        'X-JC-FL-Origin': String(
          import.meta.env.VITE_FAIR_LAUNCH_LOCAL_ADMIN_ORIGIN || '',
        ),
      },
    },
  );
  const result = await response.json();
  if (!response.ok || result?.status !== true)
    throw new Error(result?.message || 'materialization_unavailable');
  return result.data as ProjectContent;
}

// —— 类型 ———————————————————————————————————————————————

/** 后端 GET /admin/v1/projects/:id/content 返回的 project 结构（entity.Project） */
export interface ProjectContent {
  id: number;
  project_id: string;
  owner_address: string;
  name_en: string;
  name_cn: string;
  tagline: string;
  description: string;
  tags: string;
  status: string;
  funding_mode?: ProjectFundingMode | string;
  contract_version: string;
  onchain_locked?: boolean;
  /** Off-chain official-page minimum per purchase; empty = disabled. */
  min_purchase_amount: string;
  min_purchase_draft_amount: string;
  min_purchase_draft_token: string;

  // Sprint A 扩展的展示字段
  category: string;
  cover_url: string;
  cover_video_url: string;
  launch_at: null | string; // RFC3339 or null
  story_html: string;
  research_html: string;
  risks_html: string;

  // 双语并列（繁體中文）；C 端根据 language pick，空则 fallback 到英语版
  tagline_zh: string;
  story_html_zh: string;
  research_html_zh: string;
  risks_html_zh: string;

  // JSON 存储字符串；前端读取时 JSON.parse 后使用
  team_json: string;
  tokenomics_json: string;
  roadmap_json: string;
  documents_json: string;
  faq_json: string;
  onchain_json: string;

  created_at: string;
  updated_at: string;

  // 现有已存在但未展示的老字段（前端可以按需回显）
  [key: string]: any;
}

/** 结构化子结构（team / tokenomics / roadmap / documents / faq / onchain） */
export interface TeamMember {
  name: string;
  name_zh?: string;
  role: string;
  role_zh?: string;
  avatar_url?: string;
  bio?: string;
  bio_zh?: string;
  links?: { label: string; url: string }[];
}

export interface TokenomicsAllocation {
  label: string;
  label_zh?: string;
  percent: number; // 0-100
  color?: string;
}

export interface TokenomicsData {
  allocations: TokenomicsAllocation[];
  tge_percent?: number;
  cliff_months?: number;
  linear_months?: number;
}

export interface RoadmapItem {
  quarter: string; // "2026 Q3"
  quarter_zh?: string;
  title: string;
  title_zh?: string;
  desc?: string;
  desc_zh?: string;
  status: 'active' | 'done' | 'planned';
}

export interface DocumentItem {
  title: string;
  title_zh?: string;
  url: string;
  kind?: string; // 'whitepaper' | 'audit' | 'paper' | 'other'
  kind_zh?: string;
}

export interface FaqItem {
  q: string;
  q_zh?: string;
  a: string;
  a_zh?: string;
}

export interface OnchainItem {
  label: string;
  label_zh?: string;
  address: string;
  chain?: string;
}

/** PUT payload —— pointer 语义：不传 = 不改；传空字符串 = 清空 */
export interface ProjectContentPatch {
  name_en?: string;
  name_cn?: string;
  category?: string;
  cover_url?: string;
  cover_video_url?: string;
  launch_at?: null | string;
  tagline?: string;
  description?: string;
  tags?: string[];
  story_html?: string;
  research_html?: string;
  risks_html?: string;
  // 双语并列（繁體中文）；不传 = 不改；传空字符串 = 清空
  tagline_zh?: string;
  story_html_zh?: string;
  research_html_zh?: string;
  risks_html_zh?: string;
  // JSON 字段前端传对象/数组，backend 会 JSON.stringify
  team_json?: TeamMember[];
  tokenomics_json?: TokenomicsData;
  roadmap_json?: RoadmapItem[];
  documents_json?: DocumentItem[];
  faq_json?: FaqItem[];
}

// —— Updates ———————————————————————————————————————————————

export interface ProjectUpdateRow {
  id: number;
  project_id: number | string;
  author_admin_id: number;
  title: string;
  content_html: string;
  is_published: number; // 0 / 1
  published_at: null | string;
  created_at: string;
  updated_at: string;
}

export interface UpdateUpsertPayload {
  title: string;
  content_html: string;
  publish?: boolean;
}

// —— Upload ———————————————————————————————————————————————

export interface UploadedFile {
  id: number;
  url: string;
  size: number;
  mime: string;
  name: string;
}

// —— 列表页行 (轻量，不带 story_html / research_html 等大字段) ——————————
export type ProjectFundingMode = 'capped_presale' | 'fair_launch';

export interface ProjectListRow {
  tracking?: ProjectTracking;
  id: number;
  project_id: string;
  name_en: string;
  name_cn: string;
  tagline: string;
  category: string;
  status: string;
  pre_archive_status?: null | string;
  restore_allowed?: boolean | number;
  restore_blocker?: null | string;
  funding_mode?: ProjectFundingMode | string;
  contract_version: string;
  /** Backend display-formatted decimal string. */
  sale_token_amount?: null | string;
  sale_token_amount_raw?: null | string;
  /** Backend display-formatted decimal string. */
  soft_cap?: null | string;
  soft_cap_raw?: null | string;
  onchain_locked: boolean;
  /** Whether the project appears in public Home and Launchpad lists. */
  is_listed?: boolean;
  active_submission?: null | {
    action:
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
      | 'v2_activate'
      | 'v2_cancel'
      | 'v2_create'
      | 'v2_end'
      | 'v2_withdraw_platform_fee';
    id: number;
    state: string;
    tx_hash?: string;
  };
  launch_at: null | string;
  end_time?: null | string;
  platform_fee_withdrawn?: number;
  cover_url: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectTracking {
  mode: 'archived' | 'maintenance' | 'normal';
  finished: boolean;
  finished_at: null | string;
  pending_reason: string;
  next_scan_at: null | string;
  last_scan_at: null | string;
  wake_block: number;
  revision: number;
  evidence_current: boolean;
}
export function changeProjectTracking(
  projectId: string,
  action: string,
  reason: string,
  revision: number,
) {
  return requestClient.post(`/v1/projects/${projectId}/tracking`, {
    action,
    reason,
    revision,
  });
}

export interface ProjectOnchainSubmission {
  project_id: number | string;
  status: string;
  submitted: boolean;
  tx_hash: string;
}

// —— API 函数 ———————————————————————————————————————————————

export function listProjects(
  params: {
    keyword?: string;
    page?: number;
    size?: number;
    status?: string;
  } = {},
) {
  return requestClient.get<{
    items: ProjectListRow[];
    page: number;
    page_size: number;
    total: number;
  }>('/v1/projects', { params });
}

export function getProjectContent(projectId: number | string) {
  if (localFairLaunchAuthority())
    return localFairLaunchProjectContent(projectId);
  return requestClient.get<ProjectContent>(`/v1/projects/${projectId}/content`);
}

export function updateProjectListingVisibility(
  projectId: number | string,
  isListed: boolean,
) {
  return requestClient.put<{ is_listed: boolean; project_id: string }>(
    `/v1/projects/${projectId}/listing-visibility`,
    { is_listed: isListed },
  );
}

export function updateProjectContent(
  projectId: number | string,
  patch: ProjectContentPatch,
) {
  return requestClient.put<{ changed: number; id: number }>(
    `/v1/projects/${projectId}/content`,
    patch,
  );
}

export function updateProjectMinPurchase(
  projectId: number | string,
  amount: string,
  paymentToken = '',
) {
  return requestClient.put(`/v1/projects/${projectId}/min-purchase`, {
    min_purchase_amount: amount,
    payment_token: paymentToken,
  });
}

export function disableProject(projectId: number | string) {
  return requestClient.post<{ id: number; status: string }>(
    `/v1/projects/${projectId}/disable`,
  );
}

export function deleteProject(projectId: number | string) {
  return requestClient.delete<{ id: number }>(`/v1/projects/${projectId}`);
}

export function restoreProject(projectId: number | string) {
  return requestClient.post<{ id: number; restored: boolean }>(
    `/v1/projects/${projectId}/restore`,
  );
}

export function createProjectOnchain(projectId: number | string) {
  return requestClient.post<ProjectOnchainSubmission>(
    `/v1/projects/${projectId}/create-onchain`,
    undefined,
    // Paced V3 reads/simulations: 150s Core < 160s client < 165s proxy.
    { timeout: 160_000 },
  );
}

export function activateProjectOnchain(projectId: number | string) {
  return requestClient.post<ProjectOnchainSubmission>(
    `/v1/projects/${projectId}/activate-onchain`,
    undefined,
    // V3 activation may audit an original creation and sign once. Keep the
    // client below the dedicated 165s Admin proxy and 180s Core server cap.
    { timeout: 160_000 },
  );
}

export function endProjectOnchain(projectId: number | string) {
  return requestClient.post<ProjectOnchainSubmission>(
    `/v1/projects/${projectId}/end`,
    undefined,
    { timeout: 160_000 },
  );
}

export function cancelProjectOnchain(projectId: number | string) {
  return requestClient.post<ProjectOnchainSubmission>(
    `/v1/projects/${projectId}/cancel`,
  );
}

export function withdrawPlatformFee(projectId: number | string) {
  return requestClient.post<ProjectOnchainSubmission>(
    `/v1/projects/${projectId}/withdraw-platform-fee`,
    undefined,
    { timeout: 160_000 },
  );
}

export function pauseIdoV2() {
  return requestClient.post<{ paused: boolean; tx_hash: string }>(
    '/v1/ido/pause',
  );
}

export function unpauseIdoV2() {
  return requestClient.post<{ paused: boolean; tx_hash: string }>(
    '/v1/ido/unpause',
  );
}

export function getIdoPauseState() {
  return requestClient.get<{ paused: boolean }>('/v1/ido/pause-state');
}

export function listProjectUpdates(
  projectId: number | string,
  page = 1,
  size = 20,
) {
  return requestClient.get<{
    items: ProjectUpdateRow[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/projects/${projectId}/updates`, { params: { page, size } });
}

export function createProjectUpdate(
  projectId: number | string,
  payload: UpdateUpsertPayload,
) {
  return requestClient.post<ProjectUpdateRow>(
    `/v1/projects/${projectId}/updates`,
    payload,
  );
}

export function updateProjectUpdate(
  updateId: number,
  payload: UpdateUpsertPayload,
) {
  return requestClient.put<{ id: number }>(
    `/v1/project-updates/${updateId}`,
    payload,
  );
}

export function deleteProjectUpdate(updateId: number) {
  return requestClient.delete<{ id: number }>(
    `/v1/project-updates/${updateId}`,
  );
}

/** 通用上传 multipart：TipTap 图片 + Documents PDF 用同一入口 */
export function uploadFile(file: File) {
  const form = new FormData();
  form.append('file', file);
  return requestClient.post<UploadedFile>('/v1/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
}

// —— 链上参数（Set 阶段专用表单，四眼审批） ——————————————————

/** POST /admin/v1/projects/:id/onchain-params 的载荷（链身份字段由服务端注入） */
export interface OnchainParamsRequest {
  contract_version?: 'ido_v3_stablecoin' | 'v2_stablecoin';
  payment_token: string; // 仅 symbol：USDT / USDC
  token_address: string;
  token_symbol?: string;
  owner_address: string;
  token_price: string;
  hard_cap: string;
  soft_cap: string;
  max_per_user: string;
  start_time: string; // ISO8601 含时区
  end_time: string;
  tge_after_end_min: number;
  tge_interval_min: number;
  tge: { percent: number }[];
}

export interface OnchainParamsPreview {
  project_id: number | string;
  chain: {
    chain_id: number;
    contract: string;
    fee_receiver: string;
    success_fee_rate: number;
  };
  payment_token: { address: string; decimals: number; symbol: string };
  project_token: {
    address: string;
    decimals: number;
    owner: string;
    symbol: string;
  };
  amounts: {
    hard_cap: string;
    hard_cap_raw: string;
    max_per_user: string;
    max_per_user_raw: string;
    soft_cap: string;
    soft_cap_raw: string;
    token_price: string;
    token_price_raw: string;
  };
  window: { end_utc: string; start_utc: string };
  tge: { index: number; percent: string; time_utc: string }[];
  soft_cap_token_estimate: string;
  hard_cap_token_estimate: string;
}

/** dry-run 等价物：全部校验 + 换算预览，不写库 */
/** 回显项目当前已保存的 Set 参数（表单回填用） */
export interface OnchainParamsCurrent {
  configured: boolean;
  locked: boolean;
  status: string;
  funding_mode: string;
  contract_version: string;
  chain_id: number;
  contract_address: string;
  payment_token: string;
  token_address: string;
  token_symbol: string;
  owner_address: string;
  token_price: string;
  hard_cap: string;
  soft_cap: string;
  max_per_user: string;
  start_time: string;
  end_time: string;
  tge: { percent: string }[];
  tge_after_end_min: number;
  tge_interval_min: number;
}

export function getOnchainParams(projectId: number | string) {
  return requestClient.get<OnchainParamsCurrent>(
    `/v1/projects/${projectId}/onchain-params`,
  );
}

// A preview is not an approval or an effective member snapshot. Review
// initiation and approval require distinct logged-in Admins with live permissions;
// only the separately governed Policy Safe may execute the on-chain change.
export interface V3PolicyPreview {
  preview_digest: string;
  chain_id: string;
  project_id: string;
  proxy: string;
  block: string;
  block_hash: string;
  state: string;
  old_enabled: boolean;
  old_root: string;
  old_version: string;
  target_version: string;
  enabled: boolean;
  root: string;
  members_digest: string;
  snapshot_digest: string;
  canonical_members: string[];
  added_members: string[];
  removed_members: string[];
  member_count: number;
  reason: string;
}

export interface V3PolicyStatus {
  /** Diagnostic only; never grants policy mutation or investment eligibility. */
  scanner_state?: 'reorg_halted' | 'running';
  chain_id: string;
  project_id: string;
  proxy: string;
  block: string;
  block_hash: string;
  project_state: string;
  enabled: boolean;
  root: string;
  version: string;
  approval_state: 'unavailable_for_proof';
}

export function getV3PolicyStatus(projectId: string) {
  return requestClient.get<V3PolicyStatus>(
    `/v1/projects/${projectId}/ido-v3/policy/status`,
    { timeout: 170_000 },
  );
}

export function previewV3InitialPolicy(
  projectId: string,
  wallets: string[],
  enabled: boolean,
  reason: string,
) {
  return requestClient.post<V3PolicyPreview>(
    `/v1/projects/${projectId}/ido-v3/policy/preview`,
    { wallets, enabled, reason },
    { timeout: 170_000 },
  );
}

export function startV3PolicyReview(
  projectId: string,
  body: {
    base_block: string;
    base_hash: string;
    enabled: boolean;
    preview_digest: string;
    reason: string;
    wallets: string[];
  },
) {
  return requestClient.post<{ review_id: string; state: string }>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews`,
    body,
    // Pinned review recomputation + independent receipt/status checks are
    // sequential. An ambiguous client timeout must be reconciled by the
    // review list, never by blindly submitting the same candidate again.
    { timeout: 170_000 },
  );
}

/** Project-scoped work items. A review decision is not a Safe execution. */
export interface V3PolicyReviewRow {
  review_id: string;
  project_id: string;
  initiator_admin_id: string;
  approver_admin_id?: string;
  status:
    | 'approved_expired'
    | 'approved_pending_safe'
    | 'expired'
    | 'observed_not_proof'
    | 'pending_second_admin'
    | 'superseded';
  superseded_by?: string;
  expected_version: string;
  target_version: string;
  enabled: boolean;
  member_count: number;
  reason: string;
  preview_digest: string;
  created_at: string;
  expires_at: string;
}

export function listV3PolicyReviews(projectId: string) {
  return requestClient.get<{ items: V3PolicyReviewRow[] }>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews`,
    { timeout: 170_000 },
  );
}

export function getV3PolicyReview(projectId: string, reviewId: string) {
  return requestClient.get<V3PolicyPreview>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}`,
    { timeout: 170_000 },
  );
}

/** Read-only exact Safe handoff, available only after the separate two-admin review. */
export interface V3PolicySafeHandoff {
  chain_id: string;
  project_id: string;
  proxy: string;
  governor: string;
  review_id: string;
  preview_digest: string;
  expected_version: string;
  target_version: string;
  enabled: boolean;
  root: string;
  calldata: string;
  value: '0';
  operation: 'CALL';
}

export function getV3PolicySafeHandoff(projectId: string, reviewId: string) {
  return requestClient.get<V3PolicySafeHandoff>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}/safe-handoff`,
    { timeout: 170_000 },
  );
}

export function approveV3PolicyReview(
  projectId: string,
  reviewId: string,
  digest: string,
) {
  return requestClient.post<{ review_id: string; state: string }>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}/approve`,
    { preview_digest: digest },
    { timeout: 170_000 },
  );
}

export function observeV3PolicyExecution(
  projectId: string,
  reviewId: string,
  receipt?: { safe_tx_hash: string; tx_hash: string },
) {
  return requestClient.post<{ review_id: string; state: string }>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}/observe`,
    receipt,
    { timeout: 170_000 },
  );
}

export function previewOnchainParams(
  projectId: number | string,
  body: OnchainParamsRequest,
) {
  return requestClient.post<OnchainParamsPreview>(
    `/v1/projects/${projectId}/onchain-params/preview`,
    body,
    {
      timeout: body.contract_version === 'ido_v3_stablecoin' ? 65_000 : 10_000,
    },
  );
}

/** 提交四眼审批（另一位 Admin approve 后落库） */
export function submitOnchainParams(
  projectId: number | string,
  body: OnchainParamsRequest,
  reason?: string,
) {
  return requestClient.post<{ approval_id: number; status: string }>(
    `/v1/projects/${projectId}/onchain-params`,
    body,
    { params: reason ? { reason } : {} },
  );
}
