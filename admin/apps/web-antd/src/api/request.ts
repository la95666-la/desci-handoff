/**
 * 该文件可自行根据业务逻辑进行调整
 */
import type { RequestClientOptions } from '@vben/request';

import { useAppConfig } from '@vben/hooks';
import { preferences } from '@vben/preferences';
import {
  authenticateResponseInterceptor,
  defaultResponseInterceptor,
  errorMessageResponseInterceptor,
  RequestClient,
} from '@vben/request';
import { useAccessStore } from '@vben/stores';

import { message } from 'ant-design-vue';

import {
  containsForbiddenInternal,
  formatProjectError,
} from '#/api/joincare/errors';
import { idoErrorMessage, isIdoRequest } from '#/api/joincare/ido-errors';
import { isAmbiguousOnchainCreateError } from '#/api/joincare/onchain-create-error';
import { useAuthStore } from '#/store';

import { refreshTokenApi } from './core';

const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);

function createRequestClient(baseURL: string, options?: RequestClientOptions) {
  const client = new RequestClient({
    ...options,
    baseURL,
  });

  /**
   * 重新认证逻辑
   */
  async function doReAuthenticate() {
    console.warn('Access token or refresh token is invalid or expired. ');
    const accessStore = useAccessStore();
    const authStore = useAuthStore();
    accessStore.setAccessToken(null);
    if (
      preferences.app.loginExpiredMode === 'modal' &&
      accessStore.isAccessChecked
    ) {
      accessStore.setLoginExpired(true);
    } else {
      await authStore.logout();
    }
  }

  /**
   * 刷新token逻辑
   */
  async function doRefreshToken() {
    const accessStore = useAccessStore();
    const resp = await refreshTokenApi();
    const newToken = resp.data;
    accessStore.setAccessToken(newToken);
    return newToken;
  }

  function formatToken(token: null | string) {
    return token ? `Bearer ${token}` : null;
  }

  // 请求头处理
  client.addRequestInterceptor({
    fulfilled: async (config) => {
      const accessStore = useAccessStore();

      config.headers.Authorization = formatToken(accessStore.accessToken);
      config.headers['Accept-Language'] = preferences.app.locale;
      return config;
    },
  });

  // JoinCare 后端响应格式：{ status: boolean, data: any, code?: string, message?: string }
  // 复用 vben 的 defaultResponseInterceptor，仅把 codeField 改成 status / successCode=true
  // 框架 successCode 类型签名是 string|number|fn，这里业务后端用 boolean，故传一个 fn 绕过
  client.addResponseInterceptor(
    defaultResponseInterceptor({
      codeField: 'status',
      dataField: 'data',
      successCode: (code: any) => code === true,
    }),
  );

  // token过期的处理
  client.addResponseInterceptor(
    authenticateResponseInterceptor({
      client,
      doReAuthenticate,
      doRefreshToken,
      enableRefreshToken: preferences.app.enableRefreshToken,
      formatToken,
    }),
  );

  // 通用的错误处理,如果没有进入上面的错误处理逻辑，就会进入这里
  client.addResponseInterceptor(
    errorMessageResponseInterceptor((msg: string, error) => {
      // The list view shows an explicit do-not-resubmit warning for ambiguous
      // create responses; avoid a contradictory generic "server timeout" toast.
      if (
        /^\/v1\/projects\/[0-9]+\/create-onchain(?:\?|$)/.test(
          String(error?.config?.url || ''),
        ) &&
        isAmbiguousOnchainCreateError(error) &&
        !formatProjectError(error?.response?.data)
      )
        return;
      // 这里可以根据业务进行定制,你可以拿到 error 内的信息进行定制化处理，根据不同的 code 做不同的提示，而不是直接使用 message.error 提示 msg
      // 当前mock接口返回的错误字段是 error 或者 message
      const responseData = (error?.response?.data ?? {}) as Record<string, any>;

      // 稳定业务错误（如 PROJECT_TOKEN_RESERVE_INSUFFICIENT）优先走可读文案（含金额），
      // 且不回退到原始 message，避免暴露 durable / estimateGas / VM Exception 等底层细节
      const businessMessage = formatProjectError(responseData);
      if (businessMessage) {
        message.error(businessMessage);
        return;
      }

      if (isIdoRequest(error?.config?.url)) {
        message.error(idoErrorMessage(error));
        return;
      }

      const errorMessage = String(
        responseData?.error ?? responseData?.message ?? '',
      );
      // 底层实现特征（durable outbox / estimateGas / VM Exception）一律不上屏
      if (errorMessage && !containsForbiddenInternal(errorMessage)) {
        message.error(errorMessage);
        return;
      }
      // 如果没有错误信息或信息含内部细节，则根据状态码进行通用提示
      message.error(
        containsForbiddenInternal(errorMessage)
          ? '请求未通过校验，请联系管理员并提供错误码。'
          : errorMessage || msg,
      );
    }),
  );

  return client;
}

export const requestClient = createRequestClient(apiURL, {
  responseReturn: 'data',
});

export const baseRequestClient = new RequestClient({ baseURL: apiURL });
