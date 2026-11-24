<script setup lang="ts">
import type { PointFlow } from '#/api/joincare/point_flows';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Table,
  Tag,
  Textarea,
} from 'ant-design-vue';

import { listPointFlows, manualAdjustPoint } from '#/api/joincare/point_flows';

const loading = ref(false);
const rows = ref<PointFlow[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const filter = reactive({
  uid: undefined as number | undefined,
  wallet: '',
  type: undefined as string | undefined,
  min_change: undefined as number | undefined,
  max_change: undefined as number | undefined,
  from: '',
  to: '',
});

const typeOptions = [
  { value: 'CHECK_IN', label: 'CHECK_IN 签到' },
  { value: 'TASK', label: 'TASK 任务' },
  { value: 'INVITE', label: 'INVITE 邀请' },
  { value: 'ADJUST', label: 'ADJUST 手工调分' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listPointFlows({
      uid: filter.uid || undefined,
      wallet: filter.wallet || undefined,
      type: filter.type || undefined,
      min_change: filter.min_change ?? undefined,
      max_change: filter.max_change ?? undefined,
      from: filter.from || undefined,
      to: filter.to || undefined,
      page: page.value,
      page_size: pageSize.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}

// ==== 手工加/扣分 ====
const adjustVisible = ref(false);
const adjustForm = reactive({
  uid: 0 as number,
  delta: 0 as number,
  reason: '',
});
function openAdjust(prefill?: PointFlow) {
  adjustForm.uid = prefill?.uid || 0;
  adjustForm.delta = 0;
  adjustForm.reason = '';
  adjustVisible.value = true;
}
async function submitAdjust() {
  if (!adjustForm.uid) {
    message.error('uid 必填');
    return;
  }
  if (!adjustForm.delta) {
    message.error('delta 必填（正数加分 / 负数扣分）');
    return;
  }
  try {
    const res = await manualAdjustPoint(
      adjustForm.uid,
      adjustForm.delta,
      adjustForm.reason,
    );
    if (res.approval_id) {
      Modal.info({
        title: '已提交 4 眼审批',
        content: `|delta|≥1000 走 4 眼；工单 ID: ${res.approval_id}，另一位 admin 到 "4 眼审批" 页批准后才生效`,
      });
    } else {
      message.success(
        `已发放 ${adjustForm.delta > 0 ? '+' : ''}${adjustForm.delta}`,
      );
    }
    adjustVisible.value = false;
    load();
  } catch (error: any) {
    message.error(error?.message || 'adjust failed');
  }
}

function shortAddr(s: string, head = 6, tail = 4) {
  if (!s) return '-';
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

const columns = [
  { title: '时间', dataIndex: 'created_at', width: 200 },
  { title: 'uid', dataIndex: 'uid', width: 90 },
  { title: '钱包', dataIndex: 'wallet_address', width: 170 },
  { title: 'Twitter', dataIndex: 'twitter_handle', width: 140 },
  { title: '类型', dataIndex: 'type', width: 130 },
  {
    title: '变动',
    dataIndex: 'change_point',
    width: 100,
    align: 'right' as const,
  },
  { title: '余额', dataIndex: 'remain', width: 100, align: 'right' as const },
  { title: '说明', dataIndex: 'desc', ellipsis: true },
  { title: '操作', dataIndex: '_action', width: 110, fixed: 'right' as const },
];

function typeColor(t: string) {
  return (
    { CHECK_IN: 'blue', TASK: 'green', INVITE: 'purple', ADJUST: 'orange' }[
      t
    ] || 'default'
  );
}

onMounted(load);
</script>

<template>
  <Page
    title="任务积分流水"
    description="C 端所有涨/扣分动作（签到 / 任务 / 邀请 / 手工调分）统一走 user_point_flow；|delta|≥1000 手工调分会自动进入 4 眼审批"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="可按 uid / 钱包（LIKE）/ 类型 / 变动范围 / 日期段筛选。手工加分/扣分：右上「+ 手工调分」按钮；|delta|<1000 直发，≥1000 走 4 眼审批"
    />
    <Card>
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Input
          v-model:value="filter.uid"
          placeholder="uid"
          style="width: 100px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Input
          v-model:value="filter.wallet"
          placeholder="钱包（LIKE）"
          style="width: 200px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Select
          v-model:value="filter.type"
          placeholder="类型"
          allow-clear
          style="width: 180px"
          :options="typeOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <InputNumber
          v-model:value="filter.min_change"
          placeholder="≥"
          style="width: 90px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <InputNumber
          v-model:value="filter.max_change"
          placeholder="≤"
          style="width: 90px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <DatePicker
          v-model:value="filter.from"
          placeholder="起"
          value-format="YYYY-MM-DD"
          style="width: 140px"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <DatePicker
          v-model:value="filter.to"
          placeholder="止"
          value-format="YYYY-MM-DD"
          style="width: 140px"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" @click="openAdjust()">+ 手工调分</Button>
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
        size="small"
        bordered
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'type'">
            <Tag :color="typeColor(record.type)">{{ record.type }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'wallet_address'">
            <code :title="record.wallet_address" style="font-size: 12px">{{
              shortAddr(record.wallet_address)
            }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'twitter_handle'">
            {{ record.twitter_handle ? `@${record.twitter_handle}` : '-' }}
          </template>
          <template v-else-if="column.dataIndex === 'change_point'">
            <span
              :style="{
                color: record.change_point >= 0 ? '#1aa64a' : '#e53e3e',
                fontWeight: 600,
              }"
            >
              {{ record.change_point >= 0 ? '+' : '' }}{{ record.change_point }}
            </span>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button
              size="small"
              type="link"
              @click="openAdjust(record as PointFlow)"
            >
              调分
            </Button>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="adjustVisible"
      title="手工加/扣分"
      width="480px"
      ok-text="提交"
      cancel-text="取消"
      @ok="submitAdjust"
    >
      <div class="space-y-3 py-2">
        <div>
          <label class="mb-1 block text-sm">用户 uid *</label>
          <InputNumber
            v-model:value="adjustForm.uid"
            :min="1"
            style="width: 100%"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm">
            变动分值 *（正数加分 / 负数扣分；|delta|≥1000 走 4 眼审批）
          </label>
          <InputNumber v-model:value="adjustForm.delta" style="width: 100%" />
        </div>
        <div>
          <label class="mb-1 block text-sm">理由（会记入积分流水说明）</label>
          <Textarea
            v-model:value="adjustForm.reason"
            :rows="2"
            placeholder="如：作弊惩罚 / 补发漏单 / 活动奖励"
          />
        </div>
        <Alert
          type="warning"
          show-icon
          message="所有手工调分都会落 user_point_flow(type=ADJUST) 并写审计日志；|delta|≥1000 会额外进 4 眼审批工单"
        />
      </div>
    </Modal>
  </Page>
</template>
