import { client } from './client'
import type { Device, Channel } from '../types'

interface ProvisionDeviceInput { workspace_id: string; name: string; description?: string }
interface ProvisionFieldInput  { name: string; label?: string; unit?: string; field_type?: string; position: number }

interface ProvisionRequest {
  device:     ProvisionDeviceInput
  channel_id: string
  fields?:    ProvisionFieldInput[]
}

interface ProvisionResponse {
  device:  Device
  channel: Channel
  fields:  unknown[]
}

export const provisionApi = {
  provision: (data: ProvisionRequest) => client.post<ProvisionResponse>('/api/v1/devices/provision', data),
}
