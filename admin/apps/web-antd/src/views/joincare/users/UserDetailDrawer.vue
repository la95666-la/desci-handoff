<script setup lang="ts">
import type {
  UserLoginLog,
  UserNote,
  UserPointFlow,
  UserSummary,
} from '#/api/joincare/user_detail';

import { ref, watch } from 'vue';

import {
  Alert,
  Button,
  Descriptions,
  Drawer,
  message,
  Popconfirm,
  Statistic,
  Table,
  Tabs,
  Tag,
  Textarea,
} from 'ant-design-vue';

import { unbindUserTelegram, unbindUserTwitter } from '#/api/joincare/user';
import {
  createUserNote,
  deleteUserNote,
  getUserSummary,
  listUserActivity,
  listUserLoginLogs,
  listUserNotes,
} from '#/api/joincare/user_detail';

const props = defineProps<{ open: boolean; uid: number }>();
const emit = defineEmits<{ (e: 'update:open', v: boolean): void }>();

const activeTab = ref('summary');
const summary = ref<null | UserSummary>(null);
const notes = ref<UserNote[]>([]);
const logins = ref<UserLoginLog[]>([]);
const activity = ref<UserPointFlow[]>([]);
const newNote = ref('');
const loading = ref(false);

async function loadAll() {
  if (!props.uid) return;
  loading.value = true;
  try {
    const [s, n, l, a] = await Promise.all([
      getUserSummary(props.uid),
      listUserNotes(props.uid),
      listUserLoginLogs(props.uid, 1, 30),
      listUserActivity(props.uid, 1, 30),
    ]);
    summary.value = s;
    notes.value = n.items || [];
    logins.value = l.items || [];
    activity.value = a.items || [];
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.uid],
  ([o]) => {
    if (o) {
      activeTab.value = 'summary';
      loadAll();
    }
  },
);

async function onAddNote() {
  const text = newNote.value.trim();
  if (!text) return;
  try {
    await createUserNote(props.uid, text);
    newNote.value = '';
    message.success('已备注');
    const n = await listUserNotes(props.uid);
    notes.value = n.items || [];
    if (summary.value)
      summary.value.note_count = (summary.value.note_count || 0) + 1;
  } catch (error: any) {
    message.error(error?.message || 'add note failed');
  }
}

async function onDelNote(noteId: number) {
  try {
    await deleteUserNote(noteId);
    notes.value = notes.value.filter((n) => n.id !== noteId);
    if (summary.value)
      summary.value.note_count = Math.max(
        0,
        (summary.value.note_count || 0) - 1,
      );
  } catch (error: any) {
    message.error(error?.message || 'delete note failed');
  }
}

const loginColumns = [
  { title: '时间', dataIndex: 'logged_in_at', width: 200 },
  { title: '方式', dataIndex: 'login_type', width: 100 },
  { title: 'IP', dataIndex: 'ip', width: 160 },
  { title: 'User Agent', dataIndex: 'user_agent', ellipsis: true },
];

const activityColumns = [
  { title: '时间', dataIndex: 'created_at', width: 200 },
  { title: '类型', dataIndex: 'type', width: 130 },
  {
    title: '变动',
    dataIndex: 'change_point',
    width: 100,
    align: 'right' as const,
  },
  { title: '余额', dataIndex: 'remain', width: 100, align: 'right' as const },
  { title: '说明', dataIndex: 'desc', ellipsis: true },
];

function typeColor(t: string) {
  return (
    { CHECK_IN: 'blue', TASK: 'green', INVITE: 'purple', ADJUST: 'orange' }[
      t
    ] || 'default'
  );
}
function loginTypeColor(t: string) {
  return (
    { wallet: 'green', auto: 'default', register: 'blue', twitter: 'cyan' }[
      t
    ] || 'default'
  );
}

function close() {
  emit('update:open', false);
}

