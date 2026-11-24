<script setup lang="ts">
import type { ProjectListRow } from '#/api/joincare/project_content';
import type { V3SenderReadiness } from '#/api/joincare/project_content';

import { h, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { $t } from '@vben/locales';

import {
  Image as AImage,
  Alert,
  Button,
  Card,
  Empty,
  Input,
  message,
  Modal,
  Select,
  Space,
  Switch,
  Table,
  Tag,
} from 'ant-design-vue';

import { projectErrorMessage as idoErrorMessage } from '#/api/joincare/errors';
import {
  activateProjectOnchain,
  cancelProjectOnchain,
  changeProjectTracking,
  createProjectOnchain,
  deleteProject,
  disableProject,
  endProjectOnchain,
  getIdoPauseState,
  getV3SenderReadiness,
  listProjects,
  pauseIdoV2,
  restoreProject,
  unpauseIdoV2,
  updateProjectListingVisibility,
  withdrawPlatformFee,
} from '#/api/joincare/project_content';
import { myPermissions } from '#/api/joincare/role';
import {
  getRestoreCapability,
  isActivateOnchainAllowed,
  isCancelOnchainAllowed,
  isCreateOnchainAllowed,
  isDeleteAllowed,
  isDisableAllowed,
  isEndOnchainAllowed,
  isMarketingContentEditAllowed,
  isOnchainParamsViewAllowed,
  isPlatformFeeWithdrawable,
  isRestorable,
} from '#/views/joincare/projects/project-predicates';

import FairLaunchReadOnlyPanel from './FairLaunchReadOnlyPanel.vue';
import {
  formatFinancialDecimal,
  fundingContractPairLabel,
  getFundingContractPair,
  isExactFairLaunchPair,
} from './funding-mode';
import { trackingActions, trackingPresentation } from './project-tracking';
import { senderReadinessAllowsSubmit, senderReadinessLabel } from './sender-readiness';

const router = useRouter();
const senderReady = ref<null | V3SenderReadiness>(null);
const senderChecking = ref(false);
let senderPoll: ReturnType<typeof setTimeout> | undefined;
let senderDisposed = false;
async function refreshSenderReadiness() {
  if (senderChecking.value || senderDisposed) return;
  if (senderPoll) clearTimeout(senderPoll);
  senderChecking.value = true;
  try {
    const value = await getV3SenderReadiness();
    if (!senderDisposed) senderReady.value = value;
  } catch {
    senderReady.value = null;
  } finally {
    senderChecking.value = false;
    if (!senderDisposed)
      senderPoll = setTimeout(refreshSenderReadiness, senderReady.value?.ready ? 30_000 : 10_000);
  }
}
async function requireSenderBeforeSubmit(row: ProjectListRow) {
  if (getFundingContractPair(row) !== 'capped_presale_v3') return true;
  if (senderChecking.value) {
    message.warning('发送钱包状态正在刷新，请等待本次只读核验完成 [SENDER_READINESS_CHECKING]');
    return false;
  }
  await refreshSenderReadiness();
  if (!senderReadinessAllowsSubmit(senderReady.value)) {
    message.warning(senderReadinessLabel(senderReady.value, senderChecking.value));
    return false;
  }
  return true;
}

const loading = ref(false);
const canExecute = ref(false);
const canWrite = ref(false);
const canApprove = ref(false);
const executingProjectId = ref<null | string>(null);
const listingVisibilityProjectId = ref<null | string>(null);
const rows = ref<ProjectListRow[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref('');
const status = ref<string | undefined>(undefined);
const fairLaunchPreview = ref<null | ProjectListRow>(null);

const statusOptions = [
  { label: '全部', value: undefined },
  { label: 'Setup', value: 'Setup' },
  { label: 'Set', value: 'Set' },
  { label: 'Pending', value: 'Pending' },
  { label: 'Active', value: 'Active' },
  { label: 'Successful', value: 'Successful' },
  { label: 'Failed', value: 'Failed' },
  { label: 'Canceled', value: 'Canceled' },
  { label: 'Archived', value: 'archived' },
];

const statusColor: Record<string, string> = {
  Setup: 'default',
  Set: 'blue',
  Pending: 'orange',
  Active: 'green',
  Successful: 'purple',
  Failed: 'red',
  Canceled: 'default',
  archived: 'default',
};

function restoreBlockerText(row: ProjectListRow): string {
  const blocker = getRestoreCapability(row).blocker;
  const messages: Record<string, string> = {
    ACTIVE_SUBMISSION: '存在进行中的链上提交，暂不可恢复',
    LEGACY_ID2_NO_SOURCE_STATUS: '缺少归档前状态，无法安全恢复',
    ONCHAIN_LOCKED: '链上生命周期已锁定，无法恢复',
    UNKNOWN: '当前项目不满足安全恢复条件',
    V2_STABLECOIN: 'V2 链上项目不可通过内容管理恢复',
    V3_UNAVAILABLE: 'V3 链上项目不可通过内容管理恢复',
    FAIR_LAUNCH: 'Fair Launch 项目当前为只读非生产原型，不可恢复',
    MODE_CONTRACT_MISMATCH: '募资模式与合约版本未知或不匹配，已安全拒绝恢复',
  };
  return blocker ? (messages[blocker] ?? '当前项目不满足安全恢复条件') : '';
}

async function loadPermissions() {
  try {
    const result = await myPermissions();
    canExecute.value =
      result.is_super || result.permissions.includes('m2_project:execute');
    canWrite.value =
      result.is_super || result.permissions.includes('m2_project:write');
    canApprove.value =
      result.is_super || result.permissions.includes('m2_project:approve');
  } catch {
    canExecute.value = false;
    canWrite.value = false;
    canApprove.value = false;
  }
}

async function load() {
  loading.value = true;
  try {
    const res = await listProjects({
      page: page.value,
      size: pageSize.value,
      keyword: keyword.value || undefined,
      status: status.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(idoErrorMessage(error, '项目列表读取失败，请刷新核对。'));
  } finally {
    loading.value = false;
  }
}

async function setListingVisibility(row: ProjectListRow, checked: boolean) {
  if (!canWrite.value) return;
  listingVisibilityProjectId.value = row.project_id;
  try {
    const result = await updateProjectListingVisibility(
      row.project_id,
      checked,
    );
    row.is_listed = result.is_listed;
    message.success(
      checked
        ? '项目已在首页和 Launchpad 列表显示'
        : '项目已从首页和 Launchpad 列表隐藏；详情 URL 仍可访问',
    );
  } catch (error: any) {
    message.error(idoErrorMessage(error, '展示状态变更未获得成功确认，请刷新项目核对。'));
  } finally {
    listingVisibilityProjectId.value = null;
  }
}

function confirmTracking(row: ProjectListRow, action: string, label: string) {
  let reason = '';
  Modal.confirm({
    title: `${label} · ${row.project_id}`,
    content: h('div', [
      h(
        'p',
        '不改变链上生命周期。低频维护每12小时保底同步，有新资金事件可提前唤醒；退款/领取数据可能延迟。归档不删除历史。',
      ),
      h(Input.TextArea, {
        placeholder: '填写审计原因（3–500字符）',
        maxlength: 500,
        onChange: (e: Event) => {
          reason = (e.target as HTMLTextAreaElement).value;
        },
      }),
    ]),
    async onOk() {
      if ([...reason.trim()].length < 3) {
        message.error('请填写至少3个字符的原因。错误码：TRACKING_REASON_REQUIRED');
        throw new Error('reason_required');
      }
      await changeProjectTracking(
        row.project_id,
        action,
        reason.trim(),
        row.tracking?.revision ?? 0,
      );
      message.success('跟踪设置已更新');
      await load();
    },
  });
}

function editContent(row: ProjectListRow) {
  router.push({
    path: '/joincare/projects/content',
    query: { id: String(row.project_id) },
  });
}
function editUpdates(row: ProjectListRow) {
  router.push({
    path: '/joincare/projects/updates',
    query: { id: String(row.project_id) },
  });
}
function editOnchainParams(row: ProjectListRow) {
  router.push({
    path: `/joincare/projects/${row.project_id}/onchain-params`,
  });
}
function openFairLaunchPreview(row: ProjectListRow) {
  fairLaunchPreview.value = row;
}

function openFairLaunchOperations(row: ProjectListRow) {
  router.push({
    path: `/joincare/projects/${row.project_id}/fair-launch/operations`,
  });
}

function openV3PolicyPreview(row: ProjectListRow) {
  router.push({
    path: `/joincare/projects/${row.project_id}/ido-v3/policy-preview`,
  });
}

function pairTagColor(row: ProjectListRow): string {
  const pair = getFundingContractPair(row);
  if (pair === 'fair_launch_v1') return 'purple';
  if (pair === 'capped_presale_v2') return 'blue';
  if (pair === 'capped_presale_v3') return 'orange';
  if (pair === 'mismatch' || pair === 'unknown') return 'red';
  return 'default';
}

function viewOnSite(row: ProjectListRow) {
  const w = window.open(
    `http://154.26.137.111:8080/launchpad/${row.project_id}`,
    '_blank',
  );
  if (!w) message.warning($t('page.projects.list.popupBlocked'));
}

function confirmDisable(row: ProjectListRow) {
  if (!isDisableAllowed(row)) {
    message.error(
      '当前项目身份不允许停用；Fair Launch 及未知/不匹配项目保持只读。错误码：IDO_DISABLE_NOT_ALLOWED',
    );
    return;
  }
  Modal.confirm({
    title: $t('page.projects.list.confirmDisableTitle'),
    content: $t('page.projects.list.confirmDisableContent', {
      name: row.name_en || String(row.project_id),
    }),
    okText: $t('page.projects.list.disable'),
    okType: 'danger',
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      await disableProject(row.project_id);
      message.success($t('page.projects.list.disableSuccess'));
      await load();
    },
  });
}

function confirmDelete(row: ProjectListRow) {
  if (!isDeleteAllowed(row)) {
    message.error(
      '当前项目身份不允许删除；Fair Launch 及未知/不匹配项目保持只读。错误码：IDO_DELETE_NOT_ALLOWED',
    );
    return;
  }
  Modal.confirm({
    title: $t('page.projects.list.confirmDeleteTitle'),
    content: $t('page.projects.list.confirmDeleteContent', {
      name: row.name_en || String(row.project_id),
    }),
    okText: $t('page.projects.list.delete'),
    okType: 'danger',
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      await deleteProject(row.project_id);
      message.success($t('page.projects.list.deleteSuccess'));
      await load();
    },
  });
}

function confirmRestore(row: ProjectListRow) {
  if (!isRestorable(row)) {
    message.error(`${restoreBlockerText(row) || '当前项目不满足安全恢复条件'}。错误码：IDO_RESTORE_NOT_ALLOWED`);
    return;
  }
  Modal.confirm({
    title: $t('page.projects.list.confirmRestoreTitle'),
    content: $t('page.projects.list.confirmRestoreContent', {
      name: row.name_en || String(row.project_id),
    }),
    okText: $t('page.projects.list.restore'),
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      await restoreProject(row.project_id);
      message.success($t('page.projects.list.restoreSuccess'));
      await load();
    },
  });
}

function submitOnchain(
  row: ProjectListRow,
  action: 'activate' | 'create' | 'end',
) {
  // The shared endpoint dispatches by the persisted contract identity. Never
  // route a V3 project through the V2-only cancel or restoration controls.
  const actions = {
    create: {
      label: 'page.projects.list.createOnchain',
      submitted: 'page.projects.list.createOnchainSubmitted',
      confirm: 'page.projects.list.confirmCreateOnchainTitle',
      submit: createProjectOnchain,
    },
    activate: {
      label: 'page.projects.list.activateOnchain',
      submitted: 'page.projects.list.activateOnchainSubmitted',
      confirm: 'page.projects.list.confirmActivateOnchainTitle',
      submit: activateProjectOnchain,
    },
    end: {
      label: 'page.projects.list.endOnchain',
      submitted: 'page.projects.list.endOnchainSubmitted',
      confirm: 'page.projects.list.confirmEndOnchainTitle',
      submit: endProjectOnchain,
    },
  } as const;
  const selected = actions[action];
  const label = $t(selected.label);
  const submittedLabel = $t(selected.submitted);
  Modal.confirm({
    title: $t(selected.confirm),
    content: $t('page.projects.list.confirmOnchainContent'),
    okText: label,
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      if (!(await requireSenderBeforeSubmit(row))) return;
      executingProjectId.value = row.project_id;
      try {
        const result = await selected.submit(row.project_id);
        message.success(
          result.tx_hash
            ? `${submittedLabel} · ${result.tx_hash}`
            : submittedLabel,
        );
        await load();
      } catch (error) {
        message.error(idoErrorMessage(error, '操作结果未确认，请刷新项目并核对原交易，切勿重复提交。'), 8);
        await load();
      } finally {
        executingProjectId.value = null;
      }
    },
  });
}
function onSearch() {
  page.value = 1;
  load();
}

