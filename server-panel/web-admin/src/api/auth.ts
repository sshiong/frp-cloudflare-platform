import { apiGet, apiPost } from './client'
import type { LoginRequest, LoginResponse, UserInfo, ApiResponse } from '@/types'

export const authApi = {
  /**
   * Login with username and password
   */
  login(data: LoginRequest): Promise<ApiResponse<LoginResponse>> {
    return apiPost('/auth/login', data)
  },

  /**
   * Logout current session
   */
  logout(): Promise<ApiResponse<void>> {
    return apiPost('/auth/logout')
  },

  /**
   * Get current user info
   */
  getUserInfo(): Promise<ApiResponse<UserInfo>> {
    return apiGet('/auth/me')
  },

  /**
   * Refresh auth token
   */
  refreshToken(): Promise<ApiResponse<{ token: string }>> {
    return apiPost('/auth/refresh')
  },

  /**
   * Change password
   */
  changePassword(data: { old_password: string; new_password: string }): Promise<ApiResponse<void>> {
    return apiPost('/auth/change-password', data)
  },

  /**
   * Get CSRF token
   */
  getCsrfToken(): Promise<ApiResponse<{ token: string }>> {
    return apiGet('/auth/csrf')
  },
}
