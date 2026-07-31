import { apiGet, apiPost, apiPut, apiDelete } from './client'
import type { User, UserCreateRequest, UserUpdateRequest, ApiResponse, PaginatedData, PaginationParams } from '@/types'

export const usersApi = {
  /**
   * Get user list
   */
  getList(params?: PaginationParams & { role?: string; status?: string; keyword?: string }): Promise<ApiResponse<PaginatedData<User>>> {
    return apiGet('/admin/users', { params })
  },

  /**
   * Get user by ID
   */
  getById(id: number): Promise<ApiResponse<User>> {
    return apiGet(`/admin/users/${id}`)
  },

  /**
   * Create user
   */
  create(data: UserCreateRequest): Promise<ApiResponse<User>> {
    return apiPost('/admin/users', data)
  },

  /**
   * Update user
   */
  update(id: number, data: UserUpdateRequest): Promise<ApiResponse<User>> {
    return apiPut(`/admin/users/${id}`, data)
  },

  /**
   * Delete user
   */
  delete(id: number): Promise<ApiResponse<void>> {
    return apiDelete(`/admin/users/${id}`)
  },

  /**
   * Enable user
   */
  enable(id: number): Promise<ApiResponse<void>> {
    return apiPut(`/admin/users/${id}/enable`)
  },

  /**
   * Disable user
   */
  disable(id: number): Promise<ApiResponse<void>> {
    return apiPut(`/admin/users/${id}/disable`)
  },

  /**
   * Reset user password
   */
  resetPassword(id: number, newPassword: string): Promise<ApiResponse<void>> {
    return apiPost(`/admin/users/${id}/reset-password`, { password: newPassword })
  },
}
