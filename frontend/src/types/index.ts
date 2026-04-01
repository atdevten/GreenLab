export interface LoginRequest { email: string; password: string }
export interface LoginResponse { access_token: string; refresh_token: string; token_type: string; expires_at: string }
export interface SignupRequest { email: string; password: string }
export interface SignupResponse { access_token: string; refresh_token: string; token_type: string; expires_at: string; user: User }
export interface User {
  id: string
  tenant_id: string
  email: string
  first_name: string
  last_name: string
  roles: string[]
  status: string
  email_verified: boolean
  created_at: string
  updated_at: string
}

export interface Org { id: string; name: string; slug: string; plan?: string; owner_user_id?: string; logo_url?: string; website?: string; created_at?: string }

export interface Workspace { id: string; org_id?: string; name: string; slug: string; description?: string; plan?: string; device_count: number; channel_count: number; member_count: number; created_at: string }
export interface WorkspaceMember { id: string; workspace_id?: string; user_id: string; name: string; email: string; role: string; joined_at: string }

export interface Device { id: string; name: string; workspace_id: string; description?: string; tags?: string[]; icon?: string; status: 'active' | 'inactive' | 'blocked'; api_key: string; channel_count: number; reads_24h: number; last_seen?: string; lat?: number; lng?: number }
export interface CreateDeviceRequest { name: string; workspace_id: string; description?: string; tags?: string[]; icon?: string }
export interface UpdateDeviceRequest { name?: string; description?: string; tags?: string[]; icon?: string; status?: string }

export interface Channel { id: string; device_id: string; name: string; visibility: 'public' | 'private'; tags?: string[]; fields: FieldDef[]; last_reading?: string; reads_24h: number; updated_at: string }
export interface FieldDef { id?: string; key: string; name: string; unit?: string; type: 'float' | 'integer' | 'string' | 'boolean'; color?: string; enabled?: boolean }
export interface CreateChannelRequest { workspace_id: string; device_id?: string; name: string; description?: string; visibility: 'public' | 'private' }
export interface CreateFieldRequest { channel_id: string; name: string; label?: string; unit?: string; field_type: 'float' | 'integer' | 'string' | 'boolean'; position: number; description?: string }

export interface Field { id: string; channel_id: string; key: string; name: string; unit?: string; type: 'float' | 'integer' | 'string' | 'boolean' }

export type Severity = 'critical' | 'warning' | 'info'
export type Operator = '>' | '>=' | '<' | '<=' | '==' | '!='
export interface AlertRule { id: string; name: string; device_id: string; field_id: string; operator: Operator; threshold: number; unit?: string; severity: Severity; enabled: boolean; notification_channels: string[]; last_triggered?: string }
export interface CreateAlertRuleRequest { workspace_id: string; name: string; device_id: string; field_id: string; operator: Operator; threshold: number; unit?: string; severity: Severity; enabled?: boolean; notification_channels?: string[] }

export type NotifType = 'critical' | 'warning' | 'info' | 'resolved'
export interface Notification { id: string; type: NotifType; title: string; message: string; read: boolean; created_at: string; rule_id?: string }

export interface QueryParams { channel_id: string; field?: string; start?: string; end?: string; aggregate?: string; limit?: number; window?: string }
export type QueryResult = { timestamp: string; value: number; field: string }[]
export interface QueryResponse { channel_id: string; field_name: string; data_points: QueryResult; count: number; start: string; end: string }

export interface DashboardStats { active_devices: number; readings_24h: number; active_alerts: number; total_channels: number }

export interface AuditEvent { id: string; user_id: string; user_name: string; action: string; resource_type: string; resource_id: string; target: string; ip: string; created_at: string }

export interface ApiKey { id: string; tenant_id?: string; user_id?: string; name: string; key_prefix: string; scopes: string[]; created_at: string; last_used?: string }
