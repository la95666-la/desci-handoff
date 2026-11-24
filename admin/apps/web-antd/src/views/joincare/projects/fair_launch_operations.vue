<script setup lang="ts">
import type {
  FairLaunchAdminAction,
  FairLaunchCapabilityEnvelope,
} from '#/api/joincare/fair_launch';
import type { ProjectListRow } from '#/api/joincare/project_content';

import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Input,
  message,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';

import {
  getFairLaunchCapabilities,
  isFairLaunchCapabilityUnavailable,
} from '#/api/joincare/fair_launch';
import { getProjectContent } from '#/api/joincare/project_content';
import { myPermissions } from '#/api/joincare/role';

import {
  FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE,
  fairLaunchActionReasonText,
  getFairLaunchActionAccess,
} from './fair-launch-admin';
import { formatFinancialDecimal, getFundingContractPair } from './funding-mode';

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const projectId = String(route.params.id ?? '');

const loading = ref(true);
const project = ref<null | ProjectListRow>(null);
const capability = ref<FairLaunchCapabilityEnvelope | null>(null);
const capabilityMessage = ref('Backend capability unavailable');
const permissions = ref<string[]>([]);
const projectLookupId = ref(projectId);

const currentAdminId = computed(() => {
  const raw = userStore.userInfo?.userId;
  const id = raw === undefined ? undefined : Number(raw);
  return Number.isInteger(id) ? id : undefined;
});
const exactPair = computed(
  () =>
    !!project.value &&
    getFundingContractPair(project.value) === 'fair_launch_v1',
);
const localOnlyAuthority = computed(() => {
  try {
    const api = new URL(
      String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''),
    );
    return (
      !import.meta.env.PROD &&
      import.meta.env.VITE_ENVIRONMENT === 'local' &&
      api.protocol === 'http:' &&
      api.hostname === '127.0.0.1' &&
      /^\d+$/.test(api.port) &&
      /^[0-9a-f]{64}$/.test(
        String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN || ''),
      ) &&
      Date.now() <
        Date.parse(
          String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT || ''),
        )
    );
  } catch {
    return false;
  }
});

const actionGroups: Array<{
  actions: FairLaunchAdminAction[];
  description: string;
  title: string;
}> = [
  {
    title: 'Operator durable lifecycle',
    description:
      'Admin 仅提交 immutable approval payload；Operator/Gateway/outbox 执行。',
    actions: ['create', 'activate', 'end', 'cancel', 'release'],
  },
  {
    title: 'Receiver / asset flows',
    description:
      'Admin 不代签提款；Backend descriptor 必须指明外部 Safe 观察或受信执行域。',
    actions: [
      'withdraw_project_proceeds',
      'withdraw_platform_fee',
      'withdraw_failed_project_tokens',
    ],
  },
  {
    title: 'Three-domain Pause',
    description:
      'Pause Guardian、24h Unpause Timelock 与 Operator 是独立签名域。',
    actions: [
      'pause_contributions',
      'unpause_contributions',
      'pause_payouts',
      'unpause_payouts',
      'pause_claims',
      'unpause_claims',
    ],
  },
  {
    title: '48h Governance / Recovery',
    description: '必须三权分离、step-up MFA、四眼审批；无 Super Admin bypass。',
    actions: [
      'invalidate_project',
      'invalidate_payment_token',
      'set_operator',
      'set_platform_fee',
      'register_payment_token_policy',
      'set_payment_token_admission',
      'set_payment_token_emergency_blocked',
      'configure_project_token',
      'set_project_token_emergency_blocked',
      'configure_beneficiary_safe',
      'configure_release_policy',
      'recover_excess_tokens',
      'upgrade_proxy',
    ],
  },
];

const actionLabels: Record<FairLaunchAdminAction, string> = {
  activate: 'Activate',
  cancel: 'Cancel',
  configure_beneficiary_safe: 'Beneficiary Safe policy',
  configure_project_token: 'Project Token policy',
  configure_release_policy: 'Release policy',
  create: 'Create on-chain',
  end: 'End / Settle',
  invalidate_payment_token: 'E8-A Payment Token invalidation',
  invalidate_project: 'Governance invalidation',
  pause_claims: 'Pause Claims',
  pause_contributions: 'Pause Contributions',
  pause_payouts: 'Pause Payouts',
  recover_excess_tokens: 'Recover excess tokens',
  register_payment_token_policy: 'Register Payment Token revision',
  release: 'Release Successful',
  set_operator: 'Rotate Operator',
  set_payment_token_admission: 'Payment Token admission',
  set_payment_token_emergency_blocked: 'Payment Token emergency block',
  set_platform_fee: 'Platform fee policy',
  set_project_token_emergency_blocked: 'Project Token emergency block',
  unpause_claims: 'Unpause Claims',
  unpause_contributions: 'Unpause Contributions',
  unpause_payouts: 'Unpause Payouts',
  upgrade_proxy: 'UUPS Upgrade',
  withdraw_failed_project_tokens: 'Failed Project Tokens',
  withdraw_platform_fee: 'Platform Fee',
  withdraw_project_proceeds: 'Project Proceeds',
};

