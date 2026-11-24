<script setup lang="ts">
import type { Banner } from '#/api/joincare/banner';

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
  createBanner,
  deleteBanner,
  listBanners,
  updateBanner,
} from '#/api/joincare/banner';

const loading = ref(false);
const rows = ref<Banner[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const langFilter = ref<string | undefined>(undefined);

const heroAnimationOptions = [
  { value: 'dandelion', label: '蒲公英 Dandelion（默认）' },
  { value: 'dna', label: 'DNA 双螺旋（旧）' },
  { value: 'none', label: '无（不渲染 3D）' },
];

const languageOptions = [
  { value: 'en', label: 'English' },
  { value: 'tw', label: '繁體中文' },
  { value: 'zh', label: '简体中文' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listBanners({
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
  subtitle: '',
  cta_label: '',
  cta_link: '',
  image_url: '',
  hero_animation: 'dandelion' as 'dandelion' | 'dna' | 'none',
  language: 'en' as 'en' | 'tw' | 'zh',
  start_at: undefined as string | undefined,
  end_at: undefined as string | undefined,
  sort: 100,
  is_enable: 1 as 0 | 1,
});

function resetForm() {
  Object.assign(form, {
    id: 0,
    title: '',
    subtitle: '',
    cta_label: '',
    cta_link: '',
    image_url: '',
    hero_animation: 'dandelion',
    language: langFilter.value || 'en',
    start_at: undefined,
    end_at: undefined,
    sort: 100,
    is_enable: 1,
  });
}

function openCreate() {
  isCreating.value = true;
  resetForm();
  editVisible.value = true;
}

function openEdit(row: any) {
  isCreating.value = false;
  Object.assign(form, {
    id: row.id,
    title: row.title,
    subtitle: row.subtitle,
    cta_label: row.cta_label,
    cta_link: row.cta_link,
    image_url: row.image_url,
    hero_animation: row.hero_animation || 'dandelion',
    language: row.language || 'en',
    start_at: row.start_at || undefined,
    end_at: row.end_at || undefined,
    sort: row.sort ?? 0,
    is_enable: (row.is_enable ?? 1) as 0 | 1,
  });
  editVisible.value = true;
}

async function onSave() {
  const body = {
    title: form.title,
    subtitle: form.subtitle,
    cta_label: form.cta_label,
    cta_link: form.cta_link,
    image_url: form.image_url,
    hero_animation: form.hero_animation,
    language: form.language,
    start_at: form.start_at,
    end_at: form.end_at,
    sort: form.sort,
    is_enable: form.is_enable,
  };
  try {
    if (form.id === 0) {
      await createBanner(body);
      message.success('已创建');
    } else {
      await updateBanner(form.id, body);
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
    await deleteBanner(row.id);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '语言', dataIndex: 'language', width: 90 },
  { title: '主标题', dataIndex: 'title', ellipsis: true },
  { title: '3D 动效', dataIndex: 'hero_animation', width: 130 },
  { title: 'CTA', dataIndex: 'cta_label', width: 140, ellipsis: true },
  { title: '排序', dataIndex: 'sort', width: 80 },
  { title: '启用', dataIndex: 'is_enable', width: 90 },
  { title: '操作', dataIndex: '_action', width: 180, fixed: 'right' as const },
];

function animLabel(v: string) {
  switch (v) {
    case 'dandelion': {
      return '蒲公英';
    }
    case 'dna': {
      return 'DNA';
    }
    case 'none': {
      return '无';
    }
    default: {
      return v;
    }
  }
}

onMounted(load);
</script>

<template>
  <Page
    title="首页 Banner 管理"
    description="按 (语种 + 启用 + 排期) 过滤后，sort 最高的一条作为当前 hero。每条 banner 可独立配置 3D 动效。"
  >
    <Alert
      type="info"
      message="C 端 /client/banner/v1/current?language=xx 取每语种最高优先级当前 banner；如无匹配回 fallback（dandelion + 空文案）"
      show-icon
      banner
      class="mb-3"
    />
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Select
          v-model:value="langFilter"
          placeholder="按语种筛选（全部）"
          allow-clear
          style="width: 180px"
          :options="languageOptions"
          @change="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增 Banner</Button>
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
          <template v-if="column.dataIndex === 'language'">
            <Tag color="purple">{{ record.language }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'hero_animation'">
            <Tag
              :color="record.hero_animation === 'none' ? 'default' : 'green'"
            >
              {{ animLabel(record.hero_animation) }}
            </Tag>
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
            <Popconfirm title="确认删除该 Banner？" @confirm="onDelete(record)">
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="isCreating ? '新增 Banner' : `编辑 Banner #${form.id}`"
      width="640px"
      ok-text="保存"
      cancel-text="取消"
      @ok="onSave"
    >
      <div class="space-y-3 py-2">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">语言 *</label>
            <Select
              v-model:value="form.language"
              style="width: 100%"
              :options="languageOptions"
            />
          </div>
          <div>
            <label class="mb-1 block text-sm">3D 动效 *</label>
            <Select
              v-model:value="form.hero_animation"
              style="width: 100%"
              :options="heroAnimationOptions"
            />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm">主标题</label>
          <Input
            v-model:value="form.title"
            placeholder="Own the Future of Medicine"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm">副标题</label>
          <Textarea
            v-model:value="form.subtitle"
            :rows="2"
            placeholder="A biomedical innovation protocol owned and driven by..."
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">CTA 按钮文字</label>
            <Input
              v-model:value="form.cta_label"
              placeholder="Explore Launchpad"
            />
          </div>
          <div>
            <label class="mb-1 block text-sm">CTA 跳转链接</label>
            <Input v-model:value="form.cta_link" placeholder="/launchpad" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm">背景图 URL（可选）</label>
          <Input v-model:value="form.image_url" placeholder="https://..." />
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
            <label class="mb-1 block text-sm">排序（越大越靠前）</label>
            <InputNumber v-model:value="form.sort" style="width: 100%" />
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