function confirmWithdrawPlatformFee(row: ProjectListRow) {
  Modal.confirm({
    title: $t('page.projects.list.confirmFeeWithdrawTitle'),
    content: $t('page.projects.list.confirmFeeWithdrawContent', {
      name: row.name_cn || row.name_en,
      id: row.project_id,
    }),
    okText: $t('page.projects.list.withdrawFeeBtn'),
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      if (!(await requireSenderBeforeSubmit(row))) return;
      executingProjectId.value = row.project_id;
      try {
        const result = await withdrawPlatformFee(row.project_id);
        message.success(
          result.tx_hash
            ? `${$t('page.projects.list.feeWithdrawn')} · ${result.tx_hash}`
            : $t('page.projects.list.feeWithdrawn'),
        );
        await load();
      } finally {
        executingProjectId.value = null;
      }
    },
  });
}

function confirmCancelOnchain(row: ProjectListRow) {
  Modal.confirm({
    title: $t('page.projects.list.confirmCancelOnchainTitle'),
    content: $t('page.projects.list.confirmCancelOnchainContent', {
      name: row.name_cn || row.name_en,
      id: row.project_id,
    }),
    okText: $t('page.projects.list.cancelOnchain'),
    okType: 'danger',
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      executingProjectId.value = row.project_id;
      try {
        const result = await cancelProjectOnchain(row.project_id);
        message.success(
          result.tx_hash
            ? `${$t('page.projects.list.cancelOnchainSubmitted')} · ${result.tx_hash}`
            : $t('page.projects.list.cancelOnchainSubmitted'),
        );
        await load();
      } finally {
        executingProjectId.value = null;
      }
    },
  });
}

