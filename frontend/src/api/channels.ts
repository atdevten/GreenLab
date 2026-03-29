import { client } from './client'
import type { Channel, CreateChannelRequest } from '../types'

export const channelsApi = {
  list: (params?: { device_id?: string; workspace_id?: string }) => client.get<Channel[]>('/api/v1/channels', { params }),
  get: (id: string) => client.get<Channel>(`/api/v1/channels/${id}`),
  create: (data: CreateChannelRequest) => client.post<Channel>('/api/v1/channels', data),
  update: (id: string, data: Partial<Channel>) => client.put<Channel>(`/api/v1/channels/${id}`, data),
  delete: (id: string) => client.delete(`/api/v1/channels/${id}`),
}
