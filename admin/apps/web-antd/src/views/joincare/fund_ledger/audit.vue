<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Col,
  Input,
  Row,
  Statistic,
  Table,
  Tag,
} from 'ant-design-vue';

import type {
  IDOReconciliationSummary,
  IDOReconciliationWallet,
  OnchainReconciliationCheck,
} from '#/api/joincare/fund_ledger';
import { useFundLedgerStore } from '#/store/fund-ledger';

import ProjectPicker from './components/ProjectPicker.vue';

const store = useFundLedgerStore();

onMounted(() => {
  void store.loadPermissions();
});

onBeforeUnmount(() => {
  store.clearAuditPoll();
});
</script>

<template>
  <Page
    title="审计相关"
    description="持久化链上审计任务：创建、状态跟踪、链上快照与逐钱包固定区块观测。"
  >
    <ProjectPicker>
      <Button
        :disabled="
          !store.reconciliation ||
          !store.canExecute ||
          !store.loadedProjectId ||
          store.loadedProjectId !== store.projectId
        "
        :loading="store.onchainLoading"
        danger
        ghost
        @click="store.createOnchainAudit"
      >
        创建持久化链上审计任务
      </Button>
      <Input
        v-model:value="store.reconciliationReason"
        :maxlength="500"
        placeholder="审计理由（3–500字）"
        style="width: 280px"
      />
      <span class="text-sm text-gray-500">
        仅支持 contract_version=v2_stablecoin
      </span>
    </ProjectPicker>

    <template v-if="store.canRead && store.reconciliation">
      <Card title="持久化链上审计任务" style="margin-bottom: 24px">
        <Alert
          type="info"
          show-icon
          class="mb-3"
          message="任务由独立 joincare-job 分页执行并固定在创建时的 canonical DB cursor block。queued/running/retrying/cancel_requested/finalizing/failed/invalidated/cancelled 的 balanced 均为空，不会把部分结果展示为BALANCED。"
          description="数据库内manifest使用SHA-256摘要链并保留不可变源快照；当前不代表已接入KMS签名或外部WORM锚定。"
        />
        <div class="mb-2 flex items-center justify-between">
          <span class="font-medium">
            最近任务（{{ store.reconciliationTaskTotal }}）
          </span>
          <Button
            size="small"
            :loading="store.onchainLoading"
            @click="store.refreshAuditHistory()"
          >
            刷新历史
          </Button>
        </div>
        <Table
          :data-source="store.reconciliationTasks"
          :pagination="false"
          :row-key="(row: IDOReconciliationSummary) => row.run_id"
          :columns="[
            { title: '任务 Run', dataIndex: 'run_id', width: 90 },
            { title: '状态', dataIndex: 'state', width: 140 },
            { title: '结论 Outcome', dataIndex: 'outcome', width: 130 },
            {
              title: '区块 Block',
              dataIndex: 'evidence_block_number',
              width: 110,
            },
            {
              title: '钱包进度',
              dataIndex: 'processed_wallet_count',
              width: 140,
            },
            { title: '请求人', dataIndex: 'requested_by_admin_id', width: 100 },
            { title: '请求时间', dataIndex: 'requested_at', width: 180 },
            { title: '操作', dataIndex: '_audit_action', width: 90 },
          ]"
          size="small"
          bordered
          :scroll="{ x: 1050 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'state'">
              <Tag
                :color="
                  record.state === 'completed'
                    ? 'green'
                    : ['failed', 'invalidated', 'cancelled'].includes(
                          record.state,
                        )
                      ? 'red'
                      : 'blue'
                "
              >
                {{ record.state }}
              </Tag>
            </template>
            <template v-else-if="column.dataIndex === 'outcome'">
              <Tag v-if="record.invalidated_by_reorg" color="red">
                INVALIDATED_BY_REORG
              </Tag>
              <Tag
                v-else-if="record.outcome"
                :color="record.outcome === 'BALANCED' ? 'green' : 'red'"
              >
                {{ record.outcome }}
              </Tag>
              <span v-else>—</span>
            </template>
            <template v-else-if="column.dataIndex === 'processed_wallet_count'">
              {{ record.processed_wallet_count }} /
              {{ record.expected_wallet_count }}
            </template>
            <template v-else-if="column.dataIndex === 'requested_at'">
              {{ store.fmtTime(record.requested_at) }}
            </template>
            <template v-else-if="column.dataIndex === '_audit_action'">
              <Button
                type="link"
                size="small"
                @click="store.selectAudit(record.run_id)"
              >
                查看
              </Button>
            </template>
          </template>
        </Table>

        <template v-if="store.reconciliationTask">
          <div class="mb-2 mt-6 flex items-center justify-between">
            <span class="font-medium">
              任务 #{{ store.reconciliationTask.summary.run_id }} 详情
            </span>
            <div class="flex gap-2">
              <Button
                size="small"
                :loading="store.onchainLoading"
                @click="
                  store.openAudit(store.reconciliationTask.summary.run_id)
                "
              >
                刷新任务
              </Button>
              <Button
                v-if="
                  store.canExecute &&
                  ['queued', 'retrying', 'running'].includes(
                    store.reconciliationTask.summary.state,
                  )
                "
                danger
                size="small"
                @click="store.requestCancelAudit"
              >
                取消任务
              </Button>
            </div>
          </div>
          <Alert
            :type="
              store.reconciliationTask.summary.invalidated_by_reorg
                ? 'error'
                : store.reconciliationTask.summary.outcome === 'BALANCED'
                  ? 'success'
                  : store.reconciliationTask.summary.outcome === 'UNBALANCED'
                    ? 'error'
                    : 'info'
            "
            show-icon
            class="mb-3"
            :message="
              store.reconciliationTask.summary.invalidated_by_reorg
                ? 'INVALIDATED_BY_REORG · 历史manifest不再代表当前canonical事实'
                : store.reconciliationTask.summary.outcome ||
                  `任务状态：${store.reconciliationTask.summary.state}（尚无终态结论）`
            "
            :description="`证据区块 #${store.reconciliationTask.summary.evidence_block_number} · ${store.shortHash(store.reconciliationTask.summary.evidence_block_hash)}；source=${store.shortHash(store.reconciliationTask.summary.source_digest)}；manifest=${store.shortHash(store.reconciliationTask.summary.manifest_digest)}`"
          />
          <Alert
            v-if="store.reconciliationTask.summary.last_error_code"
            type="warning"
            show-icon
            class="mb-3"
            :message="store.reconciliationTask.summary.last_error_code"
            :description="store.reconciliationTask.summary.last_error_message"
          />
        </template>

        <template v-if="store.reconciliationTask?.onchain">
          <Alert
            :type="
              store.reconciliationTask.summary.invalidated_by_reorg
                ? 'warning'
                : store.reconciliationTask.summary.balanced
                  ? 'success'
                  : 'error'
            "
            show-icon
            class="mb-3"
            :message="
              store.reconciliationTask.summary.invalidated_by_reorg
                ? '该历史快照已被后续深重组事件失效；仅保留为历史证据，不得作为当前BALANCED结论'
                : store.reconciliationTask.summary.balanced
                  ? '数据库账本与固定区块链上快照一致'
                  : '链上快照ATTENTION：请核对项目/储备检查及逐钱包差异'
            "
            description="余额与reservedBalances是全局token偿付检查，不是单项目余额等式。"
          />
          <Row :gutter="12" class="mb-3">
            <Col :xs="12" :md="6">
              <Statistic
                title="综合状态"
                :value="
                  store.reconciliationTask.summary.invalidated_by_reorg
                    ? '已失效 INVALIDATED'
                    : store.reconciliationTask.summary.balanced
                      ? '平衡 BALANCED'
                      : '需关注 ATTENTION'
                "
                :value-style="{
                  color:
                    !store.reconciliationTask.summary.invalidated_by_reorg &&
                    store.reconciliationTask.summary.balanced
                      ? '#3f8600'
                      : '#cf1322',
                }"
              />
            </Col>
            <Col :xs="12" :md="6">
              <Statistic
                title="快照区块 Snapshot block"
                :value="store.reconciliationTask.onchain.block_number"
              />
            </Col>
            <Col :xs="12" :md="6">
              <Statistic
                title="RPC最新块 RPC latest"
                :value="store.reconciliationTask.onchain.latest_block"
              />
            </Col>
            <Col :xs="12" :md="6">
              <Statistic
                title="钱包证据"
                :value="store.reconciliationTask.summary.expected_wallet_count"
              />
            </Col>
          </Row>
          <Table
            :data-source="store.reconciliationTask.onchain.checks"
            :pagination="false"
            :row-key="(row: OnchainReconciliationCheck) => row.code"
            :columns="[
              { title: '检查', dataIndex: 'code', width: 260 },
              { title: '结果', dataIndex: 'ok', width: 100 },
              { title: '预期值 Expected', dataIndex: 'expected', width: 250 },
              { title: '实际值 Actual', dataIndex: 'actual', width: 250 },
              { title: '说明', dataIndex: 'message' },
            ]"
            size="small"
            bordered
            :scroll="{ x: 1200 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'code'">
                <span class="font-medium">{{
                  store.chainCheckLabel(record.code)
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
                  column.dataIndex === 'expected' ||
                  column.dataIndex === 'actual'
                "
              >
                <code>{{ record[column.dataIndex] || '—' }}</code>
              </template>
            </template>
          </Table>

          <div class="mb-3 mt-6 font-medium">全局Token储备快照</div>
          <Table
            :data-source="store.reconciliationTask.onchain.token_reserves"
            :pagination="false"
            :row-key="(row: any) => row.role"
            :columns="[
              { title: '角色 Role', dataIndex: 'role', width: 100 },
              { title: '代币 Token', dataIndex: 'symbol', width: 100 },
              { title: '允许量 Allowed', dataIndex: 'allowed', width: 100 },
              { title: '精度 Decimals', dataIndex: 'decimals', width: 100 },
              {
                title: '代理余额 Proxy balance',
                dataIndex: 'proxy_balance_raw',
              },
              {
                title: '全局预留 Global reserved',
                dataIndex: 'global_reserved_raw',
              },
              {
                title: '项目负债 Project liability',
                dataIndex: 'project_liability_raw',
              },
            ]"
            size="small"
            bordered
            :scroll="{ x: 1400 }"
          >
            <template #bodyCell="{ column, record }">
              <template
                v-if="
                  column.dataIndex === 'proxy_balance_raw' ||
                  column.dataIndex === 'global_reserved_raw' ||
                  column.dataIndex === 'project_liability_raw'
                "
              >
                {{ store.formatUnits(record[column.dataIndex], record.decimals) }}
              </template>
            </template>
          </Table>

          <div class="mb-3 mt-6 font-medium">逐钱包固定区块观测（分页）</div>
          <Table
            :data-source="store.reconciliationWallets"
            :pagination="{
              current: store.reconciliationWalletPage,
              pageSize: store.reconciliationWalletPageSize,
              total: store.reconciliationWalletTotal,
            }"
            :row-key="(row: IDOReconciliationWallet) => row.id"
            :columns="[
              { title: '序号 Ordinal', dataIndex: 'ordinal', width: 90 },
              {
                title: '钱包 Wallet',
                dataIndex: 'wallet_address',
                width: 300,
              },
              {
                title: '预期支付 Expected payment',
                dataIndex: 'expected_payment_raw',
              },
              {
                title: '实际支付 Actual payment',
                dataIndex: 'actual_payment_raw',
              },
              {
                title: '预期Token Expected tokens',
                dataIndex: 'expected_tokens_raw',
              },
              {
                title: '实际Token Actual tokens',
                dataIndex: 'actual_tokens_raw',
              },
              {
                title: '预期Claim Expected claimed',
                dataIndex: 'expected_claimed_raw',
              },
              {
                title: '实际Claim Actual claimed',
                dataIndex: 'actual_claimed_raw',
              },
              {
                title: '预期退款 Expected refunded',
                dataIndex: 'expected_refunded',
              },
              {
                title: '实际退款 Actual refunded',
                dataIndex: 'actual_refunded',
              },
            ]"
            size="small"
            bordered
            :scroll="{ x: 1900 }"
            @change="
              (pagination: any) => {
                store.reconciliationWalletPage = pagination.current || 1;
                store.reconciliationWalletPageSize = pagination.pageSize || 50;
                store.openAudit(store.reconciliationTask!.summary.run_id);
              }
            "
          >
            <template #bodyCell="{ column, record }">
              <template
                v-if="
                  column.dataIndex === 'expected_payment_raw' ||
                  column.dataIndex === 'actual_payment_raw'
                "
              >
                {{
                  store.formatUnits(
                    record[column.dataIndex],
                    store.reconciliationTask!.onchain!.project
                      .payment_token_decimals,
                  )
                }}
              </template>
              <template
                v-else-if="
                  column.dataIndex === 'expected_tokens_raw' ||
                  column.dataIndex === 'actual_tokens_raw' ||
                  column.dataIndex === 'expected_claimed_raw' ||
                  column.dataIndex === 'actual_claimed_raw'
                "
              >
                {{
                  store.formatUnits(
                    record[column.dataIndex],
                    store.reconciliationTask!.onchain!.project
                      .project_token_decimals,
                  )
                }}
              </template>
            </template>
          </Table>
        </template>
      </Card>
    </template>
  </Page>
</template>