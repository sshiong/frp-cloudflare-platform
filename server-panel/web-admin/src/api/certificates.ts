import { apiGet, apiPost } from './client'
import type { Certificate, CertificateIssueRequest, ApiResponse, PaginatedData, PaginationParams } from '@/types'

export const certificatesApi = {
  /**
   * Get certificate list
   */
  getList(params?: PaginationParams & { status?: string; domain?: string }): Promise<ApiResponse<PaginatedData<Certificate>>> {
    return apiGet('/certificates', { params })
  },

  /**
   * Get certificate by ID
   */
  getById(id: number): Promise<ApiResponse<Certificate>> {
    return apiGet(`/certificates/${id}`)
  },

  /**
   * Issue certificate
   */
  issue(data: CertificateIssueRequest): Promise<ApiResponse<Certificate>> {
    return apiPost('/certificates/issue', data)
  },

  /**
   * Renew certificate
   */
  renew(id: number): Promise<ApiResponse<void>> {
    return apiPost(`/certificates/${id}/renew`)
  },

  /**
   * Get certificate statistics
   */
  getStats(): Promise<ApiResponse<{ valid: number; expired: number; expiring_soon: number; pending: number; total: number }>> {
    return apiGet('/certificates/stats')
  },
}
