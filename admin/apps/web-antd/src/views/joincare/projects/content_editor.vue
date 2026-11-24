<script setup lang="ts">
import type {
  DocumentItem,
  FaqItem,
  OnchainItem,
  ProjectContent,
  RoadmapItem,
  TeamMember,
  TokenomicsData,
} from '#/api/joincare/project_content';

import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  InputNumber,
  message,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Textarea,
  Upload,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  getProjectContent,
  updateProjectContent,
  updateProjectMinPurchase,
  uploadFile,
} from '#/api/joincare/project_content';
import RichEditor from '#/components/joincare/RichEditor.vue';

import { supportsMinPurchasePolicy } from './min-purchase-policy';

const route = useRoute();
const projectId = ref(String(route.query.id ?? route.params.id ?? ''));

const loading = ref(false);
const saving = ref(false);
const activeTab = ref('basic');
const fundingMode = ref('');
const contractVersion = ref('');
const projectStatus = ref('');
const minPurchaseAmount = ref('');
const showMinPurchasePolicy = computed(
  () =>
    supportsMinPurchasePolicy(fundingMode.value, contractVersion.value) ||
    isSetupMinPurchase.value,
);
const savingMinPurchase = ref(false);
const paymentToken = ref('');
const isSetupMinPurchase = computed(
  () =>
    projectStatus.value === 'Setup' &&
    fundingMode.value === 'capped_presale' &&
    contractVersion.value === 'unassigned',
);

// —— 表单状态 —— 所有 tab 数据存这里 ———————————————————————
const form = reactive({
  // Basic tab
  name_en: '',
  name_cn: '',
  status: 'pre_launch',
  category: '',
  cover_url: '',
  cover_video_url: '',
  launch_at: null as null | string, // ISO or null

  // Tagline (双语)
  tagline: '',
  tagline_zh: '',
  description: '',
  tags: [] as string[],

  // Story / Research / Risks 富文本 (双语)
  story_html: '',
  research_html: '',
  risks_html: '',
  story_html_zh: '',
  research_html_zh: '',
  risks_html_zh: '',

  // 结构化
  team: [] as TeamMember[],
  tokenomics: {
    allocations: [],
    tge_percent: 0,
    cliff_months: 0,
    linear_months: 0,
  } as TokenomicsData,
  roadmap: [] as RoadmapItem[],
  documents: [] as DocumentItem[],
  faq: [] as FaqItem[],
  onchain: [] as OnchainItem[],
});

const statusOptions = [
  { label: 'Pre-launch', value: 'pre_launch' },
  { label: 'Warmup / Whitelist', value: 'warmup' },
  { label: 'Active (Phase 2)', value: 'active', disabled: true },
  { label: 'Sold-out', value: 'sold_out', disabled: true },
  { label: 'Ended (Success)', value: 'ended_success', disabled: true },
  { label: 'Ended (Failed)', value: 'ended_failed', disabled: true },
  { label: 'Vesting / Delivery', value: 'vesting', disabled: true },
  { label: 'Archived', value: 'archived', disabled: true },
];

const categoryOptions = [
  'antibody',
  'neuro',
  'compound',
  'data',
  'rare-disease',
  'diagnostics',
  'device',
  'other',
].map((v) => ({ label: v, value: v }));

