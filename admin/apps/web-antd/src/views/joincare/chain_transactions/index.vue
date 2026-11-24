<script setup lang="ts">
import type {
  ChainTransaction,
  ChainTransactionAction,
  ChainTransactionState,
  ChainTransactionSummary,
} from '#/api/joincare/chain_transaction';

import { computed, h, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Empty,
  InputNumber,
  message,
  Modal,
  Row,
  Select,
  Statistic,
  Table,
  Tag,
  Textarea,
} from 'ant-design-vue';

import {
  cancelChainTransaction,
  continueV3Original,
  getChainTransaction,
  getChainTransactionSummary,
  listChainTransactions,
} from '#/api/joincare/chain_transaction';
import { idoErrorMessage, storedIdoFailure } from '#/api/joincare/ido-errors';
import { myPermissions } from '#/api/joincare/role';

import { isFairLaunchDurableAction } from '../projects/fair-launch-admin';
import { canContinueV3Original } from './v3-original';

const loading = ref(false);
const rows = ref<ChainTransaction[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const summary = ref<ChainTransactionSummary | null>(null);
const permissionReady = ref(false);
const canRead = ref(false);
const canExecute = ref(false);

const filter = reactive<{
  action?: ChainTransactionAction;
  projectId?: number;
  state?: ChainTransactionState;
}>({});

const stateOptions: Array<{
  label: string;
  value: ChainTransactionState;
}> = [
  { value: 'intent', label: 'intent · 待预检' },
  { value: 'ready', label: 'ready · 待签名' },
  { value: 'signed', label: 'signed · 已持久化签名' },
  { value: 'submitted', label: 'submitted · 已广播' },
  { value: 'broadcast_unknown', label: 'broadcast_unknown · 广播未知' },
  { value: 'mined_success', label: 'mined_success · 已上链成功' },
  { value: 'mined_revert', label: 'mined_revert · 链上回退' },
  { value: 'final_success', label: 'final_success · Receipt已确认' },
  { value: 'final_revert', label: 'final_revert · 最终回退' },
  { value: 'applied', label: 'applied · 业务事件已应用' },
  { value: 'cancelled', label: 'cancelled · 自动取消' },
  { value: 'admin_cancelled', label: 'admin_cancelled · 人工取消' },
  { value: 'ambiguous', label: 'ambiguous · 需人工对账' },
];

const actionOptions: Array<{
  label: string;
  value: ChainTransactionAction;
}> = [
  { value: 'ido_v3_create', label: 'V3 Create' },
  { value: 'ido_v3_activate', label: 'V3 Activate' },
  { value: 'ido_v3_end', label: 'V3 End' },
  { value: 'ido_v3_withdraw_platform_fee', label: 'V3 Platform Fee' },
  { value: 'v2_create', label: 'V2 Create' },
  { value: 'v2_activate', label: 'V2 Activate' },
  { value: 'project_end', label: 'Project End' },
  { value: 'fair_launch_create', label: 'Fair Launch Create' },
  { value: 'fair_launch_activate', label: 'Fair Launch Activate' },
  { value: 'fair_launch_end', label: 'Fair Launch End' },
  { value: 'fair_launch_cancel', label: 'Fair Launch Cancel' },
  { value: 'fair_launch_release', label: 'Fair Launch Release' },
  {
    value: 'fair_launch_withdraw_project_proceeds',
    label: 'Fair Launch Project Proceeds',
  },
  {
    value: 'fair_launch_withdraw_platform_fee',
    label: 'Fair Launch Platform Fee',
  },
  {
    value: 'fair_launch_withdraw_failed_project_tokens',
    label: 'Fair Launch Failed Tokens',
  },
];

const stateColors: Record<ChainTransactionState, string> = {
  intent: 'default',
  ready: 'blue',
  signed: 'cyan',
  submitted: 'geekblue',
  broadcast_unknown: 'orange',
  mined_success: 'processing',
  mined_revert: 'volcano',
  final_success: 'green',
  final_revert: 'red',
  applied: 'success',
  cancelled: 'default',
  admin_cancelled: 'purple',
  ambiguous: 'magenta',
};

const actionLabels: Record<ChainTransactionAction, string> = {
  ido_v3_create: 'V3 Create',
  ido_v3_activate: 'V3 Activate',
  ido_v3_end: 'V3 End',
  ido_v3_withdraw_platform_fee: 'V3 Platform Fee',
  fair_launch_activate: 'Fair Launch Activate',
  fair_launch_cancel: 'Fair Launch Cancel',
  fair_launch_create: 'Fair Launch Create',
  fair_launch_end: 'Fair Launch End',
  fair_launch_release: 'Fair Launch Release',
  fair_launch_withdraw_failed_project_tokens: 'Fair Launch Failed Tokens',
  fair_launch_withdraw_platform_fee: 'Fair Launch Platform Fee',
  fair_launch_withdraw_project_proceeds: 'Fair Launch Project Proceeds',
  project_end: 'Project End',
  v2_activate: 'V2 Activate',
  v2_create: 'V2 Create',
};

const stateCounts = computed(() => {
  const counts = new Map<ChainTransactionState, number>();
  for (const item of summary.value?.state_counts || []) {
    counts.set(item.state, item.count);
  }
  return counts;
});
const totalCount = computed(() =>
  [...stateCounts.value.values()].reduce((sum, count) => sum + count, 0),
);
const attentionCount = computed(
  () =>
    (stateCounts.value.get('ambiguous') || 0) +
    (stateCounts.value.get('final_revert') || 0) +
    (summary.value?.stale_active || 0),
);

async function loadPermissions() {
  try {
    const result = await myPermissions();
    canRead.value =
      result.is_super || result.permissions.includes('m2_project:read');
    canExecute.value =
      result.is_super || result.permissions.includes('m2_project:execute');
  } catch {
    canRead.value = false;
    canExecute.value = false;
  } finally {
    permissionReady.value = true;
  }
}

async function load() {
  if (!canRead.value) return;
  loading.value = true;
  try {
    const [list, nextSummary] = await Promise.all([
      listChainTransactions({
        action: filter.action,
        page: page.value,
        page_size: pageSize.value,
        project_id: filter.projectId,
        state: filter.state,
      }),
      getChainTransactionSummary(),
    ]);
    rows.value = list.items || [];
    total.value = list.total || 0;
    summary.value = nextSummary;
  } catch (error: any) {
    message.error(idoErrorMessage(error, '链交易记录读取失败，请刷新核对。'));
  } finally {
    loading.value = false;
  }
}

function search() {
  page.value = 1;
  void load();
}

function reset() {
  filter.action = undefined;
  filter.projectId = undefined;
  filter.state = undefined;
  page.value = 1;
  void load();
}

const detailVisible = ref(false);
const detailLoading = ref(false);
const detail = ref<ChainTransaction | null>(null);

async function openDetail(row: ChainTransaction) {
  detailVisible.value = true;
  detailLoading.value = true;
  detail.value = null;
  try {
    detail.value = await getChainTransaction(row.id);
  } catch (error: any) {
    detailVisible.value = false;
    message.error(idoErrorMessage(error, '交易详情读取失败，请核对原编号。'));
  } finally {
    detailLoading.value = false;
  }
}

function isCancellable(row: ChainTransaction) {
  return (
    canExecute.value &&
    row.admin_cancellable &&
    !isFairLaunchDurableAction(row.action)
  );
}

function openOriginal(row: ChainTransaction) {
  if (!canExecute.value || !canContinueV3Original(row) || !row.tx_hash) return;
  const hash = row.tx_hash;
  let reason = '';
  Modal.confirm({
    title: `原签名首次广播预检 · #${row.id}`,
    content: h('div', [
      h(
        'p',
        '不会创建交易或重新签名。预检通过后仍需再次确认首次广播；已广播或结果不明的原交易禁止重发。',
      ),
      h('code', hash),
      h(Textarea, {
        placeholder: '审计原因（3–500字符）',
        maxlength: 500,
        onChange: (e: Event) => {
          reason = (e.target as HTMLTextAreaElement).value;
        },
      }),
    ]),
    okText: '仅预检',
    async onOk() {
      if ([...reason.trim()].length < 3) throw new Error('请填写审计原因');
      await continueV3Original(row.id, hash, reason.trim(), false);
      Modal.confirm({
        title: `确认首次广播原交易 · #${row.id}`,
        content: h('div', [
          h(
            'p',
            `${actionLabels[row.action]} · nonce ${row.nonce} · gas ${row.gas_limit} · gasPrice ${row.gas_price_raw}`,
          ),
          h('code', hash),
          h(
            'p',
            '服务器将再次核验原签名、当前链上状态、双RPC、nonce槽及权限。只允许首次广播一次，响应不明后只能查询原哈希。',
          ),
        ]),
        okText: '首次广播一次',
        async onOk() {
          try {
            await continueV3Original(row.id, hash, reason.trim(), true);
            message.success(
              '原交易已提交，等待链上确认和Job对账；不是业务完成。',
            );
          } catch (error) {
            message.warning(
              idoErrorMessage(error, '未确认广播结果。不要重复提交，请刷新并按原交易哈希核对。'),
            );
          } finally {
            await load();
          }
        },
      });
    },
  });
}

const cancelVisible = ref(false);
const cancelLoading = ref(false);
const cancelTarget = ref<ChainTransaction | null>(null);
const cancelReason = ref('');
const cancelReasonLength = computed(
  () => [...cancelReason.value.trim()].length,
);
const cancelReasonValid = computed(
  () => cancelReasonLength.value >= 3 && cancelReasonLength.value <= 500,
);

function openCancel(row: ChainTransaction) {
  if (!isCancellable(row)) return;
  cancelTarget.value = row;
  cancelReason.value = '';
  cancelVisible.value = true;
}

async function submitCancel() {
  const target = cancelTarget.value;
  if (!target || !cancelReasonValid.value) return;
  if (isFairLaunchDurableAction(target.action)) {
    cancelVisible.value = false;
    cancelTarget.value = null;
    message.error('当前 Fair Launch 执行入口不可用，保持只读。错误码：FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE');
    return;
  }
  cancelLoading.value = true;
  try {
    await cancelChainTransaction(target.id, cancelReason.value.trim());
    cancelVisible.value = false;
    message.success(`交易 #${target.id} 的人工取消请求已由服务器接受`);
    try {
      const confirmed = await getChainTransaction(target.id);
      if (confirmed.state === 'admin_cancelled') {
        if (detail.value?.id === target.id) detail.value = confirmed;
      } else {
        message.warning(
          `取消请求已成功，但确认查询返回状态 ${confirmed.state}，请人工复核`,
        );
      }
    } catch {
      message.warning('取消请求已成功，但详情确认失败；请刷新列表复核最终状态');
    }
    await load();
  } catch (error: any) {
    message.error(idoErrorMessage(error, '取消请求结果未确认，请刷新原记录核对。'));
  } finally {
    cancelLoading.value = false;
  }
}

function fmtTime(value?: string) {
  if (!value) return '—';
  return value.replace('T', ' ').replace(/\.\d+/, '').replace(/Z$/, ' UTC');
}

function shortHash(value?: string) {
  if (!value) return '—';
  return value.length > 20 ? `${value.slice(0, 10)}…${value.slice(-8)}` : value;
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 80, fixed: 'left' as const },
  { title: '项目', dataIndex: 'project_id', width: 90 },
  { title: '动作', dataIndex: 'action', width: 130 },
  { title: '状态', dataIndex: 'state', width: 170 },
  { title: 'Attempt', dataIndex: 'attempt_no', width: 90 },
  { title: 'Chain', dataIndex: 'chain_id', width: 100 },
  { title: 'Nonce', dataIndex: 'nonce', width: 90 },
  { title: 'Tx Hash', dataIndex: 'tx_hash', width: 220 },
  { title: '广播', dataIndex: 'broadcast_count', width: 80 },
  { title: '最近错误', dataIndex: 'last_error', width: 280, ellipsis: true },
  { title: '创建时间', dataIndex: 'created_at', width: 180 },
  { title: '操作', dataIndex: '_action', width: 150, fixed: 'right' as const },
];

