import { ElNotification, ElMessage, ElMessageBox } from 'element-plus'

export function useNotification() {
  function success(message: string, title: string = '成功') {
    ElNotification.success({
      title,
      message,
      duration: 3000,
    })
  }

  function error(message: string, title: string = '错误') {
    ElNotification.error({
      title,
      message,
      duration: 5000,
    })
  }

  function warning(message: string, title: string = '警告') {
    ElNotification.warning({
      title,
      message,
      duration: 4000,
    })
  }

  function info(message: string, title: string = '提示') {
    ElNotification.info({
      title,
      message,
      duration: 3000,
    })
  }

  function msgSuccess(message: string) {
    ElMessage.success(message)
  }

  function msgError(message: string) {
    ElMessage.error(message)
  }

  function msgWarning(message: string) {
    ElMessage.warning(message)
  }

  function msgInfo(message: string) {
    ElMessage.info(message)
  }

  async function confirm(
    message: string,
    title: string = '确认操作',
    type: 'warning' | 'info' | 'success' | 'error' = 'warning'
  ): Promise<boolean> {
    try {
      await ElMessageBox.confirm(message, title, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type,
      })
      return true
    } catch {
      return false
    }
  }

  async function prompt(
    message: string,
    title: string = '请输入',
    inputPattern?: RegExp,
    inputErrorMessage?: string
  ): Promise<string | null> {
    try {
      const { value } = await ElMessageBox.prompt(message, title, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputPattern,
        inputErrorMessage,
      })
      return value
    } catch {
      return null
    }
  }

  return {
    success,
    error,
    warning,
    info,
    msgSuccess,
    msgError,
    msgWarning,
    msgInfo,
    confirm,
    prompt,
  }
}
