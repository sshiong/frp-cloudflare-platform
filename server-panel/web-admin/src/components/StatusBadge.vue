<template>
  <el-tag :type="tagType" :effect="effect" :size="size" :round="round">
    <span v-if="dot" class="status-dot-inline" :class="statusClass"></span>
    {{ text }}
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  status: string
  statusMap?: Record<string, { text: string; type: string }>
  size?: 'large' | 'default' | 'small'
  effect?: 'dark' | 'light' | 'plain'
  round?: boolean
  dot?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'small',
  effect: 'light',
  round: false,
  dot: false,
  statusMap: undefined,
})

const defaultMap: Record<string, { text: string; type: string }> = {
  active: { text: '正常', type: 'success' },
  online: { text: '在线', type: 'success' },
  verified: { text: '已验证', type: 'success' },
  valid: { text: '有效', type: 'success' },
  success: { text: '成功', type: 'success' },
  disabled: { text: '已禁用', type: 'info' },
  offline: { text: '离线', type: 'info' },
  pending: { text: '待处理', type: 'warning' },
  running: { text: '执行中', type: 'warning' },
  expiring_soon: { text: '即将过期', type: 'warning' },
  configured: { text: '已配置', type: 'warning' },
  error: { text: '异常', type: 'danger' },
  failed: { text: '失败', type: 'danger' },
  expired: { text: '已过期', type: 'danger' },
  locked: { text: '已锁定', type: 'danger' },
  cancelled: { text: '已取消', type: 'info' },
  not_configured: { text: '未配置', type: 'info' },
}

const statusInfo = computed(() => {
  const map = props.statusMap || defaultMap
  return map[props.status] || { text: props.status, type: 'info' }
})

const text = computed(() => statusInfo.value.text)
const tagType = computed(() => statusInfo.value.type as any)

const statusClass = computed(() => {
  const typeMap: Record<string, string> = {
    success: 'dot-success',
    warning: 'dot-warning',
    danger: 'dot-danger',
    info: 'dot-info',
  }
  return typeMap[statusInfo.value.type] || 'dot-info'
})
</script>

<style scoped>
.status-dot-inline {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
  vertical-align: middle;
}

.dot-success {
  background-color: var(--color-success);
}

.dot-warning {
  background-color: var(--color-warning);
}

.dot-danger {
  background-color: var(--color-error);
}

.dot-info {
  background-color: var(--color-text-secondary);
}
</style>
