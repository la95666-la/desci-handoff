import { requestClient } from '#/api/request';

export interface UserNote {
  id: number;
  uid: number;
  admin_id: number;
  admin_username: string;
  note: string;
  created_at: string;
}

export interface UserLoginLog {
  id: number;
  uid: number;
  ip: string;
  user_agent: string;
  login_type: string;
  logged_in_at: string;
}

export interface UserPointFlow {
  id: number;
  uid: number;
  change_point: number;
  remain: number;
  type: string;
  desc: string;
  created_at: string;
  updated_at: string;
}

export interface UserSummary {
  user: any;
  note_count: number;
  login_count: number;
  task_completed: number;
  total_point: number;
  continuous_checkin_days: number;
  total_checkin_days: number;
  last_checkin_at: null | string;
}

export function getUserSummary(id: number) {
  return requestClient.get<UserSummary>(`/v1/users/${id}/summary`);
}

export function listUserNotes(id: number) {
  return requestClient.get<{ items: UserNote[] }>(`/v1/users/${id}/notes`);
}

export function createUserNote(id: number, note: string) {
  return requestClient.post<UserNote>(`/v1/users/${id}/notes`, { note });
}

export function deleteUserNote(noteId: number) {
  return requestClient.delete<{ id: number }>(`/v1/users/notes/${noteId}`);
}

export function listUserLoginLogs(id: number, page = 1, pageSize = 20) {
  return requestClient.get<{
    items: UserLoginLog[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/users/${id}/login-logs`, { params: { page, page_size: pageSize } });
}

export function listUserActivity(
  id: number,
  page = 1,
  pageSize = 20,
  type?: string,
) {
  return requestClient.get<{
    items: UserPointFlow[];
    page: number;
    page_size: number;
    total: number;
  }>(`/v1/users/${id}/activity`, {
    params: { page, page_size: pageSize, type },
  });
}
