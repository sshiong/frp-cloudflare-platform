<template>
  <div class="login-container">
    <div class="login-card">
      <!-- Logo -->
      <div class="login-header">
        <div class="login-logo">
          <el-icon :size="32" color="#2563EB"><Connection /></el-icon>
        </div>
        <h1 class="login-title">FRP 云隧道管理平台</h1>
        <p class="login-subtitle">请登录您的账户</p>
      </div>

      <!-- Login Form -->
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        size="large"
        @submit.prevent="handleLogin"
      >
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="请输入用户名"
            prefix-icon="User"
            :disabled="loading"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            prefix-icon="Lock"
            show-password
            :disabled="loading"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <!-- Advanced Settings -->
        <el-collapse v-model="showAdvanced" class="advanced-collapse">
          <el-collapse-item title="高级设置" name="advanced">
            <el-form-item label="服务器地址">
              <el-input
                v-model="form.serverAddress"
                placeholder="默认: 当前域名"
                prefix-icon="Link"
                :disabled="loading"
              />
            </el-form-item>
          </el-collapse-item>
        </el-collapse>

        <el-form-item class="login-actions">
          <el-button
            type="primary"
            :loading="loading"
            class="login-button"
            @click="handleLogin"
          >
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>

      <!-- Footer -->
      <div class="login-footer">
        <p>FRP 云隧道管理系统 v1.0</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNotification } from '@/composables/useNotification'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { error: showError, msgSuccess } = useNotification()

const formRef = ref<FormInstance>()
const loading = ref(false)
const showAdvanced = ref<string[]>([])

const form = reactive({
  username: '',
  password: '',
  serverAddress: '',
})

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 32, message: '用户名长度在 2 到 32 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度在 6 到 64 个字符', trigger: 'blur' },
  ],
}

async function handleLogin() {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      await authStore.login(form.username, form.password)
      msgSuccess('登录成功')

      // Redirect to intended page or dashboard
      const redirect = (route.query.redirect as string) || '/dashboard'
      router.push(redirect)
    } catch (err: any) {
      showError(err?.message || '登录失败，请检查用户名和密码')
    } finally {
      loading.value = false
    }
  })
}

onMounted(() => {
  // Focus username input
  const usernameInput = document.querySelector('.login-container input')
  if (usernameInput) {
    (usernameInput as HTMLInputElement).focus()
  }
})
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-secondary);
  padding: var(--spacing-lg);
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: var(--color-bg-primary);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
  padding: var(--spacing-2xl);
  border: 1px solid var(--color-border);
}

.login-header {
  text-align: center;
  margin-bottom: var(--spacing-xl);
}

.login-logo {
  width: 64px;
  height: 64px;
  background: var(--color-primary-bg);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto var(--spacing-md);
}

.login-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: var(--spacing-xs);
}

.login-subtitle {
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.advanced-collapse {
  margin-bottom: var(--spacing-md);
  border: none;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  font-size: 13px;
  color: var(--color-text-secondary);
  border: none;
  height: 32px;
  line-height: 32px;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border: none;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 0;
}

.login-actions {
  margin-top: var(--spacing-lg);
  margin-bottom: 0;
}

.login-button {
  width: 100%;
  height: 44px;
  font-size: 15px;
  font-weight: 600;
  border-radius: var(--radius-md);
}

.login-footer {
  text-align: center;
  margin-top: var(--spacing-xl);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--color-border-light);
}

.login-footer p {
  font-size: 12px;
  color: var(--color-text-placeholder);
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--color-text-primary);
  font-size: 13px;
}

:deep(.el-input__wrapper) {
  border-radius: var(--radius-md);
}
</style>
