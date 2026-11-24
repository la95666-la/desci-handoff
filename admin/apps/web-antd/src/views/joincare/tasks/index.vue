<script setup lang="ts">
import type { JCTask } from '#/api/joincare/task';

import { h, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Input,
  InputNumber,
  message,
  Modal,
  Pagination,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  getInviteSettings,
  updateInviteSettings,
} from '#/api/joincare/invite_settings';
import {
  createTask,
  deleteTask,
  listTasks,
  syncPinnedTweet,
  updateTask,
} from '#/api/joincare/task';
import {
  getTwitterVerificationSettings,
  updateTwitterVerificationSettings,
} from '#/api/joincare/twitter_verification_settings';

const inviteSettingsLoading = ref(false);
const inviteSettingsSaving = ref(false);
const inviteDailyCap = ref<number>(10);
const twitterVerificationSettingsLoading = ref(false);
const twitterVerificationSettingsSaving = ref(false);
const twitterVerificationDisabled = ref(false);

async function loadInviteSettings() {
  inviteSettingsLoading.value = true;
  try {
    const result = await getInviteSettings();
    inviteDailyCap.value = result.dailyCap;
  } catch (error: any) {
    message.error(error?.message || '邀请设置加载失败');
  } finally {
    inviteSettingsLoading.value = false;
  }
}

async function saveInviteSettings() {
  const value = Number(inviteDailyCap.value);
  if (!Number.isInteger(value) || value < 1 || value > 100_000) {
    message.warning('每日邀请上限必须是 1–100000 的整数');
    return;
  }
  inviteSettingsSaving.value = true;
  try {
    const result = await updateInviteSettings(value);
    inviteDailyCap.value = result.dailyCap;
    message.success('邀请设置已保存');
  } catch (error: any) {
    message.error(error?.message || '邀请设置保存失败');
  } finally {
    inviteSettingsSaving.value = false;
  }
}

async function loadTwitterVerificationSettings() {
  twitterVerificationSettingsLoading.value = true;
  try {
    const result = await getTwitterVerificationSettings();
    twitterVerificationDisabled.value = result.verificationDisabled;
  } catch (error: any) {
    // Fail closed in the UI as well: a failed read must never visually imply
    // that external verification has been bypassed.
    twitterVerificationDisabled.value = false;
    message.error(error?.message || 'X 验证策略加载失败');
  } finally {
    twitterVerificationSettingsLoading.value = false;
  }
}

async function saveTwitterVerificationSettings() {
  if (twitterVerificationDisabled.value) {
    Modal.confirm({
      title: '确认跳过 X API 验证？',
      content:
        '开启后，所有支持的 X 任务将不再查询外部 X API，用户无需绑定 X 即可提交并获得积分。任务轮次和数据库唯一约束仍会阻止重复发分。',
      okText: '确认开启不验证',
      okType: 'danger',
      cancelText: '取消',
      onOk: persistTwitterVerificationSettings,
    });
    return;
  }
  await persistTwitterVerificationSettings();
}

async function persistTwitterVerificationSettings() {
  twitterVerificationSettingsSaving.value = true;
  try {
    const result = await updateTwitterVerificationSettings(
      twitterVerificationDisabled.value,
    );
    twitterVerificationDisabled.value = result.verificationDisabled;
    message.success('X 验证策略已保存');
  } catch (error: any) {
    message.error(error?.message || 'X 验证策略保存失败');
    await loadTwitterVerificationSettings();
  } finally {
    twitterVerificationSettingsSaving.value = false;
  }
}

