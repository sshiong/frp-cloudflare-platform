// ============================================================
// User Types
// ============================================================
export interface User {
  id: number
  username: string
  role: UserRole
  status: UserStatus
  created_at: string
  updated_at: string
  last_login_at?: string
  note?: string
  max_mappings?: number
  max_bandwidth?: number
  max_devices?: number
}

export type UserRole = 'admin' | 'user'
export type UserStatus = 'active' | 'disabled' | 'locked'

export interface UserCreateRequest {
  username: string
  password: string
  role?: UserRole
  note?: string
  max_mappings?: number
  max_bandwidth?: number
  max_devices?: number
}

export interface UserUpdateRequest {
  password?: string
  role?: UserRole
  status?: UserStatus
  note?: string
  max_mappings?: number
  max_bandwidth?: number
  max_devices?: number
}

// ============================================================
// Auth Types
// ============================================================
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface UserInfo {
  id: number
  username: string
  role: UserRole
}

// ============================================================
// Device Types
// ============================================================
export interface Device {
  id: number
  user_id: number
  username?: string
  device_id: string
  device_name: string
  os?: string
  ip?: string
  frpc_version?: string
  status: DeviceStatus
  created_at: string
  last_seen_at?: string
}

export type DeviceStatus = 'online' | 'offline'

export interface DeviceRevokeRequest {
  reason?: string
}

// ============================================================
// Mapping Types
// ============================================================
export interface Mapping {
  id: number
  user_id: number
  username?: string
  name: string
  protocol: MappingProtocol
  local_ip: string
  local_port: number
  remote_port?: number
  custom_domain?: string
  proxy_protocol_version?: string
  status: MappingStatus
  bandwidth_limit?: number
  created_at: string
  updated_at: string
}

export type MappingProtocol = 'tcp' | 'udp' | 'http' | 'https'
export type MappingStatus = 'active' | 'disabled' | 'error'

export interface MappingCreateRequest {
  name: string
  protocol: MappingProtocol
  local_ip: string
  local_port: number
  remote_port?: number
  custom_domain?: string
  proxy_protocol_version?: string
  bandwidth_limit?: number
}

export interface MappingUpdateRequest {
  name?: string
  local_ip?: string
  local_port?: number
  remote_port?: number
  custom_domain?: string
  proxy_protocol_version?: string
  bandwidth_limit?: number
  status?: MappingStatus
}

// ============================================================
// Domain Types
// ============================================================
export interface Domain {
  id: number
  user_id: number
  username?: string
  domain: string
  status: DomainStatus
  certificate_id?: number
  dns_records?: DnsRecord[]
  created_at: string
  updated_at: string
}

export type DomainStatus = 'pending' | 'verified' | 'error'

export interface DnsRecord {
  type: string
  name: string
  value: string
  ttl?: number
}

export interface DomainCreateRequest {
  domain: string
}

// ============================================================
// Certificate Types
// ============================================================
export interface Certificate {
  id: number
  domain_id?: number
  domain?: string
  issuer: string
  status: CertificateStatus
  not_before: string
  not_after: string
  auto_renew: boolean
  created_at: string
  updated_at: string
}

export type CertificateStatus = 'valid' | 'expired' | 'expiring_soon' | 'pending' | 'error'

export interface CertificateIssueRequest {
  domain: string
}

// ============================================================
// Cloudflare Types
// ============================================================
export interface CloudflareToken {
  status: TokenStatus
  account_id?: string
  zone_count?: number
  zones?: CloudflareZone[]
  verified_at?: string
}

export type TokenStatus = 'not_configured' | 'configured' | 'verified' | 'error'

export interface CloudflareZone {
  id: string
  name: string
  status: string
  name_servers?: string[]
}

export interface CloudflareTokenRequest {
  api_token: string
}

// ============================================================
// Operation Types
// ============================================================
export interface Operation {
  id: number
  user_id: number
  username?: string
  type: OperationType
  target: string
  status: OperationStatus
  detail?: string
  created_at: string
  completed_at?: string
  cancelled_at?: string
}

export type OperationType =
  | 'mapping_create'
  | 'mapping_update'
  | 'mapping_delete'
  | 'domain_create'
  | 'domain_verify'
  | 'certificate_issue'
  | 'certificate_renew'
  | 'token_upload'
  | 'token_verify'
  | 'backup_create'
  | 'backup_restore'
  | 'user_create'
  | 'user_update'
  | 'user_delete'
  | 'device_revoke'
  | string

export type OperationStatus = 'pending' | 'running' | 'success' | 'failed' | 'cancelled'

// ============================================================
// Audit Types
// ============================================================
export interface AuditLog {
  id: number
  user_id: number
  username?: string
  action: string
  target_type: string
  target_id?: number
  detail?: string
  ip?: string
  created_at: string
}

// ============================================================
// System Types
// ============================================================
export interface SystemInfo {
  version: string
  uptime: string
  server_port: number
  dashboard_port: number
  database_type: string
  database_size: string
  total_users: number
  total_devices: number
  total_mappings: number
  total_domains: number
  online_clients: number
  frps_version: string
  frps_status: string
  router_status: string
}

export interface SystemSettings {
  server_name: string
  frps_port: number
  http_port: number
  https_port: number
  max_users: number
  max_mappings_per_user: number
  default_bandwidth_limit: number
  registration_enabled: boolean
  email_notifications: boolean
}

// ============================================================
// Backup Types
// ============================================================
export interface Backup {
  id: number
  filename: string
  size: number
  type: BackupType
  status: BackupStatus
  created_at: string
  restored_at?: string
}

export type BackupType = 'full' | 'config' | 'data'
export type BackupStatus = 'pending' | 'completed' | 'failed'

export interface BackupPreflightCheck {
  valid: boolean
  version: string
  size: number
  tables: string[]
  warnings: string[]
  errors: string[]
}

// ============================================================
// API Response Types
// ============================================================
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface PaginatedData<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface PaginationParams {
  page?: number
  page_size?: number
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

// ============================================================
// Dashboard Types
// ============================================================
export interface DashboardStats {
  total_users: number
  total_devices: number
  total_mappings: number
  total_domains: number
  online_clients: number
  offline_clients: number
  active_mappings: number
  valid_certificates: number
  recent_operations: Operation[]
}

// ============================================================
// Table Column Definition
// ============================================================
export interface TableColumn {
  prop: string
  label: string
  width?: number | string
  minWidth?: number | string
  fixed?: 'left' | 'right'
  sortable?: boolean | 'custom'
  formatter?: (row: any) => string
  slot?: string
}
