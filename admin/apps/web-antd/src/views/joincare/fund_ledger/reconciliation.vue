<script setup lang="ts">
import { onMounted } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Row,
  Statistic,
  Table,
  Tag,
} from 'ant-design-vue';

import { useFundLedgerStore } from '#/store/fund-ledger';

import ProjectPicker from './components/ProjectPicker.vue';

const store = useFundLedgerStore();

onMounted(() => {
  void store.loadPermissions();
});
</script>

<template>
  <Page
    title="项目资金身份及账本状态、对账检查"
    description="项目资金身份、数据库账本状态统计与逐项对账检查。"
  >
    <ProjectPicker />

    <template v-if="store.canRead && store.reconciliation">
      <Row :gutter="12" style="margin-bottom: 24px">
        <Col :xs="12" :md="6">
          <Card size="small">
            <Statistic
              title="数据库账本状态"
              :value="store.reconciliation.balanced ? 'BALANCED' : 'ATTENTION'"
              :value-style="{
                color: store.reconciliation.balanced ? '#3f8600' : '#cf1322',
              }"
            />
          </Card>
        </Col>
        <Col :xs="12" :md="6">
          <Card size="small">
            <Statistic title="异常检查" :value="store.failedChecks.length" />
          </Card>
        </Col>
        <Col :xs="12" :md="6">
          <Card size="small">
            <Statistic
              :title="`已筹金额 Raised · ${store.reconciliation.project.payment_token}`"
              :value="store.reconciliation.project.total_raised_display"
            />
          </Card>
        </Col>
        <Col :xs="12" :md="6">
          <Card size="small">
            <Statistic
              :title="`已存入金额 Deposited · ${store.reconciliation.project.project_token}`"
              :value="store.reconciliation.project.total_deposited_display"
            />
          </Card>
        </Col>
      </Row>

      <Card title="项目资金身份" style="margin-bottom: 24px">
        <Descriptions :column="2" bordered size="small">
          <DescriptionsItem label="项目 Project">
            #{{ store.reconciliation.project.project_id }} ·
            {{ store.reconciliation.project.name_en }}
          </DescriptionsItem>
          <DescriptionsItem label="状态">
            {{ store.reconciliation.project.status }}
          </DescriptionsItem>
          <DescriptionsItem label="链 ID Chain ID">
            {{ store.reconciliation.project.chain_id }}
          </DescriptionsItem>
          <DescriptionsItem label="合约 Contract">
            <code>{{ store.reconciliation.project.contract_address }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="支付币 Payment token">
            {{ store.reconciliation.project.payment_token }} /
            {{ store.reconciliation.project.payment_token_decimals }} decimals
          </DescriptionsItem>
          <DescriptionsItem label="支付币地址 Payment token address">
            <code>{{
              store.reconciliation.project.payment_token_address
            }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="项目币 Project token">
            {{ store.reconciliation.project.project_token }} /
            {{ store.reconciliation.project.project_token_decimals }} decimals
          </DescriptionsItem>
          <DescriptionsItem label="项目币地址 Project token address">
            <code>{{
              store.reconciliation.project.project_token_address
            }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="Token price" :span="2">
            {{ store.reconciliation.project.token_price_display }}
            {{ store.reconciliation.project.payment_token }}
          </DescriptionsItem>
          <DescriptionsItem label="已筹金额 Raised" :span="2">
            {{ store.reconciliation.project.total_raised_display }}
            {{ store.reconciliation.project.payment_token }}
          </DescriptionsItem>
          <DescriptionsItem label="已存金额 Deposited" :span="2">
            {{ store.reconciliation.project.total_deposited_display }}
            {{ store.reconciliation.project.project_token }}
          </DescriptionsItem>
          <DescriptionsItem label="费用快照 Fee snapshot">
            {{ store.reconciliation.project.success_fee_rate }} bps ·
            <code>{{ store.reconciliation.project.fee_receiver }}</code>
          </DescriptionsItem>
          <DescriptionsItem label="标志 Flags">
            proceeds={{ store.reconciliation.project.fund_withdrawn }} · fee={{
              store.reconciliation.project.platform_fee_withdrawn
            }}
            · token={{ store.reconciliation.project.token_withdrawn }}
          </DescriptionsItem>
        </Descriptions>
      </Card>

      <Card title="逐项对账检查" style="margin-bottom: 24px">
        <Table
          :data-source="store.reconciliation.checks"
          :pagination="false"
          :row-key="(row: any) => row.code"
          :columns="[
            { title: '检查', dataIndex: 'code', width: 220 },
            { title: '结果', dataIndex: 'ok', width: 100 },
            {
              title: '预期值 Expected',
              dataIndex: 'expected',
              width: 260,
              ellipsis: true,
            },
            {
              title: '实际值 Actual',
              dataIndex: 'actual',
              width: 260,
              ellipsis: true,
            },
            { title: '说明', dataIndex: 'message' },
          ]"
          size="small"
          bordered
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'code'">
              <span class="font-medium">{{
                store.checkCodeLabels[record.code] || record.code
              }}</span>
              <code class="ml-1 text-xs text-gray-500">{{ record.code }}</code>
            </template>
            <template v-else-if="column.dataIndex === 'ok'">
              <Tag :color="record.ok ? 'green' : 'red'">
                {{ record.ok ? 'OK' : 'ATTENTION' }}
              </Tag>
            </template>
            <template
              v-else-if="
                column.dataIndex === 'expected' || column.dataIndex === 'actual'
              "
            >
              <code :title="record[column.dataIndex]">{{
                record[column.dataIndex] || '—'
              }}</code>
            </template>
          </template>
        </Table>
      </Card>
    </template>
  </Page>
</template>
