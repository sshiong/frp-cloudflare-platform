import { apiGet, apiPut, apiDelete } from './client'
import type { Device, DeviceRevokeRequest, ApiResponse, PaginatedData, PaginationParams } from '@/types'

export const devicesApi = {
  /**
   * Get device list
   */
  getList(params?: PaginationParams & { user_id?: number; status?: string; keyword?: string }): Promise<ApiResponse<PaginatedData<Device>>> {
    return apiGet('/devices', { params })
  },

  /**
   * Get device by ID
   */
  getById(id: number): Promise<ApiResponse<Device>> {
    return apiGet(`/devices/${id}`)
  },

  /**
   * Get devices for a specific user
   */
  getByUserId(userId: number, params?: PaginationParams): Promise<ApiResponse<PaginatedData<Device>>> {
    return apiGet(`/users/${userId}/devices`, { params })
  },

  /**
   * Revoke device
   */
  revoke(id: number, data?: DeviceRevokeRequest): Promise<ApiResponse<void>> {
    return apiDelete(`/devices/${id}`, { data })
  },

  /**
   * Get device statistics
   */
  getStats(): Promise<ApiResponse<{ online: number; offline: number; total: number }>> {
    return apiGet('/devices/stats')
  },
}
