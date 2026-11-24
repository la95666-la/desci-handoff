<script setup lang="ts">
import type { JoinCareUser } from '#/api/joincare/user';

import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Avatar,
  Button,
  Card,
  Input,
  message,
  Pagination,
  Space,
  Table,
  Tag,
} from 'ant-design-vue';

import { listJoinCareUsers } from '#/api/joincare/user';

import UserDetailDrawer from './UserDetailDrawer.vue';

const detailOpen = ref(false);
const detailUid = ref(0);
function openDetail(uid: number) {
  detailUid.value = uid;
  detailOpen.value = true;
}

const loading = ref(false);
const rows = ref<JoinCareUser[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref('');
const sort = ref<'id_asc' | 'id_desc' | 'point_asc' | 'point_desc'>(
  'point_desc',
);

async function load() {
  loading.value = true;
  try {
    const res = await listJoinCareUsers({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      sort: sort.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load users failed');
  } finally {
    loading.value = false;
  }
}

function onTableChange(_pag: any, _filt: any, sorter: any) {
  if (sorter && sorter.field === 'point') {
    sort.value = sorter.order === 'ascend' ? 'point_asc' : 'point_desc';
  } else if (sorter && sorter.field === 'id') {
    sort.value = sorter.order === 'ascend' ? 'id_asc' : 'id_desc';
  }
  page.value = 1;
  load();
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

function shorten(s: string, head = 6, tail = 4) {
  if (!s) return '';
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

const columns = [
  {
    title: '排名',
    dataIndex: 'rank',
    width: 90,
  },
  {
    title: 'ID',
    dataIndex: 'id',
    width: 80,
    sorter: true,
  },
  {
    title: '积分',
    dataIndex: 'point',
    width: 130,
    sorter: true,
    defaultSortOrder: 'descend' as const,
  },
  {
    title: '钱包地址',
    dataIndex: 'wallet_address',
    customRender: ({ text }: any) => shorten(text, 8, 6),
    width: 160,
  },
  {
    title: 'Twitter',
    dataIndex: 'twitter_handle',
    customRender: ({ record }: any) => {
      if (!record.twitter_handle) return '-';
      return `@${record.twitter_handle} (${record.twitter_display_name || ''})`;
    },
  },
  {
    title: 'Telegram',
    dataIndex: 'telegram_username',
    customRender: ({ text }: any) => text || '-',
  },
  {
    title: '类型',
    dataIndex: 'type',
    width: 90,
    customRender: ({ text }: any) => text || '-',
  },
  { title: '上次登录', dataIndex: 'last_login_at', width: 170 },
  {
    title: '登录 IP',
    dataIndex: 'last_login_ip',
    width: 140,
    customRender: ({ text }: any) => text || '-',
  },
  { title: '注册时间', dataIndex: 'created_at', width: 170 },
  { title: '操作', dataIndex: '_action', width: 100, fixed: 'right' as const },
];

onMounted(load);
</script>

<template>
  <Page
    title="用户管理"
    description="JoinCare C 端注册用户列表（Phase 1 只读）"
  >
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="keyword"
          placeholder="钱包地址 / Twitter / TG 用户名"
          style="width: 320px"
          allow-clear
          @press-enter="onSearch"
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
        :row-key="(r: any) => r.id"
        size="middle"
        bordered
        :scroll="{ x: 1500 }"
        @change="onTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'rank'">
            <Tag
              v-if="record.rank && record.rank <= 3"
              :color="['gold', 'default', 'volcano'][record.rank - 1]"
              style="font-weight: 700"
            >
              #{{ record.rank }}
            </Tag>
            <span v-else-if="record.rank">#{{ record.rank }}</span>
            <span v-else class="text-gray-400">-</span>
          </template>
          <template v-else-if="column.dataIndex === 'point'">
            <span
              :style="{
                color: record.point > 0 ? '#1aa64a' : undefined,
                fontWeight: record.point > 0 ? 700 : 400,
                fontSize: '15px',
              }"
              >{{ record.point.toLocaleString() }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'twitter_handle'">
            <Space>
              <Avatar
                v-if="record.twitter_avatar"
                :src="record.twitter_avatar"
                size="small"
              />
              <span v-if="record.twitter_handle">
                @{{ record.twitter_handle }}
                <Tag v-if="record.twitter_display_name" color="blue">{{
                  record.twitter_display_name
                }}</Tag>
              </span>
              <span v-else>-</span>
            </Space>
          </template>
          <template v-else-if="column.dataIndex === 'type'">
            <Tag v-if="record.type === 'backer'" color="green">Backer</Tag>
            <Tag v-else-if="record.type === 'creator'" color="purple">
              Creator
            </Tag>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openDetail(record.id)">
              详情
            </Button>
          </template>
        </template>
      </Table>

      <UserDetailDrawer v-model:open="detailOpen" :uid="detailUid" />

      <div class="mt-4 flex justify-end">
        <Pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :show-size-changer="true"
          :page-size-options="['10', '20', '50', '100']"
          show-quick-jumper
          @change="onPageChange"
        />
      </div>
    </Card>
  </Page>
</template>
