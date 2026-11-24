<script setup lang="ts">
import type { Dayjs } from 'dayjs';

import type { OnchainParamsPreview } from '#/api/joincare/project_content';

import { computed, h, onMounted, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Input as AInput,
  Alert,
  Button,
  Card,
  DatePicker,
  Divider,
  Form,
  InputNumber,
  message,
  Modal,
  Select,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import { projectErrorMessage as idoErrorMessage } from '#/api/joincare/errors';
import {
  getOnchainParams,
  previewOnchainParams,
  submitOnchainParams,
} from '#/api/joincare/project_content';
import { myPermissions } from '#/api/joincare/role';

import { formatOnchainTime } from './onchain-time';

defineOptions({ name: 'JoinCareProjectOnchainParams' });

const FormItem = Form.Item;
const SelectOption = Select.Option;

const route = useRoute();
const projectId = String(route.params?.id ?? '');
const currentVersion = ref('');
const currentStatus = ref('');
const currentFundingMode = ref('');
const versionChoice = ref<'' | 'ido_v3_stablecoin' | 'v2_stablecoin'>('');
const canWriteParams = ref(false);
const permissionChecked = ref(false);
const permissionError = ref('正在核对项目编辑权限 [IDO_PERMISSION_CHECKING]');
async function refreshWritePermission() {
  canWriteParams.value = false;
  permissionChecked.value = false;
  try {
    const access = await myPermissions();
    canWriteParams.value = access.is_super === true || access.permissions.includes('m2_project:write');
    permissionChecked.value = true;
    permissionError.value = canWriteParams.value ? '' : '当前账号缺少项目编辑权限（m2_project:write），可查看但不能预览或提交 Set 审批。请联系权限管理员核对角色。错误码：ADMIN_PERM_DENIED';
  } catch (failure) {
    permissionError.value = idoErrorMessage(failure, '项目编辑权限读取失败，暂不允许预览或提交。');
  }
  return canWriteParams.value;
}
const isV3 = computed(() => versionChoice.value === 'ido_v3_stablecoin');
const versionReady = computed(
  () =>
    (currentVersion.value === 'unassigned' &&
      currentFundingMode.value === 'capped_presale' &&
      currentStatus.value === 'Setup' &&
      versionChoice.value !== '') ||
    (currentVersion.value === 'v2_stablecoin' &&
      versionChoice.value === 'v2_stablecoin'),
);

const requestBody = () => ({
  ...form,
  // Preserve the historical V2 request shape; only V3 opts into the
  // separate project-scoped four-eyes review. Never infer V3 from token choice.
  ...(isV3.value ? { contract_version: 'ido_v3_stablecoin' as const } : {}),
  // V3 commits calldata in whole Unix seconds, while projects stores
  // DATETIME(0). Preview and submission must use the same exact-second input.
  start_time: formatOnchainTime(startTimeD.value, isV3.value),
  end_time: formatOnchainTime(endTimeD.value, isV3.value),
  tge: tgeRows.value,
});

// 时间窗用选择框（防手输错误），提交前转 ISO UTC 字符串
const startTimeD = ref<Dayjs | undefined>(dayjs().add(1, 'hour').minute(0));
const endTimeD = ref<Dayjs | undefined>(dayjs().add(3, 'hour').minute(0));
const disabledStart = (current: Dayjs) => current.isBefore(dayjs(), 'minute');

const form = reactive({
  payment_token: 'USDT',
  token_address: '',
  token_symbol: '',
  owner_address: '',
  token_price: '',
  hard_cap: '',
  soft_cap: '',
  max_per_user: '',
  tge_after_end_min: 10,
  tge_interval_min: 0,
});

// 默认单期 100%
const tgeRows = ref<{ percent: number }[]>([{ percent: 100 }]);

const previewing = ref(false);
const submitting = ref(false);
const preview = ref<null | OnchainParamsPreview>(null);
const previewedRequest = ref('');

const canPreview = computed(
  () =>
    /^[1-9]\d*$/.test(projectId) &&
    versionReady.value &&
    form.payment_token !== '' &&
    (!isV3.value || form.payment_token === 'USDT') &&
    form.token_address !== '' &&
    form.owner_address !== '' &&
    form.token_price !== '' &&
    form.hard_cap !== '' &&
    form.soft_cap !== '' &&
    form.max_per_user !== '' &&
    !!startTimeD.value &&
    !!endTimeD.value &&
    tgeRows.value.length > 0,
);

const tgeSum = computed(() =>
  tgeRows.value.reduce((acc, r) => acc + (r.percent || 0), 0),
);

// A draft is input to independent review, not a signed Set manifest or a
// backend admission. If used for a write, the independent signer must
// re-encode the typed request and verify its digest; a change invalidates it.
function downloadV3Draft() {
  if (!isV3.value || !canPreview.value || locked.value || loadError.value)
    return;
  const payload = JSON.stringify(
    {
      project_id: projectId,
      request: requestBody(),
      status: 'OFFLINE REVIEW DRAFT ONLY; NOT V3 SET ADMISSION',
    },
    null,
    2,
  );
  const url = URL.createObjectURL(
    new Blob([payload], { type: 'application/json' }),
  );
  try {
    const link = document.createElement('a');
    link.href = url;
    link.download = `ido-v3-set-review-${projectId}.json`;
    link.click();
  } finally {
    URL.revokeObjectURL(url);
  }
}

// 在 GET 返回之前保持只读；读取失败不能把默认空表单误当成可提交参数。
const locked = ref(true);
const configured = ref(false);
const loadError = ref(false);

// 回填：项目已有 Set 参数时，用服务端值覆盖默认表单
onMounted(async () => {
  if (!/^[1-9]\d*$/.test(projectId)) return;
  void refreshWritePermission();
  try {
    const cur = await getOnchainParams(projectId);
    currentVersion.value = cur.contract_version;
    currentStatus.value = cur.status;
    currentFundingMode.value = cur.funding_mode;
    if (
      cur.contract_version === 'ido_v3_stablecoin' ||
      cur.contract_version === 'v2_stablecoin'
    ) {
      versionChoice.value = cur.contract_version;
    }
    locked.value =
      cur.locked ||
      cur.contract_version === 'ido_v3_stablecoin' ||
      !['unassigned', 'v2_stablecoin'].includes(cur.contract_version) ||
      (cur.contract_version === 'unassigned' &&
        (cur.status !== 'Setup' || cur.funding_mode !== 'capped_presale'));
    configured.value = cur.configured;
    // The approved application already fixed the Setup project's owner. Do
    // not silently replace it with the operator's wallet or a V2 project.
    if (cur.owner_address) form.owner_address = cur.owner_address;
    if (!cur.configured) return;
    if (cur.payment_token) form.payment_token = cur.payment_token;
    if (cur.token_address) form.token_address = cur.token_address;
    if (cur.token_symbol) form.token_symbol = cur.token_symbol;
    if (cur.token_price && cur.token_price !== '0')
      form.token_price = cur.token_price;
    if (cur.hard_cap && cur.hard_cap !== '0') form.hard_cap = cur.hard_cap;
    if (cur.soft_cap && cur.soft_cap !== '0') form.soft_cap = cur.soft_cap;
    if (cur.max_per_user && cur.max_per_user !== '0')
      form.max_per_user = cur.max_per_user;
    // RFC3339（带 +08:00 偏移）→ dayjs 自动转浏览器本地时间显示
    if (cur.start_time) startTimeD.value = dayjs(cur.start_time);
    if (cur.end_time) endTimeD.value = dayjs(cur.end_time);
    if (cur.tge?.length) {
      tgeRows.value = cur.tge.map((t) => ({ percent: Number(t.percent) }));
      if (cur.tge_after_end_min > 0)
        form.tge_after_end_min = cur.tge_after_end_min;
      if (cur.tge_interval_min > 0)
        form.tge_interval_min = cur.tge_interval_min;
    }
    if (cur.locked) {
      message.info('参数已锁定上链身份，仅可查看，不可再提交');
    } else {
      message.info('已回填当前已保存的参数，可直接修改后重新提交审批');
    }
  } catch {
    locked.value = true;
    loadError.value = true;
    message.error('链上参数读取失败，请刷新页面核对。错误码：IDO_PARAMS_READ_UNAVAILABLE');
  }
});

async function doPreview() {
  if (!canPreview.value || locked.value) return;
  if (!(await refreshWritePermission())) {message.error(permissionError.value);return;}
  previewing.value = true;
  preview.value = null;
  previewedRequest.value = '';
  try {
    const request = requestBody();
    const fingerprint = JSON.stringify(request);
    preview.value = await previewOnchainParams(projectId, request);
    previewedRequest.value = fingerprint;
    message.success(
      isV3.value
        ? '链上身份与参数预览通过；仍需另一位管理员审批后才会写入 Set'
        : '校验通过',
    );
  } catch (error: any) {
    Modal.error({
      title: '参数预览未完成',
      content: idoErrorMessage(error, '参数预览未通过，请核对 Set 参数并联系运维查看失败校验项。'),
    });
  } finally {
    previewing.value = false;
  }
}

function renderPreviewNode() {
  if (!preview.value) return null;
  const p = preview.value;
  const kv = (label: string, value: string) =>
    h('div', { style: 'display:flex;gap:12px;padding:2px 0' }, [
      h('span', { style: 'min-width:180px;color:#888' }, label),
      h('span', { style: 'font-family:monospace' }, value),
    ]);
  return h('div', { style: 'max-height:60vh;overflow:auto' }, [
    h('h4', `链身份 · chain ${p.chain.chain_id}`),
    kv('IDO 合约', p.chain.contract),
    kv(
      '费率快照',
      `${p.chain.success_fee_rate / 100}% → ${p.chain.fee_receiver}`,
    ),
    h('h4', '支付币（服务端受信注入）'),
    kv(
      `${p.payment_token.symbol}`,
      `${p.payment_token.address} (${p.payment_token.decimals}d)`,
    ),
    h('h4', '项目币'),
    kv(
      `${p.project_token.symbol}`,
      `${p.project_token.address} (${p.project_token.decimals}d)`,
    ),
    kv('项目方', p.project_token.owner),
    h('h4', '金额（display → raw）'),
    kv(
      'token_price',
      `${p.amounts.token_price} → ${p.amounts.token_price_raw}`,
    ),
    kv('soft_cap', `${p.amounts.soft_cap} → ${p.amounts.soft_cap_raw}`),
    kv('hard_cap', `${p.amounts.hard_cap} → ${p.amounts.hard_cap_raw}`),
    kv(
      'max_per_user',
      `${p.amounts.max_per_user} → ${p.amounts.max_per_user_raw}`,
    ),
    kv(
      'soft/hard 所需项目币',
      `${p.soft_cap_token_estimate} / ${p.hard_cap_token_estimate}`,
    ),
    h('h4', '时间窗（UTC）'),
    kv('start', p.window.start_utc),
    kv('end', p.window.end_utc),
    h('h4', 'TGE 计划'),
    ...p.tge.map((t) => kv(`#${t.index} · ${t.percent}%`, t.time_utc)),
  ]);
}

async function doSubmit() {
  if (locked.value || !canPreview.value) return;
  if (!(await refreshWritePermission())) {message.error(permissionError.value);return;}
  if (
    !preview.value ||
    previewedRequest.value !== JSON.stringify(requestBody())
  ) {
    preview.value = null;
    previewedRequest.value = '';
    message.warning('参数已变化；请重新预览并确认');
    return;
  }
  submitting.value = true;
  try {
    const res = await submitOnchainParams(projectId, requestBody());
    Modal.success({
      title: '已提交四眼审批',
      content: `审批单 #${res.approval_id}（${res.status}）。另一位管理员审批通过后参数才会写入。`,
    });
  } catch (error: any) {
    Modal.error({
      title: '提交失败',
      content: idoErrorMessage(error, '参数审批提交结果未确认，请先查询原审批单。'),
    });
  } finally {
    submitting.value = false;
  }
}

function confirmSubmit() {
  if (
    !preview.value ||
    locked.value ||
    previewedRequest.value !== JSON.stringify(requestBody())
  ) {
    preview.value = null;
    previewedRequest.value = '';
    message.warning('参数已变化；请重新预览并确认');
    return;
  }
  Modal.confirm({
    title: '确认提交链上参数审批？',
    content: () => renderPreviewNode(),
    width: 640,
    okText: '提交审批',
    onOk: doSubmit,
  });
}
</script>

<template>
  <Page
    :title="`链上参数 · 项目 #${projectId}`"
    description="已上链锁定的项目可以查看已保存的参数，但不可修改或再次提交审批。"
  >
    <Alert v-if="!canWriteParams" type="warning" show-icon :message="permissionError" style="margin-bottom: 16px" />
    <Alert
      v-if="isV3"
      type="warning"
      show-icon
      message="V3 预览仅核对固定 Proxy、项目币准入、链上治理身份及参数换算；它不授权 Set。提交后须由另一位管理员四眼审批，服务端再次核验并原子保存不可变参数。Safe 链上交易另行签署和确认；任何准入未就绪时均拒绝写入。"
      style="margin-bottom: 16px"
    />
    <Alert
      v-if="loadError"
      type="error"
      show-icon
      message="链上参数读取失败；已禁止编辑和提交，请刷新页面重试"
      style="margin-bottom: 16px"
    />
    <Alert
      v-else-if="locked && configured"
      type="info"
      show-icon
      message="以下为已保存的链上参数；V3 已 Set 或项目已上链锁定时仅供查看"
      style="margin-bottom: 16px"
    />
    <div class="jc-grid">
      <Card
        :title="locked && configured ? '已保存的参数（只读）' : '参数表单'"
        class="form-card"
      >
        <Form layout="vertical" :disabled="locked">
          <FormItem label="合约版本（未分配项目必须明确选择）" required>
            <Select
              v-model:value="versionChoice"
              :disabled="currentVersion !== 'unassigned'"
              @change="preview = null"
            >
              <SelectOption value="ido_v3_stablecoin">
                IDO V3 · 独立合约
              </SelectOption>
              <SelectOption value="v2_stablecoin">
                IDO V2 · 历史流程
              </SelectOption>
            </Select>
          </FormItem>
          <FormItem label="支付币（服务端白名单注入）" required>
            <Select v-model:value="form.payment_token" @change="preview = null">
              <SelectOption value="USDT">USDT</SelectOption>
              <SelectOption v-if="!isV3" value="USDC">USDC</SelectOption>
            </Select>
          </FormItem>
          <FormItem label="项目币地址" required>
            <AInput v-model:value="form.token_address" placeholder="0x..." />
          </FormItem>
          <FormItem label="项目币符号（选填，仅展示用）">
            <AInput v-model:value="form.token_symbol" placeholder="如 FMVT" />
          </FormItem>
          <FormItem label="项目方收款地址" required>
            <AInput v-model:value="form.owner_address" placeholder="0x..." />
          </FormItem>

          <Divider>经济参数</Divider>
          <div class="row2">
            <FormItem label="代币价格（支付币）" required>
              <AInput v-model:value="form.token_price" placeholder="0.0025" />
            </FormItem>
            <FormItem label="单户上限" required>
              <AInput v-model:value="form.max_per_user" placeholder="1000" />
            </FormItem>
            <FormItem label="软顶 Soft Cap" required>
              <AInput v-model:value="form.soft_cap" placeholder="100000" />
            </FormItem>
            <FormItem label="硬顶 Hard Cap" required>
              <AInput v-model:value="form.hard_cap" placeholder="500000" />
            </FormItem>
          </div>

          <Divider>时间窗（本地时间选择，服务端按 UTC 校验与展示）</Divider>
          <div class="row2">
            <FormItem label="开始时间" required>
              <DatePicker
                v-model:value="startTimeD"
                :disabled-date="disabledStart"
                show-time
                format="YYYY-MM-DD HH:mm"
                placeholder="选择开始时间"
                style="width: 100%"
              />
            </FormItem>
            <FormItem label="结束时间" required>
              <DatePicker
                v-model:value="endTimeD"
                :disabled-date="
                  (c) => (startTimeD ? c.isBefore(startTimeD, 'minute') : false)
                "
                show-time
                format="YYYY-MM-DD HH:mm"
                placeholder="选择结束时间"
                style="width: 100%"
              />
            </FormItem>
          </div>

          <Divider>
            TGE 计划
            <Tag :color="tgeSum === 100 ? 'green' : 'red'">
              合计 {{ tgeSum }}%
            </Tag>
          </Divider>
          <div v-for="(row, i) in tgeRows" :key="i" class="tge-row">
            <InputNumber
              v-model:value="row.percent"
              :min="0.01"
              :max="100"
              style="width: 140px"
              addon-after="%"
            />
            <Button
              v-if="tgeRows.length > 1 && !locked"
              danger
              size="small"
              @click="tgeRows.splice(i, 1)"
            >
              删除
            </Button>
          </div>
          <Button
            v-if="tgeRows.length < 10 && !locked"
            size="small"
            style="margin-top: 8px"
            @click="tgeRows.push({ percent: 0 })"
          >
            + 加一期
          </Button>
          <div class="row2" style="margin-top: 12px">
            <FormItem label="首期距结束（分钟）">
              <InputNumber
                v-model:value="form.tge_after_end_min"
                :min="0"
                style="width: 160px"
              />
            </FormItem>
            <FormItem label="期间隔（分钟，0=单期）">
              <InputNumber
                v-model:value="form.tge_interval_min"
                :min="0"
                style="width: 160px"
              />
            </FormItem>
          </div>

          <div class="actions">
            <Button
              v-if="isV3"
              :disabled="!canPreview || locked || loadError"
              @click="downloadV3Draft"
            >
              导出 V3 参数草稿供复核（不提交）
            </Button>
            <Button
              :disabled="!canPreview || locked || !permissionChecked || !canWriteParams"
              :loading="previewing"
              type="primary"
              ghost
              @click="doPreview"
            >
              ① 预览校验（dry-run）
            </Button>
            <Button
              :disabled="!preview || locked || !permissionChecked || !canWriteParams"
              :loading="submitting"
              type="primary"
              @click="confirmSubmit"
            >
              ② 确认并提交审批
            </Button>
          </div>
        </Form>
      </Card>

      <Card
        v-if="preview"
        :title="
          isV3 ? '只读诊断结果（不代表可提交）' : '预览结果（提交前请逐项核对）'
        "
        class="preview-card"
      >
        <component :is="() => renderPreviewNode()" />
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.jc-grid {
  display: grid;
  grid-template-columns: minmax(360px, 1fr) minmax(320px, 1fr);
  gap: 16px;
}

@media (max-width: 960px) {
  .jc-grid {
    grid-template-columns: 1fr;
  }
}

.row2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 12px;
}

.tge-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 6px;
}

.actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}

.preview-card :deep(h4) {
  margin: 12px 0 4px;
}
</style>
