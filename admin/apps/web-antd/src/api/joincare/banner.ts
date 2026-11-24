import { requestClient } from '#/api/request';

export interface Banner {
  id: number;
  title: string;
  subtitle: string;
  cta_label: string;
  cta_link: string;
  image_url: string;
  hero_animation: 'dandelion' | 'dna' | 'none';
  language: 'en' | 'tw' | 'zh';
  start_at?: null | string;
  end_at?: null | string;
  sort: number;
  is_enable: number;
  created_at: string;
  updated_at: string;
}

export interface BannerUpsert {
  title: string;
  subtitle: string;
  cta_label: string;
  cta_link: string;
  image_url: string;
  hero_animation: 'dandelion' | 'dna' | 'none';
  language: 'en' | 'tw' | 'zh';
  start_at?: null | string;
  end_at?: null | string;
  sort: number;
  is_enable: number;
}

export interface ListBannerResp {
  items: Banner[];
  total: number;
  page: number;
  page_size: number;
}

export function listBanners(params: {
  language?: string;
  page?: number;
  page_size?: number;
}) {
  return requestClient.get<ListBannerResp>('/v1/banners', { params });
}

export function createBanner(body: BannerUpsert) {
  return requestClient.post<{ id: number }>('/v1/banners', body);
}

export function updateBanner(id: number, body: BannerUpsert) {
  return requestClient.put<{ id: number }>(`/v1/banners/${id}`, body);
}

export function deleteBanner(id: number) {
  return requestClient.delete<{ id: number }>(`/v1/banners/${id}`);
}
