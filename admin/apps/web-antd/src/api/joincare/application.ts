import { requestClient } from '#/api/request';

export interface ApplyProject {
  id: number;
  name: string;
  city: string;
  email: string;
  telegram: string;
  wechat: string;
  major: string;
  identity: string;
  organization: string;
  project_name: string;
  project_desc: string;
  project_category: string;
  current_step: string;
  possessions: string;
  apply_reason: string;
  support_wanted: string;
  project_link: string;
  additional_info: string;
  status: number; // 0 pending / 1 approved / 2 rejected
  wallet_address: string;
  created_at: string;
}

export interface ApplyListResp {
  items: ApplyProject[];
  total: number;
  page: number;
  page_size: number;
}

export function listApplications(params: {
  keyword?: string;
  page?: number;
  page_size?: number;
  status?: number;
}) {
  return requestClient.get<ApplyListResp>('/v1/applications', { params });
}

export function getApplication(id: number) {
  return requestClient.get<ApplyProject>(`/v1/applications/${id}`);
}

/** Admin代提交：只复制已通过的申请资料，生成新的待审批申请；不冒充钱包签名。 */
export function cloneApprovedApplication(id: number) {
  return requestClient.post<{
    id: number;
    source_application_id: number;
    status: number;
  }>(`/v1/applications/${id}/clone`, {});
}

export function approveApplication(id: number) {
  return requestClient.post<{ id: number; status: number }>(
    `/v1/applications/${id}/approve`,
    {},
  );
}

export function rejectApplication(id: number, reason?: string) {
  return requestClient.post<{ id: number; reason: string; status: number }>(
    `/v1/applications/${id}/reject`,
    { reason: reason || '' },
  );
}
