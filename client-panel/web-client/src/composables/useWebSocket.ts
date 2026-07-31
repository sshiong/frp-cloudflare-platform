import { ref, onMounted, onUnmounted, type Ref } from 'vue'
import { useFrpcStore } from '@/stores/frpc'
import type { WsMessage, WsStatusUpdate, LogEntry } from '@/types'

export interface UseWebSocketOptions {
  url?: string
  reconnectInterval?: number
  maxReconnectAttempts?: number
  onMessage?: (message: WsMessage) => void
  onConnected?: () => void
  onDisconnected?: () => void
  onError?: (error: Event) => void
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
  const {
    url = `ws://${window.location.host}/ws`,
    reconnectInterval = 3000,
    maxReconnectAttempts = 10,
    onMessage,
    onConnected,
    onDisconnected,
    onError,
  } = options

  const frpcStore = useFrpcStore()

  // State
  const isConnected = ref(false)
  const reconnectAttempts = ref(0)
  const lastMessage: Ref<WsMessage | null> = ref(null)

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null

  // Connect to WebSocket
  function connect() {
    if (ws?.readyState === WebSocket.OPEN) return

    try {
      // Get auth token
      const token = localStorage.getItem('frp_token')
      const wsUrl = token ? `${url}?token=${token}` : url

      ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        console.log('[WebSocket] Connected')
        isConnected.value = true
        reconnectAttempts.value = 0
        onConnected?.()

        // Start ping interval
        startPing()
      }

      ws.onmessage = (event) => {
        try {
          const message: WsMessage = JSON.parse(event.data)
          lastMessage.value = message
          handleMessage(message)
          onMessage?.(message)
        } catch (error) {
          console.error('[WebSocket] Failed to parse message:', error)
        }
      }

      ws.onclose = (event) => {
        console.log('[WebSocket] Disconnected:', event.code, event.reason)
        isConnected.value = false
        stopPing()
        onDisconnected?.()

        // Attempt to reconnect
        if (reconnectAttempts.value < maxReconnectAttempts) {
          scheduleReconnect()
        }
      }

      ws.onerror = (event) => {
        console.error('[WebSocket] Error:', event)
        onError?.(event)
      }
    } catch (error) {
      console.error('[WebSocket] Connection failed:', error)
      scheduleReconnect()
    }
  }

  // Disconnect from WebSocket
  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    stopPing()

    if (ws) {
      ws.close(1000, 'Client disconnect')
      ws = null
    }

    isConnected.value = false
    reconnectAttempts.value = 0
  }

  // Schedule reconnect
  function scheduleReconnect() {
    if (reconnectTimer) return

    reconnectAttempts.value++
    const delay = Math.min(reconnectInterval * Math.pow(1.5, reconnectAttempts.value - 1), 30000)

    console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${reconnectAttempts.value}/${maxReconnectAttempts})`)

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  // Start ping interval
  function startPing() {
    stopPing()
    pingTimer = setInterval(() => {
      if (ws?.readyState === WebSocket.OPEN) {
        send({ type: 'ping', payload: {} })
      }
    }, 30000)
  }

  // Stop ping interval
  function stopPing() {
    if (pingTimer) {
      clearInterval(pingTimer)
      pingTimer = null
    }
  }

  // Handle incoming message
  function handleMessage(message: WsMessage) {
    switch (message.type) {
      case 'status_update':
        const statusUpdate = message.payload as WsStatusUpdate
        frpcStore.updateStatus(statusUpdate.frpc)
        break

      case 'log':
        const logEntry = message.payload as LogEntry
        frpcStore.addLog(logEntry)
        break

      case 'pong':
        // Keep alive response
        break

      default:
        console.log('[WebSocket] Unknown message type:', message.type)
    }
  }

  // Send message
  function send(message: { type: string; payload: any }) {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        ...message,
        timestamp: new Date().toISOString(),
      }))
    }
  }

  // Lifecycle
  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    disconnect()
  })

  return {
    isConnected,
    lastMessage,
    reconnectAttempts,
    connect,
    disconnect,
    send,
  }
}