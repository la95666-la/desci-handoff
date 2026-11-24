import { requestClient } from '#/api/request';

export interface NewsTranslation {
  id?: number;
  news_id?: number;
  language: string;
  title: string;
  summary: string;
  content: string;
  author: string;
  author_role: string;
}

export interface News {
  id: number;
  slug: string;
  category: string;
  cover_url: string;
  source_url: string;
  source_name: string;
  is_featured: number;
  is_important: number;
  alert_priority: number;
  alert_start_at?: null | string;
  alert_end_at?: null | string;
  alert_frequency: 'daily' | 'once' | 'session';
  alert_pages: string;
  alert_delay_seconds: number;
  alert_revision: number;
  sort: number;
  status: number;
  publish_at: string;
  created_at: string;
  updated_at: string;
}

export interface NewsListResp {
  items: News[];
  total: number;
  page: number;
  page_size: number;
}

export interface NewsDetail {
  news: News;
  translations: NewsTranslation[];
}

export interface NewsUpsert {
  slug: string;
  category?: string;
  cover_url?: string;
  source_url?: string;
  source_name?: string;
  is_featured?: number;
  is_important?: number;
  alert_priority?: number;
  alert_start_at?: null | string;
  alert_end_at?: null | string;
  alert_frequency?: 'daily' | 'once' | 'session';
  alert_pages?: string;
  alert_delay_seconds?: number;
  alert_revision?: number;
  sort?: number;
  status?: number;
  publish_at?: string;
  translations?: NewsTranslation[];
}

export function listNews(params: {
  category?: string;
  keyword?: string;
  page?: number;
  page_size?: number;
  status?: number;
}) {
  return requestClient.get<NewsListResp>('/v1/news', { params });
}

export function getNews(id: number) {
  return requestClient.get<NewsDetail>(`/v1/news/${id}`);
}

export function createNews(body: NewsUpsert) {
  return requestClient.post<{ id: number }>('/v1/news', body);
}

export function updateNews(id: number, body: NewsUpsert) {
  return requestClient.put<{ id: number }>(`/v1/news/${id}`, body);
}

export function deleteNews(id: number) {
  return requestClient.delete<{ id: number }>(`/v1/news/${id}`);
}
