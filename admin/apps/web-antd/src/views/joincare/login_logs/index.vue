<script setup lang="ts">
import type { LoginLog } from '#/api/joincare/login_logs';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  message,
  Select,
  Table,
  Tag,
} from 'ant-design-vue';

import { listAllLoginLogs } from '#/api/joincare/login_logs';

const loading = ref(false);
const rows = ref<LoginLog[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const filter = reactive({
  uid: undefined as number | undefined,
  ip: '',
  wallet: '',
  login_type: undefined as string | undefined,
  from: '',
  to: '',
});

const loginTypeOptions = [
  { value: 'wallet', label: 'wallet 钱包登录' },
  { value: 'auto', label: 'auto 自动登录（token 保持）' },
  { value: 'register', label: 'register 注册即登录' },
  { value: 'twitter', label: 'twitter X 登录' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listAllLoginLogs({
      uid: filter.uid || undefined,
      ip: filter.ip || undefined,
      wallet: filter.wallet || undefined,
      login_type: filter.login_type || undefined,
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

function shortAddr(s: string, head = 6, tail = 4) {
  if (!s) return '-';
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 90 },
  { title: '时间', dataIndex: 'logged_in_at', width: 200 },
  { title: 'uid', dataIndex: 'uid', width: 90 },
  { title: '钱包地址', dataIndex: 'wallet_address', width: 200 },
  { title: '方式', dataIndex: 'login_type', width: 120 },
  { title: 'IP', dataIndex: 'ip', width: 150 },
  { title: 'User Agent', dataIndex: 'user_agent', ellipsis: true },
];

function loginTypeColor(t: string) {
  return (
    { wallet: 'green', auto: 'default', register: 'blue', twitter: 'cyan' }[
      t
    ] || 'default'
  );
}

onMounted(load);
</script>

<template>
  <Page
    title="用户登录记录"
    description="C 端用户每次 Login / Register / AutoLogin 都会追加一行；异步写入不阻塞登录响应"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="每条记录含 IP + UA + 登录方式；可按 uid / IP / 时间段 / 方式筛选。同一 uid 详细看单人时间线，去用户管理 → 详情 → 登录记录"
    />
    <Card>
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Input
          v-model:value="filter.uid"
          placeholder="uid"
          style="width: 120px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Input
          v-model:value="filter.ip"
          placeholder="IP"
          style="width: 160px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Input
          v-model:value="filter.wallet"
          placeholder="钱包地址（LIKE）"
          style="width: 220px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Select
          v-model:value="filter.login_type"
          placeholder="登录方式"
          allow-clear
          style="width: 200px"
          :options="loginTypeOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <DatePicker
          v-model:value="filter.from"
          placeholder="起 (YYYY-MM-DD)"
          value-format="YYYY-MM-DD"
          style="width: 160px"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <DatePicker
          v-model:value="filter.to"
          placeholder="止 (YYYY-MM-DD)"
          value-format="YYYY-MM-DD"
          style="width: 160px"
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
        size="small"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'login_type'">
            <Tag :color="loginTypeColor(record.login_type)">
              {{ record.login_type }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'wallet_address'">
            <span :title="record.wallet_address">
              <code style="font-size: 12px">{{
                shortAddr(record.wallet_address)
              }}</code>
            </span>
          </template>
        </template>
      </Table>
    </Card>
  </Page>
</template>
