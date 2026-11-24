<script setup lang="ts">
import type { CSSProperties } from 'vue';

import type { V3CurrentMembers } from './v3-current-members';

import type {
  V3PolicyPreview,
  V3PolicyReviewRow,
  V3PolicySafeHandoff,
  V3PolicyStatus,
} from '#/api/joincare/project_content';
import type { V3SafeProposal } from '#/api/joincare/v3_safe_proposal';

import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Input,
  message,
  Modal,
  Switch,
  Table,
  Tag,
  theme,
} from 'ant-design-vue';

import { idoErrorMessage as v3DiagnosticMessage } from '#/api/joincare/ido-errors';
import {
  approveV3PolicyReview,
  getOnchainParams,
  getV3PolicyReview,
  getV3PolicySafeHandoff,
  getV3PolicyStatus,
  listV3PolicyReviews,
  observeV3PolicyExecution,
  previewV3InitialPolicy,
  startV3PolicyReview,
} from '#/api/joincare/project_content';
import { myPermissions } from '#/api/joincare/role';
import {
  previewV3PolicySafeProposal,
  signV3SafeProposal,
  submitV3PolicySafeProposal,
} from '#/api/joincare/v3_safe_proposal';

import { loadV3CurrentMembers } from './v3-current-members';
import {
  isV3PolicyPreviewProjectCreated,
  isV3PolicyReadableProject,
} from './v3-policy-eligibility';
import { v3PolicySafeAbi } from './v3-policy-safe-abi';

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const projectId = String(route.params.id ?? '');
const { token: themeToken } = theme.useToken();
const policyTheme = computed(
  () =>
    ({
      '--policy-surface': themeToken.value.colorBgContainer,
      '--policy-inset': themeToken.value.colorFillAlter,
      '--policy-border': themeToken.value.colorBorderSecondary,
      '--policy-text': themeToken.value.colorText,
      '--policy-muted': themeToken.value.colorTextSecondary,
      '--policy-accent': themeToken.value.colorPrimary,
      '--policy-accent-bg': themeToken.value.colorPrimaryBg,
      '--policy-positive': themeToken.value.colorSuccessText,
      '--policy-positive-bg': themeToken.value.colorSuccessBg,
      '--policy-positive-border': themeToken.value.colorSuccessBorder,
      '--policy-negative': themeToken.value.colorErrorText,
      '--policy-negative-bg': themeToken.value.colorErrorBg,
      '--policy-negative-border': themeToken.value.colorErrorBorder,
    }) as CSSProperties,
);
const walletCount = computed(
  () => wallets.value.split(/[\s,]+/).filter(Boolean).length,
);
const pendingReviewCount = computed(
  () =>
    reviews.value.filter((row) => row.status === 'pending_second_admin').length,
);
const workflowSteps = [
  { id: 'policy-draft', title: '准备候选', description: '完整名单 · 只读预览' },
  { id: 'policy-reviews', title: '双人审批', description: '独立管理员核对' },
  { id: 'policy-safe', title: 'Safe 交接', description: 'Owner 签名与执行' },
  {
    id: 'policy-receipt',
    title: '回执核验',
    description: '原交易 · 等待 Job / proof',
  },
];
const wallets = ref('');
const reason = ref('');
const enabled = ref(true);
const busy = ref(false);
const result = ref<null | V3PolicyPreview>(null);
const status = ref<null | V3PolicyStatus>(null);
const statusFailureMessage = ref('');
const projectCreated = ref(false);
const policyReadable = ref(false);
const projectStateKnown = ref(false);
const canWritePolicy = ref(false);
const canApprovePolicy = ref(false);
const reviewID = ref('');
const reviews = ref<V3PolicyReviewRow[]>([]);
const reviewsLoading = ref(false);
const reviewsUnavailable = ref(false);
const currentMembers = ref<null | V3CurrentMembers>(null);
const currentMembersLoading = ref(false);
const currentMembersUnavailable = ref(false);
let currentMembersRequest = 0;
const currentMemberSearch = ref('');
const filteredCurrentMembers = computed(
  () =>
    currentMembers.value?.members.filter((wallet) =>
      wallet.includes(currentMemberSearch.value.trim().toLowerCase()),
    ) ?? [],
);

watch(
  [status, reviews, reviewsUnavailable, reviewsLoading],
  async () => {
    const request = ++currentMembersRequest;
    currentMembers.value = null;
    currentMembersUnavailable.value = false;
    currentMembersLoading.value = false;
    currentMemberSearch.value = '';
    const live = status.value;
    if (!live?.enabled || reviewsUnavailable.value || reviewsLoading.value)
      return;
    currentMembersLoading.value = true;
    try {
      const loaded = await loadV3CurrentMembers(
        live,
        reviews.value,
        (id) => getV3PolicyReview(projectId, id),
        () => getV3PolicyStatus(projectId),
      );
      if (request === currentMembersRequest) currentMembers.value = loaded;
    } catch {
      if (request === currentMembersRequest)
        currentMembersUnavailable.value = true;
    } finally {
      if (request === currentMembersRequest)
        currentMembersLoading.value = false;
    }
  },
  { flush: 'post' },
);
// A timed-out POST has an unknown outcome. Require a readback before any
// further mutation; never turn a timeout into an automatic retry.
const mutationUnknown = ref(false);
const unknownDigest = ref('');
const unknownOperation = ref<'approve' | 'submit' | null>(null);
const currentAdminID = computed(() => String(userStore.userInfo?.userId ?? ''));
const selectedReview = computed(() =>
  reviews.value.find((row) => row.review_id === reviewID.value),
);
const canApproveSelected = computed(
  () =>
    selectedReview.value?.status === 'pending_second_admin' &&
    canApprovePolicy.value &&
    /^[1-9]\d*$/.test(currentAdminID.value) &&
    selectedReview.value.initiator_admin_id !== currentAdminID.value &&
    !!reviewed.value,
);
const approvedSelected = computed(
  () =>
    selectedReview.value?.status === 'approved_pending_safe' ||
    selectedReview.value?.status === 'observed_not_proof',
);
// This is a display-only hint. A canonical chain observation still requires
// the original Safe receipt, signed release attestation and Backend gates.
const policyMayHaveExecuted = computed(
  () =>
    !!reviewed.value &&
    !!status.value &&
    status.value.version === reviewed.value.target_version &&
    status.value.enabled === reviewed.value.enabled &&
    status.value.root.toLowerCase() === reviewed.value.root.toLowerCase(),
);
const handoffMatchesReview = computed(
  () =>
    !!safeHandoff.value &&
    !!reviewed.value &&
    safeHandoff.value.review_id === reviewID.value &&
    safeHandoff.value.preview_digest === reviewed.value.preview_digest &&
    safeHandoff.value.root === reviewed.value.root,
);
const reviewColumns = [
  { title: '工单', dataIndex: 'review_id' },
  { title: '目标版本', dataIndex: 'target_version' },
  { title: '发起人 ID', dataIndex: 'initiator_admin_id' },
  { title: '完整名单数', dataIndex: 'member_count' },
  { title: '状态', dataIndex: 'status' },
  { title: '到期', dataIndex: 'expires_at' },
  { title: '操作', dataIndex: 'action' },
];
const reviewStatusLabel: Record<V3PolicyReviewRow['status'], string> = {
  pending_second_admin: '待第二位管理员审批',
  approved_pending_safe: '已审批，待 Policy Safe 执行',
  observed_not_proof: '已核验回执，待 Job / proof',
  expired: '已过期',
  approved_expired: '已审批但已过期，不能继续 Safe 交接',
  superseded: '已由新工单接替，原审批仅供审计',
};
const reviewed = ref<null | V3PolicyPreview>(null);
const safeHandoff = ref<null | V3PolicySafeHandoff>(null);
const safeProposal = ref<null | V3SafeProposal>(null);
const executedTxHash = ref('');
const executedSafeTxHash = ref('');
const safeUrl = computed(() =>
  safeHandoff.value?.chain_id === '56' &&
  /^0x[\da-fA-F]{40}$/.test(safeHandoff.value.governor)
    ? `https://app.safe.global/home?safe=bnb:${safeHandoff.value.governor}`
    : '',
);
const transactionBuilderUrl = computed(() =>
  safeUrl.value && safeHandoff.value
    ? `https://app.safe.global/apps/open?safe=${encodeURIComponent(`bnb:${safeHandoff.value.governor}`)}&appUrl=${encodeURIComponent('https://apps-portal.safe.global/tx-builder')}`
    : '',
);

