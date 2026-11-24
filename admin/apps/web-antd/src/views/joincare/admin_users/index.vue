<script setup lang="ts">
import type { AdminUser } from '#/api/joincare/admin_user';
import type { Role } from '#/api/joincare/role';

import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Input,
  InputPassword,
  message,
  Modal,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
} from 'ant-design-vue';

import {
  changeAdminUserPassword,
  createAdminUser,
  deleteAdminUser,
  listAdminUsers,
  updateAdminUser,
} from '#/api/joincare/admin_user';
import { listRoles } from '#/api/joincare/role';

const loading = ref(false);
const rows = ref<AdminUser[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const search = ref('');
const roles = ref<Role[]>([]);

async function load() {
  loading.value = true;
  try {
    const res = await listAdminUsers({
      search: search.value || undefined,
      page: page.value,
      page_size: pageSize.value,
    });
    rows.value = res.items || [];
    total.value = res.total || 0;
  } catch (error: any) {
    message.error(error?.message || 'load failed');
  } finally {
    loading.value = false;
  }
}
async function loadRoles() {
  try {
    const res = await listRoles();
    roles.value = res.items || [];
  } catch (error: any) {
    message.error(error?.message || 'roles load failed');
  }
}

const editVisible = ref(false);
const isCreating = ref(false);
const form = reactive({
  id: 0,
  username: '',
  password: '',
  email: '',
  display_name: '',
  role_code: '',
  roles: [] as string[],
  is_enable: 1 as 0 | 1,
});

function openCreate() {
  isCreating.value = true;
  Object.assign(form, {
    id: 0,
    username: '',
    password: '',
    email: '',
    display_name: '',
    role_code: 'operator',
    roles: ['operator'],
    is_enable: 1,
  });
  editVisible.value = true;
}
function openEdit(row: any) {
  isCreating.value = false;
  Object.assign(form, {
    id: row.id,
    username: row.username,
    password: '',
    email: row.email,
    display_name: row.display_name,
    role_code: row.role_code,
    roles: [...(row.roles || [])],
    is_enable: (row.is_enable ?? 1) as 0 | 1,
  });
  editVisible.value = true;
}
async function onSave() {
  try {
    if (form.id === 0) {
      if (!form.username || !form.password) {
        message.error('username / password required');
        return;
      }
      const rolesSet = [
        ...new Set([form.role_code, ...form.roles].filter(Boolean)),
      ];
      const submitted = await createAdminUser({
        username: form.username,
        password: form.password,
        email: form.email,
        display_name: form.display_name,
        role_code: form.role_code,
        roles: rolesSet,
        is_enable: form.is_enable,
      });
      message.success(
        `创建工单 #${submitted.approval_id} 已提交，等待另一位管理员审批`,
      );
    } else {
      const rolesSet = [
        ...new Set([form.role_code, ...form.roles].filter(Boolean)),
      ];
      const submitted = await updateAdminUser(form.id, {
        email: form.email,
        display_name: form.display_name,
        role_code: form.role_code,
        roles: rolesSet,
        is_enable: form.is_enable,
      });
      if ('approval_id' in submitted) {
        message.success(
          `账号变更工单 #${submitted.approval_id} 已提交，等待审批`,
        );
      } else {
        message.success('基本资料已更新');
      }
    }
    editVisible.value = false;
    load();
  } catch (error: any) {
    message.error(error?.message || 'save failed');
  }
}

const pwdVisible = ref(false);
const pwdTarget = ref<AdminUser | null>(null);
const newPwd = ref('');
function openChangePwd(row: any) {
  pwdTarget.value = row;
  newPwd.value = '';
  pwdVisible.value = true;
}
async function onChangePwd() {
  if (!pwdTarget.value) return;
  if (!newPwd.value || newPwd.value.length < 8) {
    message.error('密码至少 8 位');
    return;
  }
  try {
    const submitted = await changeAdminUserPassword(
      pwdTarget.value.id,
      newPwd.value,
    );
    message.success(`密码重置工单 #${submitted.approval_id} 已提交，等待审批`);
    pwdVisible.value = false;
  } catch (error: any) {
    message.error(error?.message || 'change failed');
  }
}

async function onDelete(row: any) {
  try {
    const submitted = await deleteAdminUser(row.id);
    message.success(`删除工单 #${submitted.approval_id} 已提交，等待审批`);
  } catch (error: any) {
    message.error(error?.message || 'delete failed');
  }
}

const columns = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '用户名', dataIndex: 'username', width: 140 },
  { title: '显示名', dataIndex: 'display_name', width: 140 },
  { title: '邮箱', dataIndex: 'email', ellipsis: true },
  { title: '主角色', dataIndex: 'role_code', width: 130 },
  { title: '全部角色', dataIndex: 'roles', width: 240 },
  { title: '启用', dataIndex: 'is_enable', width: 90 },
  { title: '最近登录', dataIndex: 'last_login_at', width: 160 },
  { title: '操作', dataIndex: '_action', width: 220, fixed: 'right' as const },
];

