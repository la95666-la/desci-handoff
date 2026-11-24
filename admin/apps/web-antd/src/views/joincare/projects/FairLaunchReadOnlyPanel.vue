<script setup lang="ts">
import type { ProjectListRow } from '#/api/joincare/project_content';

import { computed } from 'vue';

import { Alert, Button, Descriptions, Tag } from 'ant-design-vue';

import { formatFinancialDecimal, getFundingContractPair } from './funding-mode';

const props = defineProps<{ project: ProjectListRow }>();

const pair = computed(() => getFundingContractPair(props.project));
const pairValid = computed(() => pair.value === 'fair_launch_v1');
</script>

<template>
  <div class="fair-launch-panel">
    <Alert
      v-if="!pairValid"
      type="error"
      show-icon
      message="Funding mode 与 contract version 不匹配"
      description="原型拒绝推断合约类型；未知或不匹配项目不会开放任何生产动作。"
    />
    <Alert
      v-else
      type="warning"
      show-icon
      message="Fair Launch V1 摘要"
      description="Fair 仅表示结束后所有参与者使用统一清算价格；募资期间估算会随后续贡献变化和稀释。生产动作必须进入独立控制台并由 Backend capability、Preview、审批与 runtime 门禁决定。"
    />

    <Descriptions class="mt-3" :column="1" bordered size="small">
      <Descriptions.Item label="模式">
        <Tag :color="pairValid ? 'purple' : 'red'">Fair Launch · V1</Tag>
      </Descriptions.Item>
      <Descriptions.Item label="销售代币总量 saleTokenAmount">
        <span class="financial-value">{{
          formatFinancialDecimal(project.sale_token_amount)
        }}</span>
      </Descriptions.Item>
      <Descriptions.Item label="Soft Cap">
        <span class="financial-value">{{
          formatFinancialDecimal(project.soft_cap)
        }}</span>
      </Descriptions.Item>
      <Descriptions.Item label="Hard Cap">
        <Tag color="default">不适用 · 无 Hard Cap</Tag>
      </Descriptions.Item>
      <Descriptions.Item label="统一清算价格">
        到期成功结算后才冻结；当前贡献不形成固定 entitlement。
      </Descriptions.Item>
    </Descriptions>

    <div class="disabled-actions">
      <Button disabled>本摘要不执行动作</Button>
      <Tag color="orange">Backend capability unavailable 时控制台全部禁用</Tag>
    </div>
  </div>
</template>

<style scoped>
.fair-launch-panel {
  padding-top: 4px;
}

.financial-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  overflow-wrap: anywhere;
}

.disabled-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
</style>