const loading = ref(false);
const rows = ref<JCTask[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(50);
const keyword = ref('');
const platformFilter = ref<string | undefined>(undefined);

// 同步 X 置顶推文（半自动更新 like/retweet 任务的 URL）
const syncingPinned = ref(false);
const syncPinnedUsername = ref('thejoincare');
async function openSyncPinned() {
  Modal.confirm({
    title: '同步 X 置顶推文',
    content: () =>
      h('div', [
        h(
          'p',
          '将查询指定 X 用户的置顶推文，并把 like / retweet 任务的 URL 更新为该置顶推文。',
        ),
        h(
          'p',
          { style: 'color: #cf1322; font-size: 12px' },
          '如果帖子 ID 发生变化，Like / Retweet 将开启新一轮，已完成用户可以再次领取对应积分。',
        ),
        h(
          'p',
          { style: 'color: #d46b08; font-size: 12px' },
          '前提：backend config/secret/config.yaml 里已配 twitter.bearerToken（Twitter App-only Bearer Token）',
        ),
        h(Input, {
          value: syncPinnedUsername.value,
          'onUpdate:value': (v: string) => (syncPinnedUsername.value = v),
          placeholder: '不含 @ 前缀，如 thejoincare',
          style: 'margin-top: 12px',
        }),
      ]),
    okText: '同步',
    cancelText: '取消',
    async onOk() {
      syncingPinned.value = true;
      try {
        const res = await syncPinnedTweet(syncPinnedUsername.value.trim());
        if (!res.pinned_tweet_id) {
          message.warning(`@${res.username} 目前没有置顶推文`);
          return;
        }
        message.success(
          res.advanced > 0
            ? `已同步 ${res.updated} 项任务，并开启 ${res.advanced} 个新轮次`
            : '已同步：帖子未变化，没有开启新轮次',
        );
        await load();
      } catch (error: any) {
        message.error(error?.message || '同步失败');
        throw error; // 让 Modal 保持打开
      } finally {
        syncingPinned.value = false;
      }
    },
  });
}

const platformOptions = [
  { label: 'Twitter / X', value: 'twitter' },
  { label: 'Telegram', value: 'telegram' },
];
const verifyKindOptions = [
  { label: 'manual（手动放过）', value: 'manual' },
  { label: 'twitter_api', value: 'twitter_api' },
  { label: 'telegram_bot', value: 'telegram_bot' },
];

async function load() {
  loading.value = true;
  try {
    const res = await listTasks({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      platform: platformFilter.value,
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
const originalTargetRef = ref('');
const form = reactive({
  id: 0,
  version: 1,
  platform: 'twitter',
  task_key: '',
  target_slot: 1,
  title: '',
  description: '',
  url: '',
  cta_label: '',
  verify_kind: 'manual',
  verify_meta: '',
  amount_point: 25,
  sort: 0,
  is_enable: 1 as 0 | 1,
});

function resetForm() {
  Object.assign(form, {
    id: 0,
    version: 1,
    platform: 'twitter',
    task_key: '',
    target_slot: 1,
    title: '',
    description: '',
    url: '',
    cta_label: '',
    verify_kind: 'manual',
    verify_meta: '',
    amount_point: 25,
    sort: 0,
    is_enable: 1,
  });
}

function openCreate() {
  resetForm();
  editVisible.value = true;
}

function openEdit(row: any) {
  Object.assign(form, {
    id: row.id,
    version: row.version || 1,
    platform: row.platform,
    task_key: row.task_key,
    target_slot: row.target_slot || 1,
    title: row.title,
    description: row.description,
    url: row.url,
    cta_label: row.cta_label,
    verify_kind: row.verify_kind || 'manual',
    verify_meta: row.verify_meta || '',
    amount_point: row.amount_point,
    sort: row.sort,
    is_enable: (row.is_enable ?? 1) as 0 | 1,
  });
  originalTargetRef.value = extractTweetId(row.url);
  editVisible.value = true;
}

function extractTweetId(rawUrl: string) {
  try {
    const parts = new URL(rawUrl).pathname.split('/').filter(Boolean);
    const statusIndex = parts.indexOf('status');
    return statusIndex === -1 ? '' : parts[statusIndex + 1] || '';
  } catch {
    return '';
  }
}

function isPostBoundXTask() {
  return (
    form.platform === 'twitter' &&
    ['bookmark', 'like', 'reply', 'retweet'].includes(
      form.task_key.toLowerCase(),
    )
  );
}

function supportsSecondTargetSlot() {
  return (
    form.platform === 'twitter' &&
    ['like', 'reply', 'retweet'].includes(form.task_key.toLowerCase())
  );
}

async function persistTask() {
  try {
    if (form.id === 0) {
      await createTask({ ...form });
      message.success('已创建');
    } else {
      const result = await updateTask(form.id, { ...form });
      message.success(
        result.round_changed
          ? `已更新，并开启第 ${result.new_version} 轮任务`
          : '已更新（任务轮次未变化）',
      );
    }
    editVisible.value = false;
    await load();
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  }
}

async function onSave() {
  if (!form.task_key.trim()) {
    message.warning('task_key 不能为空');
    return;
  }

  if (!supportsSecondTargetSlot()) {
    form.target_slot = 1;
  }

  const nextTargetRef = extractTweetId(form.url);
  if (supportsSecondTargetSlot()) {
    const duplicateSlot = rows.value.find(
      (row) =>
        row.id !== form.id &&
        row.platform === form.platform &&
        row.task_key.toLowerCase() === form.task_key.toLowerCase() &&
        (row.target_slot || 1) === form.target_slot,
    );
    if (duplicateSlot) {
      message.warning(
        `帖子槽位 ${form.target_slot} 已被任务 #${duplicateSlot.id} 使用`,
      );
      return;
    }
  }
  if (
    form.id !== 0 &&
    isPostBoundXTask() &&
    originalTargetRef.value &&
    nextTargetRef &&
    originalTargetRef.value !== nextTargetRef
  ) {
    Modal.confirm({
      title: '确认开启新一轮任务？',
      content: `帖子 ID 已变化。保存后任务将从第 ${form.version} 轮更新到第 ${form.version + 1} 轮，旧完成记录和积分保留，用户可再次完成并领取积分。`,
      okText: '保存并开启新一轮',
      cancelText: '取消',
      onOk: persistTask,
    });
    return;
  }
  await persistTask();
}

async function onDelete(row: any) {
  try {
    await deleteTask(row.id);
    message.success('已删除');
    load();
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '平台', dataIndex: 'platform', width: 100 },
  { title: 'Key', dataIndex: 'task_key', width: 120 },
  { title: '帖子槽位', dataIndex: 'target_slot', width: 90 },
  { title: '轮次', dataIndex: 'version', width: 75 },
  { title: '标题', dataIndex: 'title' },
  { title: '校验方式', dataIndex: 'verify_kind', width: 130 },
  { title: '积分', dataIndex: 'amount_point', width: 80 },
  { title: '排序', dataIndex: 'sort', width: 70 },
  { title: '启用', dataIndex: 'is_enable', width: 70 },
  { title: '操作', dataIndex: '_action', width: 160, fixed: 'right' as const },
];

onMounted(() => {
  load();
  loadInviteSettings();
  loadTwitterVerificationSettings();
});
</script>

<template>
  <Page
    title="任务管理"
    description="X / Telegram 社交任务（task 表）— Phase 1 CRUD"
  >
    <Card
      class="mb-4"
      title="X / Twitter 任务验证策略"
      :loading="twitterVerificationSettingsLoading"
    >
      <div class="flex flex-wrap items-center gap-3">
        <div class="max-w-3xl">
          <div class="mb-1 text-sm font-medium">跳过 X API 验证并直接发分</div>
          <div class="text-xs text-gray-500">
            默认关闭。开启后，用户点击支持的 X
            任务会直接提交后端完成，不要求绑定 X，也不会调用外部 X API；任务
            ID、轮次、目标槽位和数据库唯一约束仍会阻止重复发分。未知任务仍会拒绝。
          </div>
          <div class="mt-1 text-xs text-red-500">
            这是全局奖励策略开关。仅应在 X API
            不可用或明确运营批准时开启，所有修改都会写入 Admin 审计日志。
          </div>
        </div>
        <Switch
          v-model:checked="twitterVerificationDisabled"
          checked-children="不验证"
          un-checked-children="验证"
          class="ml-auto"
        />
        <Button
          type="primary"
          danger
          :loading="twitterVerificationSettingsSaving"
          @click="saveTwitterVerificationSettings"
        >
          保存 X 验证策略
        </Button>
      </div>
    </Card>
    <Card class="mb-4" title="邀请奖励设置" :loading="inviteSettingsLoading">
      <div class="flex flex-wrap items-center gap-3">
        <div>
          <div class="mb-1 text-sm font-medium">
            每个邀请人每天最多绑定的新用户数
          </div>
          <div class="text-xs text-gray-500">
            按 UTC
            自然日计算。达到上限后，新用户仍可注册，但不会建立邀请关系，也不会产生后续
            50 分邀请奖励。
          </div>
        </div>
        <InputNumber
          v-model:value="inviteDailyCap"
          :min="1"
          :max="100000"
          :precision="0"
          addon-after="人 / UTC 日"
          class="ml-auto"
          style="width: 180px"
        />
        <Button
          type="primary"
          :loading="inviteSettingsSaving"
          @click="saveInviteSettings"
        >
          保存设置
        </Button>
      </div>
    </Card>
    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="keyword"
          placeholder="task_key 或标题"
          style="width: 240px"
          allow-clear
          @press-enter="onSearch"
        />
        <Select
          v-model:value="platformFilter"
          placeholder="所有平台"
          :options="platformOptions"
          style="width: 160px"
          allow-clear
        />
        <Button type="primary" :loading="loading" @click="onSearch">
          搜索
        </Button>
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增</Button>
        <Button :loading="syncingPinned" @click="openSyncPinned">
          🔄 同步 X 置顶推文
        </Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 条</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="false"
        :row-key="(r: JCTask) => r.id"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'platform'">
            <Tag :color="record.platform === 'twitter' ? 'blue' : 'cyan'">
              {{ record.platform }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'target_slot'">
            <Tag
              v-if="['like', 'reply', 'retweet'].includes(record.task_key)"
              color="blue"
            >
              帖子 {{ record.target_slot || 1 }}
            </Tag>
            <span v-else>—</span>
          </template>
          <template v-else-if="column.dataIndex === 'is_enable'">
            <Tag :color="record.is_enable === 1 ? 'green' : 'default'">
              {{ record.is_enable === 1 ? '✓' : '×' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openEdit(record)">
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
      :title="
        form.id === 0
          ? '新增任务'
          : `编辑任务 #${form.id}（第 ${form.version} 轮）`
      "
      width="640px"
      @ok="onSave"
      ok-text="保存"
      cancel-text="取消"
    >
      <div class="space-y-3 py-2 grid grid-cols-2 gap-3">
        <div>
          <label class="block text-sm mb-1">平台 *</label>
          <Select
            v-model:value="form.platform"
            :options="platformOptions"
            style="width: 100%"
          />
        </div>
        <div>
          <label class="block text-sm mb-1">Task Key *</label>
          <Input
            v-model:value="form.task_key"
            placeholder="follow / like / join / chat"
          />
        </div>
        <div v-if="supportsSecondTargetSlot()" class="col-span-2">
          <label class="block text-sm mb-1">帖子槽位 *</label>
          <Select
            v-model:value="form.target_slot"
            :options="[
              { label: '帖子 1', value: 1 },
              { label: '帖子 2', value: 2 },
            ]"
            style="width: 180px"
          />
          <div class="mt-1 text-xs text-gray-500">
            同一种互动最多配置两个帖子；两个槽位可分别设置链接、积分、排序和启用状态。
          </div>
        </div>
        <div class="col-span-2">
          <label class="block text-sm mb-1">标题</label>
          <Input v-model:value="form.title" />
        </div>
        <div class="col-span-2">
          <label class="block text-sm mb-1">描述</label>
          <Input.TextArea v-model:value="form.description" :rows="2" />
        </div>
        <div class="col-span-2">
          <label class="block text-sm mb-1">目标 URL</label>
          <Input v-model:value="form.url" />
        </div>
        <div>
          <label class="block text-sm mb-1">CTA 文案</label>
          <Input
            v-model:value="form.cta_label"
            placeholder="Open X / Open Telegram"
          />
        </div>
        <div>
          <label class="block text-sm mb-1">校验方式</label>
          <Select
            v-model:value="form.verify_kind"
            :options="verifyKindOptions"
            style="width: 100%"
          />
        </div>
        <div class="col-span-2">
          <label class="block text-sm mb-1">校验参数 (JSON)</label>
          <Input.TextArea
            v-model:value="form.verify_meta"
            :rows="2"
            placeholder="{&quot;tweet_id&quot;:&quot;...&quot;, &quot;chat_id&quot;:&quot;...&quot;}"
          />
        </div>
        <div>
          <label class="block text-sm mb-1">奖励积分</label>
          <InputNumber v-model:value="form.amount_point" style="width: 100%" />
        </div>
        <div>
          <label class="block text-sm mb-1">排序</label>
          <InputNumber v-model:value="form.sort" style="width: 100%" />
        </div>
        <div>
          <label class="block text-sm mb-1">启用</label>
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
