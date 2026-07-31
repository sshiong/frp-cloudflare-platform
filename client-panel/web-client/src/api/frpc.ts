import { get, post } from './client'
import type { FrpcStatus, FrpcConfig, LogEntry, LogFilter } from '@/types'

/**
 * Get FRPC status
 */
export function getFrpcStatus(): Promise<FrpcStatus> {
  return get<FrpcStatus>('/frpc/status')
}

/**
 * Start FRPC
 */
export function startFrpc(): Promise<void> {
  return post('/frpc/start')
}

/**
 * Stop FRPC
 */
export function stopFrpc(): Promise<void> {
  return post('/frpc/stop')
}

/**
 * Restart FRPC
 */
export function restartFrpc(): Promise<void> {
  return post('/frpc/restart')
}

/**
 * Get FRPC configuration
 */
export function getFrpcConfig(): Promise<FrpcConfig> {
  return get<FrpcConfig>('/frpc/config')
}

/**
 * Update FRPC configuration
 */
export function updateFrpcConfig(config: Partial<FrpcConfig>): Promise<FrpcConfig> {
  return post('/frpc/config', config)
}

/**
 * Get FRPC logs
 */
export function getFrpcLogs(filter?: LogFilter): Promise<LogEntry[]> {
  return get<LogEntry[]>('/frpc/logs', { params: filter })
}

/**
 * Get FRPC version
 */
export function getFrpcVersion(): Promise<{ version: string }> {
  return get('/frpc/version')
}