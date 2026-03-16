import { client } from './client'
import type { Org, ApiKey } from '../types'

export const settingsApi = {
  getOrg: (id: string) => client.get<Org>(`/api/v1/orgs/${id}`),
  updateOrg: (id: string, data: Partial<Org>) => client.put<Org>(`/api/v1/orgs/${id}`, data),
  deleteOrg: (id: string) => client.delete(`/api/v1/orgs/${id}`),
  listApiKeys: () => client.get<ApiKey[]>('/api/v1/api-keys'),
  createApiKey: (data: { name: string; scopes: string[] }) => client.post<ApiKey & { key: string }>('/api/v1/api-keys', data),
  revokeApiKey: (id: string) => client.delete(`/api/v1/api-keys/${id}`),
}