onMounted(async () => {
  await loadPermissions();
  if (canRead.value) await load();
});
</script>

<template>
  <Page
    title="IDO · 链交易运维"
    description="Durable outbox 权威状态。只允许取消尚未签名的 V2 Create / Activate；签名后交易必须按 receipt、业务事件和人工对账流程处理。"
  >
    <Alert
      v-if="permissionReady && !canRead"
      type="error"
      show-icon
      banner
      message="当前管理员没有 m2_project:read 权限，未请求或展示链交易数据。"
    />

    <Row v-if="canRead" :gutter="12" class="mb-3">
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic title="全部记录" :value="totalCount" />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            title="关注信号（可能重叠）"
            :value="attentionCount"
            :value-style="{ color: attentionCount > 0 ? '#cf1322' : '#3f8600' }"
          />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            title="Ambiguous"
            :value="stateCounts.get('ambiguous') || 0"
          />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            :title="`Active 超过 ${summary?.stale_after_seconds || 180}s`"
            :value="summary?.stale_active || 0"
          />
        </Card>
      </Col>
    </Row>

    <Card v-if="canRead">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <InputNumber
          v-model:value="filter.projectId"
          :min="1"
          :precision="0"
          placeholder="project_id"
          style="width: 150px"
          @press-enter="search"
        />
        <Select
          v-model:value="filter.action"
          :options="actionOptions"
          allow-clear
          placeholder="动作"
          style="width: 180px"
          @change="search"
        />
        <Select
          v-model:value="filter.state"
          :options="stateOptions"
          allow-clear
          placeholder="状态"
          style="width: 260px"
          @change="search"
        />
        <Button type="primary" @click="search">查询</Button>
        <Button @click="reset">重置</Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <span class="ml-auto text-sm text-gray-500">
          当前筛选 {{ total }} 条
        </span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (nextPage: number, nextSize: number) => {
            page = nextPage;
            pageSize = nextSize;
            load();
          },
        }"
        :row-key="(row: ChainTransaction) => row.id"
        :scroll="{ x: 1700 }"
        bordered
        size="small"
      >
        <template #emptyText><Empty description="暂无链交易记录" /></template>
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'action'">
            <code>{{
              actionLabels[record.action as ChainTransactionAction]
            }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'state'">
            <Tag :color="stateColors[record.state as ChainTransactionState]">
              {{ record.state }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'nonce'">
            {{ record.nonce ?? '—' }}
          </template>
          <template v-else-if="column.dataIndex === 'tx_hash'">
            <code :title="record.tx_hash">{{ shortHash(record.tx_hash) }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'created_at'">
            {{ fmtTime(record.created_at) }}
          </template>
          <template v-else-if="column.dataIndex === 'last_error'">
            <span :title="storedIdoFailure(record.last_error)">{{
              storedIdoFailure(record.last_error)
            }}</span>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button
              v-if="
                canExecute && canContinueV3Original(record as ChainTransaction)
              "
              size="small"
              type="link"
              @click="openOriginal(record as ChainTransaction)"
              >
原签名预检 / 续跑
</Button>
            <Button
              size="small"
              type="link"
              @click="openDetail(record as ChainTransaction)"
            >
              详情
            </Button>
            <Button
              v-if="isCancellable(record as ChainTransaction)"
              danger
              size="small"
              type="link"
              @click="openCancel(record as ChainTransaction)"
            >
              人工取消
            </Button>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="detailVisible"
      :footer="null"
      title="链交易详情（安全字段）"
      width="900px"
    >
      <div v-if="detailLoading" class="py-8 text-center">加载中…</div>
      <Descriptions v-else-if="detail" :column="2" bordered size="small">
        <DescriptionsItem label="ID">
          {{ detail.id }}
        </DescriptionsItem>
        <DescriptionsItem label="Project ID">
          {{ detail.project_id }}
        </DescriptionsItem>
        <DescriptionsItem label="动作">
          {{ actionLabels[detail.action] }}
        </DescriptionsItem>
        <DescriptionsItem label="状态">
          <Tag :color="stateColors[detail.state]">{{ detail.state }}</Tag>
        </DescriptionsItem>
        <DescriptionsItem label="Chain ID">
          {{ detail.chain_id }}
        </DescriptionsItem>
        <DescriptionsItem label="Attempt">
          {{ detail.attempt_no }}
        </DescriptionsItem>
        <DescriptionsItem label="Sender" :span="2">
          <code>{{ detail.sender_address }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Contract" :span="2">
          <code>{{ detail.contract_address }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Nonce">
          {{ detail.nonce ?? '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Gas limit">
          {{ detail.gas_limit ?? '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Tx hash" :span="2">
          <code>{{ detail.tx_hash || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="广播次数">
          {{ detail.broadcast_count }}
        </DescriptionsItem>
        <DescriptionsItem label="最后广播">
          {{ fmtTime(detail.last_broadcast_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="Receipt status">
          {{ detail.receipt_status ?? '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Receipt block">
          {{ detail.receipt_block_number ?? '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="最后错误" :span="2">
          {{ storedIdoFailure(detail.last_error) }}
        </DescriptionsItem>
        <DescriptionsItem label="取消管理员">
          {{ detail.cancelled_by_admin_id ?? '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="取消时间">
          {{ fmtTime(detail.cancelled_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="取消原因" :span="2">
          {{ detail.cancel_reason || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="创建时间">
          {{ fmtTime(detail.created_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="更新时间">
          {{ fmtTime(detail.updated_at) }}
        </DescriptionsItem>
      </Descriptions>
    </Modal>

    <Modal
      v-model:open="cancelVisible"
      :confirm-loading="cancelLoading"
      :ok-button-props="{ danger: true, disabled: !cancelReasonValid }"
      cancel-text="返回"
      ok-text="确认人工取消"
      title="人工取消未签名链交易"
      width="680px"
      @ok="submitCancel"
    >
      <Alert
        type="error"
        show-icon
        class="mb-4"
        message="该操作不可撤销，并会以 admin_cancelled 状态和管理员身份写入 durable outbox。"
      />
      <Descriptions
        v-if="cancelTarget"
        :column="2"
        bordered
        size="small"
        class="mb-4"
      >
        <DescriptionsItem label="Submission">
          #{{ cancelTarget.id }}
        </DescriptionsItem>
        <DescriptionsItem label="Project">
          #{{ cancelTarget.project_id }}
        </DescriptionsItem>
        <DescriptionsItem label="动作">
          {{ actionLabels[cancelTarget.action] }}
        </DescriptionsItem>
        <DescriptionsItem label="状态">
          {{ cancelTarget.state }}
        </DescriptionsItem>
      </Descriptions>
      <p class="mb-2 text-sm">
        审计原因（去除首尾空格后 3–500 个Unicode字符）：
      </p>
      <Textarea
        v-model:value="cancelReason"
        :rows="5"
        placeholder="说明取消原因、核对依据及后续处理责任人；不要填写私钥、签名或敏感凭据。"
      />
      <div
        class="mt-1 text-right text-xs"
        :class="cancelReasonValid ? 'text-gray-500' : 'text-red-500'"
      >
        {{ cancelReasonLength }} / 500
      </div>
    </Modal>
  </Page>
</template>
