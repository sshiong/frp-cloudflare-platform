<template>
  <span :class="['status-badge', `status-badge--${type}`]">
    <span class="status-badge__dot" />
    <span class="status-badge__text">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
interface Props {
  type: 'primary' | 'success' | 'warning' | 'danger' | 'info' | undefined
  label: string
  pulse?: boolean
}

withDefaults(defineProps<Props>(), {
  pulse: false,
})
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
}

.status-badge__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-badge__text {
  white-space: nowrap;
}

/* Success */
.status-badge--success {
  background-color: #ECFDF5;
  color: #16A34A;
}

.status-badge--success .status-badge__dot {
  background-color: #16A34A;
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.2);
}

/* Warning */
.status-badge--warning {
  background-color: #FFFBEB;
  color: #92400E;
}

.status-badge--warning .status-badge__dot {
  background-color: #EAB308;
  box-shadow: 0 0 0 2px rgba(234, 179, 8, 0.2);
}

/* Danger */
.status-badge--danger {
  background-color: #FEF2F2;
  color: #DC2626;
}

.status-badge--danger .status-badge__dot {
  background-color: #DC2626;
  box-shadow: 0 0 0 2px rgba(220, 38, 38, 0.2);
}

/* Info */
.status-badge--info {
  background-color: #F4F4F5;
  color: #71717A;
}

.status-badge--info .status-badge__dot {
  background-color: #71717A;
  box-shadow: 0 0 0 2px rgba(113, 113, 122, 0.2);
}

/* Pulse animation */
.status-badge--success .status-badge__dot {
  animation: pulse-success 2s infinite;
}

@keyframes pulse-success {
  0%, 100% {
    box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.2);
  }
  50% {
    box-shadow: 0 0 0 4px rgba(22, 163, 74, 0.1);
  }
}
</style>