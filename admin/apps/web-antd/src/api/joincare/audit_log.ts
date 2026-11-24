import { requestClient } from '#/api/request';

export interface AuditLog {
  id: number;
  admin_id: number;
  username: string;
  role_codes: string;
  ip: string;
  user_agent: string;
  method: string;
  path: string;
  route: string;
  target_id: string;
  req_body: string;
  resp_status: number;
  resp_hash: string;
  prev_hash: string;
  hash: string;
  duration_ms: number;
  note: string;
  created_at: string;
}

export interface ListAuditLogResp {
  items: AuditLog[];
  total: number;
  page: number;
  page_size: number;
}

export function listAuditLogs(params: {
  admin_id?: number;
  from?: string;
  method?: string;
  page?: number;
  page_size?: number;
  path_like?: string;
  to?: string;
}) {
  return requestClient.get<ListAuditLogResp>('/v1/audit-logs', { params });
}

export function verifyAuditChain() {
  return requestClient.get<{
    broken_at_id?: number;
    message: string;
    ok: boolean;
  }>('/v1/audit-logs/verify-chain');
}
