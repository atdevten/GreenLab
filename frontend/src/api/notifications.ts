import { client } from './client'
import type { Notification } from '../types'

export const notificationsApi = {
  list: (params?: { type?: string; workspace_id?: string }) => client.get<Notification[]>('/api/v1/notifications', { params }),
  get: (id: string) => client.get<Notification>(`/api/v1/notifications/${id}`),
  markRead: (id: string) => client.patch<Notification>(`/api/v1/notifications/${id}/read`),
  markAllRead: () => client.post('/api/v1/notifications/read-all'),
}
