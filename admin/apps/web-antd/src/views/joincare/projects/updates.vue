<script setup lang="ts">
import type { ProjectUpdateRow } from '#/api/joincare/project_content';

import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Input,
  message,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  createProjectUpdate,
  deleteProjectUpdate,
  listProjectUpdates,
  updateProjectUpdate,
} from '#/api/joincare/project_content';
import RichEditor from '#/components/joincare/RichEditor.vue';

const route = useRoute();
const projectId = ref(String(route.query.id ?? route.params.id ?? ''));

const loading = ref(false);
const rows = ref<ProjectUpdateRow[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

// —— 弹窗表单 ———————————————————————————————————————————
const editOpen = ref(false);
const editSaving = ref(false);
const editingId = ref<null | number>(null);
const form = reactive({
  title: '',
  content_html: '',
});

const editorTitle = computed(() =>
  editingId.value ? `编辑 Update #${editingId.value}` : '新建 Update',
);

async function load() {
  if (!projectId.value) {
    message.warning('请在 URL 里带 ?id=<project_id>');
    return;
  }
  loading.value = true;
  try {
    const res = (await listProjectUpdates(
      projectId.value,
      page.value,
      pageSize.value,
    )) as any;
    rows.value = res?.items || [];
    total.value = res?.total || 0;
  } catch (error: any) {
    message.error(error?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  form.title = '';
  form.content_html = '';
  editOpen.value = true;
}

function openEdit(row: ProjectUpdateRow) {
  editingId.value = row.id;
  form.title = row.title;
  form.content_html = row.content_html || '';
  editOpen.value = true;
}

async function saveDraft() {
  await save(false);
}
async function savePublish() {
  await save(true);
}
async function save(publish: boolean) {
  if (!form.title.trim()) {
    message.warning('请填写标题');
    return;
  }
  editSaving.value = true;
  try {
    if (editingId.value) {
      await updateProjectUpdate(editingId.value, {
        title: form.title,
        content_html: form.content_html,
        publish,
      });
      message.success(publish ? '发布成功（已推送订阅者）' : '草稿保存成功');
    } else {
      await createProjectUpdate(projectId.value, {
        title: form.title,
        content_html: form.content_html,
        publish,
      });
      message.success(publish ? '发布成功（已推送订阅者）' : '草稿保存成功');
    }
    editOpen.value = false;
    load();
  } catch (error: any) {
    message.error(error?.message || '保存失败');
  } finally {
    editSaving.value = false;
  }
}

async function remove(row: ProjectUpdateRow) {
  try {
    await deleteProjectUpdate(row.id);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || '删除失败');
  }
}

const columns = [
  { title: '#', dataIndex: 'id', width: 70 },
  { title: '标题', dataIndex: 'title' },
  { title: '状态', dataIndex: 'is_published', width: 110 },
  { title: '发布时间', dataIndex: 'published_at', width: 170 },
  { title: '更新时间', dataIndex: 'updated_at', width: 170 },
  { title: '操作', dataIndex: '_action', width: 160 },
];

function fmtDate(v: null | string) {
  if (!v) return '—';
  return v.replace('T', ' ').replace('Z', '').split('.')[0] || v;
}

onMounted(load);
watch(
  () => route.query.id,
  (v) => {
    if (v && String(v) !== projectId.value) {
      projectId.value = String(v);
      page.value = 1;
      load();
    }
  },
);
</script>

<template>
  <Page
    title="项目 Updates 时间线"
    :description="`project_id: ${projectId} · 项目方向订阅者的持续沟通（Sprint C）`"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="发布后异步推送到所有 project_subscribers（Notify me 用户）；重复保存已发布态不再推送"
    />

    <Card>
      <template #title>
        <Space>
          <span>#{{ projectId }} · Updates</span>
          <Tag color="blue">总数 {{ total }}</Tag>
        </Space>
      </template>
      <template #extra>
        <Space>
          <Button @click="load">刷新</Button>
          <Button type="primary" @click="openCreate">+ 新建 Update</Button>
        </Space>
      </template>

      <Table
        :columns="columns"
        :data-source="rows"
        :pagination="{
          current: page,
          pageSize,
          total,
          onChange: (p: number) => {
            page = p;
            load();
          },
        }"
        row-key="id"
        :loading="loading"
        size="middle"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'is_published'">
            <Tag v-if="record.is_published === 1" color="green">已发布</Tag>
            <Tag v-else color="orange">草稿</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'published_at'">
            {{ fmtDate(record.published_at) }}
          </template>
          <template v-else-if="column.dataIndex === 'updated_at'">
            {{ fmtDate(record.updated_at) }}
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Space :size="4">
              <Button
                size="small"
                @click="openEdit(record as ProjectUpdateRow)"
              >
                {{
                  (record as ProjectUpdateRow).is_published === 1
                    ? '查看'
                    : '编辑'
                }}
              </Button>
              <Popconfirm
                title="确认删除？删除后无法恢复"
                @confirm="remove(record as ProjectUpdateRow)"
              >
                <Button size="small" danger>删</Button>
              </Popconfirm>
            </Space>
          </template>
        </template>
      </Table>
    </Card>

    <!-- 编辑弹窗 -->
    <Modal
      v-model:open="editOpen"
      :title="editorTitle"
      :width="900"
      :footer="null"
      :destroy-on-close="true"
      :mask-closable="false"
    >
      <div class="mb-3">
        <div class="mb-1 text-sm text-gray-500">标题</div>
        <Input
          v-model:value="form.title"
          placeholder="Milestone 3 达成 / 论文接收 / 融资到位 ..."
        />
      </div>
      <div class="mb-4">
        <div class="mb-1 text-sm text-gray-500">正文</div>
        <RichEditor
          v-model="form.content_html"
          placeholder="正文（Markdown 风格富文本，图片可直接上传）"
          min-height="320px"
        />
      </div>
      <Space class="w-full justify-end">
        <Button @click="editOpen = false">取消</Button>
        <Button :loading="editSaving" @click="saveDraft">保存草稿</Button>
        <Button type="primary" :loading="editSaving" @click="savePublish">
          保存并发布
        </Button>
      </Space>
    </Modal>
  </Page>
</template>
