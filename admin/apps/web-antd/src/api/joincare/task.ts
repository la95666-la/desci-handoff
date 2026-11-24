import { requestClient } from '#/api/request';

export interface JCTask {
  id: number;
  platform: string;
  task_key: string;
  target_slot: number;
  title: string;
  description: string;
  url: string;
  cta_label: string;
  verify_kind: string;
  verify_meta: string;
  amount_point: number;
  sort: number;
  is_enable: number;
  version: number;
  target_ref: string;
  round_started_at: string;
  created_at: string;
  updated_at: string;
}

export interface TaskListResp {
  items: JCTask[];
  total: number;
  page: number;
  page_size: number;
}

export interface TaskUpsert {
  platform: string;
  task_key: string;
  target_slot?: number;
  title: string;
  description: string;
  url: string;
  cta_label: string;
  verify_kind: string;
  verify_meta?: string;
  amount_point: number;
  sort: number;
  is_enable: number;
}

export interface TaskUpdateResult {
  id: number;
  round_changed: boolean;
  old_version: number;
  new_version: number;
  target_ref: string;
}

export function listTasks(params: {
  is_enable?: number;
  keyword?: string;
  page?: number;
  page_size?: number;
  platform?: string;
}) {
  return requestClient.get<TaskListResp>('/v1/tasks', { params });
}

export function createTask(body: TaskUpsert) {
  return requestClient.post<{ id: number }>('/v1/tasks', body);
}

export function updateTask(id: number, body: TaskUpsert) {
  return requestClient.put<TaskUpdateResult>(`/v1/tasks/${id}`, body);
}

export function deleteTask(id: number) {
  return requestClient.delete<{ id: number }>(`/v1/tasks/${id}`);
}

/** 同步 Twitter 用户的置顶推文 → 更新 like/retweet 任务的 URL */
export function syncPinnedTweet(username: string) {
  return requestClient.post<{
    advanced: number;
    pinned_tweet_id: string;
    updated: number;
    url: string;
    username: string;
    versions: Array<{
      new_version: number;
      old_version: number;
      task_id: number;
      task_key: string;
    }>;
  }>(`/v1/tasks/sync-pinned-tweet`, { username });
}
