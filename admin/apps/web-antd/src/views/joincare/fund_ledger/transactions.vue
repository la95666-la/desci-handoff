<script setup lang="ts">
import type {
  FundLedgerEntryType,
  FundLedgerRow,
  PurchaseEntitlementRow,
} from '#/api/joincare/fund_ledger';

import { onBeforeUnmount, onMounted, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Drawer,
  Empty,
  message,
  Modal,
  Row,
  Select,
  Spin,
  Statistic,
  Table,
  Tag,
} from 'ant-design-vue';

import { listProjectFundLedger } from '#/api/joincare/fund_ledger';
import { useFundLedgerStore } from '#/store/fund-ledger';

import ProjectPicker from './components/ProjectPicker.vue';
import {
  collectLedgerExport,
  createLedgerWorkbook,
  ledgerExportCells,
} from './ledger-export';

const store = useFundLedgerStore();

const BSC_SCAN = 'https://bscscan.com';
const exporting = ref(false);
const exportProgress = ref('');
let exportGeneration = 0;
const exportScope = () =>
  JSON.stringify([
    store.projectId,
    store.loadedProjectId,
    store.canRead,
    store.filter.entryType,
    store.filter.status,
  ]);
watch(exportScope, () => {
  exportGeneration++;
});
onBeforeUnmount(() => {
  exportGeneration++;
});

