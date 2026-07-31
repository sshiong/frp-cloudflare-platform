import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as authApi from '@/api/auth'
import type { User, LoginRequest } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const router = useRouter()

  // State
  const token = ref<string | null>(localStorage.getItem('frp_token'))
  const user = ref<User | null>(null)
  const loading = ref(false)
  const serverAddr = ref<string>(localStorage.getItem('frp_server_addr') || '')

  // Getters
  const isAuthenticated = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')
  const userRole = computed(() => user.value?.role || 'user')

  // Actions
  async function login(data: LoginRequest) {
    loading.value = true
    try {
      // Save server address if provided
      if (data.serverAddr) {
        serverAddr.value = data.serverAddr
        localStorage.setItem('frp_server_addr', data.serverAddr)
      }

      const response = await authApi.login(data)
      token.value = response.token
      user.value = response.user

      // Save to localStorage
      localStorage.setItem('frp_token', response.token)
      localStorage.setItem('frp_user', JSON.stringify(response.user))

      ElMessage.success('登录成功')
      router.push('/dashboard')
    } catch (error: any) {
      ElMessage.error(error.message || '登录失败')
      throw error
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout API error
    } finally {
      // Clear state
      token.value = null
      user.value = null
      localStorage.removeItem('frp_token')
      localStorage.removeItem('frp_user')
      router.push('/login')
    }
  }

  async function fetchCurrentUser() {
    if (!token.value) return

    try {
      const userData = await authApi.getCurrentUser()
      user.value = userData
      localStorage.setItem('frp_user', JSON.stringify(userData))
    } catch {
      // If fetch fails, clear auth
      token.value = null
      user.value = null
      localStorage.removeItem('frp_token')
      localStorage.removeItem('frp_user')
    }
  }

  function initAuth() {
    // Try to restore user from localStorage
    const savedUser = localStorage.getItem('frp_user')
    if (savedUser) {
      try {
        user.value = JSON.parse(savedUser)
      } catch {
        localStorage.removeItem('frp_user')
      }
    }

    // If we have a token, fetch current user
    if (token.value) {
      fetchCurrentUser()
    }
  }

  function setServerAddr(addr: string) {
    serverAddr.value = addr
    localStorage.setItem('frp_server_addr', addr)
  }

  return {
    // State
    token,
    user,
    loading,
    serverAddr,

    // Getters
    isAuthenticated,
    username,
    userRole,

    // Actions
    login,
    logout,
    fetchCurrentUser,
    initAuth,
    setServerAddr,
  }
})