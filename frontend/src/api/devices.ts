import { client } from './client'
import type { Device, CreateDeviceRequest, UpdateDeviceRequest, CreateDeviceResponse } from '../types'

export const devicesApi = {
  list: (params?: { workspace_id?: string; status?: string }) => client.get<Device[]>('/api/v1/devices', { params }),
  get: (id: string) => client.get<Device>(`/api/v1/devices/${id}`),
  create: (data: CreateDeviceRequest) => client.post<CreateDeviceResponse>('/api/v1/devices', data),
  update: (id: string, data: UpdateDeviceRequest) => client.put<Device>(`/api/v1/devices/${id}`, data),
  delete: (id: string) => client.delete(`/api/v1/devices/${id}`),
  rotateKey: (id: string) => client.post<{ api_key: string }>(`/api/v1/devices/${id}/rotate-key`),
  block: (id: string) => client.put<Device>(`/api/v1/devices/${id}`, { status: 'blocked' }),
  unblock: (id: string) => client.put<Device>(`/api/v1/devices/${id}`, { status: 'active' }),
}
