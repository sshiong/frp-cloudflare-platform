import { apiGet, apiPost, apiDelete } from './client'
import type { CloudflareToken, CloudflareZone, CloudflareTokenRequest, ApiResponse } from '@/types'

export const cloudflareApi = {
  /**
   * Get token status
   */
  getTokenStatus(): Promise<ApiResponse<CloudflareToken>> {
    return apiGet('/cloudflare/token')
  },

  /**
   * Upload/replace token
   */
  uploadToken(data: CloudflareTokenRequest): Promise<ApiResponse<void>> {
    return apiPost('/cloudflare/token', data)
  },

  /**
   * Verify token
   */
  verifyToken(): Promise<ApiResponse<{ valid: boolean; account_id?: string; zone_count?: number }>> {
    return apiPost('/cloudflare/token/verify')
  },

  /**
   * Clear token
   */
  clearToken(): Promise<ApiResponse<void>> {
    return apiDelete('/cloudflare/token')
  },

  /**
   * Get zone list
   */
  getZones(): Promise<ApiResponse<CloudflareZone[]>> {
    return apiGet('/cloudflare/zones')
  },

  /**
   * Get zone by ID
   */
  getZoneById(zoneId: string): Promise<ApiResponse<CloudflareZone>> {
    return apiGet(`/cloudflare/zones/${zoneId}`)
  },
}
