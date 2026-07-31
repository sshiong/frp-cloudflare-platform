import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import type { ApiResponse, ApiError } from '@/types'

// Create axios instance
const client: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Add auth token if exists
    const token = localStorage.getItem('frp_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // Add server address if configured
    const serverAddr = localStorage.getItem('frp_server_addr')
    if (serverAddr) {
      config.headers['X-Server-Addr'] = serverAddr
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
client.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const { data } = response

    // Check if response has standard API structure
    if (data && typeof data.code === 'number') {
      if (data.code !== 0 && data.code !== 200) {
        // API returned an error
        const error: ApiError = {
          code: data.code,
          message: data.message || '请求失败',
        }
        return Promise.reject(error)
      }
      // Return the data field
      return data.data
    }

    // Return raw response if not standard structure
    return data
  },
  (error) => {
    if (error.response) {
      const { status, data } = error.response

      // Handle specific HTTP status codes
      switch (status) {
        case 401:
          // Unauthorized - clear token and redirect to login
          localStorage.removeItem('frp_token')
          localStorage.removeItem('frp_user')
          if (window.location.pathname !== '/login') {
            window.location.href = '/login'
          }
          ElMessage.error('登录已过期，请重新登录')
          break

        case 403:
          ElMessage.error('没有权限执行此操作')
          break

        case 404:
          ElMessage.error('请求的资源不存在')
          break

        case 422:
          // Validation error
          const validationErrors = data?.details || {}
          const firstError = Object.values(validationErrors)[0]
          if (Array.isArray(firstError) && firstError.length > 0) {
            ElMessage.error(firstError[0] as string)
          } else {
            ElMessage.error(data?.message || '请求参数错误')
          }
          break

        case 429:
          ElMessage.error('请求过于频繁，请稍后再试')
          break

        case 500:
          ElMessage.error('服务器内部错误')
          break

        case 502:
        case 503:
          ElMessage.error('服务暂时不可用，请稍后再试')
          break

        default:
          ElMessage.error(data?.message || `请求失败 (${status})`)
      }

      // Return structured error
      const apiError: ApiError = {
        code: status,
        message: data?.message || '请求失败',
        details: data?.details,
      }
      return Promise.reject(apiError)
    }

    if (error.request) {
      // Request was made but no response received
      ElMessage.error('网络连接失败，请检查网络')
      return Promise.reject({
        code: -1,
        message: '网络连接失败',
      })
    }

    // Something else happened
    ElMessage.error('请求配置错误')
    return Promise.reject({
      code: -1,
      message: error.message || '请求失败',
    })
  }
)

// Helper functions for common request patterns
export async function get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return client.get(url, config) as Promise<T>
}

export async function post<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  return client.post(url, data, config) as Promise<T>
}

export async function put<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  return client.put(url, data, config) as Promise<T>
}

export async function patch<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  return client.patch(url, data, config) as Promise<T>
}

export async function del<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return client.delete(url, config) as Promise<T>
}

export default client