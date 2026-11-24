import { requestClient } from '#/api/request';

export interface InviteSettings {
  dailyCap: number;
  updatedAt: number;
}

export function getInviteSettings() {
  return requestClient.get<InviteSettings>('/v1/invite-settings');
}

export function updateInviteSettings(dailyCap: number) {
  return requestClient.put<InviteSettings>('/v1/invite-settings', {
    dailyCap,
  });
}
