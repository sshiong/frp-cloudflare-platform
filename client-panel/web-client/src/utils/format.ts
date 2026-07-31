import type { LogLevel, MappingStatus, DomainStatus, Protocol } from '@/types'

/**
 * Format bytes to human readable string
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

/**
 * Format duration in seconds to human readable string
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}秒`
  }

  if (seconds < 3600) {
    const minutes = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${minutes}分${secs > 0 ? `${secs}秒` : ''}`
  }

  if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}小时${minutes > 0 ? `${minutes}分` : ''}`
  }

  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days}天${hours > 0 ? `${hours}小时` : ''}`
}

/**
 * Format timestamp to local date string
 */
export function formatDate(timestamp: string | number | Date, format: 'full' | 'date' | 'time' | 'relative' = 'full'): string {
  const date = new Date(timestamp)

  if (format === 'relative') {
    return formatRelativeTime(date)
  }

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')

  switch (format) {
    case 'date':
      return `${year}-${month}-${day}`
    case 'time':
      return `${hours}:${minutes}:${seconds}`
    case 'full':
    default:
      return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  }
}

/**
 * Format relative time
 */
export function formatRelativeTime(date: Date): string {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (seconds < 60) {
    return '刚刚'
  }

  if (minutes < 60) {
    return `${minutes}分钟前`
  }

  if (hours < 24) {
    return `${hours}小时前`
  }

  if (days < 30) {
    return `${days}天前`
  }

  return formatDate(date, 'date')
}

/**
 * Format number with commas
 */
export function formatNumber(num: number): string {
  return num.toLocaleString('zh-CN')
}

/**
 * Format percentage
 */
export function formatPercentage(value: number, total: number, decimals = 1): string {
  if (total === 0) return '0%'
  const percentage = (value / total) * 100
  return `${percentage.toFixed(decimals)}%`
}

/**
 * Get status label in Chinese
 */
export function getStatusLabel(status: MappingStatus | DomainStatus): string {
  const statusMap: Record<string, string> = {
    active: '运行中',
    inactive: '已停止',
    error: '错误',
    pending: '等待中',
    expired: '已过期',
  }
  return statusMap[status] || status
}

/**
 * Get status type for Element Plus tag
 */
export function getStatusType(status: MappingStatus | DomainStatus): 'success' | 'warning' | 'danger' | 'info' {
  const typeMap: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    active: 'success',
    inactive: 'info',
    error: 'danger',
    pending: 'warning',
    expired: 'danger',
  }
  return typeMap[status] || 'info'
}

/**
 * Get log level label in Chinese
 */
export function getLogLevelLabel(level: LogLevel): string {
  const levelMap: Record<LogLevel, string> = {
    trace: '跟踪',
    debug: '调试',
    info: '信息',
    warn: '警告',
    error: '错误',
  }
  return levelMap[level] || level
}

/**
 * Get log level type for Element Plus tag
 */
export function getLogLevelType(level: LogLevel): 'success' | 'warning' | 'danger' | 'info' | '' {
  const typeMap: Record<LogLevel, 'success' | 'warning' | 'danger' | 'info' | ''> = {
    trace: 'info',
    debug: 'info',
    info: '',
    warn: 'warning',
    error: 'danger',
  }
  return typeMap[level] || ''
}

/**
 * Get protocol label in Chinese
 */
export function getProtocolLabel(protocol: Protocol): string {
  const protocolMap: Record<Protocol, string> = {
    tcp: 'TCP',
    udp: 'UDP',
    http: 'HTTP',
    https: 'HTTPS',
  }
  return protocolMap[protocol] || protocol.toUpperCase()
}

/**
 * Get protocol color
 */
export function getProtocolColor(protocol: Protocol): string {
  const colorMap: Record<Protocol, string> = {
    tcp: '#2563EB',
    udp: '#7C3AED',
    http: '#16A34A',
    https: '#EAB308',
  }
  return colorMap[protocol] || '#71717A'
}

/**
 * Validate IP address
 */
export function isValidIP(ip: string): boolean {
  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/
  const ipv6Regex = /^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$/

  if (ipv4Regex.test(ip)) {
    const parts = ip.split('.')
    return parts.every((part) => {
      const num = parseInt(part, 10)
      return num >= 0 && num <= 255
    })
  }

  return ipv6Regex.test(ip) || ip === 'localhost'
}

/**
 * Validate port number
 */
export function isValidPort(port: number): boolean {
  return port >= 1 && port <= 65535
}

/**
 * Validate domain name
 */
export function isValidDomain(domain: string): boolean {
  const domainRegex = /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/
  return domainRegex.test(domain)
}

/**
 * Truncate string
 */
export function truncate(str: string, length: number): string {
  if (str.length <= length) return str
  return str.substring(0, length) + '...'
}

/**
 * Sleep utility
 */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * Generate random ID
 */
export function generateId(): string {
  return Math.random().toString(36).substring(2, 15)
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // Fallback for older browsers
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.left = '-9999px'
    document.body.appendChild(textArea)
    textArea.select()
    try {
      document.execCommand('copy')
      return true
    } catch {
      return false
    } finally {
      document.body.removeChild(textArea)
    }
  }
}