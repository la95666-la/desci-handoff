<script setup lang="ts">
import type { FairLaunchExecutionQueueItem } from '#/api/joincare/fair_launch';

import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  message,
  Space,
  Table,
  Tag,
  Tooltip,
} from 'ant-design-vue';

import { getFairLaunchExecutionQueue } from '#/api/joincare/fair_launch';
import { getProjectContent } from '#/api/joincare/project_content';

import { loadExactFairLaunchExecutionQueue } from './fair-launch-admin';

const route = useRoute();
const projectId = String(route.params.id ?? '');
const loading = ref(false);
const rows = ref<FairLaunchExecutionQueueItem[]>([]);
const columns = [
  { title: 'Request', dataIndex: 'id', key: 'id' },
  { title: 'Action', dataIndex: 'action', key: 'action' },
  { title: 'Domain', dataIndex: 'execution_domain', key: 'execution_domain' },
  {
    title: 'Initiator',
    dataIndex: 'initiator_admin_id',
    key: 'initiator_admin_id',
  },
  {
    title: 'Approver',
    dataIndex: 'approver_admin_id',
    key: 'approver_admin_id',
  },
  {
    title: 'Executor',
    dataIndex: 'executor_admin_id',
    key: 'executor_admin_id',
  },
  { title: 'Status', dataIndex: 'status', key: 'status' },
  { title: 'Transitions', key: 'transitions' },
  {
    title: 'Expires',
    dataIndex: 'execution_expires_at',
    key: 'execution_expires_at',
  },
  { title: 'Availability', key: 'availability' },
];

async function copyAudit(value: string) {
  await navigator.clipboard.writeText(value);
  message.success('Audit value copied');
}

async function load() {
  loading.value = true;
  try {
    const result = await loadExactFairLaunchExecutionQueue({
      loadProject: getProjectContent,
      loadQueue: getFairLaunchExecutionQueue,
      projectId,
    });
    rows.value = result.items;
  } catch (error: any) {
    rows.value = [];
    message.error(error?.message || 'Execution queue unavailable');
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page
    title="Fair Launch · Third executor queue"
    description="Approval never dispatches. A distinct Admin with fresh MFA must execute each durable request."
  >
    <Alert
      type="warning"
      show-icon
      banner
      class="mb-3"
      message="initiator / approver / executor must be non-zero and pairwise distinct"
    />
    <Alert
      type="error"
      show-icon
      banner
      class="mb-3"
      message="Third execution is disabled while Fair Launch materialization is unavailable"
      description="fair_launch_materialization_unavailable：队列及 append-only transitions 仅供只读审计；不会收集 MFA assertion，也不会提交执行请求。"
    />
    <Card class="mb-3" title="Read-only execution audit">
      <Button @click="load">Refresh</Button>
    </Card>
    <Table
      :columns="columns"
      :data-source="rows"
      :loading="loading"
      row-key="id"
    >
      <template #bodyCell="{ column, record: rawRecord }">
        <template v-if="column.key === 'transitions'">
          <Space direction="vertical" size="small">
            <Card
              v-for="transition in (rawRecord as FairLaunchExecutionQueueItem)
                .transitions"
              :key="transition.sequence_no"
              size="small"
            >
              <div>
                <strong>#{{ transition.sequence_no }}</strong> ·
                {{ transition.created_at }}
              </div>
              <div>
                {{ transition.from_status }} → {{ transition.to_status }} ·
                admin {{ transition.actor_admin_id }}
              </div>
              <Tooltip :title="transition.evidence_digest">
                <code>evidence {{ transition.evidence_digest }}</code>
              </Tooltip>
              <Button
                type="link"
                size="small"
                @click="copyAudit(transition.evidence_digest)"
              >
                Copy evidence
              </Button>
              <Tooltip :title="transition.mfa_evidence_ref">
                <code>provider evidence {{ transition.mfa_evidence_ref }}</code>
              </Tooltip>
              <Button
                type="link"
                size="small"
                @click="copyAudit(transition.mfa_evidence_ref)"
              >
                Copy provider evidence
              </Button>
            </Card>
          </Space>
        </template>
        <template v-else-if="column.key === 'availability'">
          <Tag color="default">materialization unavailable · audit only</Tag>
        </template>
      </template>
    </Table>
  </Page>
</template>