// —— 加载 ———————————————————————————————————————————————
async function load() {
  if (!projectId.value) {
    message.warning('请在 URL 里带 ?id=<project_id>');
    return;
  }
  loading.value = true;
  try {
    const res = (await getProjectContent(projectId.value)) as ProjectContent;
    form.name_en = res.name_en || '';
    form.name_cn = res.name_cn || '';
    form.status = res.status || 'pre_launch';
    fundingMode.value = res.funding_mode || '';
    contractVersion.value = res.contract_version || '';
    projectStatus.value = res.status || '';
    minPurchaseAmount.value = isSetupMinPurchase.value
      ? res.min_purchase_draft_amount || ''
      : res.min_purchase_amount || '';
    paymentToken.value = isSetupMinPurchase.value
      ? res.min_purchase_draft_token || ''
      : res.payment_token || '';
    form.category = res.category || '';
    form.cover_url = res.cover_url || '';
    form.cover_video_url = res.cover_video_url || '';
    form.launch_at = res.launch_at || null;
    form.tagline = res.tagline || '';
    form.tagline_zh = res.tagline_zh || '';
    form.description = res.description || '';
    form.tags = safeParse<string[]>(res.tags, []);
    form.story_html = res.story_html || '';
    form.research_html = res.research_html || '';
    form.risks_html = res.risks_html || '';
    form.story_html_zh = res.story_html_zh || '';
    form.research_html_zh = res.research_html_zh || '';
    form.risks_html_zh = res.risks_html_zh || '';
    form.team = safeParse<TeamMember[]>(res.team_json, []);
    form.tokenomics = safeParse<TokenomicsData>(res.tokenomics_json, {
      allocations: [],
      tge_percent: 0,
      cliff_months: 0,
      linear_months: 0,
    });
    form.roadmap = safeParse<RoadmapItem[]>(res.roadmap_json, []);
    form.documents = safeParse<DocumentItem[]>(res.documents_json, []);
    form.faq = safeParse<FaqItem[]>(res.faq_json, []);
    form.onchain = safeParse<OnchainItem[]>(res.onchain_json, []);
  } catch (error: any) {
    message.error(error?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

function safeParse<T>(raw: string, fallback: T): T {
  if (!raw) return fallback;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

// Independent from the locked on-chain Set form and marketing content save.
async function saveMinPurchase() {
  if (!projectId.value) return;
  savingMinPurchase.value = true;
  try {
    await updateProjectMinPurchase(
      projectId.value,
      minPurchaseAmount.value.trim(),
      isSetupMinPurchase.value ? paymentToken.value : '',
    );
    message.success('官方页面每笔最低参与额已保存');
    await load();
  } catch (error: any) {
    message.error(error?.message || '最低参与额保存失败');
  } finally {
    savingMinPurchase.value = false;
  }
}

// —— 保存 —— 一次性提交所有字段 ———————————————————————
async function save() {
  if (!projectId.value) return;
  if (!form.name_en.trim() || !form.name_cn.trim()) {
    message.warning(
      'English and Traditional Chinese project names are required',
    );
    return;
  }
  saving.value = true;
  try {
    await updateProjectContent(projectId.value, {
      name_en: form.name_en,
      name_cn: form.name_cn,
      category: form.category,
      cover_url: form.cover_url,
      cover_video_url: form.cover_video_url,
      launch_at: form.launch_at || '', // "" = 清空
      tagline: form.tagline,
      description: form.description,
      tags: form.tags.map((tag) => tag.trim()).filter(Boolean),
      story_html: form.story_html,
      research_html: form.research_html,
      risks_html: form.risks_html,
      tagline_zh: form.tagline_zh,
      story_html_zh: form.story_html_zh,
      research_html_zh: form.research_html_zh,
      risks_html_zh: form.risks_html_zh,
      team_json: form.team,
      tokenomics_json: form.tokenomics,
      roadmap_json: form.roadmap,
      documents_json: form.documents,
      faq_json: form.faq,
    });
    message.success('保存成功');
  } catch (error: any) {
    message.error(error?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

// —— Cover 图上传 ———————————————————————————————————————
const coverUploading = ref(false);
async function onCoverChange(info: any) {
  const file = info?.file?.originFileObj || info?.file;
  if (!file) return;
  coverUploading.value = true;
  try {
    const res = (await uploadFile(file)) as any;
    if (res?.url) {
      form.cover_url = res.url;
      await updateProjectContent(projectId.value, { cover_url: res.url });
      message.success('Cover 上传并保存成功');
    }
  } catch (error: any) {
    message.error(error?.message || '上传失败');
  } finally {
    coverUploading.value = false;
  }
}

// —— Documents 上传 ———————————————————————————————————————
async function onDocumentUpload(info: any) {
  const file = info?.file?.originFileObj || info?.file;
  if (!file) return;
  try {
    const res = (await uploadFile(file)) as any;
    if (res?.url) {
      form.documents.push({
        title: file.name,
        url: res.url,
        kind: guessKind(file.name),
      });
    }
  } catch (error: any) {
    message.error(error?.message || '上传失败');
  }
}
function guessKind(name: string): string {
  const s = name.toLowerCase();
  if (s.includes('whitepaper')) return 'whitepaper';
  if (s.includes('audit')) return 'audit';
  if (s.endsWith('.pdf')) return 'paper';
  return 'other';
}

// —— 结构化子行 helpers ———————————————————————————————————
function addTeam() {
  form.team.push({
    name: '',
    name_zh: '',
    role: '',
    role_zh: '',
    bio: '',
    bio_zh: '',
    links: [],
  });
}
function removeTeam(i: number) {
  form.team.splice(i, 1);
}

function addTokenomicsRow() {
  form.tokenomics.allocations.push({ label: '', label_zh: '', percent: 0 });
}
function removeTokenomicsRow(i: number) {
  form.tokenomics.allocations.splice(i, 1);
}

function addRoadmap() {
  form.roadmap.push({
    quarter: '',
    quarter_zh: '',
    title: '',
    title_zh: '',
    desc: '',
    desc_zh: '',
    status: 'planned',
  });
}
function removeRoadmap(i: number) {
  form.roadmap.splice(i, 1);
}

function addFaq() {
  form.faq.push({ q: '', q_zh: '', a: '', a_zh: '' });
}
function removeFaq(i: number) {
  form.faq.splice(i, 1);
}

function removeDocument(i: number) {
  form.documents.splice(i, 1);
}

// —— launch_at Date <-> ISO 互转 ———————————————————————
// Do not use a writable computed as DatePicker's v-model here. With show-time,
// the picker can emit its previous Dayjs value again while closing after a
// clear action, which silently restores the timestamp before Save is clicked.
const launchAtValue = computed(() =>
  form.launch_at ? dayjs(form.launch_at) : undefined,
);
const launchAtClearPending = ref(false);

function updateLaunchAt(value: any) {
  if (!value) {
    launchAtClearPending.value = true;
    form.launch_at = null;
    return;
  }
  // Ignore a stale value emitted while DatePicker closes after an explicit
  // clear. Opening the picker again signals a deliberate new edit.
  if (launchAtClearPending.value) return;
  form.launch_at = value.toISOString();
}

function clearLaunchAt() {
  launchAtClearPending.value = true;
  form.launch_at = null;
}

function onLaunchAtOpenChange(open: boolean) {
  if (open) launchAtClearPending.value = false;
}

// —— Tokenomics 总占比展示 ———————————————————————
const totalAlloc = computed(() =>
  form.tokenomics.allocations.reduce(
    (sum, a) => sum + (Number(a.percent) || 0),
    0,
  ),
);

onMounted(load);
watch(
  () => route.query.id,
  (v) => {
    if (v && String(v) !== projectId.value) {
      projectId.value = String(v);
      load();
    }
  },
);
</script>

<template>
  <Page title="项目内容编辑" :description="`project_id: ${projectId}`">
    <Card :loading="loading">
      <template #title>
        <span>#{{ projectId }}</span>
        <Tag class="ml-2" color="green">
          {{
            statusOptions.find((o) => o.value === form.status)?.label ||
            form.status
          }}
        </Tag>
      </template>
      <template #extra>
        <Space>
          <span style="font-size: 12px; color: var(--ant-color-text-secondary)">
            📝 English 主 + 繁體中文 副 · 繁中未填时 C 端 fallback 到 English
          </span>
          <Button @click="load">重载</Button>
          <Button type="primary" :loading="saving" @click="save">
            保存全部
          </Button>
        </Space>
      </template>

      <Alert
        v-if="
          fundingMode === 'fair_launch' || contractVersion === 'fair_launch_v1'
        "
        type="warning"
        show-icon
        class="mb-3"
        message="Fair Launch Phase 1：此页仅编辑营销展示内容"
        description="项目生命周期状态、链上参数和合约动作均不由本页修改。"
      />

      <Tabs v-model:active-key="activeTab">
        <Tabs.TabPane
          v-if="showMinPurchasePolicy"
          key="ui-policy"
          tab="官方页面购买策略"
        >
          <Alert
            type="warning"
            show-icon
            class="mb-4"
            message="仅限制官方页面每笔购买，不是链上参数"
            :description="
              isSetupMinPurchase
                ? 'Setup 阶段先保存意向金额及支付币；Set 时会与可信支付币精度、单钱包上限一起校验，不通过则不能进入 Set。直接调用合约不受此页面策略约束。'
                : '直接调用合约可绕过；已上链成功的交易仍正常记账。此策略可在项目上链锁定后独立修改。'
            "
          />
          <div v-if="isSetupMinPurchase" class="mb-3" style="max-width: 320px">
            <div class="label">计划支付币</div>
            <Select
              v-model:value="paymentToken"
              placeholder="请选择 USDT / USDC"
              style="width: 100%"
            >
              <Select.Option value="USDT">USDT</Select.Option>
              <Select.Option value="USDC">USDC</Select.Option>
            </Select>
          </div>
          <div class="label">
            每笔最低支付金额（{{
              paymentToken || '请选择支付币'
            }}；留空表示不限制）
          </div>
          <Input
            v-model:value="minPurchaseAmount"
            style="max-width: 320px"
            placeholder="例如 10.5；留空关闭"
          />
          <Button
            type="primary"
            class="ml-3"
            :loading="savingMinPurchase"
            @click="saveMinPurchase"
          >
            单独保存页面策略
          </Button>
        </Tabs.TabPane>
        <!-- ============ Basic ============ -->
        <Tabs.TabPane key="basic" tab="Basic">
          <div class="grid grid-cols-2 gap-4" style="max-width: 900px">
            <div class="col-span-2">
              <div class="label">Project name</div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <div class="sub-label">
                    <span class="lang-chip lang-en">EN</span> English
                  </div>
                  <Input
                    v-model:value="form.name_en"
                    placeholder="Project name in English"
                  />
                </div>
                <div>
                  <div class="sub-label">
                    <span class="lang-chip lang-zh">繁中</span> Traditional
                    Chinese
                  </div>
                  <Input
                    v-model:value="form.name_cn"
                    placeholder="繁體中文項目名稱"
                  />
                </div>
              </div>
            </div>
            <div>
              <div class="label">状态 status</div>
              <Select
                v-model:value="form.status"
                :options="statusOptions"
                disabled
                style="width: 100%"
              />
              <div class="sub-label">
                生命周期状态只读；请使用授权的专用流程。
              </div>
            </div>
            <div>
              <div class="label">分类 category</div>
              <Select
                v-model:value="form.category"
                :options="categoryOptions"
                allow-clear
                placeholder="antibody / neuro / ..."
                style="width: 100%"
              />
            </div>
            <div class="col-span-2">
              <div class="label">
                Cover 图 URL（可上传或粘贴外链）
                <Upload
                  :show-upload-list="false"
                  :before-upload="() => false"
                  @change="onCoverChange"
                >
                  <Button size="small" class="ml-2" :loading="coverUploading">
                    上传
                  </Button>
                </Upload>
              </div>
              <Input
                v-model:value="form.cover_url"
                placeholder="/uploads/2026-07/xxx.png 或 https://..."
              />
              <img
                v-if="form.cover_url"
                :src="form.cover_url"
                alt="cover preview"
                class="mt-2 rounded"
                style="max-width: 240px; max-height: 120px; object-fit: cover"
              />
            </div>
            <div class="col-span-2">
              <div class="label">Pitch 视频 URL（可选，YouTube / Vimeo）</div>
              <Input
                v-model:value="form.cover_video_url"
                placeholder="https://youtu.be/..."
              />
            </div>
            <div>
              <div class="label">计划开售时间 launch_at</div>
              <Space.Compact style="width: 100%">
                <DatePicker
                  :value="launchAtValue"
                  :allow-clear="false"
                  input-read-only
                  show-time
                  style="width: 100%"
                  placeholder="Pre-launch 阶段倒计时用"
                  @open-change="onLaunchAtOpenChange"
                  @update:value="updateLaunchAt"
                />
                <Button :disabled="!form.launch_at" @click="clearLaunchAt">
                  清空时间
                </Button>
              </Space.Compact>
            </div>
            <div class="col-span-2">
              <div class="label">Tagline · 双语并排</div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <div class="sub-label">
                    <span class="lang-chip lang-en">EN</span> English (主)
                  </div>
                  <Input
                    v-model:value="form.tagline"
                    placeholder="One-liner that captures the project positioning"
                  />
                </div>
                <div>
                  <div class="sub-label">
                    <span class="lang-chip lang-zh">繁中</span> 繁體中文 (副 ·
                    未填時 C 端 fallback)
                  </div>
                  <Input
                    v-model:value="form.tagline_zh"
                    placeholder="一句話說明項目定位"
                  />
                </div>
              </div>
            </div>
            <div class="col-span-2">
              <div class="label">
                Overview / Description (active-stage detail)
              </div>
              <Textarea
                v-model:value="form.description"
                :rows="5"
                placeholder="Project overview shown on the active-stage detail page"
              />
            </div>
            <div class="col-span-2">
              <div class="label">Tags</div>
              <Select
                v-model:value="form.tags"
                mode="tags"
                :token-separators="[',']"
                placeholder="Type a tag and press Enter"
                style="width: 100%"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Story · 双语并排 ============ -->
        <Tabs.TabPane key="story" tab="Story">
          <div class="grid grid-cols-2 gap-4">
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-en">EN</span> English (主)
              </div>
              <RichEditor
                v-model="form.story_html"
                placeholder="Problem — Solution — Why now — Founder's letter..."
                min-height="420px"
              />
            </div>
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-zh">繁中</span> 繁體中文 (副 ·
                未填時 C 端 fallback)
              </div>
              <RichEditor
                v-model="form.story_html_zh"
                placeholder="問題 — 解決方案 — 為什麼是現在 — 創始人的信..."
                min-height="420px"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Research · 双语并排 ============ -->
        <Tabs.TabPane key="research" tab="Research & Data">
          <Alert
            type="info"
            show-icon
            banner
            class="mb-3"
            message="DeSci 核心板块：Hypothesis / Methodology / Preliminary data / IP status / Regulatory path / Open-science links"
          />
          <div class="grid grid-cols-2 gap-4">
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-en">EN</span> English (主)
              </div>
              <RichEditor
                v-model="form.research_html"
                placeholder="Research question / Methodology / Publications ..."
                min-height="420px"
              />
            </div>
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-zh">繁中</span> 繁體中文 (副 ·
                未填時 C 端 fallback)
              </div>
              <RichEditor
                v-model="form.research_html_zh"
                placeholder="研究問題 / 方法論 / 已發表論文 / IP 進度 / 監管路徑..."
                min-height="420px"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Team ============ -->
        <Tabs.TabPane key="team" tab="Team">
          <div class="mb-2 flex justify-between items-center">
            <span class="text-sm text-gray-500">
              Founders / Core / Advisors 一起列，前端会按 role 分组渲染
            </span>
            <Button type="primary" size="small" @click="addTeam">
              + 添加成员
            </Button>
          </div>
          <Table
            :data-source="form.team"
            :pagination="false"
            row-key="(_, i) => i"
            :scroll="{ x: 1400 }"
            :columns="[
              { title: '姓名', dataIndex: 'name', width: 160 },
              { title: '角色', dataIndex: 'role', width: 200 },
              { title: '头像 URL', dataIndex: 'avatar_url', width: 260 },
              { title: 'Bio (一句话)', dataIndex: 'bio' },
              { title: '操作', dataIndex: '_action', width: 80 },
            ]"
          >
            <template #bodyCell="{ column, index }">
              <template v-if="column.dataIndex === 'name'">
                <Input v-model:value="form.team[index]!.name" />
              </template>
              <template v-else-if="column.dataIndex === 'role'">
                <Input
                  v-model:value="form.team[index]!.role"
                  placeholder="Founder / CTO / Advisor / ..."
                />
              </template>
              <template v-else-if="column.dataIndex === 'avatar_url'">
                <Input
                  v-model:value="form.team[index]!.avatar_url"
                  placeholder="/uploads/xxx.png"
                />
              </template>
              <template v-else-if="column.dataIndex === 'bio'">
                <Input v-model:value="form.team[index]!.bio" />
              </template>
              <template v-else-if="column.dataIndex === '_action'">
                <Button size="small" danger @click="removeTeam(index)">
                  删
                </Button>
              </template>
            </template>
          </Table>
          <div
            class="mt-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese
              fields
            </div>
            <div
              v-for="(row, i) in form.team"
              :key="`team-zh-${i}`"
              class="mb-3 grid grid-cols-3 gap-3"
            >
              <Input
                v-model:value="row.name_zh"
                :placeholder="`Member #${i + 1} name TW`"
              />
              <Input
                v-model:value="row.role_zh"
                :placeholder="`Member #${i + 1} role TW`"
              />
              <Input
                v-model:value="row.bio_zh"
                :placeholder="`Member #${i + 1} bio TW`"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Tokenomics ============ -->
        <Tabs.TabPane key="tokenomics" tab="Tokenomics">
          <div class="grid grid-cols-3 gap-4 mb-3" style="max-width: 720px">
            <div>
              <div class="label">TGE 释放 %</div>
              <InputNumber
                v-model:value="form.tokenomics.tge_percent"
                :min="0"
                :max="100"
                style="width: 100%"
              />
            </div>
            <div>
              <div class="label">Cliff (月)</div>
              <InputNumber
                v-model:value="form.tokenomics.cliff_months"
                :min="0"
                :max="120"
                style="width: 100%"
              />
            </div>
            <div>
              <div class="label">Linear vesting (月)</div>
              <InputNumber
                v-model:value="form.tokenomics.linear_months"
                :min="0"
                :max="120"
                style="width: 100%"
              />
            </div>
          </div>
          <div class="mb-2 flex justify-between items-center">
            <span class="text-sm text-gray-500">
              分配饼 · 总占比 {{ totalAlloc }} %
              <span v-if="totalAlloc !== 100" class="text-orange-500 ml-2">
                （建议合计 100）
              </span>
            </span>
            <Button type="primary" size="small" @click="addTokenomicsRow">
              + 添加分配项
            </Button>
          </div>
          <Table
            :data-source="form.tokenomics.allocations"
            :pagination="false"
            row-key="(_, i) => i"
            :columns="[
              { title: '类别', dataIndex: 'label', width: 260 },
              { title: '占比 %', dataIndex: 'percent', width: 130 },
              { title: '色号 (可选)', dataIndex: 'color', width: 160 },
              { title: '操作', dataIndex: '_action', width: 80 },
            ]"
          >
            <template #bodyCell="{ column, index }">
              <template v-if="column.dataIndex === 'label'">
                <Input
                  v-model:value="form.tokenomics.allocations[index]!.label"
                  placeholder="Community / Team / Treasury / ..."
                />
              </template>
              <template v-else-if="column.dataIndex === 'percent'">
                <InputNumber
                  v-model:value="form.tokenomics.allocations[index]!.percent"
                  :min="0"
                  :max="100"
                  style="width: 100%"
                />
              </template>
              <template v-else-if="column.dataIndex === 'color'">
                <Input
                  v-model:value="form.tokenomics.allocations[index]!.color"
                  placeholder="#74f5a1"
                />
              </template>
              <template v-else-if="column.dataIndex === '_action'">
                <Button size="small" danger @click="removeTokenomicsRow(index)">
                  删
                </Button>
              </template>
            </template>
          </Table>
          <div
            class="mt-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese
              allocation labels
            </div>
            <div
              v-for="(row, i) in form.tokenomics.allocations"
              :key="`tokenomics-zh-${i}`"
              class="mb-3"
            >
              <Input
                v-model:value="row.label_zh"
                :placeholder="`Allocation #${i + 1} label TW`"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Roadmap ============ -->
        <Tabs.TabPane key="roadmap" tab="Roadmap">
          <div class="mb-2 flex justify-between items-center">
            <span class="text-sm text-gray-500">
              时间线：done → active → planned
            </span>
            <Button type="primary" size="small" @click="addRoadmap">
              + 添加里程碑
            </Button>
          </div>
          <Table
            :data-source="form.roadmap"
            :pagination="false"
            row-key="(_, i) => i"
            :columns="[
              { title: 'Quarter', dataIndex: 'quarter', width: 140 },
              { title: '标题', dataIndex: 'title', width: 260 },
              { title: '描述', dataIndex: 'desc' },
              { title: '状态', dataIndex: 'status', width: 130 },
              { title: '操作', dataIndex: '_action', width: 80 },
            ]"
          >
            <template #bodyCell="{ column, index }">
              <template v-if="column.dataIndex === 'quarter'">
                <Input
                  v-model:value="form.roadmap[index]!.quarter"
                  placeholder="2026 Q3"
                />
              </template>
              <template v-else-if="column.dataIndex === 'title'">
                <Input v-model:value="form.roadmap[index]!.title" />
              </template>
              <template v-else-if="column.dataIndex === 'desc'">
                <Input v-model:value="form.roadmap[index]!.desc" />
              </template>
              <template v-else-if="column.dataIndex === 'status'">
                <Select
                  v-model:value="form.roadmap[index]!.status"
                  :options="[
                    { label: 'Done', value: 'done' },
                    { label: 'Active', value: 'active' },
                    { label: 'Planned', value: 'planned' },
                  ]"
                  style="width: 100%"
                />
              </template>
              <template v-else-if="column.dataIndex === '_action'">
                <Button size="small" danger @click="removeRoadmap(index)">
                  删
                </Button>
              </template>
            </template>
          </Table>
          <div
            class="mt-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese
              roadmap fields
            </div>
            <div
              v-for="(row, i) in form.roadmap"
              :key="`roadmap-zh-${i}`"
              class="mb-3 grid grid-cols-3 gap-3"
            >
              <Input
                v-model:value="row.quarter_zh"
                :placeholder="`Milestone #${i + 1} quarter TW`"
              />
              <Input
                v-model:value="row.title_zh"
                :placeholder="`Milestone #${i + 1} title TW`"
              />
              <Input
                v-model:value="row.desc_zh"
                :placeholder="`Milestone #${i + 1} description TW`"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ Documents ============ -->
        <Tabs.TabPane key="documents" tab="Documents">
          <div class="mb-3 flex justify-between items-center">
            <span class="text-sm text-gray-500">
              Whitepaper / 论文 / Audit report / 数据可用性声明 等
            </span>
            <Upload
              :show-upload-list="false"
              :before-upload="() => false"
              @change="onDocumentUpload"
            >
              <Button type="primary" size="small">+ 上传文件</Button>
            </Upload>
          </div>
          <Table
            :data-source="form.documents"
            :pagination="false"
            row-key="(_, i) => i"
            :columns="[
              { title: '标题', dataIndex: 'title', width: 260 },
              { title: 'URL', dataIndex: 'url' },
              { title: '类型', dataIndex: 'kind', width: 160 },
              { title: '操作', dataIndex: '_action', width: 80 },
            ]"
          >
            <template #bodyCell="{ column, index }">
              <template v-if="column.dataIndex === 'title'">
                <Input v-model:value="form.documents[index]!.title" />
              </template>
              <template v-else-if="column.dataIndex === 'url'">
                <Input
                  v-model:value="form.documents[index]!.url"
                  placeholder="/uploads/xxx.pdf 或外链"
                />
              </template>
              <template v-else-if="column.dataIndex === 'kind'">
                <Select
                  v-model:value="form.documents[index]!.kind"
                  :options="[
                    { label: 'Whitepaper', value: 'whitepaper' },
                    { label: 'Audit', value: 'audit' },
                    { label: 'Paper', value: 'paper' },
                    { label: 'Other', value: 'other' },
                  ]"
                  style="width: 100%"
                />
              </template>
              <template v-else-if="column.dataIndex === '_action'">
                <Button size="small" danger @click="removeDocument(index)">
                  删
                </Button>
              </template>
            </template>
          </Table>
          <div
            class="mt-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese
              document fields
            </div>
            <div
              v-for="(row, i) in form.documents"
              :key="`document-zh-${i}`"
              class="mb-3 grid grid-cols-2 gap-3"
            >
              <Input
                v-model:value="row.title_zh"
                :placeholder="`Document #${i + 1} title TW`"
              />
              <Input
                v-model:value="row.kind_zh"
                :placeholder="`Document #${i + 1} type TW`"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ FAQ ============ -->
        <Tabs.TabPane key="faq" tab="FAQ">
          <div
            class="mb-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese FAQ
              fields
            </div>
            <div v-for="(row, i) in form.faq" :key="`faq-zh-${i}`" class="mb-3">
              <Input
                v-model:value="row.q_zh"
                :placeholder="`FAQ #${i + 1} question TW`"
                class="mb-1"
              />
              <Textarea
                v-model:value="row.a_zh"
                :placeholder="`FAQ #${i + 1} answer TW`"
                :rows="3"
              />
            </div>
          </div>
          <div class="mb-3 flex justify-between items-center">
            <span class="text-sm text-gray-500">
              常见问题（时区 / KYC / claim 流程 / 项目终止后果 等）
            </span>
            <Button type="primary" size="small" @click="addFaq">
              + 添加 Q&A
            </Button>
          </div>
          <div v-for="(row, i) in form.faq" :key="i" class="mb-3">
            <div class="mb-1 flex items-center justify-between">
              <span class="text-xs text-gray-400">#{{ i + 1 }} 问答</span>
              <Button size="small" danger @click="removeFaq(i)">删</Button>
            </div>
            <Input v-model:value="row.q" placeholder="问题 (Q)" class="mb-1" />
            <Textarea v-model:value="row.a" placeholder="回答 (A)" :rows="3" />
          </div>
        </Tabs.TabPane>

        <!-- ============ Risks · 双语并排 ============ -->
        <Tabs.TabPane key="risks" tab="Risks">
          <Alert
            type="warning"
            show-icon
            banner
            class="mb-3"
            message="科学风险 / 监管风险 / 财务风险 / 免责声明 —— DeSci 项目必须明示，不隐瞒可能失败"
          />
          <div class="grid grid-cols-2 gap-4">
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-en">EN</span> English (主)
              </div>
              <RichEditor
                v-model="form.risks_html"
                placeholder="Regulatory risk / Scientific risk / Financial risk / Not investment advice ..."
                min-height="360px"
              />
            </div>
            <div>
              <div class="sub-label">
                <span class="lang-chip lang-zh">繁中</span> 繁體中文 (副 ·
                未填時 C 端 fallback)
              </div>
              <RichEditor
                v-model="form.risks_html_zh"
                placeholder="監管風險 / 科學風險 / 財務風險 / 非投資建議..."
                min-height="360px"
              />
            </div>
          </div>
        </Tabs.TabPane>

        <!-- ============ On-chain ============ -->
        <Tabs.TabPane key="onchain" tab="On-chain">
          <Alert
            type="warning"
            show-icon
            banner
            class="mb-3"
            message="On-chain 系统资料由 Set / 审批流程管理；内容编辑器仅提供只读预览，不会保存此区变更。"
          />
          <Table
            :data-source="form.onchain"
            :pagination="false"
            row-key="(_, i) => i"
            :columns="[
              { title: '标签', dataIndex: 'label', width: 220 },
              { title: '地址', dataIndex: 'address' },
              { title: '链', dataIndex: 'chain', width: 130 },
            ]"
          >
            <template #bodyCell="{ column, index }">
              <template v-if="column.dataIndex === 'label'">
                <Input
                  v-model:value="form.onchain[index]!.label"
                  disabled
                  placeholder="Project contract / Token / Treasury multisig"
                />
              </template>
              <template v-else-if="column.dataIndex === 'address'">
                <Input v-model:value="form.onchain[index]!.address" disabled />
              </template>
              <template v-else-if="column.dataIndex === 'chain'">
                <Input
                  v-model:value="form.onchain[index]!.chain"
                  disabled
                  placeholder="BSC / Ethereum / ..."
                />
              </template>
            </template>
          </Table>
          <div
            class="mt-4 rounded-md border border-dashed border-orange-200 p-3"
          >
            <div class="sub-label">
              <span class="lang-chip lang-zh">TW</span> Traditional Chinese
              on-chain labels
            </div>
            <div
              v-for="(row, i) in form.onchain"
              :key="`onchain-zh-${i}`"
              class="mb-3"
            >
              <Input
                v-model:value="row.label_zh"
                disabled
                :placeholder="`On-chain item #${i + 1} label TW`"
              />
            </div>
          </div>
        </Tabs.TabPane>
      </Tabs>
    </Card>
  </Page>
</template>

<style scoped>
.label {
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--ant-color-text-secondary);
}

.lang-hint {
  margin-left: 6px;
  font-size: 11px;
  font-weight: normal;
  color: var(--ant-color-text-quaternary);
}

.sub-label {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--ant-color-text-secondary);
}

.lang-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 1px 6px;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.4;
  letter-spacing: 0.5px;
  border-radius: 4px;
}

.lang-chip.lang-en {
  color: #0958d9;
  background: #e6f4ff;
}

.lang-chip.lang-zh {
  color: #d46b08;
  background: #fff7e6;
}
</style>