async function exportExcel() {
  if (
    exporting.value ||
    store.loading ||
    !store.canRead ||
    !store.loadedProjectId ||
    store.loadedProjectId !== store.projectId
  )
    return;
  const generation = ++exportGeneration;
  const scope = exportScope();
  const projectId = String(store.loadedProjectId);
  const entryType = store.filter.entryType;
  const status = store.filter.status;
  const current = () =>
    generation === exportGeneration && scope === exportScope();
  exporting.value = true;
  exportProgress.value = '准备导出…';
  try {
    const rows = await collectLedgerExport(
      (page, pageSize) =>
        listProjectFundLedger(projectId, {
          page,
          page_size: pageSize,
          entry_type: entryType,
          status,
        }),
      current,
      (loaded, total) => {
        if (current())
          exportProgress.value = `正在导出 ${loaded} / ${total} 条`;
      },
    );
    if (!current()) return;
    const now = new Date();
    const workbook = createLedgerWorkbook(
      ledgerExportCells(rows, projectId, store.entryLabels),
      [
        ['字段', '内容'],
        ['项目ID', projectId],
        ['Entry类型', entryType || '全部'],
        ['账本状态', status || '全部'],
        ['记录数', String(rows.length)],
        ['导出时间（UTC）', now.toISOString()],
        ['精度说明', '金额、ID、地址与哈希按文本保存，避免Excel数值精度丢失。'],
        [
          '读取范围',
          '当前项目与筛选条件下全部分页记录；不包含逐笔认购权益表。',
        ],
        [
          '一致性说明',
          '分页只读导出，并非固定区块或数据库事务快照。账本可能在导出后变化，财务核对以正式账本与审计证据为准。',
        ],
      ],
    );
    if (!current()) return;
    const blob = new Blob([new Uint8Array(workbook)], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `fund-ledger-${projectId}-${now.toISOString().replaceAll(/[:.]/g, '-')}.xlsx`;
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    message.success(`已导出 ${rows.length} 条逐笔账本`);
  } catch (error: unknown) {
    if (current())
      message.error(
        error instanceof Error ? error.message : 'Excel导出失败，未生成文件',
      );
  } finally {
    exporting.value = false;
    exportProgress.value = '';
  }
}

function cancelExport() {
  exportGeneration++;
  exportProgress.value = '正在取消…';
}

function copyWallet(addr: string) {
  navigator.clipboard.writeText(addr).then(
    () => message.success('已复制钱包地址'),
    () => message.error('复制失败'),
  );
}

onMounted(() => {
  void store.loadPermissions();
});
</script>

<template>
  <Page
    title="用户认购及权益及逐笔交易账本"
    description="逐笔认购支付/退款/Claim账本、认购权益事实与Entry聚合。"
  >
    <ProjectPicker />

    <template v-if="store.canRead && store.reconciliation">
      <Row :gutter="12" style="margin-bottom: 24px">
        <Col :xs="24" :md="8">
          <Card size="small">
            <Statistic
              title="认购权益事实"
              :value="store.reconciliation.purchase_entitlements.count"
            />
          </Card>
        </Col>
        <Col :xs="24" :md="8">
          <Card size="small">
            <Statistic
              :title="`权益支付 · ${store.reconciliation.project.payment_token}`"
              :value="
                store.reconciliation.purchase_entitlements
                  .payment_amount_display
              "
            />
          </Card>
        </Col>
        <Col :xs="24" :md="8">
          <Card size="small">
            <Statistic
              :title="`衍生已售 Derived sold · ${store.reconciliation.project.project_token}`"
              :value="
                store.reconciliation.purchase_entitlements
                  .total_tokens_sold_display
              "
            />
          </Card>
        </Col>
      </Row>

      <Card title="流水聚合 Entry聚合" style="margin-bottom: 24px">
        <Table
          :data-source="store.reconciliation.aggregates"
          :pagination="false"
          :row-key="(row: any) => row.entry_type"
          :columns="[
            { title: '类型', dataIndex: 'entry_type', width: 220 },
            { title: '资产', dataIndex: 'token_symbol', width: 100 },
            { title: '笔数', dataIndex: 'count', width: 90 },
            { title: '金额', dataIndex: 'amount_display', width: 220 },
          ]"
          size="small"
          bordered
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'entry_type'">
              {{ store.entryLabels[record.entry_type as FundLedgerEntryType] }}
              <code class="ml-1 text-xs">{{ record.entry_type }}</code>
            </template>
          </template>
        </Table>
      </Card>

      <Card title="逐笔认购权益（非资金流）" style="margin-bottom: 24px">
        <Alert
          type="info"
          show-icon
          class="mb-3"
          message="每行与一笔 purchase_payment 账本一一绑定；project token金额表示tokensBought权益，不表示认购时已发生ERC20转出。"
        />
        <Table
          :columns="store.entitlementColumns"
          :data-source="store.entitlementRows"
          :loading="store.loading"
          :pagination="{
            current: store.entitlementPage,
            pageSize: store.entitlementPageSize,
            total: store.entitlementTotal,
            showSizeChanger: true,
            onChange: (nextPage: number, nextSize: number) => {
              store.entitlementPage = nextPage;
              store.entitlementPageSize = nextSize;
              store.load();
            },
          }"
          :row-key="(row: PurchaseEntitlementRow) => row.id"
          :scroll="{ x: 2100 }"
          size="small"
          bordered
        >
          <template #emptyText>
            <Empty description="暂无认购权益事实" />
          </template>
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'wallet_address'">
              <a
                class="text-blue-600"
                @click="store.openWalletDetail(record.wallet_address)"
                >{{ record.wallet_address }}</a>
            </template>
            <template v-else-if="column.dataIndex === 'twitter_handle'">
              {{ record.twitter_handle || '—' }}
            </template>
            <template v-else-if="column.dataIndex === 'telegram_username'">
              {{ record.telegram_username || '—' }}
            </template>
            <template v-else-if="column.dataIndex === 'tx_hash'">
              <code :title="record.tx_hash">{{
                store.shortHash(record.tx_hash)
              }}</code>
            </template>
          </template>
        </Table>
      </Card>

      <Card title="逐笔账本">
        <div class="mb-3 flex flex-wrap items-center gap-2">
          <Select
            v-model:value="store.filter.entryType"
            :options="store.entryOptions"
            allow-clear
            placeholder="Entry类型"
            style="width: 220px"
            @change="store.searchProject"
          />
          <Select
            v-model:value="store.filter.status"
            :options="store.statusOptions"
            allow-clear
            placeholder="账本状态"
            style="width: 180px"
            @change="store.searchProject"
          />
          <Button @click="store.resetFilters">重置筛选</Button>
          <Button :loading="store.loading" @click="store.load">刷新</Button>
          <Button
            type="primary"
            :loading="exporting"
            :disabled="
              store.loading ||
              !store.canRead ||
              store.loadedProjectId !== store.projectId
            "
            @click="exportExcel"
          >
            导出 Excel
          </Button>
          <Button v-if="exporting" @click="cancelExport">取消导出</Button>
          <span v-if="exporting" role="status" class="text-sm text-gray-500">{{
            exportProgress
          }}</span>
          <span class="ml-auto text-sm text-gray-500">
            共 {{ store.total }} 条
          </span>
        </div>
        <Table
          :columns="store.columns"
          :data-source="store.rows"
          :loading="store.loading"
          :pagination="{
            current: store.page,
            pageSize: store.pageSize,
            total: store.total,
            showSizeChanger: true,
            onChange: (nextPage: number, nextSize: number) => {
              store.page = nextPage;
              store.pageSize = nextSize;
              store.load();
            },
          }"
          :row-key="(row: FundLedgerRow) => row.id"
          :scroll="{ x: 1900 }"
          size="small"
          bordered
        >
          <template #emptyText><Empty description="暂无账本记录" /></template>
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'entry_type'">
              {{ store.entryLabels[record.entry_type as FundLedgerEntryType] }}
            </template>
            <template v-else-if="column.dataIndex === 'status'">
              <Tag :color="record.status === 'canonical' ? 'green' : 'red'">
                {{ record.status }}
              </Tag>
            </template>
            <template v-else-if="column.dataIndex === 'wallet_address'">
              <a
                class="text-blue-600"
                @click="store.openWalletDetail(record.wallet_address)"
                >{{ record.wallet_address }}</a>
            </template>
            <template v-else-if="column.dataIndex === 'twitter_handle'">
              {{ record.twitter_handle || '—' }}
            </template>
            <template v-else-if="column.dataIndex === 'telegram_username'">
              {{ record.telegram_username || '—' }}
            </template>
            <template v-else-if="column.dataIndex === 'tx_hash'">
              <code :title="record.tx_hash">{{
                store.shortHash(record.tx_hash)
              }}</code>
            </template>
            <template v-else-if="column.dataIndex === 'created_at'">
              {{ store.fmtTime(record.created_at) }}
            </template>
            <template v-else-if="column.dataIndex === '_action'">
              <Button
                size="small"
                type="link"
                @click="store.openDetail(record as FundLedgerRow)"
              >
                详情
              </Button>
            </template>
          </template>
        </Table>
      </Card>
    </template>

    <Modal
      v-model:open="store.detailVisible"
      :footer="null"
      title="逐笔账本详情（安全字段）"
      width="900px"
    >
      <Descriptions v-if="store.detail" :column="2" bordered size="small">
        <DescriptionsItem label="ID">{{ store.detail.id }}</DescriptionsItem>
        <DescriptionsItem label="类型 Entry">
          {{ store.detail.entry_type }}
        </DescriptionsItem>
        <DescriptionsItem label="资产 Asset">
          {{ store.detail.token_symbol }} /
          {{ store.detail.token_decimals }}
        </DescriptionsItem>
        <DescriptionsItem label="状态 Status">
          {{ store.detail.status }}
        </DescriptionsItem>
        <DescriptionsItem label="金额">
          {{ store.detail.amount_display }}
        </DescriptionsItem>
        <DescriptionsItem label="原始值 Amount raw">
          <code>{{ store.detail.amount_raw }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="钱包 Wallet" :span="2">
          <code>{{ store.detail.wallet_address || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="对手方 Counterparty" :span="2">
          <code>{{ store.detail.counterparty_address || '—' }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="代币 Token" :span="2">
          <code>{{ store.detail.token_address }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="交易哈希 Tx hash" :span="2">
          <code>{{ store.detail.tx_hash }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="区块 Block">
          {{ store.detail.block_number }}
        </DescriptionsItem>
        <DescriptionsItem label="日志索引 Log index">
          {{ store.detail.log_index }}
        </DescriptionsItem>
        <DescriptionsItem label="区块哈希 Block hash" :span="2">
          <code>{{ store.detail.block_hash }}</code>
        </DescriptionsItem>
        <DescriptionsItem label="创建时间 Created">
          {{ store.fmtTime(store.detail.created_at) }}
        </DescriptionsItem>
        <DescriptionsItem label="更新时间 Updated">
          {{ store.fmtTime(store.detail.updated_at) }}
        </DescriptionsItem>
      </Descriptions>
    </Modal>

    <Drawer
      v-model:open="store.walletDetailVisible"
      title="用户认购账本"
      width="66vw"
    >
      <Spin :spinning="store.walletDetailLoading">
        <template v-if="store.walletDetail">
          <Descriptions :column="2" bordered size="small" class="mb-4">
            <DescriptionsItem label="钱包" :span="2">
              <code>{{ store.walletDetail.wallet_address }}</code>
              <a
                class="ml-2"
                @click="copyWallet(store.walletDetail.wallet_address)"
                >复制</a>
              <a
                class="ml-2"
                :href="`${BSC_SCAN}/address/${store.walletDetail.wallet_address}`"
                target="_blank"
                rel="noopener"
                >BSC Scan</a>
            </DescriptionsItem>
            <DescriptionsItem label="X账号">
              {{ store.walletDetail.twitter_handle || '—' }}
            </DescriptionsItem>
            <DescriptionsItem label="TG账号">
              {{ store.walletDetail.telegram_username || '—' }}
            </DescriptionsItem>
            <DescriptionsItem label="权益笔数">
              {{ store.walletDetail.entitlement_count }}
            </DescriptionsItem>
            <DescriptionsItem label="已付金额">
              {{ store.walletDetail.payment_display }}
            </DescriptionsItem>
            <DescriptionsItem label="获得Token">
              {{ store.walletDetail.tokens_bought_display }}
            </DescriptionsItem>
            <DescriptionsItem label="项目状态">
              {{ store.walletDetail.project_status || '—' }}
            </DescriptionsItem>
            <DescriptionsItem label="预期退款（实时）">
              <Tag v-if="store.walletDetail.refund_expected_now" color="orange">
                是（应退未退）
              </Tag>
              <Tag v-else>否</Tag>
            </DescriptionsItem>
            <DescriptionsItem label="实际退款（实时）">
              <Tag v-if="store.walletDetail.refunded_now" color="green">
                是 · {{ store.walletDetail.refunded_now_display }}
              </Tag>
              <Tag v-else>否</Tag>
            </DescriptionsItem>
          </Descriptions>

          <template v-if="store.walletDetail.onchain">
            <div class="mb-2 font-medium">
              链上记录（审计任务 #{{ store.walletDetail.onchain_run_id }}
              固定区块观测）
            </div>
            <Descriptions :column="2" bordered size="small" class="mb-4">
              <DescriptionsItem label="预期支付">
                {{ store.walletDetail.onchain.expected_payment_display }}
                <span class="ml-1 text-xs opacity-50">raw:
                  {{ store.walletDetail.onchain.expected_payment_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="实际支付">
                {{ store.walletDetail.onchain.actual_payment_display }}
                <span class="ml-1 text-xs opacity-50">raw:
                  {{ store.walletDetail.onchain.actual_payment_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="预期Token">
                {{ store.walletDetail.onchain.expected_tokens_display }}
                <span class="ml-1 text-xs opacity-50">raw:
                  {{ store.walletDetail.onchain.expected_tokens_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="实际Token">
                {{ store.walletDetail.onchain.actual_tokens_display }}
                <span class="ml-1 text-xs opacity-50">raw: {{ store.walletDetail.onchain.actual_tokens_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="预期Claim">
                {{ store.walletDetail.onchain.expected_claimed_display }}
                <span class="ml-1 text-xs opacity-50">raw:
                  {{ store.walletDetail.onchain.expected_claimed_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="实际Claim">
                {{ store.walletDetail.onchain.actual_claimed_display }}
                <span class="ml-1 text-xs opacity-50">raw:
                  {{ store.walletDetail.onchain.actual_claimed_raw }}</span>
              </DescriptionsItem>
              <DescriptionsItem label="预期退款（快照）">
                {{ store.walletDetail.onchain.expected_refunded ? '是' : '否' }}
              </DescriptionsItem>
              <DescriptionsItem label="实际退款（快照）">
                {{ store.walletDetail.onchain.actual_refunded ? '是' : '否' }}
              </DescriptionsItem>
              <DescriptionsItem label="观测时间" :span="2">
                {{
                  store.walletDetail.onchain.observed_at || '—'
                }}（若晚于最近链上事件，请重新发起审计）
              </DescriptionsItem>
            </Descriptions>
          </template>

          <div class="mb-2 font-medium">流水</div>
          <Table
            :columns="[
              { title: 'ID', dataIndex: 'id', width: 70 },
              { title: '类型', dataIndex: 'entry_type', width: 150 },
              { title: '资产', dataIndex: 'token_symbol', width: 90 },
              {
                title: '金额',
                dataIndex: 'amount_display',
                width: 160,
              },
              { title: '状态', dataIndex: 'status', width: 110 },
              { title: '区块', dataIndex: 'block_number', width: 110 },
              {
                title: '交易哈希 Tx Hash',
                dataIndex: 'tx_hash',
                width: 200,
              },
              { title: '时间', dataIndex: 'created_at', width: 170 },
            ]"
            :data-source="store.walletDetail.ledger"
            :pagination="{ pageSize: 10, showSizeChanger: true }"
            :row-key="(row: FundLedgerRow) => row.id"
            :scroll="{ x: 1060 }"
            size="small"
            bordered
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'entry_type'">
                {{
                  store.entryLabels[record.entry_type as FundLedgerEntryType]
                }}
              </template>
              <template v-else-if="column.dataIndex === 'status'">
                <Tag :color="record.status === 'canonical' ? 'green' : 'red'">
                  {{ record.status }}
                </Tag>
              </template>
              <template v-else-if="column.dataIndex === 'tx_hash'">
                <a
                  :href="`${BSC_SCAN}/tx/${record.tx_hash}`"
                  target="_blank"
                  rel="noopener"
                  :title="record.tx_hash"
                >
                  {{ store.shortHash(record.tx_hash) }}
                </a>
              </template>
              <template v-else-if="column.dataIndex === 'created_at'">
                {{ store.fmtTime(record.created_at) }}
              </template>
            </template>
          </Table>
        </template>
        <Empty
          v-else-if="!store.walletDetailLoading"
          description="暂无该钱包的认购数据"
        />
      </Spin>
    </Drawer>
  </Page>
</template>
