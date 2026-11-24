<script setup lang="ts">
import type { Approval } from '#/api/joincare/approval';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';

import {
  Alert,
  Button,
  Card,
  Input,
  message,
  Modal,
  Popconfirm,
  Select,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  approveApproval,
  cancelApproval,
  listApprovalOps,
  listApprovals,
  rejectApproval,
} from '#/api/joincare/approval';
import { idoErrorMessage, storedIdoFailure } from '#/api/joincare/ido-errors';
import { myPermissions } from '#/api/joincare/role';

import {
  FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE,
  isFairLaunchApprovalMutationAvailable,
  isFairLaunchApprovalOperation,
  runApprovalMutation,
} from '../projects/fair-launch-admin';

const userStore = useUserStore();
const loading = ref(false);
const rows = ref<Approval[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const filter = reactive({
  status: 'pending' as string | undefined,
  operation: undefined as string | undefined,
});
const ops = ref<string[]>([]);
const currentAdminId = computed(() => Number(userStore.userInfo?.userId || 0));
const canApproveFairLaunch = ref(false);

function isFairLaunchApproval(row: Approval): boolean {
  return isFairLaunchApprovalOperation(row.operation);
}

function approvalSeparationHint(row: Approval): string {
  if (!isFairLaunchApproval(row)) return '';
  if (row.initiator_id === currentAdminId.value) {
    return '发起人不得自批';
  }
  return 'Backend 将强制四眼审批；高危动作执行人还必须与发起人/审批人两两不同';
}

const statusOptions = [
  { value: 'pending', label: 'pending 待审批' },
  { value: 'executed', label: 'executed 已执行' },
  { value: 'rejected', label: 'rejected 已驳回' },
  { value: 'expired', label: 'expired 已过期' },
  { value: 'failed', label: 'failed 执行失败' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listApprovals({
      status: filter.status || undefined,
      operation: filter.operation || undefined,
      page: page.value,
      page_size: pageSize.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(idoErrorMessage(error, '审批列表读取失败，请刷新核对。'));
  } finally {
    loading.value = false;
  }
}
async function loadOps() {
  try {
    const res = await listApprovalOps();
    ops.value = res.ops || [];
  } catch (error: any) {
    console.warn(error);
  }
}

async function onApprove(row: Approval) {
  try {
    const res = await runApprovalMutation(row.operation, () =>
      approveApproval(row.id),
    );
    if (res.status === 'executed') {
      message.success(`工单 #${row.id} 已批准并执行`);
    } else if (res.status === 'failed') {
      Modal.error({
        title: '批准通过但执行失败',
        content: storedIdoFailure(res.result),
      });
    } else {
      message.info(`状态：${res.status}`);
    }
    load();
  } catch (error: any) {
    message.error(idoErrorMessage(error, '审批结果未确认，请先刷新原工单。'));
  }
}

const rejectVisible = ref(false);
const rejectTarget = ref<Approval | null>(null);
const rejectReason = ref('');
function openReject(row: Approval) {
  if (!isFairLaunchApprovalMutationAvailable(row.operation)) {
    message.error(`当前 Fair Launch 执行入口不可用，保持只读。错误码：${FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE}`);
    return;
  }
  rejectTarget.value = row;
  rejectReason.value = '';
  rejectVisible.value = true;
}
async function onReject() {
  const target = rejectTarget.value;
  if (!target) return;
  try {
    await runApprovalMutation(target.operation, () =>
      rejectApproval(target.id, rejectReason.value),
    );
    message.success('已驳回');
    rejectVisible.value = false;
    load();
  } catch (error: any) {
    message.error(idoErrorMessage(error, '驳回结果未确认，请先刷新原工单。'));
  }
}

async function onCancel(row: Approval) {
  try {
    await runApprovalMutation(row.operation, () => cancelApproval(row.id));
    message.success('已撤单');
    load();
  } catch (error: any) {
    message.error(idoErrorMessage(error, '撤单结果未确认，请先刷新原工单。'));
  }
}

const detailVisible = ref(false);
const detail = ref<Approval | null>(null);
function openDetail(row: any) {
  detail.value = row;
  detailVisible.value = true;
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '操作', dataIndex: 'operation', width: 180 },
  { title: '目标', dataIndex: 'target_kind', width: 140 },
  { title: '发起人', dataIndex: 'initiator_name', width: 140 },
  { title: '审批人', dataIndex: 'approver_name', width: 140 },
  { title: '状态', dataIndex: 'status', width: 130 },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  { title: '过期', dataIndex: 'expires_at', width: 170 },
  { title: '操作', dataIndex: '_action', width: 200, fixed: 'right' as const },
];

function statusColor(s: string) {
  return (
    {
      pending: 'gold',
      executed: 'green',
      approved: 'blue',
      rejected: 'red',
      expired: 'default',
      failed: 'volcano',
    }[s] || 'default'
  );
}

onMounted(() => {
  load();
  loadOps();
  myPermissions()
    .then((result) => {
      canApproveFairLaunch.value =
        result.permissions.includes('m2_project:approve');
    })
    .catch(() => {
      canApproveFairLaunch.value = false;
    });
});
</script>

<template>
  <Page
    title="4 眼审批工单中心"
    description="高风险操作必须发起人 + 审批人四眼分离；Fair Launch 治理、Pause、Release 和资产动作还需 Backend 强制 step-up MFA 与发起/审批/执行三权分离。"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="批准动作会用当时的 payload 重放执行，结果落 result 字段（失败会 status=failed）。审批 TTL 72 小时，过期自动 expired"
    />
    <Card>
      <Alert
        type="warning"
        show-icon
        message="Fair Launch approvals are read-only while materialization is unavailable"
        description="fair_launch_materialization_unavailable：仅可查看详情和 append-only 审计信息；批准、驳回、撤单与第三执行入口全部不可用，不收集 MFA assertion。"
        class="mb-3"
      />
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Select
          v-model:value="filter.status"
          placeholder="状态"
          allow-clear
          style="width: 180px"
          :options="statusOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Select
          v-model:value="filter.operation"
          placeholder="操作码"
          allow-clear
          style="width: 220px"
          :options="ops.map((o) => ({ value: o, label: o }))"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Button :loading="loading" @click="load">刷新</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 条</span>
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
          onChange: (p: number, ps: number) => {
            page = p;
            pageSize = ps;
            load();
          },
        }"
        :row-key="(r: any) => r.id"
        size="middle"
        bordered
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record: rawRecord }">
          <template v-if="column.dataIndex === 'status'">
            <Tag :color="statusColor((rawRecord as Approval).status)">
              {{ (rawRecord as Approval).status }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'operation'">
            <code style="font-size: 12px">{{
              (rawRecord as Approval).operation
            }}</code>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button
              size="small"
              type="link"
              @click="openDetail(rawRecord as Approval)"
            >
              详情
            </Button>
            <template
              v-if="
                (rawRecord as Approval).status === 'pending' &&
                isFairLaunchApprovalMutationAvailable(
                  (rawRecord as Approval).operation,
                )
              "
            >
              <Popconfirm
                :title="
                  approvalSeparationHint(rawRecord as Approval) ||
                  '确认批准并执行？'
                "
                @confirm="onApprove(rawRecord as Approval)"
              >
                <Button
                  size="small"
                  type="link"
                  style="color: #52c41a"
                  :disabled="
                    isFairLaunchApproval(rawRecord as Approval) &&
                    (!canApproveFairLaunch ||
                      (rawRecord as Approval).initiator_id === currentAdminId)
                  "
                >
                  批准
                </Button>
              </Popconfirm>
              <Button
                size="small"
                type="link"
                danger
                @click="openReject(rawRecord as Approval)"
              >
                驳回
              </Button>
              <Popconfirm
                title="确认撤单？（仅发起人本人可操作）"
                @confirm="onCancel(rawRecord as Approval)"
              >
                <Button size="small" type="link">撤单</Button>
              </Popconfirm>
            </template>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="rejectVisible"
      title="驳回工单"
      width="480px"
      ok-text="确认驳回"
      cancel-text="取消"
      ok-type="danger"
      @ok="onReject"
    >
      <div v-if="rejectTarget" class="space-y-3 py-2">
        <div>
          工单：<code>{{ rejectTarget.operation }}</code> #{{ rejectTarget.id }}
        </div>
        <div>
          <label class="mb-1 block text-sm">驳回理由</label>
          <Input
            v-model:value="rejectReason"
            placeholder="必要时给发起人一个说法"
          />
        </div>
      </div>
    </Modal>

    <Modal
      v-model:open="detailVisible"
      title="工单详情"
      width="720px"
      :footer="null"
    >
      <div v-if="detail" class="space-y-3 text-sm">
        <div>
          <b>ID:</b> {{ detail.id }} &nbsp;|&nbsp; <b>操作:</b>
          <code>{{ detail.operation }}</code>
        </div>
        <div><b>目标:</b> {{ detail.target_kind }} #{{ detail.target_id }}</div>
        <div>
          <b>状态:</b>
          <Tag :color="statusColor(detail.status)">{{ detail.status }}</Tag>
        </div>
        <div>
          <b>发起人:</b> {{ detail.initiator_name }} (id={{
            detail.initiator_id
          }})
        </div>
        <div v-if="detail.approver_id">
          <b>审批人:</b> {{ detail.approver_name }} (id={{
            detail.approver_id
          }})
        </div>
        <div v-if="detail.executor_id">
          <b>执行人:</b> {{ detail.executor_name || 'admin' }} (id={{
            detail.executor_id
          }})
        </div>
        <div v-if="detail.execution_domain">
          <b>执行域:</b> <code>{{ detail.execution_domain }}</code>
        </div>
        <div v-if="detail.canonical_digest">
          <b>Canonical digest:</b> <code>{{ detail.canonical_digest }}</code>
        </div>
        <div v-if="detail.durable_submission_id">
          <b>Durable submission:</b> #{{ detail.durable_submission_id }}
        </div>
        <div v-if="isFairLaunchApproval(detail)">
          <b>Step-up MFA:</b>
          <Tag :color="detail.step_up_mfa_verified ? 'green' : 'red'">
            {{
              detail.step_up_mfa_verified
                ? 'verified'
                : 'unverified / unavailable'
            }}
          </Tag>
        </div>
        <div><b>发起理由:</b> {{ detail.reason || '(none)' }}</div>
        <div v-if="detail.reject_reason">
          <b>驳回理由:</b> {{ detail.reject_reason }}
        </div>
        <Alert
          v-if="isFairLaunchApproval(detail)"
          type="warning"
          show-icon
          :message="approvalSeparationHint(detail)"
          description="批准不等于链上完成。Operator durable、Safe、Timelock 和自动 executor 是不同执行域；请在链交易、事件 applied 与 reconciliation 页面继续核对。"
        />
        <div>
          <b>创建 / 审批 / 执行:</b> {{ detail.created_at }} /
          {{ detail.reviewed_at || '-' }} / {{ detail.executed_at || '-' }}
        </div>
        <div><b>过期:</b> {{ detail.expires_at || '(none)' }}</div>
        <div>
          <b>Payload (审计层 mask 后):</b>
          <pre class="audit-output">{{ detail.payload || '(empty)' }}</pre>
        </div>
        <div v-if="detail.result">
          <b>Result:</b>
          <pre class="audit-output">{{ detail.status === 'failed' ? storedIdoFailure(detail.result) : detail.result }}</pre>
        </div>
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.audit-output {
  max-height: 200px;
  padding: 8px;
  overflow-x: auto;
  font-size: 12px;
  color: #d4d4d4;
  background: #1e1e1e;
  border-radius: 6px;
}
</style>
