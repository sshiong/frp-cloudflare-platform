import { apiGet, apiPost, apiUpload, apiDownload } from './client'
import type {
  ApiResponse,
  PaginatedData,
  PaginationParams,
  Operation,
  AuditLog,
  SystemInfo,
  SystemSettings,
  Backup,
  BackupPreflightCheck,
  DashboardStats,
} from '@/types'

export const adminApi = {
  // Dashboard
  getDashboardStats(): Promise<ApiResponse<DashboardStats>> {
    return apiGet('/admin/dashboard/stats')
  },

  // System
  getSystemInfo(): Promise<ApiResponse<SystemInfo>> {
    return apiGet('/admin/system/info')
  },

  getSystemSettings(): Promise<ApiResponse<SystemSettings>> {
    return apiGet('/admin/system/settings')
  },

  updateSystemSettings(data: Partial<SystemSettings>): Promise<ApiResponse<SystemSettings>> {
    return apiPost('/admin/system/settings', data)
  },

  // Operations
  getOperations(params?: PaginationParams & { type?: string; status?: string; user_id?: number }): Promise<ApiResponse<PaginatedData<Operation>>> {
    return apiGet('/admin/operations', { params })
  },

  getOperationById(id: number): Promise<ApiResponse<Operation>> {
    return apiGet(`/admin/operations/${id}`)
  },

  cancelOperation(id: number): Promise<ApiResponse<void>> {
    return apiPost(`/admin/operations/${id}/cancel`)
  },

  retryOperation(id: number): Promise<ApiResponse<void>> {
    return apiPost(`/admin/operations/${id}/retry`)
  },

  forceCompleteOperation(id: number): Promise<ApiResponse<void>> {
    return apiPost(`/admin/operations/${id}/force-complete`)
  },

  // Audit Logs
  getAuditLogs(params?: PaginationParams & { action?: string; user_id?: number; start_date?: string; end_date?: string }): Promise<ApiResponse<PaginatedData<AuditLog>>> {
    return apiGet('/admin/audit-logs', { params })
  },

  // Backups
  getBackups(params?: PaginationParams): Promise<ApiResponse<PaginatedData<Backup>>> {
    return apiGet('/admin/backups', { params })
  },

  createBackup(): Promise<ApiResponse<Backup>> {
    return apiPost('/admin/backups')
  },

  uploadBackup(file: File): Promise<ApiResponse<BackupPreflightCheck>> {
    return apiUpload('/admin/backups/upload', file)
  },

  preflightRestore(backupId: number): Promise<ApiResponse<BackupPreflightCheck>> {
    return apiGet(`/admin/backups/${backupId}/preflight`)
  },

  restoreBackup(backupId: number): Promise<ApiResponse<void>> {
    return apiPost(`/admin/backups/${backupId}/restore`)
  },

  downloadBackup(backupId: number, filename: string): Promise<void> {
    return apiDownload(`/admin/backups/${backupId}/download`, filename)
  },
}
