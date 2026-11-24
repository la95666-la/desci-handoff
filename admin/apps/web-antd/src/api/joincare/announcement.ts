import { requestClient } from '#/api/request';

export interface Announcement {
  id: number;
  title: string;
  content: string;
  type: 'banner' | 'popup';
  language: 'en' | 'tw' | 'zh';
  priority: number;
  start_at?: null | string;
  end_at?: null | string;
  is_enable: number;
  created_at: string;
  updated_at: string;
}

export interface AnnouncementUpsert {
  title: string;
  content: string;
  type: 'banner' | 'popup';
  language: 'en' | 'tw' | 'zh';
  priority: number;
  start_at?: null | string;
  end_at?: null | string;
  is_enable: number;
}

export interface ListAnnouncementResp {
  items: Announcement[];
  total: number;
  page: number;
  page_size: number;
}

export function listAnnouncements(params: {
  language?: string;
  page?: number;
  page_size?: number;
  type?: string;
}) {
  return requestClient.get<ListAnnouncementResp>('/v1/announcements', {
    params,
  });
}

export function createAnnouncement(body: AnnouncementUpsert) {
  return requestClient.post<{ id: number }>('/v1/announcements', body);
}

export function updateAnnouncement(id: number, body: AnnouncementUpsert) {
  return requestClient.put<{ id: number }>(`/v1/announcements/${id}`, body);
}

export function deleteAnnouncement(id: number) {
  return requestClient.delete<{ approval_id: number; status: 'pending' }>(
    `/v1/announcements/${id}`,
  );
}
