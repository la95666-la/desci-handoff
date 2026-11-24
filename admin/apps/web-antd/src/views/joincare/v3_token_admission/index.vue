<script setup lang="ts">
import type { V3SafeProposal } from '#/api/joincare/v3_safe_proposal';
import type {
  V3AdmissionAssetName,
  V3AdmissionReceipt,
  V3AdmissionStatus,
  V3ProjectTokenStatus,
} from '#/api/joincare/v3_token_admission';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Input,
  message,
  Tag,
} from 'ant-design-vue';

import { idoErrorMessage } from '#/api/joincare/ido-errors';
import {
  previewV3ProjectTokenSafeProposal,
  previewV3TokenSafeProposal,
  signV3SafeProposal,
  submitV3ProjectTokenSafeProposal,
  submitV3TokenSafeProposal,
} from '#/api/joincare/v3_safe_proposal';
import {
  readV3ProjectToken,
  readV3TokenAdmission,
  submitV3RuntimeRegistration,
  verifyV3ProjectTokenReceipt,
  verifyV3TokenAdmissionReceipt,
} from '#/api/joincare/v3_token_admission';

import { validObservedOwner } from './owner-identity';
import {
  v3PaymentTokenSafeAbi,
  v3ProjectTokenSafeAbi,
} from './v3-token-safe-abi';

const loading = ref(false);
const statusError = ref('');
const status = ref<null | V3AdmissionStatus>(null);
const receiptBusy = ref<null | V3AdmissionAssetName>(null);
const txHash = reactive<Record<V3AdmissionAssetName, string>>({
  usdt: '',
  testcat: '',
});
const receipts = reactive<
  Partial<Record<V3AdmissionAssetName, V3AdmissionReceipt>>
>({});
const proposals = reactive<
  Partial<Record<V3AdmissionAssetName, V3SafeProposal>>
>({});
const assets: V3AdmissionAssetName[] = ['usdt', 'testcat'];
const projectTokenAddress = ref('');
const projectTokenStatus = ref<null | V3ProjectTokenStatus>(null);
const projectTokenProposal = ref<null | V3SafeProposal>(null);
const projectTokenTxHash = ref('');
const projectTokenBusy = ref(false);
const projectTokenReceipt = ref<null | V3AdmissionReceipt>(null);
const runtimeReviewBusy = ref(false);

async function requestRuntimeReview() {
  runtimeReviewBusy.value = true;
  try {
    const result = await submitV3RuntimeRegistration();
    message.success(
      `V3 runtime 注册待另一位管理员复核：审批单 #${result.approval_id}；尚未 Set 或创建项目`,
    );
  } catch (error) {
    message.error(
      idoErrorMessage(error, 'V3 运行身份审批结果未确认，请先查原工单。'),
    );
  } finally {
    runtimeReviewBusy.value = false;
  }
}

async function inspectProjectToken() {
  projectTokenStatus.value = null;
  projectTokenProposal.value = null;
  projectTokenReceipt.value = null;
  projectTokenTxHash.value = '';
  const address = projectTokenAddress.value.trim();
  if (!/^0x[\da-fA-F]{40}$/.test(address)) {
    message.error('请输入完整的 BSC 项目代币地址。错误码：V3_TOKEN_INVALID');
    return;
  }
  projectTokenBusy.value = true;
  try {
    const row = await readV3ProjectToken(address);
    if (
      projectTokenAddress.value.trim().toLowerCase() !==
        address.toLowerCase() ||
      row.chain_id !== 56 ||
      row.confirmations !== 15 ||
      row.asset.token.toLowerCase() !== address.toLowerCase() ||
      row.proxy.toLowerCase() !==
        '0x0d4040875742975269c55e16c8884d3ee9510b8e' ||
      !validObservedOwner(row.owner_safe, row.proxy)
    ) {
      throw new Error('独立 V3 链身份不匹配');
    }
    projectTokenStatus.value = row;
  } catch (error) {
    message.error(idoErrorMessage(error, '新项目币链上核验不可用；不得提案。'));
  } finally {
    projectTokenBusy.value = false;
  }
}

