import type { UserInfo } from '@vben/types';

import { requestClient } from '#/api/request';

interface AdminMeRaw {
  admin_id: number;
  username: string;
  display_name: string;
  email: string;
  role_code: string;
}

/**
 * 获取当前管理员信息
 * 后端 GET /admin/v1/me → vite proxy 改写为 /api/v1/me
 */
export async function getUserInfoApi(): Promise<UserInfo> {
  const raw = await requestClient.get<AdminMeRaw>('/v1/me');
  return {
    userId: String(raw.admin_id),
    username: raw.username,
    realName: raw.display_name || raw.username,
    avatar: '',
    roles: [raw.role_code],
    homePath: '/',
    desc: raw.email,
  } as unknown as UserInfo;
}
