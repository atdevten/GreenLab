import { client } from './client'
import type { Workspace, WorkspaceMember, WorkspaceApiKey } from '../types'

export const workspacesApi = {
  list: (orgId: string) => client.get<Workspace[]>(`/api/v1/orgs/${orgId}/workspaces`),
  create: (data: { org_id: string; name: string; slug: string; description?: string }) => client.post<Workspace>('/api/v1/workspaces', data),
  update: (id: string, data: { name?: string; slug?: string; description?: string }) => client.put<Workspace>(`/api/v1/workspaces/${id}`, data),
  delete: (id: string) => client.delete(`/api/v1/workspaces/${id}`),
  listMembers: (id: string) => client.get<WorkspaceMember[]>(`/api/v1/workspaces/${id}/members`),
  addMember: (id: string, data: { user_id: string; role: string }) => client.post<WorkspaceMember>(`/api/v1/workspaces/${id}/members`, data),
  updateMember: (id: string, userId: string, data: { role: string }) => client.put(`/api/v1/workspaces/${id}/members/${userId}`, data),
  removeMember: (id: string, userId: string) => client.delete(`/api/v1/workspaces/${id}/members/${userId}`),
  listDevices: (id: string) => client.get(`/api/v1/workspaces/${id}/devices`),
  listApiKeys: (workspaceId: string) => client.get<WorkspaceApiKey[]>(`/api/v1/workspaces/${workspaceId}/api-keys`),
  createApiKey: (workspaceId: string, data: { name: string; scope: 'read' | 'write' }) => client.post<WorkspaceApiKey & { key: string }>(`/api/v1/workspaces/${workspaceId}/api-keys`, data),
  revokeApiKey: (workspaceId: string, keyId: string) => client.delete(`/api/v1/workspaces/${workspaceId}/api-keys/${keyId}`),
}
