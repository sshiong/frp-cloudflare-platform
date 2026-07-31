import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import * as frpcApi from '@/api/frpc'
import type { FrpcStatus, FrpcConfig, LogEntry, LogFilter } from '@/types'

export const useFrpcStore = defineStore('frpc', () => {
  // State
  const status = ref<FrpcStatus>({
    running: false,
    proxyCount: 0,
    serverAddr: '',
    serverPort: 0,
    connected: false,
  })
  const config = ref<FrpcConfig | null>(null)
  const logs = ref<LogEntry[]>([])
  const loading = ref(false)
  const actionLoading = ref(false)
  const logsLoading = ref(false)

  // Getters
  const isRunning = computed(() => status.value.running)
  const isConnected = computed(() => status.value.connected)
  const hasError = computed(() => !!status.value.lastError)
  const uptimeFormatted = computed(() => {
    if (!status.value.uptime) return '-'
    const hours = Math.floor(status.value.uptime / 3600)
    const minutes = Math.floor((status.value.uptime % 3600) / 60)
    return `${hours}小时${minutes}分`
  })

  // Actions
  async function fetchStatus() {
    try {
      const data = await frpcApi.getFrpcStatus()
      status.value = data
    } catch (error: any) {
      console.error('Failed to fetch FRPC status:', error)
    }
  }

  async function fetchConfig() {
    loading.value = true
    try {
      const data = await frpcApi.getFrpcConfig()
      config.value = data
    } catch (error: any) {
      console.error('Failed to fetch FRPC config:', error)
    } finally {
      loading.value = false
    }
  }

  async function start() {
    actionLoading.value = true
    try {
      await frpcApi.startFrpc()
      ElMessage.success('FRPC 启动成功')
      await fetchStatus()
    } catch (error: any) {
      ElMessage.error(error.message || 'FRPC 启动失败')
      throw error
    } finally {
      actionLoading.value = false
    }
  }

  async function stop() {
    actionLoading.value = true
    try {
      await frpcApi.stopFrpc()
      ElMessage.success('FRPC 已停止')
      await fetchStatus()
    } catch (error: any) {
      ElMessage.error(error.message || 'FRPC 停止失败')
      throw error
    } finally {
      actionLoading.value = false
    }
  }

  async function restart() {
    actionLoading.value = true
    try {
      await frpcApi.restartFrpc()
      ElMessage.success('FRPC 重启成功')
      await fetchStatus()
    } catch (error: any) {
      ElMessage.error(error.message || 'FRPC 重启失败')
      throw error
    } finally {
      actionLoading.value = false
    }
  }

  async function updateConfig(newConfig: Partial<FrpcConfig>) {
    loading.value = true
    try {
      const data = await frpcApi.updateFrpcConfig(newConfig)
      config.value = data
      ElMessage.success('配置已更新')
    } catch (error: any) {
      ElMessage.error(error.message || '配置更新失败')
      throw error
    } finally {
      loading.value = false
    }
  }

  async function fetchLogs(filter?: LogFilter) {
    logsLoading.value = true
    try {
      const data = await frpcApi.getFrpcLogs(filter)
      logs.value = data
    } catch (error: any) {
      console.error('Failed to fetch logs:', error)
    } finally {
      logsLoading.value = false
    }
  }

  async function fetchVersion(): Promise<string> {
    try {
      const data = await frpcApi.getFrpcVersion()
      return data.version
    } catch {
      return 'unknown'
    }
  }

  // Update status from WebSocket
  function updateStatus(newStatus: FrpcStatus) {
    status.value = { ...status.value, ...newStatus }
  }

  // Add log from WebSocket
  function addLog(log: LogEntry) {
    logs.value.unshift(log)
    // Keep only last 1000 logs in memory
    if (logs.value.length > 1000) {
      logs.value = logs.value.slice(0, 1000)
    }
  }

  return {
    // State
    status,
    config,
    logs,
    loading,
    actionLoading,
    logsLoading,

    // Getters
    isRunning,
    isConnected,
    hasError,
    uptimeFormatted,

    // Actions
    fetchStatus,
    fetchConfig,
    start,
    stop,
    restart,
    updateConfig,
    fetchLogs,
    fetchVersion,
    updateStatus,
    addLog,
  }
})