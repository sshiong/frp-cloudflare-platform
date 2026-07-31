<template>
  <div class="frp-logs">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">FRPC 日志</h1>
        <p class="page-header__description">查看 FRPC 客户端运行日志</p>
      </div>
      <div class="page-header__actions">
        <el-button @click="handleDownload">
          <el-icon><Download /></el-icon>
          <span>下载日志</span>
        </el-button>
        <el-button @click="fetchLogs">
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters">
      <el-select
        v-model="filterLevel"
        placeholder="日志级别"
        clearable
        style="width: 120px"
        @change="handleFilter"
      >
        <el-option label="全部" value="" />
        <el-option label="Trace" value="trace" />
        <el-option label="Debug" value="debug" />
        <el-option label="Info" value="info" />
        <el-option label="Warn" value="warn" />
        <el-option label="Error" value="error" />
      </el-select>

      <el-input
        v-model="searchKeyword"
        placeholder="搜索日志内容..."
        clearable
        style="width: 240px"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <div class="filters__right">
        <el-checkbox v-model="autoScroll">自动滚动</el-checkbox>
        <el-switch
          v-model="realtime"
          active-text="实时"
          inactive-text="暂停"
          @change="handleRealtimeToggle"
        />
      </div>
    </div>

    <!-- Log Viewer -->
    <div class="log-viewer-card">
      <div ref="logContainer" class="log-viewer" @scroll="handleScroll">
        <div
          v-for="(log, index) in filteredLogs"
          :key="index"
          :class="['log-entry', `log-entry--${log.level}`]"
        >
          <span class="log-entry__time">{{ formatLogTime(log.timestamp) }}</span>
          <span :class="['log-entry__level', `log-entry__level--${log.level}`]">
            {{ log.level.toUpperCase() }}
          </span>
          <span class="log-entry__message">{{ log.message }}</span>
          <span v-if="log.source" class="log-entry__source">[{{ log.source }}]</span>
        </div>

        <div v-if="filteredLogs.length === 0" class="log-empty">
          <el-icon><Document /></el-icon>
          <span>暂无日志</span>
        </div>
      </div>

      <div class="log-viewer__footer">
        <span class="log-count">共 {{ filteredLogs.length }} 条日志</span>
        <div class="log-actions">
          <el-button text size="small" @click="handleClearLogs">
            清空日志
          </el-button>
          <el-button text size="small" @click="scrollToBottom">
            滚动到底部
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { Download, Refresh, Search, Document } from '@element-plus/icons-vue'
import { useFrpcStore } from '@/stores/frpc'
import { useNotification } from '@/composables/useNotification'
import type { LogEntry, LogLevel } from '@/types'

const frpcStore = useFrpcStore()
const { confirm } = useNotification()

// State
const logContainer = ref<HTMLElement | null>(null)
const filterLevel = ref('')
const searchKeyword = ref('')
const autoScroll = ref(true)
const realtime = ref(true)
let refreshTimer: ReturnType<typeof setInterval> | null = null

// Computed filtered logs
const filteredLogs = computed(() => {
  let logs = frpcStore.logs

  if (filterLevel.value) {
    logs = logs.filter((log) => log.level === filterLevel.value)
  }

  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase()
    logs = logs.filter((log) =>
      log.message.toLowerCase().includes(keyword) ||
      (log.source && log.source.toLowerCase().includes(keyword))
    )
  }

  return logs
})

// Format log time
function formatLogTime(timestamp: string): string {
  const date = new Date(timestamp)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  const ms = String(date.getMilliseconds()).padStart(3, '0')
  return `${hours}:${minutes}:${seconds}.${ms}`
}

// Fetch logs
async function fetchLogs() {
  await frpcStore.fetchLogs({
    level: filterLevel.value as LogLevel || undefined,
    keyword: searchKeyword.value || undefined,
    limit: 500,
  })

  if (autoScroll.value) {
    scrollToBottom()
  }
}

// Handle filter
function handleFilter() {
  // Logs are already filtered in computed
}

// Handle search
function handleSearch() {
  // Logs are already filtered in computed
}

// Handle realtime toggle
function handleRealtimeToggle(value: boolean) {
  if (value) {
    startRealtimeUpdates()
  } else {
    stopRealtimeUpdates()
  }
}

// Start realtime updates
function startRealtimeUpdates() {
  stopRealtimeUpdates()
  refreshTimer = setInterval(() => {
    fetchLogs()
  }, 2000)
}

// Stop realtime updates
function stopRealtimeUpdates() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

// Handle scroll
function handleScroll() {
  if (!logContainer.value) return

  const { scrollTop, scrollHeight, clientHeight } = logContainer.value
  const isAtBottom = scrollHeight - scrollTop - clientHeight < 50

  autoScroll.value = isAtBottom
}

// Scroll to bottom
function scrollToBottom() {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

// Handle download
function handleDownload() {
  const logText = filteredLogs.value
    .map((log) => `[${log.timestamp}] [${log.level.toUpperCase()}] ${log.message}${log.source ? ` [${log.source}]` : ''}`)
    .join('\n')

  const blob = new Blob([logText], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `frpc-logs-${new Date().toISOString().slice(0, 10)}.txt`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// Handle clear logs
async function handleClearLogs() {
  const confirmed = await confirm('确定要清空日志吗？', '清空日志')
  if (!confirmed) return

  frpcStore.logs = []
}

// Watch for new logs
watch(() => frpcStore.logs.length, () => {
  if (autoScroll.value) {
    scrollToBottom()
  }
})

// Lifecycle
onMounted(() => {
  fetchLogs()
  if (realtime.value) {
    startRealtimeUpdates()
  }
})

onUnmounted(() => {
  stopRealtimeUpdates()
})
</script>

<style scoped>
.frp-logs {
  max-width: 1200px;
  margin: 0 auto;
}

.filters {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.filters__right {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-left: auto;
}

.log-viewer-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.log-viewer {
  height: 600px;
  overflow-y: auto;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
  font-size: 12px;
  line-height: 1.6;
  background: #18181B;
  color: #E4E4E7;
  padding: 16px;
}

.log-entry {
  display: flex;
  gap: 12px;
  padding: 2px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.log-entry:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-entry__time {
  color: #71717A;
  flex-shrink: 0;
  width: 100px;
}

.log-entry__level {
  flex-shrink: 0;
  width: 50px;
  font-weight: 600;
  text-transform: uppercase;
}

.log-entry__level--trace {
  color: #71717A;
}

.log-entry__level--debug {
  color: #A1A1AA;
}

.log-entry__level--info {
  color: #60A5FA;
}

.log-entry__level--warn {
  color: #FBBF24;
}

.log-entry__level--error {
  color: #F87171;
}

.log-entry__message {
  flex: 1;
  word-break: break-all;
}

.log-entry--error .log-entry__message {
  color: #FCA5A5;
}

.log-entry--warn .log-entry__message {
  color: #FDE68A;
}

.log-entry__source {
  color: #71717A;
  flex-shrink: 0;
}

.log-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: #71717A;
  gap: 8px;
}

.log-viewer__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-top: 1px solid #E4E4E7;
  background: #FAFAFA;
}

.log-count {
  font-size: 12px;
  color: #71717A;
}

.log-actions {
  display: flex;
  gap: 8px;
}

/* Responsive */
@media (max-width: 768px) {
  .filters {
    flex-direction: column;
    align-items: stretch;
  }

  .filters__right {
    margin-left: 0;
    justify-content: space-between;
  }

  .log-viewer {
    height: 400px;
  }
}
</style>