const pausing = ref(false);
const unpausing = ref(false);
const idoPaused = ref<boolean | null>(null);

async function refreshPauseState() {
  try {
    const res = await getIdoPauseState();
    idoPaused.value = res.paused;
  } catch {
    idoPaused.value = null;
  }
}

function confirmPause() {
  Modal.confirm({
    title: $t('page.projects.list.confirmPauseTitle'),
    content: $t('page.projects.list.confirmPauseContent'),
    okText: $t('page.projects.list.pauseBtn'),
    okType: 'danger',
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      pausing.value = true;
      try {
        const result = await pauseIdoV2();
        message.success(
          result.tx_hash
            ? `${$t('page.projects.list.pauseSubmitted')} · ${result.tx_hash}`
            : $t('page.projects.list.pauseAlready'),
        );
        await refreshPauseState();
      } finally {
        pausing.value = false;
      }
    },
  });
}
function confirmUnpause() {
  Modal.confirm({
    title: $t('page.projects.list.confirmUnpauseTitle'),
    content: $t('page.projects.list.confirmUnpauseContent'),
    okText: $t('page.projects.list.unpauseBtn'),
    cancelText: $t('page.projects.list.cancel'),
    async onOk() {
      unpausing.value = true;
      try {
        const result = await unpauseIdoV2();
        message.success(
          result.tx_hash
            ? `${$t('page.projects.list.unpauseSubmitted')} · ${result.tx_hash}`
            : $t('page.projects.list.unpauseAlready'),
        );
        await refreshPauseState();
      } finally {
        unpausing.value = false;
      }
    },
  });
}
function onReset() {
  keyword.value = '';
  status.value = undefined;
  page.value = 1;
  load();
}
function onPageChange(p: number, s: number) {
  page.value = p;
  pageSize.value = s;
  load();
}

