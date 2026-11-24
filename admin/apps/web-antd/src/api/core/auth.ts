import { baseRequestClient, requestClient } from '#/api/request';

export namespace AuthApi {
  export interface LoginParams {
    password?: string;
    username?: string;
  }

  /** JoinCare /admin/v1/login 实际返回 */
  export interface LoginRawResult {
    token: string;
    expire_at: number;
    admin_id: number;
    username: string;
    display_name: string;
    role_code: string;
  }

  /** Vben 框架内部使用的标准化结构 */
  export interface LoginResult {
    accessToken: string;
  }

  export interface RefreshTokenResult {
    data: string;
    status: number;
  }
}

/**
 * 登录
 * 后端 POST /admin/v1/login → 透传过 vite proxy 改写为 /api/v1/login
 */
export async function loginApi(
  data: AuthApi.LoginParams,
): Promise<AuthApi.LoginResult> {
  const raw = await requestClient.post<AuthApi.LoginRawResult>(
    '/v1/login',
    data,
  );
  // 后端字段 token → 框架字段 accessToken；其他原始信息暂存到 sessionStorage 供 user store 使用
  if (raw && (raw as any).token) {
    sessionStorage.setItem('joincare_admin_profile', JSON.stringify(raw));
  }
  return { accessToken: (raw as any)?.token ?? '' };
}

/**
 * 刷新 accessToken — Phase 1 未实现 refresh，留位
 */
export async function refreshTokenApi() {
  return baseRequestClient.post<AuthApi.RefreshTokenResult>('/v1/refresh', {
    withCredentials: true,
  });
}

/**
 * 退出登录 — 后端无状态 JWT，前端清 token 即可，调一下后端记录可选
 */
export async function logoutApi() {
  return {};
}

/**
 * 获取用户权限码 — Phase 1 根据 role_code 返回硬编码权限位
 *   super_admin → ['*']  (全部)
 *   operator    → ['m2:rw', 'm5:rw', 'm8:rw', 'news:rw']
 *   其余        → ['*:r']
 */
export async function getAccessCodesApi(): Promise<string[]> {
  const cached = sessionStorage.getItem('joincare_admin_profile');
  if (!cached) return ['*:r'];
  try {
    const profile = JSON.parse(cached) as AuthApi.LoginRawResult;
    switch (profile.role_code) {
      case 'operator': {
        return ['m2:rw', 'm5:rw', 'm8:rw', 'news:rw'];
      }
      case 'super_admin': {
        return ['*'];
      }
      default: {
        return ['*:r'];
      }
    }
  } catch {
    return ['*:r'];
  }
}
