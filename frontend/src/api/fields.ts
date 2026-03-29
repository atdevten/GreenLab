import { client } from './client'
import type { CreateFieldRequest } from '../types'

interface FieldResponse {
  id: string
  channel_id: string
  name: string
  label: string
  unit: string
  field_type: string
  position: number
  description: string
  created_at: string
  updated_at: string
}

export const fieldsApi = {
  list: (channelId: string) => client.get<FieldResponse[]>('/api/v1/fields', { params: { channel_id: channelId } }),
  create: (data: CreateFieldRequest) => client.post<FieldResponse>('/api/v1/fields', data),
  update: (id: string, data: Partial<CreateFieldRequest>) => client.put<FieldResponse>(`/api/v1/fields/${id}`, data),
  delete: (id: string) => client.delete(`/api/v1/fields/${id}`),
}
