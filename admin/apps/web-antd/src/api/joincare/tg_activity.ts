import { requestClient } from '#/api/request';

export interface TgDailyStat {
  day: string;
  chat_id: number;
  chat_title: string;
  active_users: number;
  total_messages: number;
  binded_users: number;
}

export interface TgActivityRow {
  id: number;
  chat_id: number;
  chat_title: string;
  tg_user_id: number;
  tg_username: string;
  uid: null | number;
  day: string;
  message_count: number;
  first_msg_at: string;
  last_msg_at: string;
}

export function tgDailyStats(params: {
  chat_id?: number;
  from?: string;
  to?: string;
}) {
  return requestClient.get<{ from: string; items: TgDailyStat[]; to: string }>(
    '/v1/tg-activity/daily',
    { params },
  );
}

export function tgActivityDetail(params: {
  bound_only?: '0' | '1';
  chat_id?: number;
  day: string;
  page?: number;
  page_size?: number;
}) {
  return requestClient.get<{
    items: TgActivityRow[];
    page: number;
    page_size: number;
    total: number;
  }>('/v1/tg-activity/detail', { params });
}

export interface TgChat {
  chat_id: number;
  chat_title: string;
  days: number;
}

export interface TgUserSummary {
  uid: null | number;
  tg_user_id: number;
  tg_username: string;
  wallet_address: string;
  twitter_handle: string;
  total_messages: number;
  active_days: number;
  first_active_day: string;
  last_active_day: string;
}

export function tgChats() {
  return requestClient.get<{ items: TgChat[] }>('/v1/tg-activity/chats');
}

export function tgUserSummary(params: {
  bound_only?: '0' | '1';
  chat_id: number;
  from?: string;
  page?: number;
  page_size?: number;
  sort?: 'days_desc' | 'msgs_desc' | 'recent_desc';
  to?: string;
}) {
  return requestClient.get<{
    chat_id: string;
    from: string;
    items: TgUserSummary[];
    page: number;
    page_size: number;
    to: string;
    total: number;
  }>('/v1/tg-activity/user-summary', { params });
}
