<script lang="ts" setup>
import { ref } from 'vue';

import {
  Form as AForm,
  FormItem as AFormItem,
  InputPassword as AInputPassword,
  Modal as AModal,
  message,
} from 'ant-design-vue';

import { requestClient } from '#/api/request';

const open = defineModel<boolean>('open', { default: false });

const formRef = ref();
const submitting = ref(false);
const form = ref({
  old_password: '',
  new_password: '',
  confirm_password: '',
});

async function handleOk() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  if (form.value.new_password !== form.value.confirm_password) {
    message.error('两次输入的新密码不一致');
    return;
  }
  submitting.value = true;
  try {
    await requestClient.post('/v1/self/password', {
      old_password: form.value.old_password,
      new_password: form.value.new_password,
    });
    message.success('密码已修改成功，请重新登录');
    open.value = false;
    form.value = { old_password: '', new_password: '', confirm_password: '' };
    // 强制下线让用户用新密码重登
    setTimeout(() => {
      window.location.href = '/auth/login';
    }, 1000);
  } catch (error: any) {
    message.error(error?.response?.data?.message || '修改失败');
  } finally {
    submitting.value = false;
  }
}

function handleCancel() {
  open.value = false;
  form.value = { old_password: '', new_password: '', confirm_password: '' };
}

// backend policy: >= 14 位 + 大小写 + 数字 + 符号 (4 类全都要)
// 用 Error 对象 reject（eslint prefer-promise-reject-errors 要求），antd form
// 会拿 err.message 展示给用户
function validatePasswordStrength(_rule: any, value: string) {
  if (!value) return Promise.reject(new Error('请输入新密码'));
  if (value.length < 14) return Promise.reject(new Error('密码至少 14 位'));
  if (!/[A-Z]/.test(value))
    return Promise.reject(new Error('必须包含大写字母'));
  if (!/[a-z]/.test(value))
    return Promise.reject(new Error('必须包含小写字母'));
  if (!/[0-9]/.test(value)) return Promise.reject(new Error('必须包含数字'));
  if (!/[!-/:-@[-`{-~]/.test(value))
    return Promise.reject(new Error('必须包含符号 (! @ # 等)'));
  return Promise.resolve();
}

// antd Rule 的 trigger 只接受 'blur'|'change' 字面量；type 也要显式声明才满足
// BaseRule 类型（string literal，不能 widen 成 string）
const rules = {
  old_password: [{ required: true, message: '请输入当前密码' }],
  new_password: [
    {
      required: true,
      type: 'string' as const,
      validator: validatePasswordStrength,
      trigger: 'change' as const,
    },
  ],
  confirm_password: [{ required: true, message: '请确认新密码' }],
};
</script>

<template>
  <AModal
    :open="open"
    title="修改密码"
    :confirm-loading="submitting"
    ok-text="确认修改"
    cancel-text="取消"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <AForm ref="formRef" :model="form" :rules="rules" layout="vertical">
      <div
        class="mb-3 rounded bg-yellow-50 p-3 text-xs text-yellow-900 dark:bg-yellow-900/20 dark:text-yellow-100"
      >
        <div class="font-semibold mb-1">密码规则</div>
        <ul class="list-disc pl-4 space-y-0.5">
          <li>至少 14 位</li>
          <li>包含大写字母 (A-Z)</li>
          <li>包含小写字母 (a-z)</li>
          <li>包含数字 (0-9)</li>
          <li>包含符号 (! @ # $ % 等)</li>
        </ul>
      </div>
      <AFormItem label="当前密码" name="old_password">
        <AInputPassword
          v-model:value="form.old_password"
          placeholder="输入当前密码"
          autocomplete="current-password"
        />
      </AFormItem>
      <AFormItem label="新密码" name="new_password">
        <AInputPassword
          v-model:value="form.new_password"
          placeholder="14+ 位, 含大小写数字符号"
          autocomplete="new-password"
        />
      </AFormItem>
      <AFormItem label="确认新密码" name="confirm_password">
        <AInputPassword
          v-model:value="form.confirm_password"
          placeholder="再次输入新密码"
          autocomplete="new-password"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>
