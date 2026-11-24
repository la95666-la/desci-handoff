import { requestClient } from '#/api/request';

export interface TgSettings {
  chatId: number;
  chatLink: string;
  messagePoints: number;
  dailyCap: number;
  updatedAt: number;
}

export function getTgSettings() {
  return requestClient.get<TgSettings>('/v1/tg-settings');
}

export function updateTgSettings(payload: {
  chatId: number;
  chatLink: string;
  messagePoints: number;
  dailyCap: number;
}) {
  return requestClient.put<TgSettings>('/v1/tg-settings', payload);
}
