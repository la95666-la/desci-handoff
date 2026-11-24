import { requestClient } from '#/api/request';

export interface Approval {
  id: number;
  operation: string;
  target_kind: string;
  target_id: string;
  payload: string;
  reason: string;
  initiator_id: number;
  initiator_name: string;
  approver_id: null | number;
  approver_name: string;
  executor_id?: null | number;
  executor_name?: string;
  canonical_digest?: string;
  execution_domain?: string;
  step_up_mfa_verified?: boolean;
  status:
    | 'approved'
    | 'executed'
    | 'expired'
    | 'failed'
    | 'pending'
    | 'rejected';
  reject_reason: string;
  result: null | string;
  created_at: string;
  reviewed_at: null | string;
  executed_at: null | string;
  expires_at: null | string;
  durable_submission_id?: null | number;
}

export interface ListApprovalResp {
  items: Approval[];
  total: number;
  page: number;
  page_size: number;
}

export function listApprovals(params: {
  initiator_id?: number;
  operation?: string;
  page?: number;
  page_size?: number;
  status?: string;
}) {
  return requestClient.get<ListApprovalResp>('/v1/approvals', { params });
}

export function getApproval(id: number) {
  return requestClient.get<Approval>(`/v1/approvals/${id}`);
}

export function approveApproval(
  id: number,
  mfa?: { mfa_assertion_token: string },
) {
  return requestClient.post<Approval>(`/v1/approvals/${id}/approve`, mfa);
}

export function rejectApproval(id: number, reason: string) {
  return requestClient.post<Approval>(`/v1/approvals/${id}/reject`, { reason });
}

export function cancelApproval(id: number) {
  return requestClient.post<{ id: number }>(`/v1/approvals/${id}/cancel`);
}

export function listApprovalOps() {
  return requestClient.get<{ ops: string[] }>('/v1/approvals/ops');
}