const unbindingTwitter = ref(false);
async function onUnbindTwitter() {
  if (!summary.value?.user?.id) return;
  unbindingTwitter.value = true;
  try {
    // 把当前绑定的 twitter_id / handle 一起 POST 上去 —— 落到 admin_audit_log.req_body，
    // 审计员之后翻记录时能直接看到"解绑了哪个号"
    const res = await unbindUserTwitter(summary.value.user.id, {
      prev_twitter_id: summary.value.user.twitter_id || '',
      prev_handle: summary.value.user.twitter_handle || '',
    });
    if (res.unbound) {
      message.success(`已解绑 X @${res.prev_handle || res.prev_twitter_id}`);
      summary.value.user.twitter_id = '';
      summary.value.user.twitter_handle = '';
    } else {
      message.info(res.message || '该用户未绑定 X');
    }
  } catch (error: any) {
    message.error(error?.message || 'unbind twitter failed');
  } finally {
    unbindingTwitter.value = false;
  }
}

const unbindingTelegram = ref(false);
async function onUnbindTelegram() {
  if (!summary.value?.user?.id) return;
  unbindingTelegram.value = true;
  try {
    const res = await unbindUserTelegram(summary.value.user.id, {
      prev_telegram_id: summary.value.user.telegram_id || '',
      prev_username: summary.value.user.telegram_username || '',
    });
    if (res.unbound) {
      message.success(
        `已解绑 Telegram @${res.prev_username || res.prev_telegram_id}`,
      );
      summary.value.user.telegram_id = '';
      summary.value.user.telegram_username = '';
    } else {
      message.info(res.message || '该用户未绑定 Telegram');
    }
  } catch (error: any) {
    message.error(error?.message || 'unbind telegram failed');
  } finally {
    unbindingTelegram.value = false;
  }
}
</script>

