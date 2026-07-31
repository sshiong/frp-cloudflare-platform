<template>
  <div class="frpc-status-card">
    <div class="frpc-status-card__header">
      <div class="frpc-status-card__title">
        <el-icon :size="20" :color="statusColor">
          <component :is="statusIcon" />
        </el-icon>
        <span>FRPC 状态</span>
      </div>
      <StatusBadge :type="statusType" :label="statusLabel" />
    </div>

    <div class="frpc-status-card__body">
      <div class="frpc-status-card__stats">
        <div class="stat-item">
          <span class="stat-item__label">PID</span>
          <span class="stat-item__value">{{ status.pid || '-' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-item__label">版本</span>
          <span class="stat-item__value">{{ status.version || '-' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-item__label">运行时长</span>
          <span class="stat-item__value">{{ uptimeFormatted }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-item__label">代理数量</span>
          <span class="stat-item__value">{{ status.proxyCount }}</span>
        </div>
      </div>

      <div v-if="status.lastError" class="frpc-status-card__error">
        <el-icon><WarningFilled /></el-icon>
        <span>{{ status.lastError }}</span>
      </div>
    </div>

    <div class="frpc-status-card__footer">
      <el-button-group>
        <el-button
          v-if="!status.running"
          type="success"
          size="small"
          :loading="actionLoading"
          @click="$emit('start')"
        >
          <el-icon><VideoPlay /></el-icon>
          <span>启动</span>
        </el-button>
        <el-button
          v-else
          type="danger"
          size="small"
          :loading="actionLoading"
          @click="$emit('stop')"
        >
          <el-icon><VideoPause /></el-icon>
          <span>停止</span>
        </el-button>
        <el-button
          type="warning"
          size="small"
          :disabled="!status.running"
          :loading="actionLoading"
          @click="$emit('restart')"
        >
          <el-icon><RefreshRight /></el-icon>
          <span>重启</span>
        </el-button>
      </el-button-group>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  VideoPlay,
  VideoPause,
  RefreshRight,
  WarningFilled,
  CircleCheckFilled,
  CircleCloseFilled,
  Loading,
} from '@element-plus/icons-vue'
import StatusBadge from './StatusBadge.vue'
import type { FrpcStatus } from '@/types'
import { formatDuration } from '@/utils/format'

interface Props {
  status: FrpcStatus
  actionLoading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  actionLoading: false,
})

defineEmits<{
  start: []
  stop: []
  restart: []
}>()

const statusColor = computed(() => {
  if (props.status.running && props.status.connected) return '#16A34A'
  if (props.status.running) return '#EAB308'
  return '#71717A'
})

const statusIcon = computed(() => {
  if (props.status.running && props.status.connected) return CircleCheckFilled
  if (props.status.running) return Loading
  return CircleCloseFilled
})

const statusType = computed<'success' | 'warning' | 'danger' | 'info'>(() => {
  if (props.status.running && props.status.connected) return 'success'
  if (props.status.running) return 'warning'
  return 'info'
})

const statusLabel = computed(() => {
  if (props.status.running && props.status.connected) return '已连接'
  if (props.status.running) return '运行中'
  return '已停止'
})

const uptimeFormatted = computed(() => {
  if (!props.status.uptime) return '-'
  return formatDuration(props.status.uptime)
})
</script>

<style scoped>
.frpc-status-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.frpc-status-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #E4E4E7;
}

.frpc-status-card__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #18181B;
}

.frpc-status-card__body {
  padding: 20px;
}

.frpc-status-card__stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-item__label {
  font-size: 12px;
  color: #71717A;
}

.stat-item__value {
  font-size: 14px;
  font-weight: 600;
  color: #18181B;
}

.frpc-status-card__error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  padding: 12px;
  background: #FEF2F2;
  border-radius: 6px;
  font-size: 12px;
  color: #DC2626;
}

.frpc-status-card__footer {
  display: flex;
  justify-content: flex-end;
  padding: 12px 20px;
  border-top: 1px solid #E4E4E7;
  background: #FAFAFA;
}

.frpc-status-card__footer .el-button-group {
  display: flex;
  gap: 8px;
}

.frpc-status-card__footer .el-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>