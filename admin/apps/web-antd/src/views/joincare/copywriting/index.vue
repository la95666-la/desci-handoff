<script setup lang="ts">
import type { BilingualCopy } from '#/api/joincare/copywriting';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
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
  deleteCopywriting,
  listCopywriting,
  listCopywritingPrefixes,
  upsertCopywriting,
} from '#/api/joincare/copywriting';

const loading = ref(false);
const rows = ref<BilingualCopy[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(50);
const keyword = ref('');
const prefixFilter = ref<string | undefined>(undefined);
const prefixOptions = ref<{ label: string; value: string }[]>([]);

async function loadPrefixes() {
  try {
    const res = await listCopywritingPrefixes();
    prefixOptions.value = (res.items || []).map((p) => ({
      label: `${p.prefix} (${p.count})`,
      value: p.prefix,
    }));
  } catch (error: any) {
    console.warn('[copywriting] load prefixes failed:', error?.message);
  }
}

async function load() {
  loading.value = true;
  try {
    const res = await listCopywriting({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      prefix: prefixFilter.value,
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

// ============ Edit modal ============
const editVisible = ref(false);
const isCreating = ref(false);
const editing = reactive({
  key: '',
  en: '',
  tw: '',
});

function openCreate() {
  isCreating.value = true;
  editing.key = '';
  editing.en = '';
  editing.tw = '';
  editVisible.value = true;
}

function openEdit(row: any) {
  isCreating.value = false;
  editing.key = row.key;
  editing.en = row.en;
  editing.tw = row.tw;
  editVisible.value = true;
}

async function onSave() {
  if (!editing.key.trim()) {
    message.warning('key 不能为空');
    return;
  }
  try {
    await upsertCopywriting({
      key: editing.key.trim(),
      en: editing.en,
      tw: editing.tw,
    });
    message.success(isCreating.value ? '已创建' : '已更新');
    editVisible.value = false;
    load();
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  }
}

async function onDelete(row: any) {
  try {
    await deleteCopywriting(row.key);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

function pageOf(key: string): string {
  const dot = key.indexOf('.');
  return dot === -1 ? '-' : key.slice(0, dot);
}

const columns = [
  {
    title: 'Page',
    dataIndex: '_page',
    width: 120,
    fixed: 'left' as const,
  },
  { title: 'Key', dataIndex: 'key', width: 260, fixed: 'left' as const },
  { title: 'EN', dataIndex: 'en' },
  { title: 'TW', dataIndex: 'tw' },
  { title: '更新时间', dataIndex: 'updated_at', width: 170 },
  {
    title: '操作',
    dataIndex: '_action',
    width: 160,
    fixed: 'right' as const,
  },
];

onMounted(() => {
  loadPrefixes();
  load();
});
</script>

<template>
  <Page
    title="i18n 文案管理"
    description="copywriting_config 表 — 双语聚合视图（每个 key 同行管理 EN + TW）"
  >
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Select
          v-model:value="prefixFilter"
          placeholder="所有页面"
          :options="prefixOptions"
          style="width: 200px"
          allow-clear
          @change="onSearch"
        />
        <Input
          v-model:value="keyword"
          placeholder="key 或文案内容（任一语言）"
          style="width: 320px"
          allow-clear
          @press-enter="onSearch"
        />
        <Button type="primary" :loading="loading" @click="onSearch">
          搜索
        </Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增 key</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 个 key</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="false"
        :row-key="(r: BilingualCopy) => r.key"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === '_page'">
            <Tag color="purple">{{ pageOf(record.key) }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'key'">
            <code class="text-[12px] text-[#1668dc]">{{ record.key }}</code>
          </template>
          <template v-else-if="column.dataIndex === 'en'">
            <div :class="record.en ? '' : 'text-gray-300 italic'">
              {{ record.en || '(空)' }}
            </div>
          </template>
          <template v-else-if="column.dataIndex === 'tw'">
            <div :class="record.tw ? '' : 'text-gray-300 italic'">
              {{ record.tw || '(空)' }}
            </div>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openEdit(record)">
              编辑
            </Button>
            <Popconfirm
              title="确认删除该 key 的所有语言版本？"
              @confirm="onDelete(record)"
            >
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </template>
        </template>
      </Table>

      <div class="mt-4 flex justify-end">
        <Pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :show-size-changer="true"
          :page-size-options="['20', '50', '100', '200']"
          show-quick-jumper
          @change="onPageChange"
        />
      </div>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="isCreating ? '新增文案' : `编辑文案 «${editing.key}»`"
      width="720px"
      ok-text="保存"
      cancel-text="取消"
      @ok="onSave"
    >
      <div class="space-y-3 py-2">
        <div>
          <label class="mb-1 block text-sm">Key</label>
          <Input
            v-model:value="editing.key"
            :disabled="!isCreating"
            placeholder="e.g. home.heroEyebrow"
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm font-semibold text-blue-600">
              EN（English）
            </label>
            <Input.TextArea v-model:value="editing.en" :rows="6" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-semibold text-green-700">
              TW（繁體中文）
            </label>
            <Input.TextArea v-model:value="editing.tw" :rows="6" />
          </div>
        </div>
        <div class="text-xs text-gray-500">
          C 端会通过 GET /client/copywriting/v1/list?language=en|tw
          拉取对应语言的全部 key→value 映射。
        </div>
      </div>
    </Modal>
  </Page>
</template>
