import { get, post } from './client'
import type { Domain } from '@/types'

/**
 * Get all domains
 */
export function getDomains(): Promise<Domain[]> {
  return get<Domain[]>('/domains')
}

/**
 * Get domain by ID
 */
export function getDomain(id: number): Promise<Domain> {
  return get<Domain>(`/domains/${id}`)
}

/**
 * Sync DNS records for domain
 */
export function syncDomainDns(id: number): Promise<void> {
  return post(`/domains/${id}/sync-dns`)
}

/**
 * Request SSL certificate for domain
 */
export function requestCertificate(id: number): Promise<void> {
  return post(`/domains/${id}/request-cert`)
}

/**
 * Renew SSL certificate
 */
export function renewCertificate(id: number): Promise<void> {
  return post(`/domains/${id}/renew-cert`)
}

/**
 * Verify domain ownership
 */
export function verifyDomain(id: number): Promise<{ verified: boolean }> {
  return post(`/domains/${id}/verify`)
}