<script setup lang="ts">
import type { CheckInRule } from '#/api/joincare/checkin_rule';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  InputNumber,
  message,
  Modal,
  Popconfirm,
  Switch,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  createCheckInRule,
  deleteCheckInRule,
  listCheckInRules,
  updateCheckInRule,
} from '#/api/joincare/checkin_rule';

const loading = ref(false);
const rows = ref<CheckInRule[]>([]);

async function load() {
  loading.value = true;
  try {
    const res = await listCheckInRules();
    rows.value = res.items || [];
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}

const editVisible = ref(false);
const isCreating = ref(false);
const form = reactive({
  id: 0,
  day_threshold: 1,
  bonus_point: 10,
  is_enable: 1 as 0 | 1,
  sort: 0,
});

function openCreate() {
  isCreating.value = true;
  Object.assign(form, {
    id: 0,
    day_threshold: 1,
    bonus_point: 10,
    is_enable: 1,
    sort: 0,
  });
  editVisible.value = true;
}

function openEdit(row: any) {
  isCreating.value = false;
  Object.assign(form, {
    id: row.id,
    day_threshold: row.day_threshold,
    bonus_point: row.bonus_point,
    is_enable: (row.is_enable ?? 1) as 0 | 1,
    sort: row.sort,
  });
  editVisible.value = true;
}

async function onSave() {
  const body = {
    day_threshold: form.day_threshold,
    bonus_point: form.bonus_point,
    is_enable: form.is_enable,
    sort: form.sort,
  };
  try {
    if (form.id === 0) {
      await createCheckInRule(body);
      message.success('已创建');
    } else {
      await updateCheckInRule(form.id, body);
      message.success('已更新');
    }
    editVisible.value = false;
    load();
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  }
}

async function onDelete(row: any) {
  try {
    await deleteCheckInRule(row.id);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '连续天数门槛', dataIndex: 'day_threshold', width: 160 },
  { title: '当日积分', dataIndex: 'bonus_point', width: 130 },
  { title: '启用', dataIndex: 'is_enable', width: 90 },
  { title: '排序', dataIndex: 'sort', width: 90 },
  { title: '操作', dataIndex: '_action', width: 200, fixed: 'right' as const },
];

onMounted(load);
</script>

<template>
  <Page
    title="签到阶梯奖励"
    description="按 continuousCheckInDays 取 day_threshold ≤ streak 的最大档；bonus_point 是当日实际得分（不叠加）"
  >
    <Alert
      type="info"
      message="规则示例：streak=1 → 取 day_threshold=1（10 pt） / streak=8 → 取 day_threshold=7（30 pt） / streak=30 → 取 day_threshold=30（100 pt）"
      show-icon
      banner
      class="mb-3"
    />
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增阶梯</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ rows.length }} 档</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="false"
        :row-key="(r: CheckInRule) => r.id"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'day_threshold'">
            <Tag color="blue">≥ {{ record.day_threshold }} 天</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'bonus_point'">
            <span class="font-bold text-[#1aa64a]">+{{ record.bonus_point }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'is_enable'">
            <Tag :color="record.is_enable === 1 ? 'green' : 'default'">
              {{ record.is_enable === 1 ? '✓ 启用' : '× 停用' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openEdit(record)">
              编辑
            </Button>
            <Popconfirm title="确认删除该阶梯？" @confirm="onDelete(record)">
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="isCreating ? '新增阶梯' : `编辑阶梯 #${form.id}`"
      width="480px"
      ok-text="保存"
      cancel-text="取消"
      @ok="onSave"
    >
      <div class="space-y-3 py-2">
        <div>
          <label class="mb-1 block text-sm">连续天数门槛 *</label>
          <InputNumber
            v-model:value="form.day_threshold"
            :min="1"
            :max="365"
            style="width: 100%"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm">当日积分 *（不叠加）</label>
          <InputNumber
            v-model:value="form.bonus_point"
            :min="0"
            style="width: 100%"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm">排序</label>
          <InputNumber v-model:value="form.sort" style="width: 100%" />
        </div>
        <div class="flex items-center gap-2">
          <label class="text-sm">启用</label>
          <Switch
            :checked="form.is_enable === 1"
            @update:checked="
              (v: any) => {
                form.is_enable = v ? 1 : 0;
              }
            "
          />
        </div>
      </div>
    </Modal>
  </Page>
</template>
