import { get, put } from './client'
import type { ClientSettings, UpdateSettingsRequest } from '@/types'

/**
 * Get client settings
 */
export function getSettings(): Promise<ClientSettings> {
  return get<ClientSettings>('/settings')
}

/**
 * Update client settings
 */
export function updateSettings(data: UpdateSettingsRequest): Promise<ClientSettings> {
  return put<ClientSettings>('/settings', data)
}

/**
 * Get LAN access configuration
 */
export function getLanConfig(): Promise<{
  whitelist: string[]
  hostWhitelist: string[]
}> {
  return get('/settings/lan')
}

/**
 * Update LAN access configuration
 */
export function updateLanConfig(data: {
  whitelist?: string[]
  hostWhitelist?: string[]
}): Promise<void> {
  return put('/settings/lan', data)
}

/**
 * Get log settings
 */
export function getLogSettings(): Promise<{
  level: string
  maxSize: number
  maxDays: number
}> {
  return get('/settings/logs')
}

/**
 * Update log settings
 */
export function updateLogSettings(data: {
  level?: string
  maxSize?: number
  maxDays?: number
}): Promise<void> {
  return put('/settings/logs', data)
}