import { client } from './client'
import type { Device, Channel, Field } from '../types'

interface ProvisionDeviceInput { workspace_id: string; name: string; description?: string }
interface ProvisionFieldInput  { name: string; label?: string; unit?: string; field_type?: string; position: number }

// Exactly one of channel_id (link existing) or channel (create new) must be set.
interface ProvisionRequest {
  device:      ProvisionDeviceInput
  channel_id?: string
  channel?:    { name: string; description?: string; visibility: string }
  fields?:     ProvisionFieldInput[]
}

interface ProvisionResponse {
  device:  Device
  channel: Channel
  fields:  Field[]
}

export const provisionApi = {
  provision: (data: ProvisionRequest) => client.post<ProvisionResponse>('/api/v1/devices/provision', data),
}