onMounted(() => {
  load();
  loadRoles();
});
</script>

<template>
  <Page
    title="管理员账号"
    description="每个 admin 可拥有多个角色（N:M）；主角色用于 UI 显示；权限校验取全部角色的并集"
  >
    <Alert
      type="info"
      show-icon
      banner
      class="mb-3"
      message="创建、角色、启停、重置密码、删除和恢复均走 4 眼审批；改密或撤权执行后旧 access/refresh token 立即失效。"
    />

    <Card>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="search"
          placeholder="按用户名 / 邮箱 / 显示名搜"
          style="width: 260px"
          allow-clear
          @press-enter="
            () => {
              page = 1;
              load();
            }
          "
        />
        <Button :loading="loading" @click="load">刷新</Button>
        <Button type="primary" ghost @click="openCreate">+ 新增账号</Button>
        <span class="ml-auto text-sm text-gray-500">共 {{ total }} 人</span>
      </div>

      <Table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        :pagination="{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p: number, ps: number) => {
            page = p;
            pageSize = ps;
            load();
          },
        }"
        :row-key="(r: any) => r.id"
        size="middle"
        bordered
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'role_code'">
            <Tag color="purple">{{ record.role_code }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'roles'">
            <Tag
              v-for="r in record.roles || []"
              :key="r"
              color="blue"
              style="margin-right: 4px"
            >
              {{ r }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'is_enable'">
            <Tag :color="record.is_enable === 1 ? 'green' : 'default'">
              {{ record.is_enable === 1 ? '✓ 启用' : '× 停用' }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === '_action'">
            <Button size="small" type="link" @click="openEdit(record)">
              编辑
            </Button>
            <Button size="small" type="link" @click="openChangePwd(record)">
              重置密码
            </Button>
            <Popconfirm title="确认软删除？" @confirm="onDelete(record)">
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </template>
        </template>
      </Table>
    </Card>

    <Modal
      v-model:open="editVisible"
      :title="isCreating ? '新增管理员' : `编辑管理员 #${form.id}`"
      width="640px"
      ok-text="保存"
      cancel-text="取消"
      @ok="onSave"
    >
      <div class="space-y-3 py-2">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">用户名 *</label>
            <Input
              v-model:value="form.username"
              :disabled="!isCreating"
              placeholder="仅创建时可填"
            />
          </div>
          <div v-if="isCreating">
            <label class="mb-1 block text-sm">密码 *（至少 8 位）</label>
            <InputPassword v-model:value="form.password" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm">邮箱</label>
            <Input v-model:value="form.email" />
          </div>
          <div>
            <label class="mb-1 block text-sm">显示名</label>
            <Input v-model:value="form.display_name" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm">主角色（登录后 UI 显示用）</label>
          <Select
            v-model:value="form.role_code"
            style="width: 100%"
            :options="
              roles.map((r) => ({
                value: r.code,
                label: `${r.code} — ${r.name}`,
              }))
            "
          />
        </div>
        <div>
          <label class="mb-1 block text-sm">全部角色（主角色自动加入）</label>
          <Select
            v-model:value="form.roles"
            mode="multiple"
            style="width: 100%"
            :options="
              roles.map((r) => ({
                value: r.code,
                label: `${r.code} — ${r.name}`,
              }))
            "
          />
        </div>
        <div class="flex items-center gap-2">
          <label class="text-sm">启用</label>
          <Switch
            :checked="form.is_enable === 1"
            @update:checked="
              (v: any) => {
                form.is_enable = v ? 1 : 0;
              }
            "
          />
        </div>
      </div>
    </Modal>

    <Modal
      v-model:open="pwdVisible"
      title="重置密码"
      width="420px"
      ok-text="重置"
      cancel-text="取消"
      @ok="onChangePwd"
    >
      <div v-if="pwdTarget" class="space-y-3 py-2">
        <div>
          目标账号：<b>{{ pwdTarget.username }}</b>
        </div>
        <div>
          <label class="mb-1 block text-sm">新密码（至少 8 位）</label>
          <InputPassword v-model:value="newPwd" />
        </div>
        <Alert
          type="warning"
          show-icon
          message="重置后请立即告知本人；重置动作会写审计日志"
        />
      </div>
    </Modal>
  </Page>
</template>
