<script setup lang="ts">
import type { News, NewsTranslation } from '#/api/joincare/news';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  DatePicker,
  Divider,
  Input,
  InputNumber,
  message,
  Modal,
  Pagination,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tabs,
  Tag,
  Upload,
} from 'ant-design-vue';

import {
  createNews,
  deleteNews,
  getNews,
  listNews,
  updateNews,
} from '#/api/joincare/news';
import { uploadFile } from '#/api/joincare/project_content';

const loading = ref(false);
const rows = ref<News[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref('');

const STATUS_LABEL: Record<number, { color: string; text: string }> = {
  0: { text: '草稿', color: 'default' },
  1: { text: '已发布', color: 'green' },
  2: { text: '已下架', color: 'red' },
};

async function load() {
  loading.value = true;
  try {
    const res = await listNews({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
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
const editLoading = ref(false);
const coverUploading = ref(false);
const activeLangTab = ref('en');

interface FormState {
  id: number;
  slug: string;
  category: string;
  cover_url: string;
  source_url: string;
  source_name: string;
  is_featured: boolean;
  is_important: boolean;
  alert_priority: number;
  alert_start_at?: string;
  alert_end_at?: string;
  alert_frequency: 'daily' | 'once' | 'session';
  alert_pages: string[];
  alert_delay_seconds: number;
  alert_revision: number;
  sort: number;
  status: number;
  publish_at: string;
  translations: NewsTranslation[];
}

const initLangs = ['en', 'tw'] as const;

function newEmptyForm(): FormState {
  return {
    id: 0,
    slug: '',
    category: 'press',
    cover_url: '',
    source_url: '',
    source_name: '',
    is_featured: false,
    is_important: false,
    alert_priority: 100,
    alert_start_at: undefined,
    alert_end_at: undefined,
    alert_frequency: 'once',
    alert_pages: ['home', 'ecosystem', 'launchpad'],
    alert_delay_seconds: 3,
    alert_revision: 1,
    sort: 0,
    status: 0,
    publish_at: new Date().toISOString().slice(0, 19).replace('T', ' '),
    translations: initLangs.map((lang) => ({
      language: lang,
      title: '',
      summary: '',
      content: '',
      author: '',
      author_role: '',
    })),
  };
}

const form = reactive<FormState>(newEmptyForm());

function ensureTranslation(lang: string): NewsTranslation {
  let t = form.translations.find((x) => x.language === lang);
  if (!t) {
    t = {
      language: lang,
      title: '',
      summary: '',
      content: '',
      author: '',
      author_role: '',
    };
    form.translations.push(t);
  }
  return t;
}

function openCreate() {
  Object.assign(form, newEmptyForm());
  activeLangTab.value = 'en';
  editVisible.value = true;
}

async function openEdit(row: any) {
  editLoading.value = true;
  try {
    const detail = await getNews(row.id);
    const empty = newEmptyForm();
    Object.assign(form, empty, {
      id: detail.news.id,
      slug: detail.news.slug,
      category: detail.news.category,
      cover_url: detail.news.cover_url,
      source_url: detail.news.source_url,
      source_name: detail.news.source_name,
      is_featured: detail.news.is_featured === 1,
      is_important: detail.news.is_important === 1,
      alert_priority: detail.news.alert_priority ?? 100,
      alert_start_at: detail.news.alert_start_at || undefined,
      alert_end_at: detail.news.alert_end_at || undefined,
      alert_frequency: detail.news.alert_frequency || 'once',
      alert_pages: (detail.news.alert_pages || 'home,ecosystem,launchpad')
        .split(',')
        .filter(Boolean),
      alert_delay_seconds: detail.news.alert_delay_seconds ?? 3,
      alert_revision: detail.news.alert_revision || 1,
      sort: detail.news.sort,
      status: detail.news.status,
      publish_at:
        detail.news.publish_at?.replace?.('T', ' ').slice(0, 19) ||
        empty.publish_at,
      translations: empty.translations.map((t) => {
        const found = detail.translations.find(
          (x: NewsTranslation) => x.language === t.language,
        );
        return found ? { ...t, ...found } : t;
      }),
    });
    activeLangTab.value = 'en';
    editVisible.value = true;
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    editLoading.value = false;
  }
}

async function onCoverChange(info: any) {
  const file = info?.file?.originFileObj || info?.file;
  if (!file) return;
  coverUploading.value = true;
  try {
    const res = (await uploadFile(file)) as any;
    if (res?.url) {
      form.cover_url = res.url;
      message.success('封面上传成功，请保存新闻');
    }
  } catch (error: any) {
    message.error(error?.message || '封面上传失败');
  } finally {
    coverUploading.value = false;
  }
}

async function onSave() {
  if (!form.slug.trim()) {
    message.warning('slug 不能为空');
    return;
  }
  if (form.is_important && form.alert_pages.length === 0) {
    message.warning('重要新闻至少选择一个展示页面');
    return;
  }
  if (
    form.is_important &&
    form.alert_start_at &&
    form.alert_end_at &&
    new Date(form.alert_end_at) <= new Date(form.alert_start_at)
  ) {
    message.warning('提醒结束时间必须晚于开始时间');
    return;
  }
  if (
    (form.is_important || form.is_featured) &&
    initLangs.some((lang) => !ensureTranslation(lang).title.trim())
  ) {
    message.warning('首页推荐或重要新闻需填写英文和繁体中文标题');
    return;
  }
  if (form.is_featured && !form.cover_url.trim()) {
    message.warning('首页推荐新闻需要填写封面 URL');
    return;
  }
  const body = {
    slug: form.slug,
    category: form.category,
    cover_url: form.cover_url,
    source_url: form.source_url,
    source_name: form.source_name,
    is_featured: form.is_featured ? 1 : 0,
    is_important: form.is_important ? 1 : 0,
    alert_priority: form.alert_priority,
    alert_start_at: form.alert_start_at,
    alert_end_at: form.alert_end_at,
    alert_frequency: form.alert_frequency,
    alert_pages: form.alert_pages.join(','),
    alert_delay_seconds: form.alert_delay_seconds,
    alert_revision: form.alert_revision,
    sort: form.sort,
    status: form.status,
    publish_at: form.publish_at,
    translations: form.translations.filter(
      (t) => t.title || t.summary || t.content,
    ),
  };
  try {
    if (form.id === 0) {
      await createNews(body);
      message.success('已创建');
    } else {
      await updateNews(form.id, body);
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
    await deleteNews(row.id);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

onMounted(load);

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: 'Slug', dataIndex: 'slug', width: 240 },
  { title: '分类', dataIndex: 'category', width: 100 },
  { title: '来源', dataIndex: 'source_name', width: 140 },
  { title: '首页推荐', dataIndex: 'is_featured', width: 95 },
  { title: '重要提醒', dataIndex: 'is_important', width: 100 },
  { title: '排序', dataIndex: 'sort', width: 70 },
  { title: '状态', dataIndex: 'status', width: 90 },
  { title: '发布时间', dataIndex: 'publish_at', width: 170 },
  { title: '操作', dataIndex: '_action', width: 160, fixed: 'right' as const },
];
</script>

<template>
  <Page
    title="新闻管理"
    description="维护多语言新闻；已发布的首页推荐新闻同时用于 Hero 浮层和 Latest Media。"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="Hero 浮层取已发布、已到发布时间的首页推荐新闻中排序值最小的一条；请同时填写英文、繁体中文标题和封面。"
    />
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="keyword"
          placeholder="slug 或来源"
          style="width: 280px"
          allow-clear
          @press-enter="onSearch"
        />
        <Button type="primary" :loading="loading" @click="onSearch">
          搜索
        </Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 条</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="false"
        :row-key="(r: News) => r.id"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'is_featured'">
            <Tag v-if="record.is_featured" color="gold">推荐</Tag>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.dataIndex === 'is_important'">
            <Tag v-if="record.is_important" color="red">提醒中</Tag>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.dataIndex === 'status'">
            <Tag :color="STATUS_LABEL[record.status]?.color || 'default'">
              {{ STATUS_LABEL[record.status]?.text || '未知' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button
              size="small"
              type="link"
              :loading="editLoading"
              @click="openEdit(record)"
            >
              编辑
            </Button>
            <Popconfirm title="确认删除？" @confirm="onDelete(record)">
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
          :page-size-options="['20', '50', '100']"
          show-quick-jumper
          @change="onPageChange"
        />
      </div>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="form.id === 0 ? '新增新闻' : `编辑新闻 #${form.id}`"
      width="800px"
      @ok="onSave"
      ok-text="保存"
      cancel-text="取消"
    >
      <div class="space-y-3 py-2">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-sm mb-1">Slug *</label>
            <Input
              v-model:value="form.slug"
              placeholder="e.g. joincare-token2049-singapore"
            />
          </div>
          <div>
            <label class="block text-sm mb-1">分类</label>
            <Select
              v-model:value="form.category"
              :options="[
                { label: 'press', value: 'press' },
                { label: 'announcement', value: 'announcement' },
                { label: 'blog', value: 'blog' },
              ]"
              style="width: 100%"
            />
          </div>
          <div class="col-span-2">
            <div class="mb-1 flex items-center gap-2">
              <label class="block text-sm">封面图片</label>
              <Upload
                :show-upload-list="false"
                :before-upload="() => false"
                accept="image/*"
                @change="onCoverChange"
              >
                <Button size="small" :loading="coverUploading">
                  上传图片
                </Button>
              </Upload>
            </div>
            <Input
              v-model:value="form.cover_url"
              placeholder="上传图片后自动回填，或粘贴外部图片 URL"
            />
            <img
              v-if="form.cover_url"
              :src="form.cover_url"
              alt="新闻封面预览"
              class="mt-2 rounded"
              style="max-width: 320px; max-height: 180px; object-fit: cover"
            />
          </div>
          <div>
            <label class="block text-sm mb-1">来源名</label>
            <Input v-model:value="form.source_name" />
          </div>
          <div class="col-span-2">
            <label class="block text-sm mb-1">原文链接</label>
            <Input v-model:value="form.source_url" />
          </div>
          <div>
            <label class="block text-sm mb-1">发布时间</label>
            <Input
              v-model:value="form.publish_at"
              placeholder="YYYY-MM-DD HH:mm:ss"
            />
          </div>
          <div>
            <label class="block text-sm mb-1">状态</label>
            <Select
              v-model:value="form.status"
              :options="[
                { label: '草稿', value: 0 },
                { label: '已发布', value: 1 },
                { label: '已下架', value: 2 },
              ]"
              style="width: 100%"
            />
          </div>
          <div>
            <label class="block text-sm mb-1">排序（越小越靠前）</label>
            <InputNumber v-model:value="form.sort" style="width: 100%" />
          </div>
          <div>
            <label class="block text-sm mb-1">首页推荐（Hero / Latest Media）</label>
            <Switch v-model:checked="form.is_featured" />
            <div class="mt-1 text-xs text-gray-500">
              多条推荐时，排序值最小的一条显示在首页 Hero 浮层。
            </div>
          </div>
        </div>

        <Divider orientation="left">重要新闻提醒</Divider>
        <div class="space-y-3">
          <div class="flex items-center gap-3">
            <Switch v-model:checked="form.is_important" />
            <div>
              <div class="text-sm font-medium">网页打开后提醒用户</div>
              <div class="text-xs text-gray-500">
                适合融资、重大合作或里程碑；普通新闻请保持关闭。
              </div>
            </div>
          </div>

          <Alert
            v-if="form.is_important"
            type="info"
            show-icon
            message="仅“已发布”且处于生效排期内的新闻会显示；多条重叠时优先级最高的一条生效。"
          />

          <div v-if="form.is_important" class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-sm mb-1">生效起（留空=立即）</label>
              <DatePicker
                v-model:value="form.alert_start_at"
                value-format="YYYY-MM-DDTHH:mm:ssZ"
                show-time
                style="width: 100%"
              />
            </div>
            <div>
              <label class="block text-sm mb-1">生效止（留空=长期）</label>
              <DatePicker
                v-model:value="form.alert_end_at"
                value-format="YYYY-MM-DDTHH:mm:ssZ"
                show-time
                style="width: 100%"
              />
            </div>
            <div>
              <label class="block text-sm mb-1">提醒频率</label>
              <Select
                v-model:value="form.alert_frequency"
                style="width: 100%"
                :options="[
                  { label: '每个版本仅一次（推荐）', value: 'once' },
                  { label: '每天一次', value: 'daily' },
                  { label: '每次浏览会话一次', value: 'session' },
                ]"
              />
            </div>
            <div>
              <label class="block text-sm mb-1">展示页面</label>
              <Select
                v-model:value="form.alert_pages"
                mode="multiple"
                style="width: 100%"
                :options="[
                  { label: '首页', value: 'home' },
                  { label: 'Ecosystem', value: 'ecosystem' },
                  { label: 'Launchpad', value: 'launchpad' },
                  { label: 'Team', value: 'team' },
                  { label: 'Task', value: 'tasks' },
                  { label: '新闻详情', value: 'news' },
                  { label: '全部页面', value: 'all' },
                ]"
              />
            </div>
            <div>
              <label class="block text-sm mb-1">优先级（越大越优先）</label>
              <InputNumber
                v-model:value="form.alert_priority"
                style="width: 100%"
              />
            </div>
            <div>
              <label class="block text-sm mb-1">页面就绪后延迟（秒）</label>
              <InputNumber
                v-model:value="form.alert_delay_seconds"
                :min="0"
                :max="15"
                style="width: 100%"
              />
            </div>
            <div class="col-span-2">
              <label class="block text-sm mb-1">提醒版本</label>
              <InputNumber
                v-model:value="form.alert_revision"
                :min="1"
                style="width: 100%"
              />
              <div class="mt-1 text-xs text-gray-500">
                用户关闭后同一版本不再打扰；需要让所有用户重新看到时再递增版本号。
              </div>
            </div>
          </div>
        </div>

        <Tabs v-model:active-key="activeLangTab">
          <Tabs.TabPane v-for="lang in initLangs" :key="lang" :tab="lang">
            <div class="space-y-2 py-1">
              <div>
                <label class="block text-sm mb-1">标题</label>
                <Input v-model:value="ensureTranslation(lang).title" />
              </div>
              <div>
                <label class="block text-sm mb-1">摘要</label>
                <Input.TextArea
                  v-model:value="ensureTranslation(lang).summary"
                  :rows="2"
                />
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-sm mb-1">作者</label>
                  <Input v-model:value="ensureTranslation(lang).author" />
                </div>
                <div>
                  <label class="block text-sm mb-1">作者职位</label>
                  <Input v-model:value="ensureTranslation(lang).author_role" />
                </div>
              </div>
              <div>
                <label class="block text-sm mb-1">正文 (Markdown / HTML)</label>
                <Input.TextArea
                  v-model:value="ensureTranslation(lang).content"
                  :rows="6"
                />
              </div>
            </div>
          </Tabs.TabPane>
        </Tabs>
      </div>
    </Modal>
  </Page>
</template>
