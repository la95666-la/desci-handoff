import { requestClient } from '#/api/request';

export interface JoinCareUser {
  id: number;
  wallet_address: string;
  twitter_id: string;
  twitter_handle: string;
  twitter_display_name: string;
  twitter_avatar: string;
  telegram_id: string;
  telegram_username: string;
  type: string;
  is_enable: number;
  point: number;
  rank: number;
  last_login_at: string;
  last_login_ip: string;
  created_at: string;
}

export interface UserListResp {
  items: JoinCareUser[];
  total: number;
  page: number;
  page_size: number;
}

export interface UserListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  sort?: 'id_asc' | 'id_desc' | 'point_asc' | 'point_desc';
}

/** GET /admin/v1/users */
export async function listJoinCareUsers(params: UserListParams) {
  return requestClient.get<UserListResp>('/v1/users', {
    params,
    // Large, indexed-in-application ranking reads should not inherit the
    // generic 10s timeout. The Backend still bounds each request to 25s.
    timeout: 30_000,
  });
}

/** GET /admin/v1/users/:id */
export async function getJoinCareUser(id: number) {
  return requestClient.get<JoinCareUser>(`/v1/users/${id}`);
}

export interface UnbindResp {
  id: number;
  unbound: boolean;
  message?: string;
  prev_twitter_id?: string;
  prev_handle?: string;
  prev_telegram_id?: string;
  prev_username?: string;
}

/**
 * POST /admin/v1/users/:id/unbind-twitter
 *
 * ctx 里的 prev_* 只用于 admin_audit_log 的 req_body 落库（审计员翻记录时能
 * 直接看到"解绑了哪个号"，不用再查 Lark 告警或响应 hash）；handler 不依赖
 * 这些值做业务判断，仍以 DB 里当前 user.twitter_* 为准。
 */
export async function unbindUserTwitter(
  id: number,
  ctx?: { prev_handle?: string; prev_twitter_id?: string; reason?: string },
) {
  return requestClient.post<UnbindResp>(
    `/v1/users/${id}/unbind-twitter`,
    ctx || {},
  );
}

/** POST /admin/v1/users/:id/unbind-telegram — ctx 同 unbindUserTwitter，仅用于审计上下文 */
export async function unbindUserTelegram(
  id: number,
  ctx?: { prev_telegram_id?: string; prev_username?: string; reason?: string },
) {
  return requestClient.post<UnbindResp>(
    `/v1/users/${id}/unbind-telegram`,
    ctx || {},
  );
}
