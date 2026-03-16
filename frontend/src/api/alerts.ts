import { client } from './client'
import type { AlertRule, CreateAlertRuleRequest } from '../types'

export const alertsApi = {
  list: (params?: { workspace_id?: string }) => client.get<AlertRule[]>('/api/v1/alert-rules', { params }),
  get: (id: string) => client.get<AlertRule>(`/api/v1/alert-rules/${id}`),
  create: (data: CreateAlertRuleRequest) => client.post<AlertRule>('/api/v1/alert-rules', data),
  update: (id: string, data: Partial<AlertRule>) => client.put<AlertRule>(`/api/v1/alert-rules/${id}`, data),
  delete: (id: string) => client.delete(`/api/v1/alert-rules/${id}`),
  toggle: (id: string, enabled: boolean) => client.put<AlertRule>(`/api/v1/alert-rules/${id}`, { enabled }),
}
