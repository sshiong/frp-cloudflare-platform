import { apiGet, apiPost, apiPut, apiDelete } from './client'
import type { Mapping, MappingCreateRequest, MappingUpdateRequest, ApiResponse, PaginatedData, PaginationParams } from '@/types'

export const mappingsApi = {
  /**
   * Get mapping list
   */
  getList(params?: PaginationParams & { user_id?: number; protocol?: string; status?: string; keyword?: string }): Promise<ApiResponse<PaginatedData<Mapping>>> {
    return apiGet('/mappings', { params })
  },

  /**
   * Get mapping by ID
   */
  getById(id: number): Promise<ApiResponse<Mapping>> {
    return apiGet(`/mappings/${id}`)
  },

  /**
   * Create mapping
   */
  create(data: MappingCreateRequest): Promise<ApiResponse<Mapping>> {
    return apiPost('/mappings', data)
  },

  /**
   * Update mapping
   */
  update(id: number, data: MappingUpdateRequest): Promise<ApiResponse<Mapping>> {
    return apiPut(`/mappings/${id}`, data)
  },

  /**
   * Delete mapping
   */
  delete(id: number, force: boolean = false): Promise<ApiResponse<void>> {
    return apiDelete(`/mappings/${id}`, { params: { force } })
  },

  /**
   * Enable mapping
   */
  enable(id: number): Promise<ApiResponse<void>> {
    return apiPut(`/mappings/${id}/enable`)
  },

  /**
   * Disable mapping
   */
  disable(id: number): Promise<ApiResponse<void>> {
    return apiPut(`/mappings/${id}/disable`)
  },

  /**
   * Get mapping statistics
   */
  getStats(): Promise<ApiResponse<{ active: number; disabled: number; error: number; total: number }>> {
    return apiGet('/mappings/stats')
  },

  /**
   * Get available ports
   */
  getAvailablePorts(): Promise<ApiResponse<number[]>> {
    return apiGet('/mappings/available-ports')
  },
}