async function refreshProjectState() {
  projectCreated.value = false;
  policyReadable.value = false;
  projectStateKnown.value = false;
  if (!/^[1-9]\d*$/.test(projectId)) return;
  try {
    const current = await getOnchainParams(projectId);
    projectCreated.value = isV3PolicyPreviewProjectCreated(current);
    policyReadable.value = isV3PolicyReadableProject(current);
    projectStateKnown.value = true;
  } catch {
    projectStateKnown.value = false;
  }
}

async function refreshPolicyContext() {
  status.value = null;
  statusFailureMessage.value = '';
  await refreshProjectState();
  if (!policyReadable.value) return;
  try {
    status.value = await getV3PolicyStatus(projectId);
  } catch (error) {
    status.value = null;
    statusFailureMessage.value = v3DiagnosticMessage(
      error,
      '链上状态核验失败，请提供错误码供管理员排查。',
    );
  }
}

async function refreshCurrentMembers() {
  await Promise.all([refreshPolicyContext(), loadReviews()]);
}

async function loadReviews() {
  reviewsLoading.value = true;
  reviewsUnavailable.value = false;
  try {
    const response = await listV3PolicyReviews(projectId);
    reviews.value = response.items;
    if (!unknownOperation.value) {
      mutationUnknown.value = false;
      unknownDigest.value = '';
      unknownOperation.value = null;
    } else if (unknownOperation.value === 'approve') {
      const matching = reviews.value.find(
        (row) =>
          row.review_id === reviewID.value &&
          row.preview_digest === unknownDigest.value,
      );
      if (matching && matching.status !== 'pending_second_admin') {
        mutationUnknown.value = false;
        unknownDigest.value = '';
        unknownOperation.value = null;
      }
    }
  } catch {
    reviewsUnavailable.value = true;
    reviews.value = [];
  } finally {
    reviewsLoading.value = false;
  }
}

onMounted(async () => {
  await Promise.all([
    refreshPolicyContext(),
    loadReviews(),
    loadPolicyPermissions(),
  ]);
  const requested = String(route.query.review ?? '');
  if (requested && reviews.value.some((row) => row.review_id === requested)) {
    await inspectReview(requested);
  }
});

async function loadPolicyPermissions() {
  try {
    const access = await myPermissions();
    canWritePolicy.value =
      access.is_super || access.permissions.includes('m2_project:write');
    canApprovePolicy.value =
      access.is_super || access.permissions.includes('m2_project:approve');
  } catch {
    canWritePolicy.value = false;
    canApprovePolicy.value = false;
  }
}

watch([wallets, reason, enabled], () => {
  result.value = null;
});

async function preview() {
  result.value = null;
  if (!canWritePolicy.value || !/^[1-9]\d*$/.test(projectId)) return;
  // Re-read on every attempt: a Set, a pending original receipt, or a stale
  // page cannot grant a governance preview. Backend independently re-admits.
  await refreshProjectState();
  if (!projectCreated.value) {
    message.error(
      projectStateKnown.value
        ? '请先创建 V3 项目，并等待原交易回执及 Job 扫描对账；Set 审批不等于链上创建。错误码：V3_POLICY_PROJECT_NOT_CREATED'
        : '无法读取项目链上状态；未生成治理预览。错误码：IDO_PROJECT_STATE_UNAVAILABLE',
    );
    return;
  }
  const addresses = wallets.value.split(/[\s,]+/).filter(Boolean);
  if (
    (enabled.value && addresses.length === 0) ||
    (!enabled.value && addresses.length > 0) ||
    addresses.length > 512 ||
    !reason.value.trim()
  ) {
    message.error(
      '启用须提供 1–512 个完整钱包；Pending 关闭须为空名单，并填写理由。错误码：V3_POLICY_INPUT_INVALID',
    );
    return;
  }
  busy.value = true;
  try {
    result.value = await previewV3InitialPolicy(
      projectId,
      addresses,
      enabled.value,
      reason.value,
    );
  } catch (error) {
    const timedOut =
      error instanceof Error &&
      (('code' in error && error.code === 'ECONNABORTED') ||
        /timeout of \d+ms exceeded/i.test(error.message));
    message.error(
      timedOut
        ? '白名单预览请求超时；本次仅只读预览，未创建审批或资格。请查询项目状态。错误码：IDO_PREVIEW_TIMEOUT'
        : v3DiagnosticMessage(
            error,
            'V3 策略预览未通过校验；未创建审批。请提供错误码和失败阶段。',
          ),
    );
  } finally {
    busy.value = false;
  }
}

