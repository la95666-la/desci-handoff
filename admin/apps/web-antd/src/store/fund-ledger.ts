import type {
  FundLedgerEntryType,
  FundLedgerRow,
  FundLedgerStatus,
  FundReconciliation,
  IDOReconciliationDetail,
  IDOReconciliationSummary,
  IDOReconciliationWallet,
  PurchaseEntitlementRow,
  WalletPurchaseDetail,
} from '#/api/joincare/fund_ledger';

import { computed, h, reactive, ref } from 'vue';

import { message, Modal, Textarea } from 'ant-design-vue';
import { defineStore } from 'pinia';

import {
  cancelIDOReconciliation,
  createProjectIDOReconciliation,
  getIDOReconciliation,
  getProjectFundReconciliation,
  getWalletPurchaseDetail,
  listIDOReconciliationWallets,
  listProjectFundLedger,
  listProjectIDOReconciliations,
  listProjectPurchaseEntitlements,
} from '#/api/joincare/fund_ledger';
import { idoErrorMessage } from '#/api/joincare/ido-errors';
import { myPermissions } from '#/api/joincare/role';

export const useFundLedgerStore = defineStore('fund-ledger', () => {
  // ============ 权限 ============
  const permissionReady = ref(false);
  const permissionsLoading = ref(false);
  const canRead = ref(false);
  const canExecute = ref(false);

  // ============ 项目上下文 ============
  const loading = ref(false);
  const requestGeneration = ref(0);
  const projectId = ref<string>();
  const loadedProjectId = ref<string>();
  const reconciliation = ref<FundReconciliation | null>(null);

  // ============ 逐笔账本 & 权益 ============
  const rows = ref<FundLedgerRow[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(20);
  const entitlementRows = ref<PurchaseEntitlementRow[]>([]);
  const entitlementTotal = ref(0);
  const entitlementPage = ref(1);
  const entitlementPageSize = ref(20);
  const filter = reactive<{
    entryType?: FundLedgerEntryType;
    status?: FundLedgerStatus;
  }>({});

  // ============ 审计任务 ============
  const onchainGeneration = ref(0);
  const onchainLoading = ref(false);
  const reconciliationTasks = ref<IDOReconciliationSummary[]>([]);
  const reconciliationTaskTotal = ref(0);
  const reconciliationTask = ref<IDOReconciliationDetail | null>(null);
  const reconciliationWallets = ref<IDOReconciliationWallet[]>([]);
  const reconciliationWalletTotal = ref(0);
  const reconciliationWalletPage = ref(1);
  const reconciliationWalletPageSize = ref(50);
  const reconciliationReason = ref('固定区块资金审计');
  let auditPollTimer: ReturnType<typeof setTimeout> | undefined;

  // ============ 详情弹窗 ============
  const detailVisible = ref(false);
  const detail = ref<FundLedgerRow | null>(null);
  const walletDetailVisible = ref(false);
  const walletDetailLoading = ref(false);
  const walletDetail = ref<null | WalletPurchaseDetail>(null);

  // ============ 枚举与映射 ============
  const entryOptions: Array<{
    label: string;
    value: FundLedgerEntryType;
  }> = [
    { value: 'project_token_deposited', label: '项目币充值' },
    { value: 'purchase_payment', label: '认购支付' },
    { value: 'payment_refunded', label: '支付币退款' },
    { value: 'project_token_claimed', label: '项目币Claim' },
    { value: 'project_proceeds_paid', label: '项目方结算' },
    { value: 'platform_fee_paid', label: '平台费结算' },
    { value: 'project_tokens_withdrawn', label: '项目币退出' },
  ];

  const entryLabels = Object.fromEntries(
    entryOptions.map((option) => [option.value, option.label]),
  ) as Record<FundLedgerEntryType, string>;

  const checkCodeLabels: Record<string, string> = {
    project_identity: '项目身份',
    project_raised_display: '已筹金额显示一致性',
    project_deposited_display: '已存金额显示一致性',
    purchase_total: '认购总额',
    deposit_total: '存入总额',
    ledger_identity: '账本身份一致性',
    ledger_amount_encoding: '账本金额编码',
    ledger_amount_semantics: '账本金额语义',
    canonical_only: '仅正式账本',
    participant_count: '参与人数',
    purchase_entitlement_integrity: '认购权益完整性',
    purchase_entitlement_payment_total: '权益支付总额',
    tokens_sold_within_deposit: '已售Token不超存入',
    refunds_within_purchases: '退款不超认购',
    wallet_refunds_match_purchases: '钱包退款匹配认购',
    terminal_refund_completion: '终态退款完成度',
    payment_outflows_within_purchases: '支付流出不超认购',
    project_token_outflows_within_deposit: '项目币流出不超存入',
    project_proceeds_flag: '项目结算标志',
    project_proceeds: '项目结算金额',
    platform_fee_flag: '平台费标志',
    platform_fee: '平台费金额',
    project_token_withdrawal_flag: '项目币退出标志',
    project_token_withdrawal_status: '项目币退出状态',
    failed_cancelled_withdrawal_amount: '失败/取消退出金额',
    successful_withdrawal_amount: '成功退出金额',
    terminal_payment_flow: '终态支付流',
    claim_status: 'Claim状态',
    claim_wallet_ownership: 'Claim钱包归属',
    claim_entitlement_evidence: 'Claim权益证据',
  };

  function chainCheckLabel(code: string): string {
    const fixed: Record<string, string> = {
      chain_project_id: '项目ID',
      chain_project_identity: '项目身份',
      chain_project_decimals_fee: '精度与费率',
      chain_project_status: '项目状态',
      chain_token_price: '代币价格',
      chain_hard_cap: '硬顶',
      chain_soft_cap: '软顶',
      chain_max_per_user: '每人上限',
      chain_start_time: '开始时间',
      chain_end_time: '结束时间',
      chain_total_raised: '总融资额',
      chain_total_tokens_sold: '总售出代币',
      chain_total_tokens_deposited: '总存入代币',
      chain_withdrawal_flags: '退出标志',
      chain_vesting_schedule: '解锁计划',
      chain_participants: '参与人',
      chain_payment_config_decimals: '支付币精度',
      chain_payment_reserve_solvency: '支付币储备偿付',
      chain_payment_project_liability: '支付币项目负债',
      chain_project_config_decimals: '项目币精度',
      chain_project_reserve_solvency: '项目币储备偿付',
      chain_project_project_liability: '项目币项目负债',
    };
    if (fixed[code]) return fixed[code];
    if (code.startsWith('chain_wallet_payment_')) return '钱包支付金额';
    if (code.startsWith('chain_wallet_tokens_')) return '钱包应得代币';
    if (code.startsWith('chain_wallet_claimed_')) return '钱包已Claim';
    if (code.startsWith('chain_wallet_refunded_')) return '钱包退款标志';
    return code;
  }

  const statusOptions: Array<{ label: string; value: FundLedgerStatus }> = [
    { value: 'canonical', label: 'canonical' },
    { value: 'reversed', label: 'reversed' },
    { value: 'reconciliation', label: 'reconciliation' },
  ];

  const failedChecks = computed(
    () => reconciliation.value?.checks.filter((check) => !check.ok) || [],
  );

  // ============ 表格列定义 ============
  const columns = [
    { title: 'ID', dataIndex: 'id', width: 80, fixed: 'left' as const },
    { title: '类型', dataIndex: 'entry_type', width: 170 },
    { title: '资产', dataIndex: 'token_symbol', width: 100 },
    { title: '金额', dataIndex: 'amount_display', width: 180 },
    { title: '钱包', dataIndex: 'wallet_address', width: 220, ellipsis: true },
    { title: 'X账号', dataIndex: 'twitter_handle', width: 150 },
    { title: 'TG账号', dataIndex: 'telegram_username', width: 150 },
    { title: '状态', dataIndex: 'status', width: 130 },
    { title: '区块', dataIndex: 'block_number', width: 120 },
    { title: '交易哈希 Tx Hash', dataIndex: 'tx_hash', width: 220 },
    { title: '日志索引 Log', dataIndex: 'log_index', width: 80 },
    { title: '时间', dataIndex: 'created_at', width: 180 },
    { title: '操作', dataIndex: '_action', width: 90, fixed: 'right' as const },
  ];

  const entitlementColumns = [
    { title: 'ID', dataIndex: 'id', width: 80, fixed: 'left' as const },
    { title: '钱包', dataIndex: 'wallet_address', width: 220, ellipsis: true },
    { title: 'X账号', dataIndex: 'twitter_handle', width: 150 },
    { title: 'TG账号', dataIndex: 'telegram_username', width: 150 },
    { title: '支付币', dataIndex: 'payment_token_symbol', width: 100 },
    {
      title: '支付金额',
      dataIndex: 'payment_amount_display',
      width: 170,
    },
    { title: '项目币', dataIndex: 'project_token_symbol', width: 100 },
    {
      title: '权益金额',
      dataIndex: 'project_token_amount_display',
      width: 170,
    },
    { title: '区块', dataIndex: 'block_number', width: 120 },
    { title: '交易哈希 Tx Hash', dataIndex: 'tx_hash', width: 220 },
    { title: '日志索引 Log', dataIndex: 'log_index', width: 80 },
  ];

  // ============ 权限加载 ============
  async function loadPermissions() {
    if (permissionReady.value || permissionsLoading.value) return;
    permissionsLoading.value = true;
    try {
      const result = await myPermissions();
      canRead.value =
        result.is_super || result.permissions.includes('m2_project:read');
      canExecute.value =
        result.is_super || result.permissions.includes('m2_project:execute');
    } catch {
      canRead.value = false;
    } finally {
      permissionsLoading.value = false;
      permissionReady.value = true;
    }
  }

  // ============ 数据加载 ============
  async function load() {
    if (!canRead.value || !projectId.value) return;
    const generation = ++requestGeneration.value;
    const requestedProjectId = projectId.value;
    loading.value = true;
    try {
      const [ledger, entitlements, nextReconciliation, tasks] =
        await Promise.all([
          listProjectFundLedger(requestedProjectId, {
            entry_type: filter.entryType,
            page: page.value,
            page_size: pageSize.value,
            status: filter.status,
          }),
          listProjectPurchaseEntitlements(requestedProjectId, {
            page: entitlementPage.value,
            page_size: entitlementPageSize.value,
          }),
          getProjectFundReconciliation(requestedProjectId),
          listProjectIDOReconciliations(requestedProjectId, {
            page: 1,
            page_size: 20,
          }),
        ]);
      if (generation !== requestGeneration.value) return;
      rows.value = ledger.items || [];
      total.value = ledger.total || 0;
      entitlementRows.value = entitlements.items || [];
      entitlementTotal.value = entitlements.total || 0;
      reconciliation.value = nextReconciliation;
      loadedProjectId.value = requestedProjectId;
      reconciliationTasks.value = tasks.items || [];
      reconciliationTaskTotal.value = tasks.total || 0;
    } catch (error: any) {
      if (generation !== requestGeneration.value) return;
      reconciliation.value = null;
      loadedProjectId.value = undefined;
      rows.value = [];
      total.value = 0;
      entitlementRows.value = [];
      entitlementTotal.value = 0;
      reconciliationTasks.value = [];
      reconciliationTaskTotal.value = 0;
      message.error(idoErrorMessage(error, '资金账本读取失败，不能据此认定资金已结清。'));
    } finally {
      if (generation === requestGeneration.value) loading.value = false;
    }
  }

  function searchProject() {
    page.value = 1;
    entitlementPage.value = 1;
    loadedProjectId.value = undefined;
    reconciliation.value = null;
    onchainGeneration.value += 1;
    onchainLoading.value = false;
    reconciliationTask.value = null;
    reconciliationWallets.value = [];
    reconciliationWalletTotal.value = 0;
    clearAuditPoll();
    void load();
  }

  function invalidateLoadedProject() {
    requestGeneration.value += 1;
    onchainGeneration.value += 1;
    loadedProjectId.value = undefined;
    reconciliation.value = null;
    reconciliationTasks.value = [];
    reconciliationTask.value = null;
    reconciliationWallets.value = [];
    reconciliationWalletTotal.value = 0;
    clearAuditPoll();
  }

  function resetFilters() {
    filter.entryType = undefined;
    filter.status = undefined;
    page.value = 1;
    void load();
  }

  // ============ 链上审计 ============
  async function createOnchainAudit() {
    if (
      !canExecute.value ||
      !loadedProjectId.value ||
      loadedProjectId.value !== projectId.value ||
      !reconciliation.value
    )
      return;
    const reason = reconciliationReason.value.trim();
    if (reason.length < 3 || reason.length > 500) {
      message.error('审计理由必须为3–500个字符。错误码：IDO_AUDIT_REASON_INVALID');
      return;
    }
    const generation = ++onchainGeneration.value;
    const requestedProjectId = loadedProjectId.value;
    onchainLoading.value = true;
    try {
      const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
      const result = await createProjectIDOReconciliation(requestedProjectId, {
        idempotency_key: idempotencyKey,
        reason,
      });
      if (generation !== onchainGeneration.value) return;
      message.success(
        result.created
          ? `审计任务 #${result.run_id} 已创建`
          : `审计任务 #${result.run_id} 已存在`,
      );
      await refreshAuditHistory(generation, requestedProjectId);
      reconciliationWalletPage.value = 1;
      await openAudit(result.run_id, generation);
    } catch (error: any) {
      if (generation !== onchainGeneration.value) return;
      message.error(idoErrorMessage(error, '链上审计任务创建结果未确认，请先查询已有任务。'));
    } finally {
      if (generation === onchainGeneration.value) onchainLoading.value = false;
    }
  }

  async function refreshAuditHistory(
    generation = onchainGeneration.value,
    requestedProjectId = loadedProjectId.value,
  ) {
    if (
      !canRead.value ||
      !requestedProjectId ||
      requestedProjectId !== loadedProjectId.value ||
      requestedProjectId !== projectId.value
    )
      return;
    const result = await listProjectIDOReconciliations(requestedProjectId, {
      page: 1,
      page_size: 20,
    });
    if (generation !== onchainGeneration.value) return;
    reconciliationTasks.value = result.items || [];
    reconciliationTaskTotal.value = result.total || 0;
  }

  async function openAudit(
    runId: number,
    generation = ++onchainGeneration.value,
  ) {
    clearAuditPoll();
    onchainLoading.value = true;
    try {
      const [detailResponse, wallets] = await Promise.all([
        getIDOReconciliation(runId),
        listIDOReconciliationWallets(runId, {
          page: reconciliationWalletPage.value,
          page_size: reconciliationWalletPageSize.value,
        }),
      ]);
      if (generation !== onchainGeneration.value) return;
      reconciliationTask.value = detailResponse;
      reconciliationWallets.value = wallets.items || [];
      reconciliationWalletTotal.value = wallets.total || 0;
      if (
        [
          'cancel_requested',
          'finalizing',
          'queued',
          'retrying',
          'running',
        ].includes(detailResponse.summary.state)
      ) {
        auditPollTimer = setTimeout(() => {
          if (generation === onchainGeneration.value) {
            void openAudit(runId, generation);
            void refreshAuditHistory(
              generation,
              String(detailResponse.summary.project_id),
            ).catch(() => undefined);
          }
        }, 5000);
      }
    } catch (error: any) {
      if (generation !== onchainGeneration.value) return;
      message.error(idoErrorMessage(error, '链上审计任务读取失败，请核对原任务编号。'));
    } finally {
      if (generation === onchainGeneration.value) onchainLoading.value = false;
    }
  }

  function clearAuditPoll() {
    if (auditPollTimer) clearTimeout(auditPollTimer);
    auditPollTimer = undefined;
  }

  function requestCancelAudit() {
    const summary = reconciliationTask.value?.summary;
    if (!summary || !canExecute.value) return;
    let reason = '';
    const generation = onchainGeneration.value;
    const requestedProjectId = loadedProjectId.value;
    Modal.confirm({
      title: `取消审计任务 #${summary.run_id}`,
      content: () =>
        h(Textarea, {
          maxlength: 500,
          placeholder: '请输入3–500字取消原因',
          rows: 4,
          'onUpdate:value': (value: string) => (reason = value),
        }),
      async onOk() {
        reason = reason.trim();
        if (reason.length < 3) throw new Error('取消原因至少3个字符');
        if (
          generation !== onchainGeneration.value ||
          requestedProjectId !== loadedProjectId.value ||
          requestedProjectId !== projectId.value ||
          summary.project_id !== requestedProjectId
        ) {
          throw new Error('项目上下文已变化，请重新打开任务后再取消');
        }
        await cancelIDOReconciliation(summary.run_id, reason);
        message.success('取消请求已记录');
        if (
          generation !== onchainGeneration.value ||
          requestedProjectId !== loadedProjectId.value ||
          requestedProjectId !== projectId.value
        )
          return;
        await openAudit(summary.run_id);
        await refreshAuditHistory();
      },
    });
  }

  function selectAudit(runId: number) {
    reconciliationWalletPage.value = 1;
    void openAudit(runId);
  }

  // ============ 详情 & 工具 ============
  function openDetail(row: FundLedgerRow) {
    detail.value = row;
    detailVisible.value = true;
  }

  async function openWalletDetail(wallet: string) {
    if (!loadedProjectId.value || !wallet) return;
    walletDetailVisible.value = true;
    walletDetailLoading.value = true;
    walletDetail.value = null;
    try {
      walletDetail.value = await getWalletPurchaseDetail(
        loadedProjectId.value,
        wallet,
      );
    } catch (error: any) {
      message.error(idoErrorMessage(error, '钱包认购详情读取失败，不能据此认定没有认购或退款义务。'));
    } finally {
      walletDetailLoading.value = false;
    }
  }

  function fmtTime(value?: string) {
    if (!value) return '—';
    return value.replace('T', ' ').replace(/\.\d+/, '').replace(/Z$/, ' UTC');
  }

  function shortHash(value?: string) {
    if (!value) return '—';
    return value.length > 20
      ? `${value.slice(0, 10)}…${value.slice(-8)}`
      : value;
  }

  function formatUnits(raw?: string, decimals = 0): string {
    if (raw === null || raw === undefined || raw === '') return '—';
    let digits = String(raw);
    if (decimals <= 0) return digits;
    if (digits.length <= decimals) {
      digits = '0'.repeat(decimals - digits.length + 1) + digits;
    }
    const cut = digits.length - decimals;
    const fraction = digits.slice(cut).replace(/0+$/, '');
    return fraction
      ? `${digits.slice(0, cut)}.${fraction}`
      : digits.slice(0, cut);
  }

  return {
    permissionReady,
    canRead,
    canExecute,
    loading,
    projectId,
    loadedProjectId,
    reconciliation,
    rows,
    total,
    page,
    pageSize,
    entitlementRows,
    entitlementTotal,
    entitlementPage,
    entitlementPageSize,
    filter,
    onchainGeneration,
    onchainLoading,
    reconciliationTasks,
    reconciliationTaskTotal,
    reconciliationTask,
    reconciliationWallets,
    reconciliationWalletTotal,
    reconciliationWalletPage,
    reconciliationWalletPageSize,
    reconciliationReason,
    detailVisible,
    detail,
    walletDetailVisible,
    walletDetailLoading,
    walletDetail,
    entryOptions,
    entryLabels,
    checkCodeLabels,
    chainCheckLabel,
    statusOptions,
    failedChecks,
    columns,
    entitlementColumns,
    loadPermissions,
    load,
    searchProject,
    invalidateLoadedProject,
    resetFilters,
    createOnchainAudit,
    refreshAuditHistory,
    openAudit,
    clearAuditPoll,
    requestCancelAudit,
    selectAudit,
    openDetail,
    openWalletDetail,
    fmtTime,
    shortHash,
    formatUnits,
  };
});
