import { requestClient } from '#/api/request';

export interface PointFlow {
  id: number;
  uid: number;
  change_point: number;
  remain: number;
  type: string; // CHECK_IN / TASK / INVITE / ADJUST
  desc: string;
  created_at: string;
  wallet_address: string;
  twitter_handle: string;
}

export interface ListPointFlowResp {
  items: PointFlow[];
  total: number;
  page: number;
  page_size: number;
}

export function listPointFlows(params: {
  from?: string;
  max_change?: number;
  min_change?: number;
  page?: number;
  page_size?: number;
  to?: string;
  type?: string;
  uid?: number;
  wallet?: string;
}) {
  return requestClient.get<ListPointFlowResp>('/v1/point-flows', { params });
}

// 手工加/扣分：|delta|≥1000 会被后端转发到 4 眼审批，返 { approval_id, status: pending }
export function manualAdjustPoint(uid: number, delta: number, reason: string) {
  return requestClient.post<{
    approval_id?: number;
    delta?: number;
    id?: number;
    message?: string;
    status?: string;
  }>(`/v1/users/${uid}/adjust-point`, { delta, reason });
}
