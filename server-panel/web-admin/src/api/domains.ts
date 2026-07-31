import { apiGet, apiPost, apiDelete } from './client'
import type { Domain, DomainCreateRequest, DnsRecord, ApiResponse, PaginatedData, PaginationParams } from '@/types'

export const domainsApi = {
  /**
   * Get domain list
   */
  getList(params?: PaginationParams & { user_id?: number; status?: string; keyword?: string }): Promise<ApiResponse<PaginatedData<Domain>>> {
    return apiGet('/domains', { params })
  },

  /**
   * Get domain by ID
   */
  getById(id: number): Promise<ApiResponse<Domain>> {
    return apiGet(`/domains/${id}`)
  },

  /**
   * Create domain
   */
  create(data: DomainCreateRequest): Promise<ApiResponse<Domain>> {
    return apiPost('/domains', data)
  },

  /**
   * Delete domain
   */
  delete(id: number): Promise<ApiResponse<void>> {
    return apiDelete(`/domains/${id}`)
  },

  /**
   * Verify domain
   */
  verify(id: number): Promise<ApiResponse<void>> {
    return apiPost(`/domains/${id}/verify`)
  },

  /**
   * Get DNS records for domain
   */
  getDnsRecords(id: number): Promise<ApiResponse<DnsRecord[]>> {
    return apiGet(`/domains/${id}/dns-records`)
  },

  /**
   * Get domain statistics
   */
  getStats(): Promise<ApiResponse<{ verified: number; pending: number; error: number; total: number }>> {
    return apiGet('/domains/stats')
  },
}
