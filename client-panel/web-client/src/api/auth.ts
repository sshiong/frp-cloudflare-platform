import { get, post } from './client'
import type { LoginRequest, LoginResponse, User } from '@/types'

/**
 * Login with username and password
 */
export function login(data: LoginRequest): Promise<LoginResponse> {
  return post<LoginResponse>('/auth/login', data)
}

/**
 * Logout
 */
export function logout(): Promise<void> {
  return post('/auth/logout')
}

/**
 * Get current user info
 */
export function getCurrentUser(): Promise<User> {
  return get<User>('/auth/me')
}

/**
 * Refresh token
 */
export function refreshToken(): Promise<{ token: string }> {
  return post('/auth/refresh')
}

/**
 * Change password
 */
export function changePassword(data: {
  oldPassword: string
  newPassword: string
  confirmPassword: string
}): Promise<void> {
  return post('/auth/change-password', data)
}