function access(action: FairLaunchAdminAction) {
  if (!project.value) {
    return {
      allowed: false,
      reason: 'backend_capability_unavailable' as const,
    };
  }
  return getFairLaunchActionAccess({
    action,
    capabilities: capability.value,
    currentAdminId: currentAdminId.value,
    permissions: permissions.value,
    project: project.value,
  });
}

function descriptorTitle(action: FairLaunchAdminAction): string {
  const value = access(action);
  return value.allowed
    ? `${value.descriptor?.execution_domain} · ${value.descriptor?.approval_operation}`
    : fairLaunchActionReasonText(value);
}

async function load() {
  loading.value = true;
  try {
    const [projectContent, permissionResult] = await Promise.all([
      getProjectContent(projectId),
      myPermissions(),
    ]);
    project.value = {
      ...projectContent,
      onchain_locked: projectContent.onchain_locked === true,
    };
    // Super Admin is not an implicit Fair Launch permission bypass. Backend
    // must return the explicit frozen m2_project permissions for this account.
    permissions.value = permissionResult.permissions;
    if (
      !project.value ||
      getFundingContractPair(project.value) !== 'fair_launch_v1'
    ) {
      capabilityMessage.value =
        '项目不是 exact fair_launch/fair_launch_v1，已拒绝动作';
      return;
    }
    try {
      capability.value = await getFairLaunchCapabilities(projectId);
      capabilityMessage.value = capability.value.available
        ? ''
        : capability.value.reason_code || 'Backend capability unavailable';
    } catch (error) {
      capability.value = null;
      capabilityMessage.value = isFairLaunchCapabilityUnavailable(error)
        ? 'Backend capability unavailable'
        : 'Capability 读取失败；为安全起见保持全部动作禁用';
    }
  } catch (error: any) {
    message.error(error?.message || 'Fair Launch 控制台加载失败');
  } finally {
    loading.value = false;
  }
}

function openOps(path: string) {
  router.push({
    path,
    query: {
      project_id: String(projectId),
      contract_version: 'fair_launch_v1',
    },
  });
}

onMounted(load);
</script>