async function prepareProjectTokenProposal() {
  if (!projectTokenStatus.value || projectTokenStatus.value.asset.allowed)
    return;
  const address = projectTokenStatus.value.asset.token;
  projectTokenProposal.value = null;
  projectTokenBusy.value = true;
  try {
    const p = await previewV3ProjectTokenSafeProposal(address);
    // configureProjectToken(address,bool), followed by address and true words.
    const calldata = `0x82f30c7c${address.slice(2).toLowerCase().padStart(64, '0')}${'1'.padStart(64, '0')}`;
    if (
      projectTokenStatus.value?.asset.token.toLowerCase() !==
        address.toLowerCase() ||
      p.safe.toLowerCase() !==
        projectTokenStatus.value.owner_safe.toLowerCase() ||
      p.to.toLowerCase() !== projectTokenStatus.value.proxy.toLowerCase() ||
      p.data.toLowerCase() !== calldata
    )
      throw new Error('Safe 提案不匹配');
    projectTokenProposal.value = p;
  } catch (error) {
    message.error(
      idoErrorMessage(error, '待签提案不可用；检查代币或 Safe 队列。'),
    );
  } finally {
    projectTokenBusy.value = false;
  }
}

async function proposeProjectToken() {
  if (!projectTokenProposal.value || !projectTokenStatus.value) return;
  const address = projectTokenStatus.value.asset.token;
  const proposal = projectTokenProposal.value;
  const hash = projectTokenProposal.value.safe_tx_hash;
  projectTokenBusy.value = true;
  try {
    const signed = await signV3SafeProposal(proposal);
    if (
      projectTokenStatus.value?.asset.token.toLowerCase() !==
        address.toLowerCase() ||
      projectTokenProposal.value?.safe_tx_hash !== hash
    )
      throw new Error('代币选择已变化');
    await submitV3ProjectTokenSafeProposal(address, signed);
    message.success(`Safe 提案已提交，等待其他 Owner 确认：${hash}`);
  } catch (error) {
    message.error(
      idoErrorMessage(
        error,
        `结果未知；先按原 Safe 哈希 ${hash} 核对，不得盲目重发。`,
      ),
    );
  } finally {
    projectTokenProposal.value = null;
    projectTokenBusy.value = false;
  }
}

async function verifyProjectToken() {
  if (
    !projectTokenStatus.value ||
    !/^0x[\da-fA-F]{64}$/.test(projectTokenTxHash.value.trim())
  ) {
    message.error('请输入原始链上交易哈希。错误码：IDO_RECEIPT_HASH_INVALID');
    return;
  }
  projectTokenBusy.value = true;
  projectTokenReceipt.value = null;
  try {
    const receipt = await verifyV3ProjectTokenReceipt(
      projectTokenStatus.value.asset.token,
      projectTokenTxHash.value.trim(),
    );
    if (
      !receipt.allowed ||
      receipt.confirmations < 15 ||
      receipt.decimals !== projectTokenStatus.value.asset.decimals
    ) {
      throw new Error('原始回执不满足准入要求');
    }
    projectTokenReceipt.value = receipt;
    message.success('原交易与当前项目币配置核验通过；请刷新链上状态');
  } catch (error) {
    message.error(idoErrorMessage(error, '原交易回执未核实；不要盲目重发。'));
  } finally {
    projectTokenBusy.value = false;
  }
}

const safeUrl = computed(() => {
  if (!status.value || status.value.chain_id !== 56) return '';
  return `https://app.safe.global/home?safe=bnb:${status.value.owner_safe}`;
});
const projectSafeUrl = computed(() =>
  projectTokenStatus.value?.chain_id === 56
    ? `https://app.safe.global/home?safe=bnb:${projectTokenStatus.value.owner_safe}`
    : '',
);
const projectBuilderUrl = computed(() =>
  projectSafeUrl.value && projectTokenStatus.value
    ? `https://app.safe.global/apps/open?safe=${encodeURIComponent(`bnb:${projectTokenStatus.value.owner_safe}`)}&appUrl=${encodeURIComponent('https://apps-portal.safe.global/tx-builder')}`
    : '',
);
const isProjectProposalCurrent = computed(
  () =>
    !!projectTokenProposal.value &&
    !!projectTokenStatus.value &&
    projectTokenProposal.value.safe.toLowerCase() ===
      projectTokenStatus.value.owner_safe.toLowerCase() &&
    projectTokenProposal.value.to.toLowerCase() ===
      projectTokenStatus.value.proxy.toLowerCase() &&
    projectTokenStatus.value.asset.token.toLowerCase() ===
      projectTokenAddress.value.trim().toLowerCase(),
);

