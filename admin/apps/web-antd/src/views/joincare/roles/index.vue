<script setup lang="ts">
import type { Permission, Role } from '#/api/joincare/role';

import { onMounted, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  message,
  Select,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  getRolePermissions,
  listRoles,
  setRolePermissions,
} from '#/api/joincare/role';

const roles = ref<Role[]>([]);
const selectedRole = ref<string>('');
const perms = ref<Permission[]>([]);
const loading = ref(false);
const saving = ref(false);

// 模块 × 动作 矩阵定义（与后端 middleware/rbac.go 常量对齐）
const MODULES = [
  { code: 'm1_identity', label: 'M1 身份 / 认证' },
  { code: 'm2_project', label: 'M2 项目管理' },
  { code: 'm5_task', label: 'M5 任务 / 积分' },
  { code: 'm7_user', label: 'M7 用户管理' },
  { code: 'm8_i18n', label: 'M8 i18n 文案' },
  { code: 'news', label: '(旧) 新闻' },
  { code: 'content', label: '内容运营（news+banner+announcement）' },
  { code: 'audit', label: '操作审计' },
  { code: 'rbac', label: '权限管理' },
];
const ACTIONS = ['read', 'write', 'approve', 'execute'];

// 编辑态：Set<"module:action">
const checked = ref<Set<string>>(new Set());

async function loadRoles() {
  try {
    const res = await listRoles();
    roles.value = res.items || [];
    if (!selectedRole.value && roles.value.length > 0) {
      selectedRole.value = roles.value[0]?.code || '';
    }
  } catch (error: any) {
    message.error(error?.message || 'roles load failed');
  }
}
async function loadPerms() {
  if (!selectedRole.value) return;
  loading.value = true;
  try {
    const res = await getRolePermissions(selectedRole.value);
    perms.value = res.items || [];
    checked.value = new Set(perms.value.map((p) => `${p.module}:${p.action}`));
  } catch (error: any) {
    message.error(error?.message || 'perms load failed');
  } finally {
    loading.value = false;
  }
}
watch(selectedRole, loadPerms);

function toggle(mod: string, act: string) {
  const key = `${mod}:${act}`;
  const s = new Set(checked.value);
  if (s.has(key)) s.delete(key);
  else s.add(key);
  checked.value = s;
}

async function onSave() {
  if (!selectedRole.value) return;
  if (selectedRole.value === 'super_admin') {
    message.warning(
      'super_admin 由代码短路，编辑此矩阵不生效（保存也允许作参考）',
    );
  }
  saving.value = true;
  try {
    const payload = [...checked.value].map((k) => {
      const parts = k.split(':');
      return { module: parts[0] || '', action: parts[1] || '', scope: 'all' };
    });
    await setRolePermissions(selectedRole.value, payload);
    message.success(`已保存 ${payload.length} 条权限`);
    loadPerms();
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  } finally {
    saving.value = false;
  }
}

const columns = [
  { title: '模块', dataIndex: 'module', width: 260, fixed: 'left' as const },
  ...ACTIONS.map((a) => ({
    title: a.toUpperCase(),
    dataIndex: a,
    width: 110,
    align: 'center' as const,
  })),
];

onMounted(loadRoles);
</script>

<template>
  <Page
    title="角色 & 权限矩阵"
    description="覆盖式编辑：勾选后保存 = 覆盖该角色的所有权限行。super_admin 由代码短路，编辑此角色矩阵不影响生效（保存仅作参考）"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="权限粒度 (module, action, scope)。当前 scope 默认 'all'。用户端 middleware 取该 admin 所有角色的权限并集（union）"
    />
    <Card>
      <div class="mb-3 flex items-center gap-3">
        <span>选择角色：</span>
        <Select
          v-model:value="selectedRole"
          style="width: 260px"
          :options="
            roles.map((r) => ({
              value: r.code,
              label: `${r.code} — ${r.name}`,
            }))
          "
        />
        <Tag v-if="selectedRole" color="purple">
          当前 {{ checked.size }} 条权限
        </Tag>
        <Button :loading="loading" @click="loadPerms">刷新</Button>
        <Button :loading="saving" type="primary" @click="onSave">保存</Button>
      </div>

      <Table
        :columns="columns"
        :data-source="MODULES"
        :pagination="false"
        :row-key="(r: any) => r.code"
        size="middle"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'module'">
            <div>
              <b>{{ record.label }}</b>
            </div>
            <div class="text-xs text-gray-500">
              code: <code>{{ record.code }}</code>
            </div>
          </template>
          <template v-else>
            <Checkbox
              :checked="checked.has(`${record.code}:${column.dataIndex}`)"
              @change="toggle(record.code, String(column.dataIndex || ''))"
            />
          </template>
        </template>
      </Table>
    </Card>
  </Page>
</template>