<template>
  <Page
    :title="`Fair Launch V1 · Admin 控制台 · 项目 #${projectId}`"
    description="Backend capability 驱动的四眼审批、durable submission、治理与财务可视化。Admin 不签名、不广播、不托管资金。"
  >
    <Spin :spinning="loading">
      <Card v-if="!project" title="选择 Fair Launch 项目" class="mb-3">
        <Space>
          <Input v-model:value="projectLookupId" placeholder="project_id" />
          <Button
            type="primary"
            @click="
              router.push(
                `/joincare/projects/${projectLookupId}/fair-launch/operations`,
              )
            "
          >
            打开
          </Button>
        </Space>
      </Card>
      <Alert
        v-if="capabilityMessage"
        type="error"
        show-icon
        banner
        class="mb-3"
        message="Fair Launch actions disabled"
        :description="capabilityMessage"
      />
      <Alert
        v-if="exactPair && !localOnlyAuthority"
        type="error"
        show-icon
        banner
        class="mb-3"
        message="Fair Launch materialization unavailable · 全部写操作已禁用"
        :description="`${FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE}：可信 runtime 身份不代表生命周期、治理或资金效果可以安全入库。当前仅保留只读 runtime、审批历史、执行历史和链事件审计。`"
      />
      <Alert
        v-if="exactPair && localOnlyAuthority"
        type="warning"
        show-icon
        banner
        class="mb-3"
        message="LOCAL HARDHAT TEST ONLY — NO REAL FUNDS"
        description="Dedicated run-bound localhost capability is active. Admin remains non-custodial and does not sign or broadcast transactions."
      />

      <Card v-if="project" title="项目 / Runtime / 财务状态" class="mb-3">
        <Descriptions :column="2" bordered size="small">
          <DescriptionsItem label="Identity">
            <Tag :color="exactPair ? 'purple' : 'red'">
              {{ project.funding_mode }} / {{ project.contract_version }}
            </Tag>
          </DescriptionsItem>
          <DescriptionsItem label="Lifecycle">
            {{ project.status }}
          </DescriptionsItem>
          <DescriptionsItem label="Runtime enabled">
            {{ capability?.runtime_enabled ? 'yes' : 'no' }}
          </DescriptionsItem>
          <DescriptionsItem label="Runtime verified">
            {{ capability?.runtime?.runtime_verified ? 'yes' : 'no' }}
          </DescriptionsItem>
          <DescriptionsItem label="Chain ID">
            {{ capability?.runtime?.chain_id ?? '—' }}
          </DescriptionsItem>
          <DescriptionsItem label="Proxy（只读 trusted runtime）">
            <code>{{ capability?.runtime?.proxy_address || '—' }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="Sale Token Amount">
            {{
              formatFinancialDecimal(
                capability?.financial_state?.sale_token_amount ||
                  project.sale_token_amount,
              )
            }}
          </DescriptionsItem>
          <DescriptionsItem label="Soft Cap">
            {{
              formatFinancialDecimal(
                capability?.financial_state?.soft_cap || project.soft_cap,
              )
            }}
          </DescriptionsItem>
          <DescriptionsItem label="Deposit / Required">
            {{
              formatFinancialDecimal(
                capability?.financial_state?.deposited_sale_tokens,
              )
            }}
            /
            {{
              formatFinancialDecimal(
                capability?.financial_state?.required_deposit,
              )
            }}
          </DescriptionsItem>
          <DescriptionsItem label="Reconciliation">
            <Tag :color="localOnlyAuthority ? 'green' : 'default'">
              {{
                localOnlyAuthority
                  ? capability?.financial_state?.reconciliation_state ||
                    'balanced'
                  : 'materialization unavailable'
              }}
            </Tag>
          </DescriptionsItem>
        </Descriptions>
      </Card>

      <Card title="项目方充值指导" class="mb-3">
        <Alert
          type="info"
          show-icon
          message="必须由 projectOwner 钱包充值"
          description="Admin、Operator、Job 不得代充。请项目方从自己的钱包调用受信 Fair Launch Proxy 的 depositProjectTokens(projectId, exact saleTokenAmount)。本页不生成签名、不接受私钥、不广播交易；地址只来自 Backend runtime。"
        />
      </Card>

      <Card title="Evidence / Governance input" class="mb-3">
        <Alert
          type="warning"
          show-icon
          :message="
            localOnlyAuthority
              ? 'LOCAL-ONLY descriptor inspection; transaction signing remains external'
              : 'Mutation inputs and MFA assertions are not collected while materialization is unavailable'
          "
          :description="
            localOnlyAuthority
              ? 'Capability descriptors expose which fixture actions are currently valid. This page still never accepts private keys, seed phrases, or raw signatures.'
              : '恢复写能力后仍只接受获批 provider 签发的 opaque、single-use assertion；本页永不接受私钥、助记词或原始签名。'
          "
        />
      </Card>

      <Card
        v-for="group in actionGroups"
        :key="group.title"
        :title="group.title"
        class="mb-3"
      >
        <p class="text-secondary">{{ group.description }}</p>
        <Space wrap>
          <Tag
            v-for="action in group.actions"
            :key="action"
            :color="access(action).allowed ? 'green' : 'default'"
            :title="descriptorTitle(action)"
          >
            {{ actionLabels[action] }} ·
            {{
              access(action).allowed
                ? 'LOCAL-ONLY authorized'
                : fairLaunchActionReasonText(access(action))
            }}
          </Tag>
        </Space>
      </Card>

      <Card title="Durable / Event / Ledger / Reconciliation" class="mb-3">
        <Alert
          type="warning"
          show-icon
          class="mb-3"
          message="Fair Launch canonical ledger / reconciliation API 尚不可用"
          description="Legacy V2 Fund Ledger 与 Reconciliation 不接受 fair_launch/fair_launch_v1 项目。本页不会导航到旧 V2 财务入口。"
        />
        <Space wrap>
          <Button @click="openOps('/joincare/approvals')">四眼审批</Button>
          <Button
            @click="
              router.push(
                `/joincare/projects/${projectId}/fair-launch/execution-queue`,
              )
            "
          >
            第三执行队列
          </Button>
          <Button @click="openOps('/joincare/chain-events')">
            Chain events / reorg
          </Button>
          <Button disabled title="fair_launch_materialization_unavailable">
            Fair Launch fund ledger unavailable
          </Button>
          <Button disabled title="fair_launch_materialization_unavailable">
            Fair Launch reconciliation unavailable
          </Button>
        </Space>
      </Card>
    </Spin>
  </Page>
</template>

<style scoped>
.evidence-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.text-secondary {
  color: var(--ant-color-text-secondary);
}

code {
  overflow-wrap: anywhere;
}

@media (max-width: 760px) {
  .evidence-grid {
    grid-template-columns: 1fr;
  }
}
</style>