async function refresh() {
  loading.value = true;
  statusError.value = '';
  status.value = null;
  delete receipts.usdt;
  delete receipts.testcat;
  delete proposals.usdt;
  delete proposals.testcat;
  try {
    const result = await readV3TokenAdmission();
    if (
      result.chain_id !== 56 ||
      result.safe_threshold !== 2 ||
      result.confirmations !== 15 ||
      !result.proxy ||
      !result.owner_safe ||
      !result.block_hash
    ) {
      throw new Error('V3 链身份或确认深度不符合预期');
    }
    status.value = result;
  } catch (error) {
    statusError.value = idoErrorMessage(
      error,
      'V3 链上预检不可用，请检查节点与运行身份。',
    );
  } finally {
    loading.value = false;
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    message.success('已复制；请在 Safe 中独立核对地址、金额和方法');
  } catch {
    message.error('复制失败，请手动核对。错误码：IDO_CLIPBOARD_UNAVAILABLE');
  }
}

async function verify(asset: V3AdmissionAssetName) {
  const hash = txHash[asset].trim();
  if (!/^0x[\da-fA-F]{64}$/.test(hash)) {
    message.error(
      '请输入原始链上交易哈希（不是 Safe 提案哈希）。错误码：IDO_RECEIPT_HASH_INVALID',
    );
    return;
  }
  receiptBusy.value = asset;
  receipts[asset] = undefined;
  try {
    const result = await verifyV3TokenAdmissionReceipt(asset, hash);
    if (
      result.asset !== asset ||
      !result.allowed ||
      result.decimals !== 18 ||
      result.confirmations < 15
    ) {
      throw new Error('回执未满足 V3 代币准入门槛');
    }
    receipts[asset] = result;
    message.success('原交易回执和 V3 代币状态已核验，请再刷新链上配置');
  } catch (error) {
    message.error(idoErrorMessage(error, '回执未核实；不得重发或跳到下一步。'));
  } finally {
    receiptBusy.value = null;
  }
}

async function prepareProposal(asset: V3AdmissionAssetName) {
  if (
    !status.value ||
    status.value[asset].allowed ||
    (asset === 'testcat' && !status.value.usdt.allowed)
  )
    return;
  proposals[asset] = undefined;
  receiptBusy.value = asset;
  try {
    const p = await previewV3TokenSafeProposal(asset);
    if (
      p.safe.toLowerCase() !== status.value.owner_safe.toLowerCase() ||
      p.to.toLowerCase() !== status.value.proxy.toLowerCase() ||
      p.data.toLowerCase() !==
        `0x${asset === 'usdt' ? '2cb96e4f' : '82f30c7c'}${status.value[asset].token.slice(2).toLowerCase().padStart(64, '0')}${'1'.padStart(64, '0')}`
    )
      throw new Error('目标不匹配');
    proposals[asset] = p;
  } catch (error) {
    message.error(
      idoErrorMessage(error, '当前 Safe 提案预览不可用；勿绕过准入检查。'),
    );
  } finally {
    receiptBusy.value = null;
  }
}

async function submitProposal(asset: V3AdmissionAssetName) {
  const p = proposals[asset];
  if (!p) return;
  const hash = p.safe_tx_hash;
  receiptBusy.value = asset;
  try {
    const signed = await signV3SafeProposal(p);
    await submitV3TokenSafeProposal(asset, signed);
    message.success(`Safe 提案已提交，待其他 Owner 确认：${hash}`);
  } catch (error) {
    message.error(
      idoErrorMessage(
        error,
        `提案未知或被拒；先在 Safe 用原哈希 ${hash} 核对，勿盲目重发。`,
      ),
    );
  } finally {
    proposals[asset] = undefined;
    receiptBusy.value = null;
  }
}

