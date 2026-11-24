<script setup lang="ts">
import type {
  ChainEventFailure,
  ChainEventFailureStatus,
  ChainEventHealth,
} from '#/api/joincare/chain_event';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Empty,
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
  getChainEventFailure,
  getChainEventHealth,
  listChainEventFailures,
  replayChainEventFailure,
} from '#/api/joincare/chain_event';
import { myPermissions } from '#/api/joincare/role';

import { isFairLaunchEventContractVersion } from '../projects/fair-launch-admin';

const loading = ref(false);
const rows = ref<ChainEventFailure[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const health = ref<ChainEventHealth | null>(null);
const permissionReady = ref(false);
const canRead = ref(false);
const canExecute = ref(false);

const filter = reactive<{ status?: ChainEventFailureStatus }>({
  status: 'dead_letter',
});

const statusOptions: Array<{
  label: string;
  value: ChainEventFailureStatus;
}> = [
  { value: 'dead_letter', label: 'dead_letter · 等待人工处理' },
  { value: 'retrying', label: 'retrying · 等待Job重试' },
  { value: 'resolved', label: 'resolved · 已由canonical事件解决' },
];

const statusColors: Record<ChainEventFailureStatus, string> = {
  dead_letter: 'red',
  retrying: 'orange',
  resolved: 'green',
};

const sync = computed(() => health.value?.sync);
const failureCounts = computed(
  () =>
    health.value?.failure_counts || {
      dead_letter: 0,
      retrying: 0,
      resolved: 0,
    },
);
const syncHealthy = computed(() => Boolean(health.value?.healthy));

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
    const [list, nextHealth] = await Promise.all([
      listChainEventFailures({
        page: page.value,
        page_size: pageSize.value,
        status: filter.status,
      }),
      getChainEventHealth(),
    ]);
    rows.value = list.items || [];
    total.value = list.total || 0;
    health.value = nextHealth;
  } catch (error: any) {
    message.error(error?.message || '链事件运维数据加载失败');
  } finally {
    loading.value = false;
  }
}

function search() {
  page.value = 1;
  void load();
}

function reset() {
  filter.status = 'dead_letter';
  page.value = 1;
  void load();
}

const detailVisible = ref(false);
const detailLoading = ref(false);
const detail = ref<ChainEventFailure | null>(null);

async function openDetail(row: ChainEventFailure) {
  detailVisible.value = true;
  detailLoading.value = true;
  detail.value = null;
  try {
    detail.value = await getChainEventFailure(row.id);
  } catch (error: any) {
    detailVisible.value = false;
    message.error(error?.message || '失败事件详情加载失败');
  } finally {
    detailLoading.value = false;
  }
}

function isReplayable(row: ChainEventFailure) {
  return (
    canExecute.value &&
    !health.value?.active_reorg_incidents &&
    row.admin_replayable &&
    !isFairLaunchEventContractVersion(row.contract_version)
  );
}

const replayVisible = ref(false);
const replayLoading = ref(false);
const replayTarget = ref<ChainEventFailure | null>(null);
const replayReason = ref('');
const replayReasonLength = computed(
  () => [...replayReason.value.trim()].length,
);
const replayReasonValid = computed(
  () => replayReasonLength.value >= 3 && replayReasonLength.value <= 500,
);

function openReplay(row: ChainEventFailure) {
  if (!isReplayable(row)) return;
  replayTarget.value = row;
  replayReason.value = '';
  replayVisible.value = true;
}

