<script setup lang="ts">
import type { SelectValue } from 'ant-design-vue/es/select';

import type { ProjectListRow } from '#/api/joincare/project_content';

import { onMounted, ref } from 'vue';

import { Alert, Button, Card, Select, Spin } from 'ant-design-vue';

import { listProjects } from '#/api/joincare/project_content';
import { useFundLedgerStore } from '#/store/fund-ledger';

import {
  clearLegacyV2ProjectSelection,
  legacyV2ProjectOptions,
  reconcileLegacyV2ProjectSelection,
} from '../project-picker-state';

const store = useFundLedgerStore();

const options = ref<ProjectListRow[]>([]);
const loadingOptions = ref(false);

onMounted(async () => {
  loadingOptions.value = true;
  try {
    const res = await listProjects({ page: 1, size: 200 });
    options.value = legacyV2ProjectOptions(res.items || []);
    reconcileLegacyV2ProjectSelection(store, options.value);
  } catch {
    options.value = [];
    clearLegacyV2ProjectSelection(store);
  } finally {
    loadingOptions.value = false;
  }
});

function onSelect(value: SelectValue) {
  if (
    typeof value !== 'string' ||
    !options.value.some((project) => project.project_id === value)
  ) {
    clearLegacyV2ProjectSelection(store);
    return;
  }
  store.projectId = value;
  store.invalidateLoadedProject();
}
</script>

<template>
  <Alert
    v-if="store.permissionReady && !store.canRead"
    type="error"
    show-icon
    banner
    message="当前管理员没有 m2_project:read 权限，未请求或展示资金账本。"
  />

  <Card v-if="store.canRead" style="margin-bottom: 24px">
    <div class="flex flex-wrap items-center gap-2">
      <Spin v-if="loadingOptions" size="small" />
      <Select
        v-else
        :value="store.projectId ?? undefined"
        show-search
        :filter-option="
          (input: string, option: any) =>
            String(option.label).toLowerCase().includes(input.toLowerCase())
        "
        :options="
          options.map((p) => ({
            value: p.project_id,
            label: `#${p.project_id} · ${p.name_cn || p.name_en} · ${p.status}`,
          }))
        "
        placeholder="选择项目"
        style="min-width: 340px"
        @change="onSelect"
      />
      <Button
        type="primary"
        :disabled="store.projectId === undefined"
        :loading="store.loading"
        @click="store.searchProject"
      >
        加载项目账本
      </Button>
      <slot></slot>
    </div>
  </Card>
</template>
