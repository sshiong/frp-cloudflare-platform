<template>
  <el-dialog
    v-model="visible"
    :title="title"
    :width="width"
    :close-on-click-modal="false"
    :close-on-press-escape="!loading"
    :show-close="!loading"
    destroy-on-close
    @close="handleClose"
  >
    <div class="confirm-content">
      <el-icon v-if="icon" :size="48" :class="iconClass">
        <component :is="icon" />
      </el-icon>
      <div class="confirm-message">
        <p v-if="message" class="message-text">{{ message }}</p>
        <slot />
      </div>
    </div>

    <template #footer>
      <div class="confirm-footer">
        <el-button @click="handleCancel" :disabled="loading">
          {{ cancelText }}
        </el-button>
        <el-button
          :type="confirmType"
          :loading="loading"
          @click="handleConfirm"
        >
          {{ confirmText }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

interface Props {
  modelValue: boolean
  title?: string
  message?: string
  confirmText?: string
  cancelText?: string
  confirmType?: 'primary' | 'success' | 'warning' | 'danger' | 'info'
  loading?: boolean
  width?: string
  icon?: string
  iconType?: 'warning' | 'danger' | 'info' | 'success'
}

const props = withDefaults(defineProps<Props>(), {
  title: '确认操作',
  message: '',
  confirmText: '确定',
  cancelText: '取消',
  confirmType: 'primary',
  loading: false,
  width: '400px',
  icon: undefined,
  iconType: 'warning',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm'): void
  (e: 'cancel'): void
  (e: 'close'): void
}>()

const visible = ref(props.modelValue)

watch(() => props.modelValue, (val) => {
  visible.value = val
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

const iconClass = computed(() => {
  const map: Record<string, string> = {
    warning: 'icon-warning',
    danger: 'icon-danger',
    info: 'icon-info',
    success: 'icon-success',
  }
  return map[props.iconType] || 'icon-info'
})

function handleConfirm() {
  emit('confirm')
}

function handleCancel() {
  visible.value = false
  emit('cancel')
}

function handleClose() {
  visible.value = false
  emit('close')
}
</script>

<style scoped>
.confirm-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: var(--spacing-md) 0;
}

.confirm-message {
  margin-top: var(--spacing-md);
}

.message-text {
  font-size: 14px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}

.icon-warning {
  color: var(--color-warning);
}

.icon-danger {
  color: var(--color-error);
}

.icon-info {
  color: var(--color-primary);
}

.icon-success {
  color: var(--color-success);
}

.confirm-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
}
</style>
