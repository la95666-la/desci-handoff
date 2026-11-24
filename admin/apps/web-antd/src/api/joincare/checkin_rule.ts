import { requestClient } from '#/api/request';

export interface CheckInRule {
  id: number;
  day_threshold: number;
  bonus_point: number;
  is_enable: number;
  sort: number;
  created_at: string;
  updated_at: string;
}

export interface CheckInRuleUpsert {
  day_threshold: number;
  bonus_point: number;
  is_enable: number;
  sort: number;
}

export function listCheckInRules() {
  return requestClient.get<{ items: CheckInRule[] }>('/v1/checkin-rules');
}

export function createCheckInRule(body: CheckInRuleUpsert) {
  return requestClient.post<{ id: number }>('/v1/checkin-rules', body);
}

export function updateCheckInRule(id: number, body: CheckInRuleUpsert) {
  return requestClient.put<{ id: number }>(`/v1/checkin-rules/${id}`, body);
}

export function deleteCheckInRule(id: number) {
  return requestClient.delete<{ id: number }>(`/v1/checkin-rules/${id}`);
}
