import { requestClient } from '#/api/request';

// === 双语聚合视图：每个 key 一行，含 en + tw 两个 value ===

export interface BilingualCopy {
  key: string;
  en: string;
  tw: string;
  updated_at: string;
}

export interface BilingualListResp {
  items: BilingualCopy[];
  total: number;
  page: number;
  page_size: number;
}

export interface BilingualListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  prefix?: string;
}

export interface PrefixItem {
  prefix: string;
  count: number;
}

export function listCopywritingPrefixes() {
  return requestClient.get<{ items: PrefixItem[] }>('/v1/copywriting/prefixes');
}

export interface BilingualUpsert {
  key: string;
  en: string;
  tw: string;
}

export function listCopywriting(params: BilingualListParams) {
  return requestClient.get<BilingualListResp>('/v1/copywriting/bilingual', {
    params,
  });
}

export function upsertCopywriting(body: BilingualUpsert) {
  return requestClient.post<{ key: string }>('/v1/copywriting/bilingual', body);
}

export function deleteCopywriting(key: string) {
  return requestClient.delete<{ key: string }>('/v1/copywriting/by-key', {
    params: { key },
  });
}
