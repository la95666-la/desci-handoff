<script setup lang="ts">
import type { AuditLog } from '#/api/joincare/audit_log';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  message,
  Modal,
  Select,
  Table,
  Tag,
} from 'ant-design-vue';

import { listAuditLogs, verifyAuditChain } from '#/api/joincare/audit_log';

const loading = ref(false);
const rows = ref<AuditLog[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

const filter = reactive({
  admin_id: undefined as number | undefined,
  method: undefined as string | undefined,
  path_like: '',
  from: '',
  to: '',
});

const methodOptions = [
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listAuditLogs({
      admin_id: filter.admin_id,
      method: filter.method,
      path_like: filter.path_like || undefined,
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

const verifying = ref(false);
async function onVerifyChain() {
  verifying.value = true;
  try {
    const res = await verifyAuditChain();
    if (res.ok) {
      Modal.success({ title: '哈希链完好', content: res.message });
    } else {
      Modal.error({
        title: '⚠️ 哈希链被篡改',
        content: `断裂点 ID: ${res.broken_at_id} — 该行及之后的日志可能被人工修改或丢失`,
      });
    }
  } catch (error: any) {
    message.error(error?.message || 'verify failed');
  } finally {
    verifying.value = false;
  }
}

const detailVisible = ref(false);
const detail = ref<AuditLog | null>(null);
function openDetail(row: any) {
  detail.value = row;
  detailVisible.value = true;
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  { title: '时间', dataIndex: 'created_at', width: 170 },
  { title: '操作人', dataIndex: 'username', width: 120 },
  { title: '方法', dataIndex: 'method', width: 90 },
  { title: '路径', dataIndex: 'path', ellipsis: true },
  { title: '状态', dataIndex: 'resp_status', width: 90 },
  { title: 'IP', dataIndex: 'ip', width: 130 },
  { title: '耗时', dataIndex: 'duration_ms', width: 90 },
  { title: 'Hash', dataIndex: 'hash', width: 120, ellipsis: true },
  { title: '', dataIndex: '_action', width: 90, fixed: 'right' as const },
];

function methodColor(m: string) {
  return (
    { POST: 'blue', PUT: 'orange', PATCH: 'gold', DELETE: 'red' }[m] ||
    'default'
  );
}
function statusColor(s: number) {
  if (s >= 500) return 'red';
  if (s >= 400) return 'orange';
  if (s >= 200 && s < 300) return 'green';
  return 'default';
}

onMounted(load);
</script>

<template>
  <Page
    title="操作审计日志"
    description="所有 admin 写操作（POST/PUT/PATCH/DELETE）自动落库；SHA256 哈希链防篡改；敏感字段（password/token/signature）自动 mask"
  >
    <Alert type="info" show-icon banner class="mb-3">
      <template #message>
        每行 hash = SHA256(prev_hash || admin_id || method || path || req_body
        || resp_status || resp_hash || created_at) —
        点右上"核对哈希链"可全表校验
      </template>
    </Alert>
    <Card>
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Input
          v-model:value="filter.admin_id"
          placeholder="admin_id"
          style="width: 130px"
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Select
          v-model:value="filter.method"
          placeholder="method"
          allow-clear
          style="width: 130px"
          :options="methodOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Input
          v-model:value="filter.path_like"
          placeholder="path 关键字（LIKE）"
          style="width: 260px"
          @press-enter="
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
        <Button
          :loading="verifying"
          type="primary"
          ghost
          @click="onVerifyChain"
        >
          核对哈希链
        </Button>
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
          <template v-if="column.dataIndex === 'method'">
            <Tag :color="methodColor(record.method)">{{ record.method }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'resp_status'">
            <Tag :color="statusColor(record.resp_status)">
              {{ record.resp_status }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'duration_ms'">
            <span
              :style="{
                color: record.duration_ms > 500 ? '#e6a23c' : undefined,
              }"
            >
              {{ record.duration_ms }} ms
            </span>
          </template>
          <template v-else-if="column.dataIndex === 'hash'">
            <code style="font-size: 11px">{{ record.hash.slice(0, 12) }}…</code>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openDetail(record)">
              详情
            </Button>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="detailVisible"
      title="审计日志详情"
      width="760px"
      :footer="null"
    >
      <div v-if="detail" class="space-y-3 text-sm">
        <div>
          <b>ID:</b> {{ detail.id }} &nbsp;|&nbsp; <b>时间:</b>
          {{ detail.created_at }}
        </div>
        <div>
          <b>操作人:</b> {{ detail.username }} (id={{ detail.admin_id }})
          &nbsp;|&nbsp; <b>角色:</b> {{ detail.role_codes }}
        </div>
        <div>
          <b>方法:</b> {{ detail.method }} &nbsp;|&nbsp; <b>路径:</b>
          <code>{{ detail.path }}</code>
        </div>
        <div>
          <b>路由模板:</b> <code>{{ detail.route }}</code> &nbsp;|&nbsp;
          <b>target_id:</b> {{ detail.target_id }}
        </div>
        <div>
          <b>IP:</b> {{ detail.ip }} &nbsp;|&nbsp; <b>User-Agent:</b>
          <span class="text-gray-500">{{ detail.user_agent }}</span>
        </div>
        <div>
          <b>返回状态:</b> {{ detail.resp_status }} &nbsp;|&nbsp; <b>耗时:</b>
          {{ detail.duration_ms }} ms
        </div>
        <div>
          <b>请求体 (mask 后):</b>
          <pre
            style="
              max-height: 240px;
              padding: 8px;
              overflow-x: auto;
              font-size: 12px;
              color: #d4d4d4;
              background: #1e1e1e;
              border-radius: 6px;
            "
            >{{ detail.req_body || '(empty)' }}</pre>
        </div>
        <div style="font-size: 11px; color: #888; word-break: break-all">
          <div><b>prev_hash:</b> {{ detail.prev_hash || '(genesis)' }}</div>
          <div><b>resp_hash:</b> {{ detail.resp_hash }}</div>
          <div><b>hash:</b> {{ detail.hash }}</div>
        </div>
      </div>
    </Modal>
  </Page>
</template>
