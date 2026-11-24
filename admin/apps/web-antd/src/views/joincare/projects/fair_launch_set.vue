<script setup lang="ts">
import type { FairLaunchSetCurrent } from '#/api/joincare/fair_launch';

import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Card,
  Descriptions,
  DescriptionsItem,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  FAIR_LAUNCH_CONTRACT_VERSION,
  FAIR_LAUNCH_FUNDING_MODE,
  getFairLaunchSet,
} from '#/api/joincare/fair_launch';

const route = useRoute();
const projectId = String(route.params.id ?? '');
const loading = ref(true);
const current = ref<FairLaunchSetCurrent | null>(null);
const loadError = ref('');
const MATERIALIZATION_UNAVAILABLE = 'fair_launch_materialization_unavailable';

async function load() {
  loading.value = true;
  loadError.value = '';
  current.value = null;
  try {
    const result = await getFairLaunchSet(projectId);
    if (
      result.funding_mode !== FAIR_LAUNCH_FUNDING_MODE ||
      result.contract_version !== FAIR_LAUNCH_CONTRACT_VERSION
    ) {
      loadError.value =
        'Set read model 的 funding_mode / contract_version 不匹配';
      return;
    }
    current.value = result;
  } catch (error: any) {
    loadError.value = error?.message || 'Fair Launch Set 只读摘要加载失败';
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page
    :title="`Fair Launch Set 只读摘要 · 项目 #${projectId}`"
    description="当前仅展示 Backend 返回的 Set read model，不提供草稿编辑、Preview、提交、审批发起或敏感认证信息输入。"
  >
    <Alert
      type="error"
      show-icon
      banner
      class="mb-3"
      message="Fair Launch materialization unavailable"
      :description="`${MATERIALIZATION_UNAVAILABLE}：Set 写入与执行入口已关闭；本页不会收集 MFA assertion 或其他敏感输入。`"
    />

    <Alert
      v-if="loadError"
      type="error"
      show-icon
      class="mb-3"
      message="Set read model 加载失败"
      :description="loadError"
    />

    <Card :loading="loading" title="Dedicated Fair Launch Set read model">
      <Descriptions v-if="current" :column="2" bordered size="small">
        <DescriptionsItem label="Funding mode">
          <code>{{ current.funding_mode }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Contract version">
          <code>{{ current.contract_version }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Configured">
          <Tag :color="current.configured ? 'green' : 'default'">
            {{ current.configured ? 'yes' : 'no' }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem label="Locked">
          <Tag :color="current.locked ? 'red' : 'default'">
            {{ current.locked ? 'onchain_locked' : 'unlocked' }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem label="Status">
          {{ current.status || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Payment Token">
          {{ current.payment_token || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Project Token">
          <code>{{ current.project_token || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Project Token Symbol">
          {{ current.project_token_symbol || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Project Owner" :span="2">
          <code>{{ current.project_owner || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Project Proceeds Receiver" :span="2">
          <code>{{ current.project_proceeds_receiver || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="Sale Token Amount">
          {{ current.sale_token_amount || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Soft Cap">
          {{ current.soft_cap || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Minimum Contribution">
          {{ current.min_contribution || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Maximum Contribution">
          {{ current.max_contribution || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="Start Time">
          {{ current.start_time || '—' }}
        </DescriptionsItem>
        <DescriptionsItem label="End Time">
          {{ current.end_time || '—' }}
        </DescriptionsItem>
      </Descriptions>

      <Alert
        v-else-if="!loading && !loadError"
        type="info"
        show-icon
        message="Set read model 暂无数据"
      />
    </Card>

    <Card v-if="current?.vesting?.length" class="mt-3" title="Vesting（只读）">
      <Table
        :columns="[
          { title: 'Unlock Time', dataIndex: 'unlock_time' },
          { title: 'BPS', dataIndex: 'bps', width: 160 },
        ]"
        :data-source="current.vesting"
        :pagination="false"
        :row-key="(row: any) => `${row.unlock_time}-${row.bps}`"
        bordered
        size="small"
      />
    </Card>
  </Page>
</template>

<style scoped>
code {
  overflow-wrap: anywhere;
}
</style>
