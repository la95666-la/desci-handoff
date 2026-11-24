<script setup lang="ts">
import type {
  TgActivityRow,
  TgChat,
  TgDailyStat,
  TgUserSummary,
} from '#/api/joincare/tg_activity';
import type { TgSettings } from '#/api/joincare/tg_settings';

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
  Select,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
} from 'ant-design-vue';

import {
  tgActivityDetail,
  tgChats,
  tgDailyStats,
  tgUserSummary,
} from '#/api/joincare/tg_activity';
import { getTgSettings, updateTgSettings } from '#/api/joincare/tg_settings';

const loading = ref(false);
const rows = ref<TgDailyStat[]>([]);

const filter = reactive({
  from: '',
  to: '',
  chat_id: undefined as number | undefined,
});

async function load() {
  loading.value = true;
  try {
    const res = await tgDailyStats({
      from: filter.from || undefined,
      to: filter.to || undefined,
      chat_id: filter.chat_id || undefined,
    });
    rows.value = res.items || [];
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}

// ==== 详情 modal ====
const detailVisible = ref(false);
const detailTitle = ref('');
const detailLoading = ref(false);
const detailRows = ref<TgActivityRow[]>([]);
const detailTotal = ref(0);
const detailPage = ref(1);
const detailPageSize = ref(50);
const detailBoundOnly = ref(false);
const detailChatId = ref(0);
const detailDay = ref('');

async function loadDetail() {
  detailLoading.value = true;
  try {
    const res = await tgActivityDetail({
      day: detailDay.value,
      chat_id: detailChatId.value,
      bound_only: detailBoundOnly.value ? '1' : '0',
      page: detailPage.value,
      page_size: detailPageSize.value,
    });
    detailRows.value = res.items || [];
    detailTotal.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load detail failed');
  } finally {
    detailLoading.value = false;
  }
}

function openDetail(row: TgDailyStat) {
  detailDay.value = row.day;
  detailChatId.value = row.chat_id;
  detailPage.value = 1;
  detailBoundOnly.value = false;
  detailTitle.value = `${row.day} · ${row.chat_title || `chat#${row.chat_id}`}`;
  detailVisible.value = true;
  loadDetail();
}

// ==== 顶部聚合数字（当日）====
const todayStat = ref({ active: 0, msgs: 0, bound: 0 });
function computeToday() {
  const today = new Date().toISOString().slice(0, 10);
  let a = 0;
  let m = 0;
  let b = 0;
  for (const r of rows.value) {
    if (r.day.startsWith(today)) {
      a += r.active_users;
      m += r.total_messages;
      b += r.binded_users;
    }
  }
  todayStat.value = { active: a, msgs: m, bound: b };
}

async function reload() {
  await load();
  computeToday();
}

// ==== 用户维度 tab ====
const activeTab = ref('group');
const chats = ref<TgChat[]>([]);
const userFilter = reactive({
  chat_id: undefined as number | undefined,
  from: '',
  to: '',
  bound_only: '1' as '0' | '1',
  sort: 'msgs_desc' as 'days_desc' | 'msgs_desc' | 'recent_desc',
});
const userLoading = ref(false);
const userRows = ref<TgUserSummary[]>([]);
const userTotal = ref(0);
const userPage = ref(1);
const userPageSize = ref(20);

async function loadChats() {
  try {
    const res = await tgChats();
    chats.value = res.items || [];
    if (!userFilter.chat_id && chats.value.length > 0) {
      userFilter.chat_id = chats.value[0]?.chat_id;
    }
  } catch (error: any) {
    console.warn(error);
  }
}

async function loadUsers() {
  if (!userFilter.chat_id) {
    message.warning('请先选择群');
    return;
  }
  userLoading.value = true;
  try {
    const res = await tgUserSummary({
      chat_id: userFilter.chat_id,
      from: userFilter.from || undefined,
      to: userFilter.to || undefined,
      bound_only: userFilter.bound_only,
      sort: userFilter.sort,
      page: userPage.value,
      page_size: userPageSize.value,
    });
    userRows.value = res.items || [];
    userTotal.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    userLoading.value = false;
  }
}

function onTabChange(k: number | string) {
  if (k === 'user' && chats.value.length === 0) {
    loadChats().then(() => {
      if (userFilter.chat_id) loadUsers();
    });
  } else if (
    k === 'user' &&
    userFilter.chat_id &&
    userRows.value.length === 0
  ) {
    loadUsers();
  } else if (k === 'settings') {
    loadSettings();
  }
}

// ==== 配置 tab ====
const settingsForm = reactive({
  chatId: 0,
  chatLink: '',
  messagePoints: 1,
  dailyCap: 10,
});
const settingsLoading = ref(false);
const settingsSaving = ref(false);
const settingsUpdatedAt = ref(0);

async function loadSettings() {
  settingsLoading.value = true;
  try {
    const res = (await getTgSettings()) as TgSettings;
    settingsForm.chatId = res?.chatId ?? 0;
    settingsForm.chatLink = res?.chatLink ?? '';
    settingsForm.messagePoints = res?.messagePoints || 1;
    settingsForm.dailyCap = res?.dailyCap || 10;
    settingsUpdatedAt.value = res?.updatedAt ?? 0;
  } catch (error: any) {
    message.error(error?.message || 'load settings failed');
  } finally {
    settingsLoading.value = false;
  }
}

async function saveSettings() {
  if (!settingsForm.chatId) {
    message.warning('chat_id 不能为 0（负数群 id）');
    return;
  }
  if (settingsForm.dailyCap < 1) {
    message.warning('daily_cap 必须 ≥ 1');
    return;
  }
  settingsSaving.value = true;
  try {
    const res = (await updateTgSettings(settingsForm)) as TgSettings;
    settingsUpdatedAt.value = res?.updatedAt ?? Math.floor(Date.now() / 1000);
    message.success('保存成功；tgbot 将在 30 秒内自动拉取新配置');
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  } finally {
    settingsSaving.value = false;
  }
}

const userColumns = [
  { title: '#', dataIndex: '_rank', width: 60 },
  { title: 'JoinCare uid', dataIndex: 'uid', width: 130 },
  { title: 'TG @', dataIndex: 'tg_username', width: 160 },
  { title: '钱包', dataIndex: 'wallet_address', width: 170 },
  { title: 'Twitter', dataIndex: 'twitter_handle', width: 140 },
  {
    title: '总消息',
    dataIndex: 'total_messages',
    width: 110,
    align: 'right' as const,
  },
  {
    title: '活跃天数',
    dataIndex: 'active_days',
    width: 110,
    align: 'right' as const,
  },
  { title: '首次活跃', dataIndex: 'first_active_day', width: 130 },
  { title: '最近活跃', dataIndex: 'last_active_day', width: 130 },
];

function shortAddr(s: string, head = 6, tail = 4) {
  if (!s) return '-';
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

const dailyColumns = [
  { title: '日期', dataIndex: 'day', width: 130 },
  { title: '群 chat_id', dataIndex: 'chat_id', width: 180 },
  { title: '群名', dataIndex: 'chat_title', ellipsis: true },
  {
    title: '活跃用户',
    dataIndex: 'active_users',
    width: 110,
    align: 'right' as const,
  },
  {
    title: '总消息数',
    dataIndex: 'total_messages',
    width: 110,
    align: 'right' as const,
  },
  {
    title: '已绑定 uid',
    dataIndex: 'binded_users',
    width: 120,
    align: 'right' as const,
  },
  { title: '操作', dataIndex: '_action', width: 100, fixed: 'right' as const },
];

const detailColumns = [
  { title: 'TG uid', dataIndex: 'tg_user_id', width: 130 },
  { title: 'TG @', dataIndex: 'tg_username', width: 160 },
  { title: 'JoinCare uid', dataIndex: 'uid', width: 130 },
  {
    title: '消息数',
    dataIndex: 'message_count',
    width: 110,
    align: 'right' as const,
  },
  { title: '首次', dataIndex: 'first_msg_at', width: 200 },
  { title: '最后', dataIndex: 'last_msg_at', width: 200 },
];

onMounted(reload);
</script>

<template>
  <Page
    title="TG 活跃监控"
    description="按天 × 群统计活跃用户数（发言 ≥1 条即算 1 个活跃）；同 (chat, tg_user, day) UPSERT 累加 message_count"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="tgbot 收到实质消息（≥3 字符）就上报，独立于 daily_cap / 是否绑定；已绑定 joincare uid 的行 uid 非空。默认展示近 30 天"
    />

    <div class="mb-3 grid grid-cols-3 gap-4">
      <Card size="small">
        <Statistic title="今日活跃用户" :value="todayStat.active" />
      </Card>
      <Card size="small">
        <Statistic title="今日总消息" :value="todayStat.msgs" />
      </Card>
      <Card size="small">
        <Statistic title="今日已绑定活跃" :value="todayStat.bound" />
      </Card>
    </div>

    <Tabs v-model:active-key="activeTab" @change="onTabChange">
      <Tabs.TabPane key="group" tab="群维度（每日）">
        <Card>
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <DatePicker
              v-model:value="filter.from"
              placeholder="起 (YYYY-MM-DD)"
              value-format="YYYY-MM-DD"
              style="width: 160px"
              @change="reload"
            />
            <DatePicker
              v-model:value="filter.to"
              placeholder="止 (YYYY-MM-DD)"
              value-format="YYYY-MM-DD"
              style="width: 160px"
              @change="reload"
            />
            <Input
              v-model:value="filter.chat_id"
              placeholder="chat_id（可选）"
              style="width: 220px"
              @press-enter="reload"
            />
            <Button :loading="loading" @click="reload">刷新</Button>
            <span class="ml-auto text-sm text-gray-500">共 {{ rows.length }} 行 (日 × 群)</span>
          </div>

          <Table
            :columns="dailyColumns"
            :data-source="rows"
            :loading="loading"
            :pagination="{ pageSize: 30 }"
            :row-key="(r: any) => `${r.day}-${r.chat_id}`"
            size="middle"
            bordered
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'active_users'">
                <span style="font-size: 15px; font-weight: 700; color: #1aa64a">
                  {{ record.active_users }}
                </span>
              </template>
              <template v-else-if="column.dataIndex === 'chat_id'">
                <code style="font-size: 12px">{{ record.chat_id }}</code>
              </template>
              <template v-else-if="column.dataIndex === '_action'">
                <Button
                  size="small"
                  type="link"
                  @click="openDetail(record as TgDailyStat)"
                >
                  详情
                </Button>
              </template>
            </template>
          </Table>
        </Card>
      </Tabs.TabPane>

      <Tabs.TabPane key="user" tab="用户维度（按群+时段）">
        <Card>
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <span class="text-sm">群：</span>
            <Select
              v-model:value="userFilter.chat_id"
              placeholder="选择 TG 群"
              style="width: 320px"
              :options="
                chats.map((c) => ({
                  value: c.chat_id,
                  label: `${c.chat_title || `chat#${c.chat_id}`} (${c.days} 天)`,
                }))
              "
              @change="
                () => {
                  userPage = 1;
                  loadUsers();
                }
              "
            />
            <DatePicker
              v-model:value="userFilter.from"
              placeholder="起"
              value-format="YYYY-MM-DD"
              style="width: 140px"
              @change="
                () => {
                  userPage = 1;
                  loadUsers();
                }
              "
            />
            <DatePicker
              v-model:value="userFilter.to"
              placeholder="止"
              value-format="YYYY-MM-DD"
              style="width: 140px"
              @change="
                () => {
                  userPage = 1;
                  loadUsers();
                }
              "
            />
            <Select
              v-model:value="userFilter.sort"
              style="width: 180px"
              :options="[
                { value: 'msgs_desc', label: '按消息数降序' },
                { value: 'days_desc', label: '按活跃天数降序' },
                { value: 'recent_desc', label: '按最近活跃' },
              ]"
              @change="
                () => {
                  userPage = 1;
                  loadUsers();
                }
              "
            />
            <span class="text-sm ml-2">仅注册用户</span>
            <Switch
              :checked="userFilter.bound_only === '1'"
              @update:checked="
                (v: any) => {
                  userFilter.bound_only = v ? '1' : '0';
                  userPage = 1;
                  loadUsers();
                }
              "
            />
            <Button :loading="userLoading" @click="loadUsers">刷新</Button>
            <span class="ml-auto text-sm text-gray-500">共 {{ userTotal }} 人</span>
          </div>

          <Alert
            v-if="!userFilter.chat_id"
            type="warning"
            show-icon
            message="请先选择一个 TG 群（下拉框仅展示已收集到消息的群；如果为空，说明 tgbot 还没上报）"
          />

          <Table
            v-else
            :columns="userColumns"
            :data-source="userRows"
            :loading="userLoading"
            :pagination="{
              current: userPage,
              pageSize: userPageSize,
              total: userTotal,
              showSizeChanger: true,
              onChange: (p: number, ps: number) => {
                userPage = p;
                userPageSize = ps;
                loadUsers();
              },
            }"
            :row-key="(r: any) => r.tg_user_id"
            size="small"
            bordered
            :scroll="{ x: 1500 }"
          >
            <template #bodyCell="{ column, record, index }">
              <template v-if="column.dataIndex === '_rank'">
                <Tag
                  v-if="userPage === 1 && index < 3"
                  :color="['gold', 'default', 'volcano'][index]"
                  style="font-weight: 700"
                >
                  #{{ (userPage - 1) * userPageSize + index + 1 }}
                </Tag>
                <span v-else>{{
                  (userPage - 1) * userPageSize + index + 1
                }}</span>
              </template>
              <template v-else-if="column.dataIndex === 'uid'">
                <Tag v-if="record.uid" color="green">#{{ record.uid }}</Tag>
                <span v-else class="text-gray-400">未绑定</span>
              </template>
              <template v-else-if="column.dataIndex === 'tg_username'">
                {{ record.tg_username ? `@${record.tg_username}` : '-' }}
                <span class="ml-2 text-xs text-gray-500">tg:{{ record.tg_user_id }}</span>
              </template>
              <template v-else-if="column.dataIndex === 'wallet_address'">
                <code
                  v-if="record.wallet_address"
                  :title="record.wallet_address"
                  style="font-size: 12px"
                  >{{ shortAddr(record.wallet_address) }}</code>
                <span v-else class="text-gray-400">-</span>
              </template>
              <template v-else-if="column.dataIndex === 'twitter_handle'">
                {{ record.twitter_handle ? `@${record.twitter_handle}` : '-' }}
              </template>
              <template v-else-if="column.dataIndex === 'total_messages'">
                <span style="font-size: 15px; font-weight: 700; color: #1aa64a">
                  {{ record.total_messages }}
                </span>
              </template>
              <template v-else-if="column.dataIndex === 'active_days'">
                <span style="font-weight: 600">{{ record.active_days }}</span>
                <span class="text-xs text-gray-500"> 天</span>
              </template>
            </template>
          </Table>
        </Card>
      </Tabs.TabPane>

      <Tabs.TabPane key="settings" tab="配置（后台可改）">
        <Card :loading="settingsLoading">
          <Alert
            type="warning"
            show-icon
            banner
            class="mb-3"
            message="修改后 tgbot 每 30 秒自动拉取一次；chat_id 从 tgbot 日志的 seen msg 里读到。切换群时只需改 chat_id + chat_link，无需重启 tgbot"
          />
          <div class="grid grid-cols-1 gap-4" style="max-width: 640px">
            <div>
              <div class="mb-1 text-sm text-gray-500">
                群 chat_id（负数：group -xxxx，supergroup -100xxxx）
              </div>
              <InputNumber
                v-model:value="settingsForm.chatId"
                :controls="false"
                placeholder="-5322296762"
                style="width: 100%"
              />
            </div>
            <div>
              <div class="mb-1 text-sm text-gray-500">
                群邀请链接（bot 绑定成功后引导用户进群用）
              </div>
              <Input
                v-model:value="settingsForm.chatLink"
                placeholder="https://t.me/+xxxxxxxxxxxxxxxxxxxxx"
              />
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <div class="mb-1 text-sm text-gray-500">
                  每条消息奖励积分 messagePoints
                </div>
                <InputNumber
                  v-model:value="settingsForm.messagePoints"
                  :min="0"
                  :max="100000"
                  style="width: 100%"
                />
              </div>
              <div>
                <div class="mb-1 text-sm text-gray-500">
                  每人每日封顶次数 dailyCap
                </div>
                <InputNumber
                  v-model:value="settingsForm.dailyCap"
                  :min="1"
                  :max="100000"
                  style="width: 100%"
                />
              </div>
            </div>
            <div class="flex items-center gap-3">
              <Button
                type="primary"
                :loading="settingsSaving"
                @click="saveSettings"
              >
                保存
              </Button>
              <Button @click="loadSettings">重载</Button>
              <span class="text-xs text-gray-400">
                {{
                  settingsUpdatedAt
                    ? new Date(settingsUpdatedAt * 1000).toLocaleString()
                    : '未加载'
                }}
              </span>
            </div>
          </div>
        </Card>
      </Tabs.TabPane>
    </Tabs>

    <Modal
      v-model:open="detailVisible"
      :title="`活跃详情: ${detailTitle}`"
      width="900px"
      :footer="null"
    >
      <div class="mb-3 flex items-center gap-3">
        <span>仅显示已绑定用户</span>
        <Switch
          :checked="detailBoundOnly"
          @update:checked="
            (v: any) => {
              detailBoundOnly = v;
              detailPage = 1;
              loadDetail();
            }
          "
        />
        <span class="ml-auto text-sm text-gray-500">共 {{ detailTotal }} 人</span>
      </div>
      <Table
        :columns="detailColumns"
        :data-source="detailRows"
        :loading="detailLoading"
        :pagination="{
          current: detailPage,
          pageSize: detailPageSize,
          total: detailTotal,
          showSizeChanger: true,
          onChange: (p: number, ps: number) => {
            detailPage = p;
            detailPageSize = ps;
            loadDetail();
          },
        }"
        :row-key="(r: any) => r.id"
        size="small"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'uid'">
            <Tag v-if="record.uid" color="green">#{{ record.uid }}</Tag>
            <span v-else class="text-gray-400">未绑定</span>
          </template>
          <template v-else-if="column.dataIndex === 'tg_username'">
            {{ record.tg_username ? `@${record.tg_username}` : '-' }}
          </template>
          <template v-else-if="column.dataIndex === 'message_count'">
            <span style="font-weight: 600">{{ record.message_count }}</span>
          </template>
        </template>
      </Table>
    </Modal>
  </Page>
</template>