onMounted(refresh);
</script>

<template>
  <Page
    title="IDO V3 · 代币准入"
    description="Owner Safe 钱包签署并通过 Safe API 提案；执行后独立核验原交易回执"
  >
    <Alert
      type="warning"
      show-icon
      message="仅针对 BSC 主网独立 V3 Proxy；USDT 与历史 TESTCAT 可单独核验，后续项目币按地址核验。只有真实 Owner 钱包签署后才能提案；此页面不执行 Safe 交易。"
      class="mb-4"
    />
    <Alert
      type="warning"
      show-icon
      message="本页面只信任指定的商用 RPC，结果不是双节点独立核验。Safe 签署人仍须自行复核交易目标、参数和链上状态。"
      class="mb-4"
    />
    <Alert
      v-if="statusError"
      type="error"
      show-icon
      :message="`历史 USDT/TESTCAT 页面预检失败；历史提案不可继续。新项目币按地址入口须单独通过链上核验：${statusError}`"
      class="mb-4"
    />
    <Button :loading="loading" @click="refresh">
      重新核对商用 RPC / Safe nonce
    </Button>
    <Card title="共享 V3 runtime 身份（一次性四眼审批）" class="mt-4">
      <p>
        对固定 BSC 56
        Proxy、Implementation、Safe、治理地址和代码身份做链上核对。发起审批不写库；另一位管理员批准才登记不可变
        runtime。此步骤不批准任何项目 Set。
      </p>
      <Button :loading="runtimeReviewBusy" @click="requestRuntimeReview">
        提交 V3 runtime 独立审批
      </Button>
    </Card>

    <Card title="项目币 · 按地址准入" class="mt-4">
      <Alert
        type="warning"
        show-icon
        message="适用于后续项目的新项目币（1–18 位精度），不是把所有代币自动放行。填写地址并核对；真实 Owner Safe 签署、执行及回执确认后才会允许。此处不会创建 IDO 项目，支付币仍仅为 USDT。"
      />
      <Input
        v-model:value="projectTokenAddress"
        placeholder="项目币 BSC 地址 0x…"
        class="my-3"
        @update:value="
          projectTokenStatus = null;
          projectTokenProposal = null;
          projectTokenReceipt = null;
          projectTokenTxHash = '';
        "
      />
      <Button :loading="projectTokenBusy" @click="inspectProjectToken">
        核对项目币链上配置
      </Button>
      <div v-if="projectTokenStatus" class="mt-4">
        <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
          <DescriptionsItem label="链 / 确认块">
            BSC {{ projectTokenStatus.chain_id }} ·
            {{ projectTokenStatus.block }}
          </DescriptionsItem>
          <DescriptionsItem label="准入状态 / 精度">
            <Tag :color="projectTokenStatus.asset.allowed ? 'green' : 'gold'">
              {{ projectTokenStatus.asset.allowed ? '已允许' : '未允许' }}
            </Tag>
            {{ projectTokenStatus.asset.decimals }} decimals
          </DescriptionsItem>
          <DescriptionsItem label="项目币 token" :span="2">
            <code class="break-all">{{ projectTokenStatus.asset.token }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="Owner Safe" :span="2">
            <code class="break-all">{{ projectTokenStatus.owner_safe }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="To：V3 Proxy" :span="2">
            <code class="break-all">{{ projectTokenStatus.proxy }}</code>
          </DescriptionsItem>
        </Descriptions>
        <Alert
          v-if="projectTokenStatus.asset.allowed"
          type="success"
          show-icon
          class="mt-4"
          message="此项目币已在 V3 链上准入；不再生成重复的 Safe 待签提案。若已执行，请在下方用原始链上交易哈希核验回执。"
        />
        <Button
          v-if="!projectTokenStatus.asset.allowed"
          class="mt-4"
          :loading="projectTokenBusy"
          @click="prepareProjectTokenProposal"
        >
          读取 Safe 待签提案
        </Button>
        <Card
          v-if="isProjectProposalCurrent"
          class="mt-4"
          title="Owner Safe · 项目币准入交易核对"
        >
          <Alert
            type="warning"
            show-icon
            message="仅由真实 Owner Safe 签署人操作。先核对下方目标、ABI、参数及完整 calldata；Admin 页面不会执行链上交易。"
            class="mb-4"
          />
          <h4 class="mb-3 font-semibold">Safe 合约交易表单 · 逐项填写</h4>
          <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
            <DescriptionsItem label="网络">
              BSC 主网 · chain ID 56
            </DescriptionsItem>
            <DescriptionsItem label="Value / Operation">
              0 BNB / CALL（非 DelegateCall）
            </DescriptionsItem>
            <DescriptionsItem label="发起者 Owner Safe" :span="2">
              <code class="break-all">{{ projectTokenStatus.owner_safe }}</code>
            </DescriptionsItem>
            <DescriptionsItem label="To：V3 Proxy" :span="2">
              <code class="break-all">{{ projectTokenStatus.proxy }}</code>
            </DescriptionsItem>
            <DescriptionsItem label="方法" :span="2">
              <code>configureProjectToken(address,bool)</code>
            </DescriptionsItem>
            <DescriptionsItem label="函数 ABI · 导入 Builder" :span="2">
              <pre class="safe-code">{{ v3ProjectTokenSafeAbi }}</pre>
            </DescriptionsItem>
            <DescriptionsItem label="token · address" :span="2">
              <code class="break-all">{{
                projectTokenStatus.asset.token
              }}</code>
            </DescriptionsItem>
            <DescriptionsItem label="allowed · bool">true</DescriptionsItem>
            <DescriptionsItem label="Safe nonce">
              {{ projectTokenProposal?.nonce }}
            </DescriptionsItem>
            <DescriptionsItem label="Safe 交易哈希" :span="2">
              <code class="break-all">{{
                projectTokenProposal?.safe_tx_hash
              }}</code>
            </DescriptionsItem>
            <DescriptionsItem label="Data · 完整 calldata" :span="2">
              <code class="break-all">{{ projectTokenProposal?.data }}</code>
            </DescriptionsItem>
          </Descriptions>
          <div class="mt-4 rounded border p-4">
            <b>Safe Transaction Builder 操作步骤</b>
            <ol class="mt-2 list-decimal space-y-2 pl-5">
              <li>
                切换 BSC 主网，打开下方
                <b>Owner Safe</b>；确认其地址等于表格中的 Owner Safe，不要用
                Policy Safe 或 relayer。
              </li>
              <li>
                新建一笔合约调用；To 填 V3 Proxy，Value 填 0，Operation 选
                CALL。
              </li>
              <li>
                导入下方 ABI，选择 <code>configureProjectToken</code>；token
                填表格中的项目币地址，allowed 填 <code>true</code>。
              </li>
              <li>
                将 Builder 编码出的完整 Data 与下方已预检 calldata
                逐字节核对；再次核对
                nonce、网络、目标及模拟结果。不同或代币地址改变即停止。
              </li>
              <li>
                由 Owner 创建提案，其他 Owner
                按门槛独立确认并执行；提案不是执行。执行后保存原始链上 tx hash
                和 Safe tx hash，返回本页核验回执。
              </li>
            </ol>
            <div class="mt-3 flex flex-wrap gap-3">
              <a
                v-if="projectSafeUrl"
                :href="projectSafeUrl"
                target="_blank"
                rel="noopener noreferrer"
                >打开 Owner Safe</a>
              <a
                v-if="projectBuilderUrl"
                :href="projectBuilderUrl"
                target="_blank"
                rel="noopener noreferrer"
                >打开 Transaction Builder</a>
            </div>
          </div>
          <div class="mt-4">
            <b>导入 Builder 的函数 ABI</b>
            <pre class="safe-code">{{ v3ProjectTokenSafeAbi }}</pre>
          </div>
          <div class="mt-4">
            <b>精确 calldata</b>
            <pre class="safe-code">{{ projectTokenProposal?.data }}</pre>
          </div>
          <Button
            danger
            class="mt-4"
            :loading="projectTokenBusy"
            @click="proposeProjectToken"
          >
            Owner 钱包签名并创建提案（不执行）
          </Button>
        </Card>
        <div class="mt-3 flex gap-2">
          <Input
            v-model:value="projectTokenTxHash"
            placeholder="Safe 执行后的原始链上交易哈希"
          />
          <Button :loading="projectTokenBusy" @click="verifyProjectToken">
            核验原交易回执
          </Button>
        </div>
        <p v-if="projectTokenReceipt">
          已核验 {{ projectTokenReceipt.confirmations }} 次确认；请刷新配置。
        </p>
      </div>
    </Card>
    <template v-if="status">
      <Card title="链身份与治理" class="mt-4">
        <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
          <DescriptionsItem label="网络 / 深度">
            BSC {{ status.chain_id }} · {{ status.confirmations }} 块
          </DescriptionsItem>
          <DescriptionsItem label="确认块">{{ status.block }}</DescriptionsItem>
          <DescriptionsItem label="区块哈希" :span="2">
            <code class="break-all">{{ status.block_hash }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="V3 Proxy" :span="2">
            <code class="break-all">{{ status.proxy }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="Owner Safe" :span="2">
            <code class="break-all">{{ status.owner_safe }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="签署门槛 / 当前 nonce">
            {{ status.safe_threshold }}/3 ·
            {{ status.safe_nonce }}
          </DescriptionsItem>
        </Descriptions>
        <a
          v-if="safeUrl"
          :href="safeUrl"
          target="_blank"
          rel="noopener noreferrer"
          >在 Safe 打开这个 Owner Safe（签署前再次核对网络、nonce 和目标）</a>
      </Card>

      <Alert
        type="info"
        show-icon
        message="历史 USDT／TESTCAT 回执分别核对。后续新项目币使用上方按地址入口独立提案与回执核验；每笔 value=0，CALL，不使用 relayer 私钥。链上结果未知时先查原哈希，不要再次提案。"
        class="mt-4"
      />

      <Card
        v-for="asset in assets"
        :key="asset"
        :title="
          asset === 'usdt' ? '第一笔 · USDT 支付币' : '第二笔 · TESTCAT 项目币'
        "
        class="mt-4"
      >
        <template v-if="status[asset]">
          <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
            <DescriptionsItem label="链上状态 / 精度">
              <Tag :color="status[asset].allowed ? 'green' : 'gold'">
                {{ status[asset].allowed ? '已允许' : '未允许' }}
              </Tag>
              {{ status[asset].decimals }} decimals
            </DescriptionsItem>
            <DescriptionsItem label="Owner Safe 模拟">
              {{ status[asset].simulation }}
            </DescriptionsItem>
            <DescriptionsItem label="token · address" :span="2">
              <code class="break-all">{{ status[asset].token }}</code><Button size="small" @click="copy(status[asset].token)">
                复制
              </Button>
            </DescriptionsItem>
            <DescriptionsItem label="To：V3 Proxy" :span="2">
              <code class="break-all">{{ status.proxy }}</code><Button size="small" @click="copy(status.proxy)"> 复制 </Button>
            </DescriptionsItem>
            <DescriptionsItem label="Owner Safe" :span="2">
              <code class="break-all">{{ status.owner_safe }}</code>
            </DescriptionsItem>
          </Descriptions>
          <Alert
            v-if="asset === 'testcat' && !status.usdt.allowed"
            type="warning"
            show-icon
            message="USDT 尚未核验为已允许；不得先执行 TESTCAT。"
            class="mb-3"
          />
          <div v-if="!status[asset].allowed" class="mb-3 break-all">
            <Button
              :disabled="asset === 'testcat' && !status.usdt.allowed"
              :loading="receiptBusy === asset"
              @click="prepareProposal(asset)"
            >
              读取 Safe 待签提案
            </Button>
            <Card
              v-if="proposals[asset]"
              class="mt-4"
              title="Owner Safe · 已预检待签交易"
            >
              <Alert
                type="warning"
                show-icon
                message="请由真实 Owner 在 Safe 中独立核对；此按钮只创建提案，不执行链上交易。"
                class="mb-4"
              />
              <Descriptions bordered size="small" :column="{ xs: 1, md: 2 }">
                <DescriptionsItem label="网络 / 操作">
                  BSC 56 · CALL · Value 0 BNB
                </DescriptionsItem>
                <DescriptionsItem label="Safe nonce">
                  {{ proposals[asset]?.nonce }}
                </DescriptionsItem>
                <DescriptionsItem label="Owner Safe" :span="2">
                  <code class="break-all">{{ status.owner_safe }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="To：V3 Proxy" :span="2">
                  <code class="break-all">{{ status.proxy }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="方法" :span="2">
                  <code>{{ status[asset].method }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="token · address" :span="2">
                  <code class="break-all">{{ status[asset].token }}</code>
                </DescriptionsItem>
                <DescriptionsItem label="allowed · bool">true</DescriptionsItem>
                <DescriptionsItem label="Safe 交易哈希" :span="2">
                  <code class="break-all">{{
                    proposals[asset]?.safe_tx_hash
                  }}</code>
                </DescriptionsItem>
              </Descriptions>
              <div class="mt-4 rounded border p-4">
                <b>Safe Transaction Builder 操作步骤</b>
                <ol class="mt-2 list-decimal space-y-2 pl-5">
                  <li>
                    打开 BSC 主网的 <b>Owner Safe</b>，核对地址；不要使用 Policy
                    Safe、Operator 或 relayer 钱包。
                  </li>
                  <li>
                    新建一笔合约调用；To 填 V3 Proxy，Value 填 0 BNB，Operation
                    选 CALL（非 DelegateCall）。
                  </li>
                  <li>
                    导入下方 ABI，选择当前方法；token 填表格中的地址，allowed 填
                    <code>true</code>。
                  </li>
                  <li>
                    将 Builder 编码出的 Data 与下方完整 calldata
                    逐字节核对，同时核对当前 Safe
                    nonce、网络、目标及模拟。变化或失败则停止。
                  </li>
                  <li>
                    Owner 创建提案、其余 Owner
                    独立确认并执行；提案不等于执行。执行后使用原始链上交易哈希核验回执，不明结果先查原
                    Safe 哈希，勿盲目重提。
                  </li>
                </ol>
              </div>
              <div class="mt-4">
                <b>导入 Builder 的函数 ABI</b>
                <pre class="safe-code">{{
                  asset === 'usdt'
                    ? v3PaymentTokenSafeAbi
                    : v3ProjectTokenSafeAbi
                }}</pre>
              </div>
              <div class="mt-4">
                <b>精确 calldata</b>
                <pre class="safe-code">{{ proposals[asset]?.data }}</pre>
              </div>
              <Button
                danger
                class="mt-4"
                :loading="receiptBusy === asset"
                @click="submitProposal(asset)"
              >
                用已连接的 Owner 钱包签名并创建提案（不执行）
              </Button>
            </Card>
          </div>
          <div class="flex flex-wrap gap-2 items-center">
            <Input
              v-model:value="txHash[asset]"
              class="max-w-xl"
              placeholder="Safe 执行后的原始链上交易哈希 0x…"
            />
            <Button
              :loading="receiptBusy === asset"
              :disabled="asset === 'testcat' && !status.usdt.allowed"
              @click="verify(asset)"
            >
              核验原交易回执
            </Button>
          </div>
          <p v-if="receipts[asset]" class="mt-2">
            已核验：区块 {{ receipts[asset]?.block }} ·
            {{ receipts[asset]?.confirmations }} 次确认；请刷新检查最新配置。
          </p>
        </template>
      </Card>
    </template>
  </Page>
</template>

<style scoped>
.safe-code {
  padding: 12px;
  overflow-x: auto;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  background: var(--ant-color-fill-quaternary, #f5f5f5);
  border-radius: 6px;
}
</style>
