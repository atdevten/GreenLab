import { client } from './client'
import type { QueryParams, QueryResponse, DashboardStats } from '../types'

export const queryApi = {
  query: (params: QueryParams) => client.get<QueryResponse>('/api/v1/query', { params }),
  latest: (params: { channel_id: string; field_key?: string }) => client.get('/api/v1/query/latest', { params }),
  stats: () => client.get<DashboardStats>('/api/v1/stats'),
  export: (params: QueryParams) => client.get('/api/v1/query', { params: { ...params, format: 'csv' }, responseType: 'blob' }),
}
