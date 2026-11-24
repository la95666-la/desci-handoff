import { requestClient } from '#/api/request';

export interface AdminUser {
  id: number;
  username: string;
  email: string;
  display_name: string;
  role_code: string;
  roles: string[];
  is_enable: number;
  last_login_at: null | string;
  last_login_ip: string;
  created_at: string;
}

export interface ListAdminUserResp {
  items: AdminUser[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateAdminUserReq {
  username: string;
  password: string;
  email?: string;
  display_name?: string;
  role_code: string;
  roles?: string[];
  is_enable?: number;
}

export interface UpdateAdminUserReq {
  email?: string;
  display_name?: string;
  role_code?: string;
  roles?: string[];
  is_enable?: number;
}

export interface ApprovalSubmission {
  approval_id: number;
  status: 'pending';
}

export function listAdminUsers(params: {
  page?: number;
  page_size?: number;
  search?: string;
}) {
  return requestClient.get<ListAdminUserResp>('/v1/admin-users', { params });
}

export function createAdminUser(body: CreateAdminUserReq) {
  return requestClient.post<ApprovalSubmission>('/v1/admin-users', body);
}

export function updateAdminUser(id: number, body: UpdateAdminUserReq) {
  return requestClient.put<
    ApprovalSubmission | { changed: number; id: number }
  >(`/v1/admin-users/${id}`, body);
}

export function changeAdminUserPassword(id: number, newPassword: string) {
  return requestClient.post<ApprovalSubmission>(
    `/v1/admin-users/${id}/password`,
    {
      new_password: newPassword,
    },
  );
}

export function setAdminUserRoles(id: number, roles: string[]) {
  return requestClient.post<{ id: number; roles: string[] }>(
    `/v1/admin-users/${id}/roles`,
    { roles },
  );
}

export function deleteAdminUser(id: number) {
  return requestClient.delete<ApprovalSubmission>(`/v1/admin-users/${id}`);
}
