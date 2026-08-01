import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { extractErrorMessage } from '@/utils/format'

// Create axios instance
const client: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true,
})

// CSRF token
let csrfToken: string | null = null

export function setCsrfToken(token: string) {
  csrfToken = token
}

export function getCsrfToken(): string | null {
  return csrfToken
}

// Request interceptor
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Add CSRF token for mutation requests
    if (csrfToken && ['post', 'put', 'delete', 'patch'].includes(config.method || '')) {
      config.headers['X-CSRF-Token'] = csrfToken
    }

    // Add auth token from localStorage
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
client.interceptors.response.use(
  (response: AxiosResponse) => {
    // Update CSRF token from response header
    const newCsrfToken = response.headers['x-csrf-token']
    if (newCsrfToken) {
      csrfToken = newCsrfToken
    }

    return response
  },
  (error) => {
    if (error.response) {
      const { status, data } = error.response

      switch (status) {
        case 401:
          // Clear auth and redirect to login
          localStorage.removeItem('auth_token')
          localStorage.removeItem('auth_user')
          router.push('/login')
          ElMessage.error('登录已过期，请重新登录')
          break

        case 403:
          ElMessage.error('没有权限执行此操作')
          break

        case 404:
          ElMessage.error('请求的资源不存在')
          break

        case 422:
          ElMessage.error(data?.message || '请求参数错误')
          break

        case 429:
          ElMessage.warning('请求过于频繁，请稍后再试')
          break

        case 500:
          ElMessage.error(data?.message || '服务器内部错误')
          break

        default:
          ElMessage.error(extractErrorMessage(error))
      }
    } else if (error.request) {
      ElMessage.error('网络错误，请检查网络连接')
    } else {
      ElMessage.error(extractErrorMessage(error))
    }

    return Promise.reject(error)
  }
)

// Type-safe API wrapper
export async function apiGet<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const response = await client.get<T>(url, config)
  return response.data
}

export async function apiPost<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  const response = await client.post<T>(url, data, config)
  return response.data
}

export async function apiPut<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  const response = await client.put<T>(url, data, config)
  return response.data
}

export async function apiDelete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const response = await client.delete<T>(url, config)
  return response.data
}

export async function apiPatch<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
  const response = await client.patch<T>(url, data, config)
  return response.data
}

// Upload helper
export async function apiUpload<T>(url: string, file: File, fieldName: string = 'file', extraData?: Record<string, any>): Promise<T> {
  const formData = new FormData()
  formData.append(fieldName, file)
  if (extraData) {
    Object.entries(extraData).forEach(([key, value]) => {
      formData.append(key, String(value))
    })
  }
  const response = await client.post<T>(url, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
  return response.data
}

// Download helper
export async function apiDownload(url: string, filename: string): Promise<void> {
  const response = await client.get(url, {
    responseType: 'blob',
  })
  const blob = new Blob([response.data])
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

export default client
