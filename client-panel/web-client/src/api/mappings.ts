import { get, post, put, del } from './client'
import type { Mapping, CreateMappingRequest, UpdateMappingRequest, HealthCheckResult, PaginatedResponse } from '@/types'

/**
 * Get all mappings
 */
export function getMappings(params?: {
  page?: number
  pageSize?: number
  protocol?: string
  status?: string
  keyword?: string
}): Promise<PaginatedResponse<Mapping>> {
  return get<PaginatedResponse<Mapping>>('/mappings', { params })
}

/**
 * Get mapping by ID
 */
export function getMapping(id: number): Promise<Mapping> {
  return get<Mapping>(`/mappings/${id}`)
}

/**
 * Create new mapping
 */
export function createMapping(data: CreateMappingRequest): Promise<Mapping> {
  return post<Mapping>('/mappings', data)
}

/**
 * Update mapping
 */
export function updateMapping(id: number, data: UpdateMappingRequest): Promise<Mapping> {
  return put<Mapping>(`/mappings/${id}`, data)
}

/**
 * Delete mapping
 */
export function deleteMapping(id: number): Promise<void> {
  return del(`/mappings/${id}`)
}

/**
 * Toggle mapping enabled/disabled
 */
export function toggleMapping(id: number, enabled: boolean): Promise<Mapping> {
  return put<Mapping>(`/mappings/${id}`, { enabled })
}

/**
 * Check local service health
 */
export function checkLocalService(data: {
  localIp: string
  localPort: number
}): Promise<HealthCheckResult> {
  return post<HealthCheckResult>('/mappings/health-check', data)
}

/**
 * Get mapping traffic stats
 */
export function getMappingTraffic(id: number): Promise<{
  trafficIn: number
  trafficOut: number
  currentConnections: number
}> {
  return get(`/mappings/${id}/traffic`)
}

/**
 * Batch enable/disable mappings
 */
export function batchToggleMappings(ids: number[], enabled: boolean): Promise<void> {
  return post('/mappings/batch-toggle', { ids, enabled })
}

/**
 * Batch delete mappings
 */
export function batchDeleteMappings(ids: number[]): Promise<void> {
  return post('/mappings/batch-delete', { ids })
}