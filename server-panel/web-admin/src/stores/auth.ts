import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import { setCsrfToken } from '@/api/client'
import type { UserInfo, UserRole } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('auth_token'))
  const user = ref<UserInfo | null>(null)
  const loading = ref(false)

  // Load user from localStorage
  const savedUser = localStorage.getItem('auth_user')
  if (savedUser) {
    try {
      user.value = JSON.parse(savedUser)
    } catch {
      localStorage.removeItem('auth_user')
    }
  }

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const username = computed(() => user.value?.username || '')

  async function login(usernameStr: string, password: string): Promise<void> {
    loading.value = true
    try {
      const res = await authApi.login({ username: usernameStr, password })
      if (res.code === 0 && res.data) {
        token.value = res.data.token
        user.value = {
          id: res.data.user.id,
          username: res.data.user.username,
          role: res.data.user.role,
        }
        localStorage.setItem('auth_token', res.data.token)
        localStorage.setItem('auth_user', JSON.stringify(user.value))
      } else {
        throw new Error(res.message || '登录失败')
      }
    } finally {
      loading.value = false
    }
  }

  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout API errors
    }
    token.value = null
    user.value = null
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
    setCsrfToken('')
  }

  async function fetchUserInfo(): Promise<void> {
    if (!token.value) return
    try {
      const res = await authApi.getUserInfo()
      if (res.code === 0 && res.data) {
        user.value = res.data
        localStorage.setItem('auth_user', JSON.stringify(user.value))
      }
    } catch {
      // Token might be invalid
      await logout()
    }
  }

  async function initCsrfToken(): Promise<void> {
    try {
      const res = await authApi.getCsrfToken()
      if (res.code === 0 && res.data) {
        setCsrfToken(res.data.token)
      }
    } catch {
      // CSRF might not be required
    }
  }

  function hasRole(role: UserRole): boolean {
    return user.value?.role === role
  }

  return {
    token,
    user,
    loading,
    isAuthenticated,
    isAdmin,
    username,
    login,
    logout,
    fetchUserInfo,
    initCsrfToken,
    hasRole,
  }
})
