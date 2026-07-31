import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

/**
 * Format date to string
 */
export function formatDate(date: string | Date, format: string = 'YYYY-MM-DD HH:mm:ss'): string {
  if (!date) return '-'
  return dayjs(date).format(format)
}

/**
 * Format date to relative time
 */
export function formatRelativeTime(date: string | Date): string {
  if (!date) return '-'
  return dayjs(date).fromNow()
}

/**
 * Format bytes to human readable
 */
export function formatBytes(bytes: number, decimals: number = 2): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

/**
 * Format duration in seconds to human readable
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}小时${Math.floor((seconds % 3600) / 60)}分钟`
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days}天${hours}小时`
}

/**
 * Format user status
 */
export function formatUserStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    active: { text: '正常', type: 'success' },
    disabled: { text: '已禁用', type: 'info' },
    locked: { text: '已锁定', type: 'warning' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format device status
 */
export function formatDeviceStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    online: { text: '在线', type: 'success' },
    offline: { text: '离线', type: 'info' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format mapping status
 */
export function formatMappingStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    active: { text: '运行中', type: 'success' },
    disabled: { text: '已禁用', type: 'info' },
    error: { text: '异常', type: 'danger' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format domain status
 */
export function formatDomainStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    pending: { text: '待验证', type: 'warning' },
    verified: { text: '已验证', type: 'success' },
    error: { text: '验证失败', type: 'danger' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format certificate status
 */
export function formatCertificateStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    valid: { text: '有效', type: 'success' },
    expired: { text: '已过期', type: 'danger' },
    expiring_soon: { text: '即将过期', type: 'warning' },
    pending: { text: '申请中', type: 'info' },
    error: { text: '申请失败', type: 'danger' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format operation status
 */
export function formatOperationStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    pending: { text: '待处理', type: 'info' },
    running: { text: '执行中', type: 'warning' },
    success: { text: '成功', type: 'success' },
    failed: { text: '失败', type: 'danger' },
    cancelled: { text: '已取消', type: 'info' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Format protocol name
 */
export function formatProtocol(protocol: string): string {
  const map: Record<string, string> = {
    tcp: 'TCP',
    udp: 'UDP',
    http: 'HTTP',
    https: 'HTTPS',
  }
  return map[protocol] || protocol.toUpperCase()
}

/**
 * Format token status
 */
export function formatTokenStatus(status: string): { text: string; type: string } {
  const map: Record<string, { text: string; type: string }> = {
    not_configured: { text: '未配置', type: 'info' },
    configured: { text: '已配置', type: 'warning' },
    verified: { text: '已验证', type: 'success' },
    error: { text: '验证失败', type: 'danger' },
  }
  return map[status] || { text: status, type: 'info' }
}

/**
 * Extract error message from API error
 */
export function extractErrorMessage(error: any): string {
  if (typeof error === 'string') return error
  if (error?.response?.data?.message) return error.response.data.message
  if (error?.response?.data?.error) return error.response.data.error
  if (error?.message) return error.message
  return '未知错误'
}

/**
 * Format operation type
 */
export function formatOperationType(type: string): string {
  const map: Record<string, string> = {
    mapping_create: '创建映射',
    mapping_update: '更新映射',
    mapping_delete: '删除映射',
    domain_create: '添加域名',
    domain_verify: '验证域名',
    certificate_issue: '签发证书',
    certificate_renew: '续签证书',
    token_upload: '上传令牌',
    token_verify: '验证令牌',
    backup_create: '创建备份',
    backup_restore: '恢复备份',
    user_create: '创建用户',
    user_update: '更新用户',
    user_delete: '删除用户',
    device_revoke: '撤销设备',
  }
  return map[type] || type
}

/**
 * Format role name
 */
export function formatRole(role: string): string {
  const map: Record<string, string> = {
    admin: '管理员',
    user: '普通用户',
  }
  return map[role] || role
}
