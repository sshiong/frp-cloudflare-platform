// User and Auth Types
export interface User {
  id: number
  username: string
  email?: string
  role: 'admin' | 'user'
  createdAt: string
  updatedAt: string
}

export interface LoginRequest {
  username: string
  password: string
  serverAddr?: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface AuthState {
  token: string | null
  user: User | null
  isAuthenticated: boolean
}

// Server Config Types
export interface ServerConfig {
  id: number
  addr: string
  port: number
  token?: string
  tlsEnabled: boolean
  tlsVerify: boolean
  connected: boolean
  lastConnected?: string
}

// FRPC Types
export interface FrpcStatus {
  running: boolean
  pid?: number
  version?: string
  uptime?: number
  startTime?: string
  proxyCount: number
  serverAddr: string
  serverPort: number
  connected: boolean
  lastError?: string
}

export interface FrpcConfig {
  serverAddr: string
  serverPort: number
  token?: string
  tlsEnable: boolean
  tlsVerify: boolean
  logLevel: 'trace' | 'debug' | 'info' | 'warn' | 'error'
  logMaxDays: number
  logFile: string
  poolCount: number
  tcpMux: boolean
  protocol: 'tcp' | 'kcp' | 'wss' | 'ws'
}

// Mapping Types
export type Protocol = 'tcp' | 'udp' | 'http' | 'https'

export type MappingStatus = 'active' | 'inactive' | 'error' | 'pending'

export interface Mapping {
  id: number
  name: string
  protocol: Protocol
  localIp: string
  localPort: number
  remotePort?: number
  customDomains?: string[]
  subdomain?: string
  status: MappingStatus
  enabled: boolean
  createdAt: string
  updatedAt: string
  trafficIn?: number
  trafficOut?: number
  currentConnections?: number
  lastActiveTime?: string
}

export interface CreateMappingRequest {
  name: string
  protocol: Protocol
  localIp: string
  localPort: number
  remotePort?: number
  customDomains?: string[]
  subdomain?: string
  useEncryption?: boolean
  useCompression?: boolean
  secretKey?: string
}

export interface UpdateMappingRequest extends Partial<CreateMappingRequest> {
  enabled?: boolean
}

// Domain Types
export type DomainStatus = 'active' | 'pending' | 'error' | 'expired'

export interface Domain {
  id: number
  domain: string
  status: DomainStatus
  sslStatus: 'none' | 'pending' | 'active' | 'error' | 'expired'
  sslExpiry?: string
  dnsRecords: DnsRecord[]
  createdAt: string
  updatedAt: string
}

export interface DnsRecord {
  type: 'A' | 'CNAME' | 'TXT'
  name: string
  value: string
  ttl: number
  status: 'active' | 'pending' | 'error'
}

// Log Types
export type LogLevel = 'trace' | 'debug' | 'info' | 'warn' | 'error'

export interface LogEntry {
  timestamp: string
  level: LogLevel
  message: string
  source?: string
}

export interface LogFilter {
  level?: LogLevel
  keyword?: string
  startTime?: string
  endTime?: string
  limit?: number
}

// Settings Types
export interface ClientSettings {
  id: number
  lanWhitelist: string[]
  hostWhitelist: string[]
  autoStart: boolean
  autoReconnect: boolean
  reconnectInterval: number
  maxRetries: number
  notificationEnabled: boolean
  logLevel: LogLevel
  logMaxSize: number
  logMaxDays: number
  theme: 'light' | 'dark' | 'system'
  language: 'zh-CN' | 'en-US'
}

export interface UpdateSettingsRequest extends Partial<ClientSettings> {}

// Health Check Types
export interface HealthCheckResult {
  success: boolean
  latency?: number
  error?: string
  timestamp: string
}

// API Response Types
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

// WebSocket Types
export interface WsMessage {
  type: string
  payload: any
  timestamp: string
}

export interface WsStatusUpdate {
  frpc: FrpcStatus
  mappings: Mapping[]
  server: ServerConfig
}

// Dashboard Stats
export interface DashboardStats {
  totalMappings: number
  activeMappings: number
  totalDomains: number
  activeDomains: number
  totalTrafficIn: number
  totalTrafficOut: number
  uptime: number
}

// Error Types
export interface ApiError {
  code: number
  message: string
  details?: Record<string, string[]>
}