async function startReview() {
  if (
    !canWritePolicy.value ||
    !result.value ||
    mutationUnknown.value ||
    busy.value
  )
    return;
  const p = result.value;
  const digest = p.preview_digest;
  busy.value = true;
  let submittedID = '';
  try {
    const row = await startV3PolicyReview(projectId, {
      wallets: p.canonical_members,
      enabled: p.enabled,
      reason: p.reason,
      base_block: p.block,
      base_hash: p.block_hash,
      preview_digest: p.preview_digest,
    });
    submittedID = row.review_id;
    reviewID.value = submittedID;
    result.value = null;
    message.success('仅已保存待第二位管理员审核的候选；未生成治理提案或资格');
  } catch (error) {
    mutationUnknown.value = true;
    unknownDigest.value = digest;
    unknownOperation.value = 'submit';
    const code = (error as { response?: { data?: { code?: string } } })
      ?.response?.data?.code;
    if (code === 'V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED') {
      message.error(
        '白名单审批数据库迁移尚未完成；请求未进入审批写入，请联系运维完成迁移。错误码：V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED',
      );
    } else {
      message.warning(
        v3DiagnosticMessage(
          error,
          '提交失败或结果未知：先刷新工单列表核对同一摘要，勿连续重复提交',
        ),
      );
    }
  } finally {
    busy.value = false;
  }
  if (submittedID) {
    await loadReviews();
    if (reviews.value.some((row) => row.review_id === submittedID)) {
      await inspectReview(submittedID);
    } else {
      message.warning(
        `工单 #${submittedID} 已提交；列表暂不可用，请刷新后核对`,
      );
    }
  }
}

async function approveReview() {
  if (
    !canApproveSelected.value ||
    !reviewed.value ||
    mutationUnknown.value ||
    busy.value
  )
    return;
  busy.value = true;
  let approved = false;
  try {
    await approveV3PolicyReview(
      projectId,
      reviewID.value,
      reviewed.value.preview_digest,
    );
    message.success(
      '双人审批记录已保存；尚未获得 Safe 授权或链上生效，不可发放 proof',
    );
    approved = true;
    safeHandoff.value = null;
    safeProposal.value = null;
  } catch (error) {
    mutationUnknown.value = true;
    unknownDigest.value = reviewed.value.preview_digest;
    unknownOperation.value = 'approve';
    const code = (error as { response?: { data?: { code?: string } } })
      ?.response?.data?.code;
    if (code === 'V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED') {
      message.error('白名单审批数据库迁移尚未完成；本次二审未写入。错误码：V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED');
    } else {
      message.warning(
        v3DiagnosticMessage(
          error,
          '二审失败或结果未知：先刷新工单状态核对，勿重复点击二审',
        ),
      );
    }
  } finally {
    busy.value = false;
  }
  if (approved) await loadReviews();
}

function confirmApproval() {
  if (
    !canApproveSelected.value ||
    !reviewed.value ||
    mutationUnknown.value ||
    busy.value
  )
    return;
  const p = reviewed.value;
  Modal.confirm({
    title: `二审白名单工单 #${reviewID.value}？`,
    content: `核对项目 ${p.project_id}、版本 ${p.old_version} → ${p.target_version}、完整名单 ${p.member_count} 个及 Root ${p.root}。批准仅保存独立决策，不会向 Safe 签名或上链。`,
    okText: '确认二审',
    onOk: approveReview,
  });
}

async function inspectReview(id: string) {
  reviewed.value = null;
  safeHandoff.value = null;
  safeProposal.value = null;
  reviewID.value = id;
  if (
    !/^[1-9]\d*$/.test(id) ||
    !reviews.value.some((row) => row.review_id === id)
  )
    return;
  busy.value = true;
  try {
    reviewed.value = await getV3PolicyReview(projectId, id);
    await router.replace({ query: { ...route.query, review: id } });
  } catch (error) {
    message.error(v3DiagnosticMessage(error, '不可变工单详情不可用；不能进行二审。'));
  } finally {
    busy.value = false;
  }
}

async function inspectSafeHandoff() {
  safeHandoff.value = null;
  safeProposal.value = null;
  if (
    !reviewed.value ||
    !approvedSelected.value ||
    policyMayHaveExecuted.value ||
    !/^[1-9]\d*$/.test(reviewID.value)
  )
    return;
  busy.value = true;
  try {
    const handoff = await getV3PolicySafeHandoff(projectId, reviewID.value);
    if (
      handoff.chain_id !== reviewed.value.chain_id ||
      handoff.project_id !== reviewed.value.project_id ||
      handoff.proxy !== reviewed.value.proxy ||
      handoff.preview_digest !== reviewed.value.preview_digest ||
      handoff.root !== reviewed.value.root ||
      handoff.review_id !== reviewID.value ||
      handoff.operation !== 'CALL' ||
      handoff.value !== '0'
    )
      throw new Error('Safe handoff differs from review');
    safeHandoff.value = handoff;
  } catch (error) {
    message.error(
      v3DiagnosticMessage(
        error,
        '执行前 Safe 交接不可用；若链上已执行，请直接进行第 4 步原交易回执核验，不要重新提案',
      ),
    );
  } finally {
    busy.value = false;
  }
}

async function observe() {
  if (!approvedSelected.value || !/^[1-9]\d*$/.test(reviewID.value)) return;
  // Hashes are locators only; Backend rechecks the approved review, original
  // Safe execution and canonical receipt before recording any observation.
  const txHash = executedTxHash.value.trim().toLowerCase();
  const safeTxHash = executedSafeTxHash.value.trim().toLowerCase();
  if (
    !/^0x[0-9a-f]{64}$/.test(txHash) ||
    !/^0x[0-9a-f]{64}$/.test(safeTxHash) ||
    txHash === safeTxHash
  ) {
    message.error('请输入原始链上交易哈希和不同的 Safe 交易哈希。错误码：V3_POLICY_INPUT_INVALID');
    return;
  }
  if (safeProposal.value?.safe_tx_hash?.toLowerCase() === txHash) {
    message.error(
      '两个哈希可能填反了：链上交易哈希请填 Safe 页面 Transaction Hash；Safe 交易哈希请填 Safe Tx Hash。不要重新执行交易。错误码：IDO_RECEIPT_HASH_INVALID',
    );
    return;
  }
  busy.value = true;
  try {
    await observeV3PolicyExecution(projectId, reviewID.value, {
      tx_hash: txHash,
      safe_tx_hash: safeTxHash,
    });
    message.success('已记录独立核验的治理回执；状态仍不是可发放 proof 的资格');
    await loadReviews();
  } catch (error) {
    message.warning(
      v3DiagnosticMessage(
        error,
        '回执核验失败或结果未知：先刷新工单状态并按原交易哈希核对，勿重复提交',
      ),
    );
  } finally {
    busy.value = false;
  }
}