<template>
  <Drawer
    :open="open"
    :title="uid ? `用户详情 #${uid}` : '用户详情'"
    width="820"
    @close="close"
    @update:open="(v: boolean) => emit('update:open', v)"
  >
    <div v-if="!summary" class="p-6 text-center text-gray-500">
      {{ loading ? '加载中…' : '暂无数据' }}
    </div>
    <Tabs v-else v-model:active-key="activeTab">
      <Tabs.TabPane key="summary" tab="概览">
        <div class="mb-4 grid grid-cols-4 gap-4">
          <Statistic title="积分余额" :value="summary.total_point" />
          <Statistic title="登录次数" :value="summary.login_count" />
          <Statistic title="任务完成" :value="summary.task_completed" />
          <Statistic
            title="连续签到"
            :value="summary.continuous_checkin_days"
            suffix="天"
          />
        </div>
        <Descriptions bordered size="small" :column="2">
          <Descriptions.Item label="ID">
            {{ summary.user.id }}
          </Descriptions.Item>
          <Descriptions.Item label="启用">
            <Tag :color="summary.user.is_enable === 1 ? 'green' : 'red'">
              {{ summary.user.is_enable === 1 ? '启用' : '停用' }}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="钱包" :span="2">
            <code>{{ summary.user.wallet_address }}</code>
          </Descriptions.Item>
          <Descriptions.Item label="Twitter">
            <span>{{
              summary.user.twitter_handle
                ? `@${summary.user.twitter_handle}`
                : '-'
            }}</span>
            <Popconfirm
              v-if="summary.user.twitter_id"
              title="确定解绑该用户的 X 账号？"
              description="解绑后用户可以重新走 OAuth 绑定；已发放的 X 任务积分不会撤销。"
              ok-text="解绑"
              cancel-text="取消"
              @confirm="onUnbindTwitter"
            >
              <Button
                size="small"
                type="link"
                danger
                :loading="unbindingTwitter"
                style="padding: 0; margin-left: 8px"
              >
                解绑
              </Button>
            </Popconfirm>
          </Descriptions.Item>
          <Descriptions.Item label="Telegram">
            <span>{{ summary.user.telegram_username || '-' }}</span>
            <Popconfirm
              v-if="summary.user.telegram_id"
              title="确定解绑该用户的 Telegram？"
              description="解绑后用户可以重新走 tgbot 绑定；已发放的 TG 任务积分不会撤销。"
              ok-text="解绑"
              cancel-text="取消"
              @confirm="onUnbindTelegram"
            >
              <Button
                size="small"
                type="link"
                danger
                :loading="unbindingTelegram"
                style="padding: 0; margin-left: 8px"
              >
                解绑
              </Button>
            </Popconfirm>
          </Descriptions.Item>
          <Descriptions.Item label="上次登录">
            {{ summary.user.last_login_at || '-' }}
          </Descriptions.Item>
          <Descriptions.Item label="上次登录 IP">
            {{ summary.user.last_login_ip || '-' }}
          </Descriptions.Item>
          <Descriptions.Item label="总签到天数">
            {{ summary.total_checkin_days }}
          </Descriptions.Item>
          <Descriptions.Item label="最近签到">
            {{ summary.last_checkin_at || '-' }}
          </Descriptions.Item>
          <Descriptions.Item label="注册时间" :span="2">
            {{ summary.user.created_at }}
          </Descriptions.Item>
        </Descriptions>
      </Tabs.TabPane>

      <Tabs.TabPane key="notes" :tab="`备注 (${summary.note_count})`">
        <div class="mb-3 flex gap-2">
          <Textarea
            v-model:value="newNote"
            :rows="2"
            placeholder="内部备注，仅 admin 可见"
            style="flex: 1"
          />
          <Button type="primary" :disabled="!newNote.trim()" @click="onAddNote">
            添加
          </Button>
        </div>
        <div v-if="notes.length === 0" class="py-6 text-center text-gray-500">
          暂无备注
        </div>
        <div v-else class="space-y-2">
          <div v-for="n in notes" :key="n.id" class="rounded border p-3">
            <div class="mb-1 flex items-center gap-2 text-xs text-gray-500">
              <b>{{ n.admin_username || `admin#${n.admin_id}` }}</b>
              <span>·</span>
              <span>{{ n.created_at }}</span>
              <Popconfirm title="删除此条备注？" @confirm="onDelNote(n.id)">
                <Button size="small" type="link" danger class="ml-auto">
                  删除
                </Button>
              </Popconfirm>
            </div>
            <div style="white-space: pre-wrap">{{ n.note }}</div>
          </div>
        </div>
      </Tabs.TabPane>

      <Tabs.TabPane key="login" :tab="`登录记录 (${summary.login_count})`">
        <Alert
          type="info"
          show-icon
          class="mb-3"
          message="最近 30 条；每次 Login / Register / AutoLogin 都会追加一行"
        />
        <Table
          :columns="loginColumns"
          :data-source="logins"
          :pagination="false"
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
          </template>
        </Table>
      </Tabs.TabPane>

      <Tabs.TabPane key="activity" tab="积分流水 / 操作记录">
        <Alert
          type="info"
          show-icon
          class="mb-3"
          message="所有涨/扣分动作走统一入口 user_point_flow；type 涵盖 CHECK_IN / TASK / INVITE / ADJUST"
        />
        <Table
          :columns="activityColumns"
          :data-source="activity"
          :pagination="false"
          :row-key="(r: any) => r.id"
          size="small"
          bordered
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'type'">
              <Tag :color="typeColor(record.type)">{{ record.type }}</Tag>
            </template>
            <template v-else-if="column.dataIndex === 'change_point'">
              <span
                :style="{
                  color: record.change_point >= 0 ? '#1aa64a' : '#e53e3e',
                  fontWeight: 600,
                }"
              >
                {{ record.change_point >= 0 ? '+' : ''
                }}{{ record.change_point }}
              </span>
            </template>
          </template>
        </Table>
      </Tabs.TabPane>
    </Tabs>
  </Drawer>
</template>