function fmtTime(s: null | string) {
  if (!s) return '—';
  return s
    .replace('T', ' ')
    .replace(/\..+$/, '')
    .replace(/[+-]\d{2}:\d{2}/, '');
}

onMounted(() => Promise.all([load(), loadPermissions(), refreshPauseState(), refreshSenderReadiness()]));
onUnmounted(() => {
  senderDisposed = true;
  if (senderPoll) clearTimeout(senderPoll);
});
</script>

<template>
  <Page
    title="Launchpad · 项目列表"
    description="选择要编辑内容 / 发 Updates 的项目 · project_id 是业务 ID，非表主键"
  >
    <Card>
      <Alert
        class="mb-3"
        :type="senderReady?.ready ? 'success' : 'warning'"
        show-icon
        :message="senderReadinessLabel(senderReady, senderChecking)"
      >
        <template #description>
          <span v-if="senderReady?.evidence_remaining_seconds !== undefined">
            最近读取的证据余量：{{ senderReady.evidence_remaining_seconds }} 秒；
            新操作要求：{{ senderReady.minimum_evidence_runway_seconds ?? 180 }} 秒。
          </span>
          <span v-if="senderReady?.active_submission_id">原交易记录：{{ senderReady.active_submission_id }}。</span>
          <Button size="small" :loading="senderChecking" @click="refreshSenderReadiness">刷新钱包状态</Button>
        </template>
      </Alert>
      <template #title>
        <Space>
          <Input
            v-model:value="keyword"
            placeholder="搜索项目名 / tagline"
            style="width: 260px"
            allow-clear
            @press-enter="onSearch"
          />
          <Select
            v-model:value="status"
            :options="statusOptions"
            placeholder="按状态筛选"
            allow-clear
            style="width: 180px"
            @change="onSearch"
          />
          <Button type="primary" @click="onSearch">搜索</Button>
          <Button @click="onReset">重置</Button>
        </Space>
      </template>
      <template #extra>
        <Space>
          <span style="font-size: 12px; color: var(--ant-color-text-secondary)">
            共 {{ total }} 条
          </span>
          <Button
            v-if="canExecute"
            danger
            :loading="pausing"
            :disabled="idoPaused === true"
            :title="$t('page.projects.list.pauseScopeHint')"
            @click="confirmPause"
          >
            {{ $t('page.projects.list.pauseBtn') }}
          </Button>
          <Button
            v-if="canExecute"
            type="primary"
            :loading="unpausing"
            :disabled="idoPaused === false"
            :title="$t('page.projects.list.pauseScopeHint')"
            @click="confirmUnpause"
          >
            {{ $t('page.projects.list.unpauseBtn') }}
          </Button>
          <Button @click="load">重载</Button>
        </Space>
      </template>

      <Table
        :data-source="rows"
        :loading="loading"
        row-key="id"
        :pagination="{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: onPageChange,
        }"
        :columns="[
          { title: 'ID', dataIndex: 'project_id', width: 70 },
          { title: '封面', dataIndex: 'cover_url', width: 90 },
          { title: '项目名', dataIndex: 'name_en' },
          { title: '分类', dataIndex: 'category', width: 140 },
          { title: '状态', dataIndex: 'status', width: 130 },
          { title: '结清 / 跟踪', dataIndex: 'tracking', width: 270 },
          { title: '公开列表', dataIndex: 'is_listed', width: 130 },
          { title: '募资模式', dataIndex: 'funding_mode', width: 230 },
          { title: '计划开售', dataIndex: 'launch_at', width: 150 },
          { title: '更新时间', dataIndex: 'updated_at', width: 150 },
          { title: '链上提交', dataIndex: 'active_submission', width: 190 },
          { title: '操作', dataIndex: '_action', width: 520, fixed: 'right' },
        ]"
      >
        <template #emptyText>
          <Empty description="暂无项目" />
        </template>

        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'cover_url'">
            <AImage
              v-if="record.cover_url"
              :src="record.cover_url"
              :width="60"
              :height="40"
              :preview="false"
              style="object-fit: cover; border-radius: 4px"
            />
            <span v-else style="color: var(--ant-color-text-quaternary)">
              (无)
            </span>
          </template>

          <template v-else-if="column.dataIndex === 'name_en'">
            <div class="flex flex-col">
              <span style="font-weight: 600">{{ record.name_en }}</span>
              <span
                v-if="record.name_cn && record.name_cn !== record.name_en"
                style="font-size: 11px; color: var(--ant-color-text-secondary)"
              >
                {{ record.name_cn }}
              </span>
              <span
                v-if="record.tagline"
                style="
                  max-width: 480px;
                  margin-top: 2px;
                  font-size: 11px;
                  color: var(--ant-color-text-secondary);
                "
                class="line-clamp-2"
              >
                {{ record.tagline }}
              </span>
            </div>
          </template>

          <template v-else-if="column.dataIndex === 'status'">
            <Tag :color="statusColor[record.status] || 'default'">
              {{ record.status || '—' }}
            </Tag>
          </template>

          <template v-else-if="column.dataIndex === 'is_listed'">
            <Switch
              :checked="record.is_listed !== false"
              :disabled="!canWrite"
              :loading="listingVisibilityProjectId === record.project_id"
              checked-children="显示"
              un-checked-children="隐藏"
              title="仅控制首页和 Launchpad 列表；隐藏后仍可通过项目详情 URL 访问，其他功能不变"
              @change="
                (checked) =>
                  setListingVisibility(
                    record as ProjectListRow,
                    checked === true,
                  )
              "
            />
          </template>

          <template v-else-if="column.dataIndex === 'tracking'">
            <div v-if="record.tracking" class="tracking-cell">
              <div class="tracking-badges">
                <Tag
                  :color="
                    trackingPresentation(record as ProjectListRow)
                      ?.settlementColor
                  "
                  class="tracking-badge"
                >
                  {{
                    trackingPresentation(record as ProjectListRow)?.settlement
                  }}
                </Tag>
                <Tag
                  :color="
                    trackingPresentation(record as ProjectListRow)?.modeColor
                  "
                  class="tracking-badge"
                >
                  {{ trackingPresentation(record as ProjectListRow)?.mode }}
                </Tag>
              </div>
              <div
                class="tracking-reason"
                :title="
                  trackingPresentation(record as ProjectListRow)?.reasonCode
                "
              >
                {{ trackingPresentation(record as ProjectListRow)?.reason }}
              </div>
              <div class="tracking-time">
                最近核验：{{ record.tracking.last_scan_at || '尚未核验' }}
              </div>
              <div v-if="record.tracking.next_scan_at" class="tracking-time">
                下次（UTC）：{{ record.tracking.next_scan_at }}
              </div>
              <Space v-if="canExecute" wrap class="tracking-actions">
                <Button
                  v-for="item in trackingActions(record as ProjectListRow)"
                  :key="item.action"
                  size="small"
                  :type="item.action === 'sync' ? 'primary' : 'default'"
                  :class="`tracking-action tracking-action--${item.action}`"
                  @click="
                    confirmTracking(
                      record as ProjectListRow,
                      item.action,
                      item.label,
                    )
                  "
                >
                  {{ item.label }}
                </Button>
              </Space>
            </div>
            <span v-else>—</span>
          </template>

          <template v-else-if="column.dataIndex === 'funding_mode'">
            <div class="flex flex-col gap-1">
              <Tag :color="pairTagColor(record as ProjectListRow)">
                {{
                  fundingContractPairLabel(
                    getFundingContractPair(record as ProjectListRow),
                  )
                }}
              </Tag>
              <template v-if="isExactFairLaunchPair(record as ProjectListRow)">
                <span class="mode-summary">
                  Sale:
                  {{ formatFinancialDecimal(record.sale_token_amount) }} · Soft
                  Cap: {{ formatFinancialDecimal(record.soft_cap) }} · 无 Hard
                  Cap
                </span>
              </template>
            </div>
          </template>

          <template v-else-if="column.dataIndex === 'launch_at'">
            <span>{{ fmtTime(record.launch_at) }}</span>
          </template>

          <template v-else-if="column.dataIndex === 'updated_at'">
            <span>{{ fmtTime(record.updated_at) }}</span>
          </template>

          <template v-else-if="column.dataIndex === 'active_submission'">
            <div v-if="record.active_submission" class="flex flex-col gap-1">
              <Tag color="blue">
                {{ record.active_submission.action }} ·
                {{ record.active_submission.state }}
              </Tag>
              <span
                v-if="record.active_submission.tx_hash"
                class="font-mono text-xs"
                :title="record.active_submission.tx_hash"
              >
                {{ record.active_submission.tx_hash.slice(0, 10) }}…
              </span>
            </div>
            <span v-else>—</span>
          </template>

          <template v-else-if="column.dataIndex === '_action'">
            <Space size="small">
              <Button
                v-if="
                  canExecute && isCreateOnchainAllowed(record as ProjectListRow)
                "
                type="primary"
                size="small"
                :loading="executingProjectId === record.project_id"
                @click="submitOnchain(record as ProjectListRow, 'create')"
              >
                {{ $t('page.projects.list.createOnchain') }}
              </Button>
              <Button
                v-if="
                  canExecute &&
                  isActivateOnchainAllowed(record as ProjectListRow)
                "
                type="primary"
                size="small"
                :loading="executingProjectId === record.project_id"
                @click="submitOnchain(record as ProjectListRow, 'activate')"
              >
                {{ $t('page.projects.list.activateOnchain') }}
              </Button>
              <Button
                v-if="
                  canExecute && isEndOnchainAllowed(record as ProjectListRow)
                "
                type="primary"
                danger
                size="small"
                :loading="executingProjectId === record.project_id"
                @click="submitOnchain(record as ProjectListRow, 'end')"
              >
                {{ $t('page.projects.list.endOnchain') }}
              </Button>
              <Button
                v-if="
                  canExecute && isCancelOnchainAllowed(record as ProjectListRow)
                "
                type="primary"
                danger
                size="small"
                :loading="executingProjectId === record.project_id"
                @click="confirmCancelOnchain(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.cancelOnchain') }}
              </Button>
              <Button
                v-if="
                  canExecute &&
                  isPlatformFeeWithdrawable(record as ProjectListRow)
                "
                type="primary"
                size="small"
                :loading="executingProjectId === record.project_id"
                @click="confirmWithdrawPlatformFee(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.withdrawFeeBtn') }}
              </Button>
              <Button
                v-if="isExactFairLaunchPair(record as ProjectListRow)"
                size="small"
                @click="openFairLaunchPreview(record as ProjectListRow)"
              >
                Fair 摘要
              </Button>
              <Button
                v-if="
                  (canWrite || canApprove) &&
                  getFundingContractPair(record as ProjectListRow) ===
                    'capped_presale_v3'
                "
                size="small"
                :title="
                  canWrite
                    ? record.onchain_locked
                      ? '预览、提交及查看白名单工单；预览不等于 Safe 执行或钱包资格'
                      : '项目尚未创建上链；可查看工单，创建预览须等待原交易回执和 Job 扫描'
                    : '查看白名单四眼审批工单；批准仍须项目读取和审批权限'
                "
                @click="openV3PolicyPreview(record as ProjectListRow)"
              >
                V3 名单治理/审批
              </Button>
              <Button
                v-if="isExactFairLaunchPair(record as ProjectListRow)"
                size="small"
                type="primary"
                ghost
                title="所有动作由 Backend capability 驱动；无 capability 时全部禁用"
                @click="openFairLaunchOperations(record as ProjectListRow)"
              >
                Fair 控制台
              </Button>
              <Button
                type="primary"
                size="small"
                :disabled="
                  !isMarketingContentEditAllowed(record as ProjectListRow)
                "
                title="仅编辑营销展示内容，不包含链上参数或生命周期动作"
                @click="editContent(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.editContent') }}
              </Button>
              <Button
                size="small"
                :disabled="
                  !isOnchainParamsViewAllowed(record as ProjectListRow)
                "
                :title="
                  record.onchain_locked &&
                  isOnchainParamsViewAllowed(record as ProjectListRow)
                    ? '查看已保存的链上参数（已锁定，不可修改）'
                    : !isOnchainParamsViewAllowed(record as ProjectListRow)
                      ? '仅 Capped Presale V2/V3 项目可使用此入口；Fair Launch 与未知身份另行查看'
                      : 'Set 阶段链上参数（四眼审批）'
                "
                @click="editOnchainParams(record as ProjectListRow)"
              >
                链上参数
              </Button>
              <Button
                size="small"
                :disabled="
                  !isMarketingContentEditAllowed(record as ProjectListRow)
                "
                title="仅编辑项目 Updates，不包含链上参数或生命周期动作"
                @click="editUpdates(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.editUpdates') }}
              </Button>
              <Button
                size="small"
                @click="viewOnSite(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.preview') }}
              </Button>
              <Button
                v-if="canWrite && record.status === 'archived'"
                type="primary"
                size="small"
                :disabled="!isRestorable(record as ProjectListRow)"
                :title="restoreBlockerText(record as ProjectListRow)"
                @click="confirmRestore(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.restore') }}
              </Button>
              <Button
                danger
                size="small"
                :disabled="!isDisableAllowed(record as ProjectListRow)"
                title="仅未分配链身份的历史内容项目可停用；Fair Launch 保持只读"
                @click="confirmDisable(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.disable') }}
              </Button>
              <Button
                danger
                size="small"
                :disabled="!isDeleteAllowed(record as ProjectListRow)"
                title="仅未分配链身份的历史内容项目可删除；Fair Launch 保持只读"
                @click="confirmDelete(record as ProjectListRow)"
              >
                {{ $t('page.projects.list.delete') }}
              </Button>
            </Space>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      :open="fairLaunchPreview !== null"
      title="Fair Launch · 只读非生产预览"
      :footer="null"
      width="720px"
      @cancel="fairLaunchPreview = null"
    >
      <FairLaunchReadOnlyPanel
        v-if="fairLaunchPreview"
        :project="fairLaunchPreview"
      />
    </Modal>
  </Page>
</template>

<style scoped>
.tracking-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 220px;
}

.tracking-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tracking-badge {
  margin-inline-end: 0;
  font-weight: 600;
}

.tracking-reason {
  font-size: 12px;
  font-weight: 500;
  color: var(--ant-color-text);
  overflow-wrap: anywhere;
}

.tracking-time {
  font-size: 11px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
}

.tracking-actions {
  padding-top: 7px;
  margin-top: 2px;
  border-top: 1px solid var(--ant-color-border-secondary);
}

.tracking-action {
  font-size: 12px;
}

.tracking-action--maintenance {
  color: var(--ant-color-warning-text);
  background: var(--ant-color-warning-bg);
  border-color: var(--ant-color-warning-border);
}

.tracking-action--normal {
  color: var(--ant-color-info-text);
  background: var(--ant-color-info-bg);
  border-color: var(--ant-color-info-border);
}

.tracking-action--archive {
  color: var(--ant-color-text-secondary);
  border-style: dashed;
}

.mode-summary {
  max-width: 210px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  color: var(--ant-color-text-secondary);
  overflow-wrap: anywhere;
}

.line-clamp-2 {
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
</style>