async function prepareSafeProposal() {
  safeProposal.value = null;
  if (!safeHandoff.value) return;
  busy.value = true;
  try {
    const p = await previewV3PolicySafeProposal(projectId, reviewID.value);
    if (
      p.safe.toLowerCase() !== safeHandoff.value.governor.toLowerCase() ||
      p.to.toLowerCase() !== safeHandoff.value.proxy.toLowerCase() ||
      p.data.toLowerCase() !== safeHandoff.value.calldata.toLowerCase()
    )
      throw new Error('目标或数据不匹配');
    safeProposal.value = p;
  } catch (error) {
    message.error(
      v3DiagnosticMessage(error, 'Safe 提案预览不可用；复查审批和链上状态'),
    );
  } finally {
    busy.value = false;
  }
}

async function proposeWithOwner() {
  if (!safeProposal.value) return;
  busy.value = true;
  const hash = safeProposal.value.safe_tx_hash;
  try {
    const signed = await signV3SafeProposal(safeProposal.value);
    await submitV3PolicySafeProposal(projectId, reviewID.value, signed);
    message.success(
      `Safe 提案已提交；待其他 Owner 在 Safe 确认。Hash: ${hash}`,
    );
  } catch (error) {
    message.error(
      v3DiagnosticMessage(error, `提案结果未知或被拒；先在 Safe 按原 Hash ${hash} 查询，不要盲目重试。`),
    );
  } finally {
    safeProposal.value = null;
    busy.value = false;
  }
}
</script>

