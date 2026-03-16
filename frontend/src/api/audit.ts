import { client } from './client'
import type { AuditEvent } from '../types'

export const auditApi = {
  list: (params?: { resource_type?: string; search?: string }) => client.get<AuditEvent[]>('/api/v1/audit/events', { params }),
  get: (id: string) => client.get<AuditEvent>(`/api/v1/audit/events/${id}`),
  export: (params?: { resource_type?: string }) => client.get('/api/v1/audit/events', { params: { ...params, format: 'csv' }, responseType: 'blob' }),
}
