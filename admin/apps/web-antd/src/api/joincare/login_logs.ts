import { requestClient } from '#/api/request';

export interface LoginLog {
  id: number;
  uid: number;
  ip: string;
  user_agent: string;
  login_type: string;
  logged_in_at: string;
  wallet_address: string;
}

export interface ListLoginLogResp {
  items: LoginLog[];
  total: number;
  page: number;
  page_size: number;
}

export function listAllLoginLogs(params: {
  from?: string;
  ip?: string;
  login_type?: string;
  page?: number;
  page_size?: number;
  to?: string;
  uid?: number;
  wallet?: string;
}) {
  return requestClient.get<ListLoginLogResp>('/v1/login-logs', { params });
}