async function submitReplay() {
  const target = replayTarget.value;
  if (!target || !replayReasonValid.value) return;
  if (isFairLaunchEventContractVersion(target.contract_version)) {
    replayVisible.value = false;
    replayTarget.value = null;
    message.error('fair_launch_materialization_unavailable');
    return;
  }
  replayLoading.value = true;
  try {
    await replayChainEventFailure(target.id, replayReason.value.trim());
    replayVisible.value = false;
    message.success(`失败事件 #${target.id} 的replay请求已由服务器接受`);
    try {
      const confirmed = await getChainEventFailure(target.id);
      if (confirmed.status === 'retrying') {
        if (detail.value?.id === target.id) detail.value = confirmed;
      } else {
        message.warning(
          `Replay请求已成功，但确认查询返回状态 ${confirmed.status}，请人工复核`,
        );
      }
    } catch {
      message.warning('Replay请求已成功，但详情确认失败；请刷新列表复核状态');
    }
    await load();
  } catch (error: any) {
    message.error(error?.message || 'Replay请求失败，请刷新后重新核对状态');
  } finally {
    replayLoading.value = false;
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
  { title: '事件', dataIndex: 'event', width: 180 },
  { title: '状态', dataIndex: 'status', width: 130 },
  { title: '区块', dataIndex: 'block_number', width: 120 },
  { title: 'Tx Hash', dataIndex: 'tx_hash', width: 220 },
  { title: 'Log', dataIndex: 'log_index', width: 80 },
  { title: '版本', dataIndex: 'contract_version', width: 160 },
  { title: '连续失败', dataIndex: 'consecutive_count', width: 100 },
  { title: 'Replay', dataIndex: 'replay_count', width: 90 },
  { title: '错误', dataIndex: 'error_message', width: 320, ellipsis: true },
  { title: '最近失败', dataIndex: 'last_failed_at', width: 180 },
  { title: '操作', dataIndex: '_action', width: 150, fixed: 'right' as const },
];

onMounted(async () => {
  await loadPermissions();
  if (canRead.value) await load();
});
</script>

<template>
  <Page
    title="IDO · 链事件运维"
    description="MySQL权威游标与失败记录。普通dead-letter只能由Admin请求Job从原链日志重新处理；reconciliation必须人工对账和补偿。"
  >
    <Alert
      v-if="permissionReady && !canRead"
      type="error"
      show-icon
      banner
      message="当前管理员没有 m2_project:read 权限，未请求或展示链事件数据。"
    />

    <Row v-if="canRead" :gutter="12" class="mb-3">
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            title="Job健康状态"
            :value="syncHealthy ? 'OK' : 'ATTENTION'"
            :value-style="{ color: syncHealthy ? '#3f8600' : '#cf1322' }"
          />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic title="游标延迟区块" :value="sync?.lag_blocks || 0" />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            title="Dead letter"
            :value="failureCounts.dead_letter"
            :value-style="{
              color: failureCounts.dead_letter ? '#cf1322' : '#3f8600',
            }"
          />
        </Card>
      </Col>
      <Col :xs="12" :md="6">
        <Card size="small">
          <Statistic
            title="Active Reorg Incident"
            :value="health?.active_reorg_incidents || 0"
            :value-style="{
              color: health?.active_reorg_incidents ? '#cf1322' : '#3f8600',
            }"
          />
        </Card>
      </Col>
    </Row>

    <Alert
      v-if="canRead && health?.active_reorg_incident"
      type="error"
      show-icon
      class="mb-3"
      message="检测到已提交游标的深重组；所有链上对账与事件同步保持关闭，禁止Replay或手工推进游标。"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            Incident #{{ health.active_reorg_incident.id }} ·
            {{ health.active_reorg_incident.detection_code }} · block
            {{ health.active_reorg_incident.cursor_block }} ·
            {{ fmtTime(health.active_reorg_incident.detected_at) }}
          </div>
          <div>
            expected=<code>{{
              health.active_reorg_incident.expected_block_hash
            }}</code>
          </div>
          <div>
            observed=<code>{{
              health.active_reorg_incident.observed_block_hash
            }}</code>
          </div>
          <div>
            P3-B1只采集固定safe-head替代分支证据；完成扫描后incident仍保持active，必须经独立补偿和切换流程才能恢复，不能在本页面关闭。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && health?.active_reorg_recovery"
      type="info"
      show-icon
      class="mb-3"
      :message="`Recovery scan #${health.active_reorg_recovery.id} · ${health.active_reorg_recovery.state}`"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            boundary={{ health.active_reorg_recovery.boundary_block }} ·
            target={{ health.active_reorg_recovery.target_safe_block }} ·
            confirmations={{ health.active_reorg_recovery.confirmation_depth }}
          </div>
          <div>
            batches={{ health.active_reorg_recovery.completed_batch_count }}/{{
              health.active_reorg_recovery.expected_batch_count
            }}
            · logs={{ health.active_reorg_recovery.observed_log_count }}
          </div>
          <div>
            boundary_hash=<code>{{
              health.active_reorg_recovery.boundary_block_hash
            }}</code>
          </div>
          <div>
            target_hash=<code>{{
              health.active_reorg_recovery.target_safe_block_hash
            }}</code>
          </div>
          <div v-if="health.active_reorg_recovery.manifest_digest">
            manifest=<code>{{
              health.active_reorg_recovery.manifest_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_recovery.last_error_code">
            error_code=<code>{{
              health.active_reorg_recovery.last_error_code
            }}</code>
          </div>
          <div>
            本页不暴露raw
            log、lease或worker错误原文，也不提供resolve/retry操作。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && health?.active_reorg_materialization"
      type="warning"
      show-icon
      class="mb-3"
      :message="`Semantic effect preparation #${health.active_reorg_materialization.id} · ${health.active_reorg_materialization.state}`"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            branch={{ health.active_reorg_materialization.next_branch }} ·
            old={{ health.active_reorg_materialization.old_effect_count }}/{{
              health.active_reorg_materialization.expected_old_record_count
            }}
            · replacement={{
              health.active_reorg_materialization.replacement_effect_count
            }}/{{
              health.active_reorg_materialization.expected_replacement_log_count
            }}
            · ignored={{
              health.active_reorg_materialization.ignored_log_count
            }}
          </div>
          <div>
            recovery_manifest=<code>{{
              health.active_reorg_materialization.recovery_manifest_digest
            }}</code>
          </div>
          <div>
            old_root=<code>{{
              health.active_reorg_materialization.old_effect_digest
            }}</code>
          </div>
          <div>
            replacement_root=<code>{{
              health.active_reorg_materialization.replacement_effect_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_materialization.manifest_digest">
            manifest=<code>{{
              health.active_reorg_materialization.manifest_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_materialization.last_error_code">
            error_code=<code>{{
              health.active_reorg_materialization.last_error_code
            }}</code>
          </div>
          <div>
            本阶段只封存old/replacement语义effect证据，不切换当前财务状态，也不解除incident。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && health?.active_reorg_assessment"
      type="error"
      show-icon
      class="mb-3"
      :message="`Reversibility assessment #${health.active_reorg_assessment.id} · ${health.active_reorg_assessment.state} · ${health.active_reorg_assessment.outcome || 'IN PROGRESS'}`"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            dispositions={{
              health.active_reorg_assessment.disposition_count
            }}/{{ health.active_reorg_assessment.expected_effect_count }} ·
            exact={{ health.active_reorg_assessment.exact_count }} ·
            conditional={{ health.active_reorg_assessment.conditional_count }} ·
            blocked={{ health.active_reorg_assessment.blocked_count }} · noop={{
              health.active_reorg_assessment.noop_count
            }}
          </div>
          <div>
            blockers={{ health.active_reorg_assessment.blocker_count }} ·
            switch_authorized=<b>{{
              health.active_reorg_assessment.switch_authorized
            }}</b>
          </div>
          <div>
            disposition_root=<code>{{
              health.active_reorg_assessment.disposition_digest
            }}</code>
          </div>
          <div>
            blocker_root=<code>{{
              health.active_reorg_assessment.blocker_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_assessment.manifest_digest">
            manifest=<code>{{
              health.active_reorg_assessment.manifest_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_assessment.last_error_code">
            error_code=<code>{{
              health.active_reorg_assessment.last_error_code
            }}</code>
          </div>
          <div
            v-for="blocker in health.active_reorg_assessment.blockers || []"
            :key="blocker.id"
            class="rounded border border-red-200 p-1"
          >
            <code>{{ blocker.blocker_code }}</code> · {{ blocker.safe_detail }}
          </div>
          <div>
            本评估仅证明可逆性分类与阻断证据完整；BLOCKED不是切换授权，不修改当前财务、游标、outbox或incident。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && health?.active_reorg_prerequisite"
      type="error"
      show-icon
      class="mb-3"
      :message="`Mandatory blocker declaration #${health.active_reorg_prerequisite.id} · ${health.active_reorg_prerequisite.state} · ${health.active_reorg_prerequisite.outcome || 'IN PROGRESS'}`"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            requirements={{
              health.active_reorg_prerequisite.requirement_count
            }}/{{
              health.active_reorg_prerequisite.expected_requirement_count
            }}
            · unmet={{ health.active_reorg_prerequisite.unmet_count }} ·
            untrusted={{ health.active_reorg_prerequisite.untrusted_count }} ·
            deferred={{ health.active_reorg_prerequisite.deferred_count }}
          </div>
          <div>
            artifacts={{ health.active_reorg_prerequisite.artifact_count }} ·
            satisfied={{ health.active_reorg_prerequisite.satisfied_count }} ·
            candidate_ready=<b>{{
              health.active_reorg_prerequisite.candidate_ready
            }}</b>
          </div>
          <div>
            requirement_root=<code>{{
              health.active_reorg_prerequisite.requirement_digest
            }}</code>
          </div>
          <div>
            artifact_root=<code>{{
              health.active_reorg_prerequisite.artifact_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_prerequisite.event_digest">
            event_root=<code>{{
              health.active_reorg_prerequisite.event_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_prerequisite.manifest_digest">
            manifest=<code>{{
              health.active_reorg_prerequisite.manifest_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_prerequisite.last_error_code">
            error_code=<code>{{
              health.active_reorg_prerequisite.last_error_code
            }}</code>
          </div>
          <div
            v-for="requirement in health.active_reorg_prerequisite
              .requirements || []"
            :key="requirement.id"
            class="rounded border border-red-200 p-1"
          >
            <code>{{ requirement.requirement_code }}</code> ·
            <b>{{ requirement.evidence_status }}</b> ·
            {{ requirement.safe_detail }}
          </div>
          <div>
            B1-A仅封存七项mandatory
            blocker声明，不盘点source/outbox行集；BLOCKED且candidate_ready=false，不创建generation、delta、outbox绑定或切换授权。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && health?.active_reorg_generation"
      type="error"
      show-icon
      class="mb-3"
      :message="`Candidate generation foundation #${health.active_reorg_generation.id} · attempt ${health.active_reorg_generation.attempt_no} · ${health.active_reorg_generation.state}/${health.active_reorg_generation.stage} · ${health.active_reorg_generation.outcome || 'IN PROGRESS'}`"
    >
      <template #description>
        <div class="space-y-1 break-all">
          <div>
            requirements={{
              health.active_reorg_generation.requirement_count
            }}/5 · unmet={{ health.active_reorg_generation.unmet_count }} ·
            verified={{ health.active_reorg_generation.verified_count }} ·
            candidate_ready=<b>{{
              health.active_reorg_generation.candidate_ready
            }}</b>
          </div>
          <div>
            runtime snapshot: senders={{
              health.active_reorg_generation.sender_count
            }}
            · submissions={{
              health.active_reorg_generation.submission_count
            }}
            · payload_bytes={{
              health.active_reorg_generation.submission_payload_bytes
            }}
            · nonces={{ health.active_reorg_generation.nonce_count }}
          </div>
          <div>
            sender_root=<code>{{
              health.active_reorg_generation.sender_root
            }}</code>
          </div>
          <div>
            submission_root=<code>{{
              health.active_reorg_generation.submission_root
            }}</code>
          </div>
          <div>
            nonce_root=<code>{{
              health.active_reorg_generation.nonce_root
            }}</code>
          </div>
          <div>
            requirement_root=<code>{{
              health.active_reorg_generation.requirement_root
            }}</code>
          </div>
          <div v-if="health.active_reorg_generation.manifest_digest">
            manifest=<code>{{
              health.active_reorg_generation.manifest_digest
            }}</code>
          </div>
          <div v-if="health.active_reorg_generation.last_error_code">
            error_code=<code>{{
              health.active_reorg_generation.last_error_code
            }}</code>
          </div>
          <div
            v-for="requirement in health.active_reorg_generation.requirements ||
            []"
            :key="requirement.id"
            class="rounded border border-red-200 p-1"
          >
            <code>{{ requirement.requirement_code }}</code> ·
            <b>{{ requirement.evidence_status }}</b> ·
            {{ requirement.safe_detail }}
          </div>
          <div>
            B1-B0只冻结generation descriptor和完整runtime
            outbox/nonce快照。快照不证明generation归属；boundary、V1
            origin、global config及outbox binding仍未满足，因此不生成candidate
            facts、signed delta、checkpoint或切换授权。
          </div>
        </div>
      </template>
    </Alert>
    <Alert
      v-if="canRead && sync?.last_error_code"
      type="error"
      show-icon
      class="mb-3"
      :message="`链事件同步失败（${sync.last_error_code}），原始错误仅保留在受限运维日志中。`"
    />
    <Alert
      v-else-if="canRead && health && !health.healthy"
      type="warning"
      show-icon
      class="mb-3"
      :message="`Job不健康：stale=${health.stale}，lagging=${health.lagging}（阈值 ${health.max_healthy_lag_blocks}），dead_letter=${failureCounts.dead_letter}，blocked_tx=${health.blocked_chain_transactions}，active_reorg=${health.active_reorg_incidents}`"
    />

    <Card v-if="canRead">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Select
          v-model:value="filter.status"
          :options="statusOptions"
          allow-clear
          placeholder="状态"
          style="width: 280px"
          @change="search"
        />
        <Button type="primary" @click="search">查询</Button>
        <Button @click="reset">重置</Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <Tag :color="sync?.v2_registered ? 'green' : 'default'">
          V2 {{ sync?.v2_registered ? 'registered' : 'not registered' }}
        </Tag>
        <Tag :color="sync?.fair_launch_registered ? 'purple' : 'default'">
          Fair Launch
          {{ sync?.fair_launch_registered ? 'registered' : 'not registered' }}
        </Tag>
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
        :row-key="(row: ChainEventFailure) => row.id"
        :scroll="{ x: 1800 }"
        bordered
        size="small"
      >
        <template #emptyText><Empty description="暂无失败事件" /></template>
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'event'">
            <Tag v-if="record.event === 'reconciliation'" color="magenta">
              reconciliation
            </Tag>
            <code v-else>{{ record.event }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'status'">
            <Tag
              :color="statusColors[record.status as ChainEventFailureStatus]"
            >
              {{ record.status }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'tx_hash'">
            <code :title="record.tx_hash">{{ shortHash(record.tx_hash) }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'last_failed_at'">
            {{ fmtTime(record.last_failed_at) }}
          </template>
          <template v-else-if="column.dataIndex === 'error_message'">
            <span :title="record.error_message">{{
              record.error_message
            }}</span>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button
              size="small"
              type="link"
              @click="openDetail(record as ChainEventFailure)"
            >
              详情
            </Button>
            <Button
              v-if="isReplayable(record as ChainEventFailure)"
              size="small"
              type="link"
              danger
              @click="openReplay(record as ChainEventFailure)"
            >
              请求Replay
            </Button>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="detailVisible"
      :footer="null"
      title="失败事件详情（安全字段）"
      width="920px"
    >
      <div v-if="detailLoading" class="py-8 text-center">加载中…</div>
      <Descriptions v-else-if="detail" :column="2" bordered size="small">
        <DescriptionsItem label="ID">{{ detail.id }}</DescriptionsItem>
        <DescriptionsItem label="事件">{{ detail.event }}</DescriptionsItem>
        <DescriptionsItem label="状态">
          <Tag :color="statusColors[detail.status]">{{ detail.status }}</Tag>
        </DescriptionsItem>
        <DescriptionsItem label="Contract version">
          {{ detail.contract_version }}
        </DescriptionsItem>
        <DescriptionsItem label="Block">
          {{ detail.block_number }}
        </DescriptionsItem>
        <DescriptionsItem label="Log index">
          {{ detail.log_index }}
        </DescriptionsItem>
        <DescriptionsItem label="Block hash" :span="2">
          <code>{{ detail.block_hash }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Tx hash" :span="2">
          <code>{{ detail.tx_hash }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Contract" :span="2">
          <code>{{ detail.contract_address }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="累计重试">
          {{ detail.retry_count }}
        </DescriptionsItem>
        <DescriptionsItem label="连续失败">
          {{ detail.consecutive_count }}
        </DescriptionsItem>
        <DescriptionsItem label="Replay次数">
          {{ detail.replay_count }}
        </DescriptionsItem>
        <DescriptionsItem label="最后Replay时间">
          {{ fmtTime(detail.replay_requested_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="解决时间">
          {{ fmtTime(detail.resolved_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="错误" :span="2">
          {{ detail.error_message }}
        </DescriptionsItem>
        <DescriptionsItem label="首次失败">
          {{ fmtTime(detail.first_failed_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="最近失败">
          {{ fmtTime(detail.last_failed_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="Replay审计历史" :span="2">
          <div v-if="detail.replay_audits?.length" class="space-y-2">
            <div
              v-for="audit in detail.replay_audits"
              :key="audit.id"
              class="rounded border p-2"
            >
              <div>
                <b>#{{ audit.replay_no }}</b> · admin_id={{ audit.admin_id }} ·
                {{ fmtTime(audit.requested_at) }}
              </div>
              <div class="mt-1 whitespace-pre-wrap">{{ audit.reason }}</div>
            </div>
          </div>
          <span v-else>—</span>
        </DescriptionsItem>
      </Descriptions>
    </Modal>

    <Modal
      v-model:open="replayVisible"
      :confirm-loading="replayLoading"
      :ok-button-props="{ danger: true, disabled: !replayReasonValid }"
      cancel-text="返回"
      ok-text="确认请求Replay"
      title="请求Job重新处理原链日志"
      width="700px"
      @ok="submitReplay"
    >
      <Alert
        type="warning"
        show-icon
        class="mb-4"
        message="该操作只把普通dead-letter恢复为retrying。Job仍会从相同canonical链日志重新执行；不会手工写资金账本或推进游标。"
      />
      <Descriptions
        v-if="replayTarget"
        :column="2"
        bordered
        size="small"
        class="mb-4"
      >
        <DescriptionsItem label="Failure">
          #{{ replayTarget.id }}
        </DescriptionsItem>
        <DescriptionsItem label="Event">
          {{ replayTarget.event }}
        </DescriptionsItem>
        <DescriptionsItem label="Block">
          {{ replayTarget.block_number }}
        </DescriptionsItem>
        <DescriptionsItem label="Log">
          {{ replayTarget.log_index }}
        </DescriptionsItem>
      </Descriptions>
      <p class="mb-2 text-sm">
        审计原因（去除首尾空格后 3–500 个Unicode字符）：
      </p>
      <Textarea
        v-model:value="replayReason"
        :rows="5"
        placeholder="说明已核对的canonical链日志、失败原因、重放依据及后续观察责任人。不要填写私钥、签名或敏感凭据。"
      />
      <div
        class="mt-1 text-right text-xs"
        :class="replayReasonValid ? 'text-gray-500' : 'text-red-500'"
      >
        {{ replayReasonLength }} / 500
      </div>
    </Modal>
  </Page>
</template>
