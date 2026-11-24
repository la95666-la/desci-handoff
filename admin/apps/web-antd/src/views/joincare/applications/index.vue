<script setup lang="ts">
import type { ApplyProject } from '#/api/joincare/application';

import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Descriptions,
  Drawer,
  Input,
  message,
  Modal,
  Pagination,
  Popconfirm,
  Select,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  approveApplication,
  cloneApprovedApplication,
  getApplication,
  listApplications,
  rejectApplication,
} from '#/api/joincare/application';

const loading = ref(false);
const rows = ref<ApplyProject[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref('');
const statusFilter = ref<number | undefined>(undefined);

const statusOptions = [
  { label: '待审核', value: 0 },
  { label: '已通过', value: 1 },
  { label: '已驳回', value: 2 },
];
const STATUS_META: Record<number, { color: string; text: string }> = {
  0: { text: '待审核', color: 'gold' },
  1: { text: '已通过', color: 'green' },
  2: { text: '已驳回', color: 'red' },
};

async function load() {
  loading.value = true;
  try {
    const res = await listApplications({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      status: statusFilter.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}

function onSearch() {
  page.value = 1;
  load();
}

function onPageChange(p: number, ps: number) {
  page.value = p;
  pageSize.value = ps;
  load();
}

// ============ 详情抽屉 ============
const detailVisible = ref(false);
const detail = ref<ApplyProject | null>(null);
const detailLoading = ref(false);

async function openDetail(row: any) {
  detailLoading.value = true;
  detailVisible.value = true;
  try {
    detail.value = await getApplication(row.id);
  } catch (error: any) {
    message.error(error?.message || 'load detail failed');
    detail.value = row;
  } finally {
    detailLoading.value = false;
  }
}

async function onApprove(row: any) {
  try {
    await approveApplication(row.id);
    message.success('已通过');
    load();
    if (detail.value && detail.value.id === row.id) detail.value.status = 1;
  } catch (error: any) {
    message.error(error?.message || 'approve failed');
  }
}

async function onClone(row: ApplyProject) {
  try {
    const result = await cloneApprovedApplication(row.id);
    message.success(
      `已生成待审批申请 #${result.id}；未由原钱包提交，也未自动审批`,
    );
    await load();
  } catch (error: any) {
    message.error(error?.message || '代提交失败');
  }
}

const rejectModalVisible = ref(false);
const rejectTarget = ref<ApplyProject | null>(null);
const rejectReason = ref('');

function openReject(row: any) {
  rejectTarget.value = row;
  rejectReason.value = '';
  rejectModalVisible.value = true;
}

async function confirmReject() {
  if (!rejectTarget.value) return;
  try {
    await rejectApplication(rejectTarget.value.id, rejectReason.value);
    message.success('已驳回');
    rejectModalVisible.value = false;
    load();
    if (detail.value && detail.value.id === rejectTarget.value.id)
      detail.value.status = 2;
  } catch (error: any) {
    message.error(error?.message || 'reject failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '申请人', dataIndex: 'name', width: 110 },
  { title: '项目', dataIndex: 'project_name' },
  { title: '分类', dataIndex: 'project_category', width: 110 },
  { title: '钱包', dataIndex: 'wallet_address', width: 160 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '提交时间', dataIndex: 'created_at', width: 170 },
  { title: '操作', dataIndex: '_action', width: 200, fixed: 'right' as const },
];

function shorten(s: string, head = 6, tail = 4) {
  if (!s) return '-';
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

onMounted(load);
</script>

<template>
  <Page
    title="项目申请管理"
    description="apply_project 表 — Phase 1 单人通过 / 驳回（不含链上 / 4-eye）"
  >
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="keyword"
          placeholder="姓名 / 项目名 / 邮箱 / 钱包"
          style="width: 280px"
          allow-clear
          @press-enter="onSearch"
        />
        <Select
          v-model:value="statusFilter"
          placeholder="所有状态"
          :options="statusOptions"
          style="width: 140px"
          allow-clear
        />
        <Button type="primary" :loading="loading" @click="onSearch">
          搜索
        </Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 条</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="false"
        :row-key="(r: ApplyProject) => r.id"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'wallet_address'">
            {{ shorten(record.wallet_address, 8, 6) }}
          </template>
          <template v-else-if="column.dataIndex === 'status'">
            <Tag :color="STATUS_META[record.status]?.color || 'default'">
              {{ STATUS_META[record.status]?.text || '未知' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openDetail(record)">
              详情
            </Button>
            <Popconfirm
              v-if="record.status === 1"
              title="以管理员身份复制此申请资料，生成新的待审批申请？不会冒充申请人钱包，也不会复制链上参数。"
              @confirm="onClone(record as ApplyProject)"
            >
              <Button size="small" type="link">代提交</Button>
            </Popconfirm>
            <template v-if="record.status === 0">
              <Popconfirm title="确认通过？" @confirm="onApprove(record)">
                <Button size="small" type="link">通过</Button>
              </Popconfirm>
              <Button
                size="small"
                type="link"
                danger
                @click="openReject(record)"
              >
                驳回
              </Button>
            </template>
          </template>
        </template>
      </Table>

      <div class="mt-4 flex justify-end">
        <Pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :show-size-changer="true"
          :page-size-options="['20', '50', '100']"
          show-quick-jumper
          @change="onPageChange"
        />
      </div>
    </Card>

    <Drawer
      v-model:open="detailVisible"
      :title="`项目申请 #${detail?.id ?? ''}`"
      :width="960"
      :loading="detailLoading"
    >
      <template v-if="detail">
        <Descriptions :column="1" bordered size="small">
          <Descriptions.Item label="状态">
            <Tag :color="STATUS_META[detail.status]?.color">
              {{ STATUS_META[detail.status]?.text }}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="申请人">
            {{ detail.name }}
          </Descriptions.Item>
          <Descriptions.Item label="联系方式">
            <div>📧 {{ detail.email || '-' }}</div>
            <div>TG: {{ detail.telegram || '-' }}</div>
            <div>WeChat: {{ detail.wechat || '-' }}</div>
          </Descriptions.Item>
          <Descriptions.Item label="身份/单位">
            {{ detail.identity }} /
            {{ detail.organization }}
          </Descriptions.Item>
          <Descriptions.Item label="所在城市">
            {{ detail.city }}
          </Descriptions.Item>
          <Descriptions.Item label="专业方向">
            {{ detail.major }}
          </Descriptions.Item>
          <Descriptions.Item label="项目名称">
            {{ detail.project_name }}
          </Descriptions.Item>
          <Descriptions.Item label="项目分类">
            {{ detail.project_category }}
          </Descriptions.Item>
          <Descriptions.Item label="项目阶段">
            {{ detail.current_step }}
          </Descriptions.Item>
          <Descriptions.Item label="项目简介">
            {{ detail.project_desc }}
          </Descriptions.Item>
          <Descriptions.Item label="申请缘由">
            <div style="white-space: pre-wrap">{{ detail.apply_reason }}</div>
          </Descriptions.Item>
          <Descriptions.Item label="期望支持">
            {{ detail.support_wanted }}
          </Descriptions.Item>
          <Descriptions.Item label="现有资源">
            {{ detail.possessions }}
          </Descriptions.Item>
          <Descriptions.Item label="项目链接">
            <a :href="detail.project_link" target="_blank">{{
              detail.project_link || '-'
            }}</a>
          </Descriptions.Item>
          <Descriptions.Item label="补充信息">
            <div style="white-space: pre-wrap">
              {{ detail.additional_info }}
            </div>
          </Descriptions.Item>
          <Descriptions.Item label="钱包地址">
            {{ detail.wallet_address }}
          </Descriptions.Item>
          <Descriptions.Item label="提交时间">
            {{ detail.created_at }}
          </Descriptions.Item>
        </Descriptions>
        <div v-if="detail.status === 1" class="mt-4 flex justify-end">
          <Popconfirm
            title="复制此已通过申请，生成新的待审批申请？原钱包并未签署新申请。"
            @confirm="onClone(detail!)"
          >
            <Button>管理员代提交新申请</Button>
          </Popconfirm>
        </div>
        <div v-if="detail.status === 0" class="mt-4 flex gap-2 justify-end">
          <Popconfirm title="确认通过？" @confirm="onApprove(detail)">
            <Button type="primary">通过</Button>
          </Popconfirm>
          <Button danger @click="openReject(detail)">驳回</Button>
        </div>
      </template>
    </Drawer>

    <Modal
      v-model:open="rejectModalVisible"
      title="驳回申请"
      @ok="confirmReject"
      ok-text="确认驳回"
      cancel-text="取消"
    >
      <p>申请 #{{ rejectTarget?.id }} 「{{ rejectTarget?.project_name }}」</p>
      <Input.TextArea
        v-model:value="rejectReason"
        :rows="3"
        placeholder="驳回原因（可选）"
        class="mt-2"
      />
    </Modal>
  </Page>
</template>
