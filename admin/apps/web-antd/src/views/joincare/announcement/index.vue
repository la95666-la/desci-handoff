<script setup lang="ts">
import type { Announcement } from '#/api/joincare/announcement';

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
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
  Textarea,
} from 'ant-design-vue';

import {
  createAnnouncement,
  deleteAnnouncement,
  listAnnouncements,
  updateAnnouncement,
} from '#/api/joincare/announcement';

const loading = ref(false);
const rows = ref<Announcement[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const typeFilter = ref<string | undefined>(undefined);
const langFilter = ref<string | undefined>(undefined);

const typeOptions = [
  { value: 'popup', label: '弹窗 popup（首次访问弹一次）' },
  { value: 'banner', label: '横幅 banner（持续显示）' },
];
const langOptions = [
  { value: 'en', label: 'English' },
  { value: 'tw', label: '繁體中文' },
  { value: 'zh', label: '简体中文' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listAnnouncements({
      type: typeFilter.value,
      language: langFilter.value,
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

const editVisible = ref(false);
const isCreating = ref(false);
const form = reactive({
  id: 0,
  title: '',
  content: '',
  type: 'popup' as 'banner' | 'popup',
  language: 'en' as 'en' | 'tw' | 'zh',
  priority: 100,
  start_at: undefined as string | undefined,
  end_at: undefined as string | undefined,
  is_enable: 1 as 0 | 1,
});

function openCreate() {
  isCreating.value = true;
  Object.assign(form, {
    id: 0,
    title: '',
    content: '',
    type: 'popup',
    language: langFilter.value || 'en',
    priority: 100,
    start_at: undefined,
    end_at: undefined,
    is_enable: 1,
  });
  editVisible.value = true;
}

function openEdit(row: any) {
  isCreating.value = false;
  Object.assign(form, {
    id: row.id,
    title: row.title,
    content: row.content,
    type: row.type || 'popup',
    language: row.language || 'en',
    priority: row.priority ?? 0,
    start_at: row.start_at || undefined,
    end_at: row.end_at || undefined,
    is_enable: (row.is_enable ?? 1) as 0 | 1,
  });
  editVisible.value = true;
}

async function onSave() {
  const body = {
    title: form.title,
    content: form.content,
    type: form.type,
    language: form.language,
    priority: form.priority,
    start_at: form.start_at,
    end_at: form.end_at,
    is_enable: form.is_enable,
  };
  try {
    if (form.id === 0) {
      await createAnnouncement(body);
      message.success('已创建');
    } else {
      await updateAnnouncement(form.id, body);
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
    const submitted = await deleteAnnouncement(row.id);
    message.success(`删除工单 #${submitted.approval_id} 已提交，等待审批`);
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '类型', dataIndex: 'type', width: 100 },
  { title: '语言', dataIndex: 'language', width: 90 },
  { title: '标题', dataIndex: 'title', ellipsis: true },
  { title: '优先级', dataIndex: 'priority', width: 90 },
  { title: '启用', dataIndex: 'is_enable', width: 90 },
  { title: '操作', dataIndex: '_action', width: 180, fixed: 'right' as const },
];

onMounted(load);
</script>

<template>
  <Page
    title="系统公告"
    description="按 (类型 + 语种 + 启用 + 排期) 筛选后取 priority 最高的一条作 'current'，C 端弹窗以 announcement.id 作 localStorage key 标记已读"
  >
    <Alert
      type="info"
      message="C 端 /client/announcement/v1/current?language=xx 返当前生效的 popup + banner 各一条；用户在 popup 上点关闭即写入 localStorage 'jc_ann_dismissed_<id>'"
      show-icon
      banner
      class="mb-3"
    />
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Select
          v-model:value="typeFilter"
          placeholder="按类型筛选"
          allow-clear
          style="width: 200px"
          :options="typeOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Select
          v-model:value="langFilter"
          placeholder="按语种筛选"
          allow-clear
          style="width: 180px"
          :options="langOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增公告</Button>
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
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'type'">
            <Tag :color="record.type === 'banner' ? 'blue' : 'orange'">
              {{ record.type === 'banner' ? '横幅' : '弹窗' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'language'">
            <Tag color="purple">{{ record.language }}</Tag>
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
            <Popconfirm title="确认删除该公告？" @confirm="onDelete(record)">
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="isCreating ? '新增公告' : `编辑公告 #${form.id}`"
      width="640px"
      ok-text="保存"
      cancel-text="取消"
      @ok="onSave"
    >
      <div class="space-y-3 py-2">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">类型 *</label>
            <Select
              v-model:value="form.type"
              style="width: 100%"
              :options="typeOptions"
            />
          </div>
          <div>
            <label class="mb-1 block text-sm">语言 *</label>
            <Select
              v-model:value="form.language"
              style="width: 100%"
              :options="langOptions"
            />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm">标题 *</label>
          <Input v-model:value="form.title" placeholder="系统维护通知" />
        </div>
        <div>
          <label class="text-sm">正文（自动清洗危险 HTML）</label>
          <Textarea
            v-model:value="form.content"
            :rows="6"
            placeholder="<p>JoinCare 将在 2026-07-01 进行升级，预计停机 30 分钟...</p>"
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">生效起（留空=立即）</label>
            <DatePicker
              v-model:value="form.start_at"
              value-format="YYYY-MM-DD HH:mm:ss"
              show-time
              style="width: 100%"
            />
          </div>
          <div>
            <label class="mb-1 block text-sm">生效止（留空=长期）</label>
            <DatePicker
              v-model:value="form.end_at"
              value-format="YYYY-MM-DD HH:mm:ss"
              show-time
              style="width: 100%"
            />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">优先级（越大越优先）</label>
            <InputNumber v-model:value="form.priority" style="width: 100%" />
          </div>
          <div class="flex items-end gap-2">
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
      </div>
    </Modal>
  </Page>
</template>