<template>
  <Page title="V3 白名单版本化治理">
    <div class="policy-workspace" :style="policyTheme">
      <header class="policy-hero">
        <div>
          <span class="policy-eyebrow">IDO V3 / ACCESS GOVERNANCE</span>
          <h1>白名单版本化治理</h1>
          <p class="policy-subtitle">
            完整名单替换、独立审批与 Safe 执行，在一个工作区完成核对。
          </p>
          <div class="policy-hero-tags">
            <Tag color="blue">项目 #{{ projectId }}</Tag>
            <Tag>{{ canWritePolicy ? '可发起候选' : '只读候选' }}</Tag>
            <Tag v-if="canApprovePolicy" color="purple">可独立二审</Tag>
          </div>
        </div>
        <Button :disabled="busy" @click="refreshPolicyContext">
          刷新链上状态
        </Button>
      </header>
      <section class="policy-summary" aria-label="当前治理状态">
        <div>
          <span>链上模式</span><strong>{{
            status ? (status.enabled ? '白名单' : '公开认购') : '待核验'
          }}</strong><small>以独立链上读回为准</small>
        </div>
        <div>
          <span>当前版本</span><strong>{{ status ? `v${status.version}` : '—' }}</strong><small>{{
            status ? `确认块 ${status.block}` : '尚无可用确认块'
          }}</small>
        </div>
        <div>
          <span>待二审工单</span><strong>{{ reviewsUnavailable ? '—' : pendingReviewCount }}</strong><small>发起人与审批人必须不同</small>
        </div>
        <div>
          <span>资格边界</span><strong class="policy-summary-label">等待 Job / proof</strong><small>审批、Safe 提案或事件均不直接授予资格</small>
        </div>
      </section>
      <nav class="policy-workflow" aria-label="治理流程">
        <a
          v-for="(step, index) in workflowSteps"
          :key="step.id"
          :href="`#${step.id}`"
        >
          <span class="policy-step-number">{{ index + 1 }}</span>
          <span><strong>{{ step.title }}</strong><small>{{ step.description }}</small></span>
        </a>
      </nav>
      <Card
        class="policy-stage-card mb-4"
        title="当前已启用的白名单"
        aria-label="当前已启用的白名单"
      >
        <template #extra>
          <Button
            :disabled="busy || currentMembersLoading"
            @click="refreshCurrentMembers"
          >
            重新核对
          </Button>
        </template>
        <p v-if="!status" class="policy-helper">
          链上策略尚未核实，暂不展示当前名单。
        </p>
        <p v-else-if="!status.enabled" class="policy-helper">
          当前链上未启用白名单；历史工单不作为当前名单展示。
        </p>
        <p
          v-else-if="currentMembersLoading || reviewsLoading"
          class="policy-helper"
          role="status"
        >
          正在核对当前版本、Root 与已执行工单的完整名单…
        </p>
        <Alert
          v-else-if="
            currentMembersUnavailable || reviewsUnavailable || !currentMembers
          "
          type="warning"
          show-icon
          message="当前链上已启用白名单，但无法匹配已核验执行记录中的完整名单。请刷新工单并核对原交易回执；不以待审批候选或历史名单替代。"
        />
        <template v-else>
          <div class="policy-candidate-summary">
            <Tag color="green">v{{ currentMembers.version }} · 当前链上匹配</Tag><strong>{{ currentMembers.members.length }} 个钱包</strong><span>来源工单 #{{ currentMembers.reviewId }}</span>
          </div>
          <div class="policy-root">
            <span>当前 Root</span><code>{{ currentMembers.root }}</code>
          </div>
          <label class="policy-field-label" for="current-member-search">查找钱包<span>支持地址片段，不区分大小写</span></label>
          <Input
            id="current-member-search"
            v-model:value="currentMemberSearch"
            allow-clear
            placeholder="输入钱包地址或片段"
          />
          <pre class="member-list" aria-label="当前白名单钱包地址">{{
            filteredCurrentMembers.join('\n') || '没有匹配的钱包地址'
          }}</pre>
          <p class="policy-helper">
            仅展示当前链上策略对应的已核验名单，不自动覆盖下方候选，也不代表当前钱包已取得
            proof 资格。
          </p>
        </template>
      </Card>
      <Card class="policy-main-card">
        <Alert
          v-if="!policyReadable"
          type="warning"
          show-icon
          :message="
            projectStateKnown
              ? '请先在项目列表创建 V3 项目，等待原交易回执和 Job 扫描后再预览白名单'
              : '项目链上状态尚未核实；白名单预览保持关闭'
          "
        />
        <Button
          v-if="!policyReadable"
          class="mt-2"
          @click="refreshPolicyContext"
        >
          重新核对项目状态
        </Button>
        <Alert
          v-if="!status && policyReadable"
          type="error"
          show-icon
          :message="
            statusFailureMessage || '链上状态未通过独立核验，不能作为资格依据'
          "
        />
        <Alert
          v-if="status?.scanner_state === 'reorg_halted'"
          type="warning"
          show-icon
          message="项目扫描已暂停；以下仅为已确认区块的链上观察，不代表项目已恢复、审批已生效或可以投资。白名单变更仍需独立门禁核验。"
        />
        <Alert
          v-if="!canWritePolicy && canApprovePolicy"
          type="info"
          show-icon
          message="当前为第二位管理员审批入口：请在下方选择待审批工单核对完整名单；无需重新计算预览"
        />
        <p
          v-if="
            policyReadable &&
            !projectCreated &&
            status?.scanner_state !== 'reorg_halted'
          "
          class="policy-helper"
          role="status"
        >
          项目已结束，当前名单仅供查看；不开放白名单变更。
        </p>
        <section v-if="canWritePolicy" id="policy-draft" class="policy-draft">
          <div class="policy-section-heading">
            <div>
              <span class="policy-eyebrow">01 / PREPARE</span>
              <h2>准备完整名单</h2>
              <p>先计算差异与摘要，核对后再创建审批工单。</p>
            </div>
            <Tag>{{ walletCount }} / 512 个地址</Tag>
          </div>
          <div class="policy-mode-row">
            <div>
              <strong>目标模式</strong>
              <p>
                {{
                  enabled
                    ? '启用 / 更新白名单，仅名单内钱包可参与'
                    : '关闭白名单，仅支持 Pending；请保持名单为空'
                }}
              </p>
            </div>
            <Switch
              v-model:checked="enabled"
              checked-children="启用"
              un-checked-children="关闭"
            />
          </div>
          <label class="policy-field-label" for="policy-wallets">完整钱包名单<span>每行一个地址，也支持逗号分隔</span></label>
          <Input.TextArea
            id="policy-wallets"
            class="policy-wallet-input"
            v-model:value="wallets"
            :rows="6"
            placeholder="每行一个钱包地址；不会从旧报名名单自动导入"
          />
          <label class="policy-field-label" for="policy-reason">变更理由<span>供第二位管理员独立审阅</span></label>
          <Input
            id="policy-reason"
            v-model:value="reason"
            class="mt-3"
            :maxlength="500"
            placeholder="说明本次开启、更新或关闭的原因"
          />
          <Button
            v-if="canWritePolicy"
            class="mt-3"
            type="primary"
            :loading="busy"
            :disabled="!canWritePolicy || !projectCreated"
            @click="preview"
          >
            计算候选与名单差异
          </Button>
          <span class="policy-action-note">只读预览，不签名、不创建审批</span>
        </section>
        <Card
          v-if="canWritePolicy && result"
          class="policy-stage-card mt-4"
          title="第 1 步 · 核对只读候选"
        >
          <Alert
            type="info"
            show-icon
            message="这里只计算候选，不产生审批、Safe 提案或钱包资格。请核对完整名单后再提交。"
            class="mb-4"
          />
          <div class="policy-candidate-summary">
            <Tag :color="result.enabled ? 'blue' : 'default'">
              {{ result.enabled ? '启用白名单' : '关闭白名单' }}
            </Tag>
            <strong>v{{ result.old_version }} → v{{ result.target_version }}</strong>
            <span>{{ result.member_count }} 个钱包</span>
          </div>
          <div class="policy-root">
            <span>目标 Root · 请独立核对</span><code>{{ result.root }}</code>
          </div>
          <details class="policy-details">
            <summary>查看链上身份、确认块与候选摘要</summary>
            <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
              <DescriptionsItem label="链 / 项目">
                {{ result.chain_id }} / {{ result.project_id }}
              </DescriptionsItem>
              <DescriptionsItem label="状态 / 版本">
                {{ result.state }} · v{{ result.old_version }} → v{{
                  result.target_version
                }}
              </DescriptionsItem>
              <DescriptionsItem label="目标模式">
                {{ result.enabled ? '启用白名单' : '关闭白名单' }}
              </DescriptionsItem>
              <DescriptionsItem label="钱包数">
                {{ result.member_count }}
              </DescriptionsItem>
              <DescriptionsItem label="V3 Proxy" :span="2">
                <code class="break-all">{{ result.proxy }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="确认区块" :span="2">
                <code>{{ result.block }}</code>
                <code class="break-all">{{ result.block_hash }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="目标 Root" :span="2">
                <code class="break-all">{{ result.root }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="名单摘要" :span="2">
                <code class="break-all">{{
                  result.members_digest || '无'
                }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="快照摘要" :span="2">
                <code class="break-all">{{
                  result.snapshot_digest || '无'
                }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="预览摘要" :span="2">
                <code class="break-all">{{ result.preview_digest }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="理由" :span="2">
                {{ result.reason }}
              </DescriptionsItem>
            </Descriptions>
          </details>
          <div class="mt-4 grid gap-3 md:grid-cols-2">
            <div class="policy-diff policy-diff-added">
              <b>新增 {{ result.added_members.length }}</b>
              <pre class="member-list">{{
                result.added_members.join('\n') || '无'
              }}</pre>
            </div>
            <div class="policy-diff policy-diff-removed">
              <b>移除 {{ result.removed_members.length }}</b>
              <pre class="member-list">{{
                result.removed_members.join('\n') || '无'
              }}</pre>
            </div>
          </div>
          <div class="policy-member-panel mt-4">
            <b>完整规范名单（须逐项核对）</b>
            <pre class="member-list">{{
              result.canonical_members.join('\n') || '空名单'
            }}</pre>
          </div>
          <Button
            class="mt-4"
            type="primary"
            :loading="busy"
            :disabled="mutationUnknown"
            @click="startReview"
          >
            提交候选 · 等待独立二审
          </Button>
        </Card>
        <section id="policy-reviews" class="policy-review-section">
          <div class="policy-section-heading">
            <div>
              <span class="policy-eyebrow">02 / REVIEW</span>
              <h2>双人审批工单</h2>
              <p>选择工单核对完整名单；二审只保存决策，不自动执行 Safe。</p>
            </div>
            <Button :loading="reviewsLoading" @click="loadReviews">
              刷新工单
            </Button>
          </div>
          <Alert
            v-if="reviewsUnavailable"
            type="error"
            show-icon
            message="工单列表暂不可用，请稍后刷新；不要重复提交候选"
          />
          <Alert
            v-if="mutationUnknown"
            type="warning"
            show-icon
            message="上次写请求结果未知：请按摘要、工单 ID 与状态人工核对；本页面不会自动重试，也不会将未找到工单当作写入失败证明"
          />
          <Table
            :columns="reviewColumns"
            :data-source="reviews"
            :loading="reviewsLoading"
            :row-key="(row: V3PolicyReviewRow) => row.review_id"
            size="small"
            class="policy-review-table mt-3"
            :scroll="{ x: 920 }"
          >
            <template #bodyCell="{ column, record }">
              <span
                v-if="column.dataIndex === 'review_id'"
                class="policy-ticket"
                >#{{ record.review_id }}</span>
              <Tag
                v-else-if="column.dataIndex === 'status'"
                :color="
                  record.status === 'pending_second_admin'
                    ? 'gold'
                    : record.status === 'approved_pending_safe'
                      ? 'blue'
                      : record.status === 'observed_not_proof'
                        ? 'green'
                        : 'default'
                "
              >
                {{
                  reviewStatusLabel[
                    record.status as V3PolicyReviewRow['status']
                  ]
                }}
              </Tag>
              <Button
                v-else-if="column.dataIndex === 'action'"
                type="link"
                :disabled="busy"
                @click="inspectReview(record.review_id)"
              >
                {{ reviewID === record.review_id ? '当前工单' : '查看详情' }}
              </Button>
            </template>
          </Table>
          <Card
            v-if="selectedReview && reviewed"
            class="policy-stage-card mt-4"
            title="第 2 步 · 不可变工单详情 / 第二位管理员核对"
          >
            <Alert
              type="info"
              show-icon
              message="请核对全部钱包、目标 Root 和摘要。二审只保存决策；不会签署或执行 Safe。"
              class="mb-4"
            />
            <div class="policy-candidate-summary">
              <Tag color="blue">工单 #{{ reviewID }}</Tag><strong>v{{ reviewed.old_version }} → v{{
                  reviewed.target_version
                }}</strong><span>{{ reviewed.member_count }} 个钱包</span>
            </div>
            <div class="policy-root">
              <span>目标 Root · 请独立核对</span><code>{{ reviewed.root }}</code>
            </div>
            <details class="policy-details">
              <summary>查看不可变工单身份、审批人及全部摘要</summary>
              <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
                <DescriptionsItem label="工单 / 状态">
                  #{{ reviewID }} ·
                  {{ reviewStatusLabel[selectedReview.status] }}
                </DescriptionsItem>
                <DescriptionsItem label="发起人 / 审批人">
                  ID {{ selectedReview.initiator_admin_id }} /
                  {{ selectedReview.approver_admin_id || '待审批' }}
                </DescriptionsItem>
                <DescriptionsItem label="链 / 项目">
                  {{ reviewed.chain_id }} /
                  {{ reviewed.project_id }}
                </DescriptionsItem>
                <DescriptionsItem label="模式 / 版本">
                  {{ reviewed.enabled ? '启用白名单' : '关闭白名单' }} · v{{
                    reviewed.old_version
                  }}
                  → v{{ reviewed.target_version }}
                </DescriptionsItem>
                <DescriptionsItem label="截止时间（UTC）" :span="2">
                  {{ selectedReview.expires_at }}
                </DescriptionsItem>
                <DescriptionsItem label="V3 Proxy" :span="2">
                  <code class="break-all">{{ reviewed.proxy }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="基础块 / 哈希" :span="2">
                  <code>{{ reviewed.block }}</code>
                  <code class="break-all">{{ reviewed.block_hash }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="目标 Root" :span="2">
                  <code class="break-all">{{ reviewed.root }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="完整名单摘要" :span="2">
                  <code class="break-all">{{
                    reviewed.members_digest || '无'
                  }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="快照摘要" :span="2">
                  <code class="break-all">{{
                    reviewed.snapshot_digest || '无'
                  }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="预览摘要" :span="2">
                  <code class="break-all">{{ reviewed.preview_digest }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="审批理由" :span="2">
                  {{ reviewed.reason }}
                </DescriptionsItem>
              </Descriptions>
            </details>
            <div class="mt-4 grid gap-3 md:grid-cols-2">
              <div class="policy-diff policy-diff-added">
                <b>新增 {{ reviewed.added_members.length }}</b>
                <pre class="member-list">{{
                  reviewed.added_members.join('\n') || '无'
                }}</pre>
              </div>
              <div class="policy-diff policy-diff-removed">
                <b>移除 {{ reviewed.removed_members.length }}</b>
                <pre class="member-list">{{
                  reviewed.removed_members.join('\n') || '无'
                }}</pre>
              </div>
            </div>
            <div class="policy-member-panel mt-4">
              <b>完整规范名单（{{ reviewed.member_count }}）</b>
              <pre class="member-list">{{
                reviewed.canonical_members.join('\n') || '空名单'
              }}</pre>
            </div>
            <Alert
              v-if="
                selectedReview.status === 'pending_second_admin' &&
                selectedReview.initiator_admin_id === currentAdminID
              "
              type="info"
              show-icon
              message="发起人不能自批；请另一位有项目审批权限的管理员登录并核对工单"
            />
            <Alert
              v-if="
                selectedReview.status === 'expired' ||
                selectedReview.status === 'approved_expired' ||
                selectedReview.status === 'superseded'
              "
              type="warning"
              show-icon
              :message="
                selectedReview.status === 'superseded'
                  ? `原审批已由工单 #${selectedReview.superseded_by} 接替，不得继续原 Safe 操作`
                  : '审批窗口已过期，请重新预览并创建新工单，不要继续 Safe 操作'
              "
            />
          </Card>
          <div class="policy-actions">
            <Button
              v-if="selectedReview?.status === 'pending_second_admin'"
              :loading="busy"
              :disabled="!canApproveSelected || mutationUnknown"
              type="primary"
              @click="confirmApproval"
            >
              确认二审 · 不执行链上交易
            </Button>
            <Button
              v-if="approvedSelected"
              :loading="busy"
              :disabled="!reviewed || policyMayHaveExecuted"
              @click="inspectSafeHandoff"
            >
              读取已审批 Safe 调用参数
            </Button>
          </div>
          <div id="policy-safe" class="policy-section-heading">
            <div>
              <span class="policy-eyebrow">03 / SAFE HANDOFF</span>
              <h2>Policy Safe 交接</h2>
              <p>仅已批准且未执行的工单可读取交接参数。</p>
            </div>
          </div>
          <p v-if="!approvedSelected" class="policy-empty-hint">
            选择已审批工单后，显示对应的 Safe 交接与回执核验操作。
          </p>
          <Alert
            v-if="approvedSelected && policyMayHaveExecuted"
            type="info"
            show-icon
            class="mt-3"
            message="当前链上版本和 Root 与工单目标一致：执行前 Safe handoff 会拒绝，不能把它当作执行失败。请从 Policy Safe 取同一笔原始链上交易哈希与 Safe 交易哈希，在第 4 步核验；本提示本身不是 proof 资格。"
          />
          <Card
            v-if="
              approvedSelected &&
              !policyMayHaveExecuted &&
              safeHandoff &&
              handoffMatchesReview
            "
            class="policy-stage-card mt-4"
            title="第 3 步 · 已审批 Policy Safe 交接"
          >
            <Alert
              type="warning"
              show-icon
              message="以下数据只来自本工单当前有效的已审批 Safe handoff。请授权的 Policy Safe Owner 独立核对；Admin 审批不是 Safe 签名，也不代表白名单已生效。"
            />
            <h4 class="mb-3 mt-4 font-semibold">
              Safe 合约交易表单 · 逐项填写
            </h4>
            <Descriptions
              bordered
              size="small"
              :column="{ xs: 1, md: 2 }"
              class="mt-4"
            >
              <DescriptionsItem label="网络">
                BNB Smart Chain · chain ID
                {{ safeHandoff.chain_id }}
              </DescriptionsItem>
              <DescriptionsItem label="Safe 操作 / BNB Value">
                {{ safeHandoff.operation }} /
                {{ safeHandoff.value }}
              </DescriptionsItem>
              <DescriptionsItem label="发起交易的 Policy Safe" :span="2">
                <code class="break-all">{{ safeHandoff.governor }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="To：IDO V3 Proxy" :span="2">
                <code class="break-all">{{ safeHandoff.proxy }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="已审批工单 / 摘要" :span="2">
                #{{ safeHandoff.review_id }} ·
                <code class="break-all">{{ safeHandoff.preview_digest }}</code>
              </DescriptionsItem>
              <DescriptionsItem label="合约方法" :span="2">
                <code>setWhitelistPolicy(uint256,uint64,bool,bytes32)</code>
              </DescriptionsItem>
              <DescriptionsItem label="projectId · uint256">
                {{ safeHandoff.project_id }}
              </DescriptionsItem>
              <DescriptionsItem label="expectedVersion · uint64">
                {{ safeHandoff.expected_version }}（旧版本；成功后变为
                {{ safeHandoff.target_version }}）
              </DescriptionsItem>
              <DescriptionsItem label="enabled · bool">
                {{ safeHandoff.enabled ? 'true' : 'false' }}
              </DescriptionsItem>
              <DescriptionsItem label="root · bytes32">
                <code class="break-all">{{ safeHandoff.root }}</code>
              </DescriptionsItem>
            </Descriptions>
            <details class="policy-details">
              <summary>Safe 手动操作指南 · 由授权 Owner 完成</summary>
              <ol class="mt-2 list-decimal space-y-2 pl-5">
                <li>
                  切换到 BNB Smart Chain（56），打开下面的
                  <b>Policy Safe</b>，确认 Safe 地址与表格中的 governor
                  完全一致；不要使用 Owner Safe 或管理员钱包直接调用。
                </li>
                <li>
                  在 Transaction Builder 新建一笔合约调用；To 填上方 V3
                  Proxy，Value 填 0 BNB，Operation 选 CALL（不是
                  DelegateCall）。
                </li>
                <li>
                  导入下方 ABI，选择
                  <code>setWhitelistPolicy</code>，按表格逐一填写四个参数；
                  <strong>expectedVersion 填旧版本</strong>
                  不可填目标版本。钱包地址及摘要不是合约参数，不能替代 root。
                </li>
                <li>
                  提交前将 Builder 编码出的完整 Data 与下面的已审批
                  <code>calldata</code> 逐字节核对；核对 Safe 当前
                  nonce、目标、完整名单、审批摘要及模拟结果。任一项不一致或工单过期即停止。
                </li>
                <li>
                  由 Safe Owner 创建提案，其他 Owner 按 Safe
                  阈值独立确认，授权执行人再执行链上交易。创建提案不等于执行；失败或结果未知时按原
                  Safe 交易哈希核对，切勿盲目重提。
                </li>
                <li>
                  执行并达到确认数后，回到本页填写
                  <b>链上原始执行交易哈希</b> 与不同的
                  <b>Safe 交易哈希</b> 做只读回执核验；仍需等待 Job 扫描及 proof
                  校验才可认为名单生效。
                </li>
              </ol>
            </details>
            <div class="policy-external-links">
              <a
                v-if="safeUrl"
                :href="safeUrl"
                target="_blank"
                rel="noopener noreferrer"
                >打开 Policy Safe ↗</a>
              <a
                v-if="transactionBuilderUrl"
                :href="transactionBuilderUrl"
                target="_blank"
                rel="noopener noreferrer"
                >打开 Transaction Builder ↗</a>
            </div>
            <details class="policy-details">
              <summary>函数 ABI 与精确 calldata · 仅来自已审批 handoff</summary>
              <div class="mt-4">
                <b>导入 Safe Transaction Builder 的函数 ABI</b>
                <pre class="handoff-code">{{ v3PolicySafeAbi }}</pre>
              </div>
              <div class="mt-4">
                <b>精确 calldata（只以已审批 handoff 为准）</b>
                <pre class="handoff-code">{{ safeHandoff.calldata }}</pre>
              </div>
            </details>
            <div class="policy-proposal-panel mt-4">
              <Button :loading="busy" @click="prepareSafeProposal">
                读取当前 Safe nonce / 待签哈希
              </Button>
              <div v-if="safeProposal" class="mt-3 break-all">
                <p>
                  Safe nonce：{{ safeProposal.nonce }} · 待签 Safe 交易哈希：
                  <span class="break-all">{{ safeProposal.safe_tx_hash }}</span>
                </p>
                <Button danger :loading="busy" @click="proposeWithOwner">
                  使用已连接的 Safe Owner 钱包签名并创建提案（不执行）
                </Button>
              </div>
            </div>
          </Card>
          <Card
            v-if="approvedSelected"
            id="policy-receipt"
            class="policy-stage-card mt-4"
            title="第 4 步 · Safe 执行后核验回执"
          >
            <Alert
              type="info"
              show-icon
              message="核验原始链上交易和 Safe 哈希；全部门禁通过后会写入一次不可变执行观察记录，但仍不是 proof 资格。不签名或发送链上交易。"
              class="mb-3"
            />
            <p class="policy-helper">
              只填写同一笔已执行交易的两个不同哈希；提案不等于执行，请勿填反。
            </p>
            <label class="policy-field-label" for="policy-tx-hash">链上原始交易哈希<span>Transaction Hash · 区块浏览器可查</span></label>
            <Input
              id="policy-tx-hash"
              v-model:value="executedTxHash"
              placeholder="链上交易哈希（Transaction Hash，区块浏览器可查）0x..."
              :disabled="busy"
            />
            <label class="policy-field-label" for="policy-safe-hash">Safe 内部交易哈希<span>Safe Tx Hash · 来自同一笔 Safe 交易</span></label>
            <Input
              id="policy-safe-hash"
              v-model:value="executedSafeTxHash"
              class="mt-2"
              placeholder="Safe 内部交易哈希（Safe Tx Hash）0x..."
              :disabled="busy"
            />
            <Button
              class="mt-3"
              :loading="busy"
              :disabled="!reviewID"
              type="primary"
              @click="observe"
            >
              核验并记录外部 Safe 执行回执
            </Button>
          </Card>
        </section>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.policy-workspace {
  max-width: 1440px;
  margin: 0 auto;
  color: var(--policy-text);
}

.policy-hero {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  justify-content: space-between;
  padding: 28px;
  background: linear-gradient(
    120deg,
    var(--policy-accent-bg),
    var(--policy-surface) 70%
  );
  border: 1px solid var(--policy-border);
  border-radius: 16px;
}

.policy-eyebrow {
  font-size: 11px;
  font-weight: 700;
  color: var(--policy-accent);
  letter-spacing: 1.4px;
}

.policy-hero h1 {
  margin: 8px 0;
  font-size: clamp(22px, 3vw, 30px);
  font-weight: 700;
  line-height: 1.35;
  color: var(--policy-text);
}

.policy-subtitle,
.policy-section-heading p,
.policy-helper,
.policy-details p {
  margin: 8px 0 16px;
  line-height: 1.75;
  color: var(--policy-muted);
}

.policy-hero-tags,
.policy-actions,
.policy-external-links,
.policy-candidate-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.policy-summary,
.policy-workflow {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.policy-summary > div {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 20px;
  background: var(--policy-surface);
  border: 1px solid var(--policy-border);
  border-radius: 12px;
}

.policy-summary span,
.policy-summary small,
.policy-workflow small {
  font-size: 12px;
  line-height: 1.6;
  color: var(--policy-muted);
}

.policy-summary strong {
  font-size: 26px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.3;
  color: var(--policy-text);
}

.policy-summary .policy-summary-label {
  font-size: 18px;
}

.policy-workflow {
  gap: 8px;
  margin-bottom: 22px;
}

.policy-workflow a {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 14px 12px;
  color: var(--policy-text);
  border-radius: 10px;
}

.policy-workflow a:hover,
.policy-workflow a:focus-visible {
  background: var(--policy-accent-bg);
}

.policy-workflow strong,
.policy-workflow small {
  display: block;
}

.policy-step-number {
  display: grid;
  flex: 0 0 32px;
  place-items: center;
  height: 32px;
  font-weight: 700;
  color: var(--policy-accent);
  background: var(--policy-surface);
  border: 1px solid var(--policy-border);
  border-radius: 50%;
}

.policy-main-card {
  border-radius: 14px;
}

.policy-main-card :deep(.ant-card-body) {
  padding: 24px;
}

.policy-stage-card :deep(.ant-card-head) {
  background: var(--policy-inset);
}

.policy-stage-card :deep(.ant-card-head-title) {
  white-space: normal;
}

.policy-workspace :deep(.ant-alert) {
  margin-bottom: 14px;
}

.policy-workspace :deep(.ant-descriptions-item-content) {
  overflow-wrap: anywhere;
}

.policy-workspace :deep(.ant-descriptions-item-label) {
  color: var(--policy-muted);
}

.policy-section-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin: 26px 0 16px;
  scroll-margin-top: 96px;
}

.policy-section-heading h2 {
  margin: 5px 0 0;
  font-size: 19px;
  font-weight: 650;
  color: var(--policy-text);
}

.policy-section-heading p {
  margin-bottom: 0;
}

.policy-mode-row {
  display: flex;
  gap: 20px;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  background: var(--policy-inset);
  border: 1px solid var(--policy-border);
  border-radius: 10px;
}

.policy-mode-row p {
  margin: 5px 0 0;
  font-size: 12px;
  color: var(--policy-muted);
}

.policy-field-label {
  display: block;
  margin: 18px 0 8px;
  font-weight: 600;
}

.policy-field-label span {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  font-weight: 400;
  color: var(--policy-muted);
}

.policy-wallet-input {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  line-height: 1.9;
}

.policy-action-note {
  display: inline-block;
  margin: 12px;
  font-size: 12px;
  color: var(--policy-muted);
}

.policy-review-section {
  padding-top: 8px;
}

.policy-review-table :deep(.ant-tag) {
  max-width: 240px;
  white-space: normal;
}

.policy-ticket {
  font-weight: 650;
  color: var(--policy-accent);
}

.policy-actions {
  margin: 20px 0;
}

.policy-candidate-summary {
  margin-bottom: 16px;
}

.policy-candidate-summary strong {
  font-size: 18px;
}

.policy-candidate-summary > span:not(.ant-tag) {
  color: var(--policy-muted);
}

.policy-details {
  padding: 14px 16px;
  margin: 16px 0;
  background: var(--policy-inset);
  border: 1px solid var(--policy-border);
  border-radius: 10px;
}

.policy-details summary {
  font-weight: 600;
  line-height: 1.6;
  color: var(--policy-text);
  cursor: pointer;
}

.policy-details[open] > summary {
  margin-bottom: 16px;
}

.policy-details summary:focus-visible {
  outline: 2px solid var(--policy-accent);
  outline-offset: 4px;
}

.policy-root,
.policy-member-panel,
.policy-proposal-panel {
  padding: 16px;
  background: var(--policy-inset);
  border: 1px solid var(--policy-border);
  border-radius: 10px;
}

.policy-root > span {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--policy-muted);
}

.policy-workspace code {
  font-size: 12px;
  line-height: 1.8;
  color: var(--policy-text);
  overflow-wrap: anywhere;
}

.policy-diff {
  padding: 16px;
  border: 1px solid;
  border-radius: 10px;
}

.policy-diff-added {
  color: var(--policy-positive);
  background: var(--policy-positive-bg);
  border-color: var(--policy-positive-border);
}

.policy-diff-removed {
  color: var(--policy-negative);
  background: var(--policy-negative-bg);
  border-color: var(--policy-negative-border);
}

.policy-empty-hint {
  padding: 24px;
  color: var(--policy-muted);
  text-align: center;
  border: 1px dashed var(--policy-border);
  border-radius: 10px;
}

.policy-external-links {
  margin-top: 16px;
}

.policy-external-links a {
  padding: 8px 12px;
  color: var(--policy-accent);
  border: 1px solid var(--policy-border);
  border-radius: 8px;
}

.member-list,
.handoff-code {
  max-height: 260px;
  padding: 12px;
  margin: 8px 0 0;
  overflow: auto;
  font-size: 12px;
  line-height: 1.8;
  color: var(--policy-text);
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  background: var(--policy-surface);
  border: 1px solid var(--policy-border);
  border-radius: 8px;
}

.policy-draft,
.policy-review-section,
#policy-receipt {
  scroll-margin-top: 96px;
}

@media (max-width: 960px) {
  .policy-summary,
  .policy-workflow {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .policy-hero {
    flex-direction: column;
    padding: 20px;
  }

  .policy-summary {
    grid-template-columns: minmax(0, 1fr);
  }

  .policy-workflow a {
    padding: 10px 0;
  }

  .policy-main-card :deep(.ant-card-body) {
    padding: 16px;
  }

  .policy-stage-card :deep(.ant-card-body) {
    padding: 12px;
  }

  .policy-action-note {
    margin-left: 0;
  }
}
</style>
