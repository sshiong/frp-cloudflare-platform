import { ElMessage, ElNotification, ElMessageBox } from 'element-plus'
import type { MessageHandler, NotificationHandle } from 'element-plus'

export function useNotification() {
  // Success message
  function success(message: string, duration = 3000): MessageHandler {
    return ElMessage({
      message,
      type: 'success',
      duration,
      showClose: true,
    })
  }

  // Error message
  function error(message: string, duration = 5000): MessageHandler {
    return ElMessage({
      message,
      type: 'error',
      duration,
      showClose: true,
    })
  }

  // Warning message
  function warning(message: string, duration = 4000): MessageHandler {
    return ElMessage({
      message,
      type: 'warning',
      duration,
      showClose: true,
    })
  }

  // Info message
  function info(message: string, duration = 3000): MessageHandler {
    return ElMessage({
      message,
      type: 'info',
      duration,
      showClose: true,
    })
  }

  // Notification
  function notify(options: {
    title: string
    message: string
    type?: 'success' | 'warning' | 'info' | 'error'
    duration?: number
    position?: 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left'
  }): NotificationHandle {
    return ElNotification({
      title: options.title,
      message: options.message,
      type: options.type || 'info',
      duration: options.duration || 4500,
      position: options.position || 'top-right',
    })
  }

  // Confirm dialog
  async function confirm(
    message: string,
    title = '确认操作',
    options?: {
      confirmButtonText?: string
      cancelButtonText?: string
      type?: 'warning' | 'info' | 'success' | 'error'
    }
  ): Promise<boolean> {
    try {
      await ElMessageBox.confirm(message, title, {
        confirmButtonText: options?.confirmButtonText || '确定',
        cancelButtonText: options?.cancelButtonText || '取消',
        type: options?.type || 'warning',
        distinguishCancelAndClose: true,
      })
      return true
    } catch {
      return false
    }
  }

  // Prompt dialog
  async function prompt(
    message: string,
    title = '输入',
    options?: {
      confirmButtonText?: string
      cancelButtonText?: string
      inputPattern?: RegExp
      inputErrorMessage?: string
      inputPlaceholder?: string
      inputValue?: string
    }
  ): Promise<string | null> {
    try {
      const { value } = await ElMessageBox.prompt(message, title, {
        confirmButtonText: options?.confirmButtonText || '确定',
        cancelButtonText: options?.cancelButtonText || '取消',
        inputPattern: options?.inputPattern,
        inputErrorMessage: options?.inputErrorMessage || '输入格式不正确',
        inputPlaceholder: options?.inputPlaceholder || '请输入',
        inputValue: options?.inputValue || '',
      })
      return value
    } catch {
      return null
    }
  }

  // Delete confirmation
  async function confirmDelete(itemName = '此项'): Promise<boolean> {
    return confirm(
      `确定要删除 ${itemName} 吗？此操作不可撤销。`,
      '删除确认',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'error',
      }
    )
  }

  // Action confirmation
  async function confirmAction(
    action: string,
    itemName = '此项'
  ): Promise<boolean> {
    return confirm(
      `确定要${action} ${itemName} 吗？`,
      '操作确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  }

  return {
    success,
    error,
    warning,
    info,
    notify,
    confirm,
    prompt,
    confirmDelete,
    confirmAction,
  }
}