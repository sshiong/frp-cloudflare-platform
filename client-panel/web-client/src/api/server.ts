import { get, post, put } from './client'
import type { ServerConfig } from '@/types'

/**
 * Get server configuration
 */
export function getServerConfig(): Promise<ServerConfig> {
  return get<ServerConfig>('/server/config')
}

/**
 * Update server configuration
 */
export function updateServerConfig(data: {
  addr?: string
  port?: number
  token?: string
  tlsEnabled?: boolean
  tlsVerify?: boolean
}): Promise<ServerConfig> {
  return put<ServerConfig>('/server/config', data)
}

/**
 * Test server connection
 */
export function testConnection(data?: {
  addr?: string
  port?: number
  token?: string
}): Promise<{
  success: boolean
  latency?: number
  error?: string
}> {
  return post('/server/test-connection', data)
}

/**
 * Switch to a different server
 */
export function switchServer(data: {
  addr: string
  port: number
  token?: string
}): Promise<void> {
  return post('/server/switch', data)
}

/**
 * Unbind device from server
 */
export function unbindDevice(): Promise<void> {
  return post('/server/unbind')
}

/**
 * Get server info
 */
export function getServerInfo(): Promise<{
  version: string
  buildDate: string
  os: string
  arch: string
}> {
  return get('/server/info')
}