<template>
  <div class="login-page">
    <div class="login-container">
      <div class="login-card">
        <div class="login-card__header">
          <div class="login-logo">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 2L2 7L12 12L22 7L12 2Z" fill="#2563EB"/>
              <path d="M2 17L12 22L22 17" stroke="#2563EB" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M2 12L12 17L22 12" stroke="#2563EB" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>
          <h1 class="login-title">FRP Client Panel</h1>
          <p class="login-subtitle">内网穿透管理平台</p>
        </div>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          class="login-form"
          @submit.prevent="handleLogin"
        >
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="请输入用户名"
              size="large"
              :prefix-icon="User"
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              size="large"
              :prefix-icon="Lock"
              show-password
              @keyup.enter="handleLogin"
            />
          </el-form-item>

          <div class="login-options">
            <el-checkbox v-model="form.remember">记住密码</el-checkbox>
            <a href="#" class="login-forgot">忘记密码？</a>
          </div>

          <el-button
            type="primary"
            size="large"
            class="login-submit"
            :loading="loading"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form>

        <div class="login-server-config">
          <div class="server-config-toggle" @click="showServerConfig = !showServerConfig">
            <el-icon><Connection /></el-icon>
            <span>配置连接域名/IP</span>
            <el-icon :class="{ 'rotate-180': showServerConfig }"><ArrowDown /></el-icon>
          </div>

          <el-collapse-transition>
            <div v-show="showServerConfig" class="server-config-content">
              <el-form-item label="服务器地址">
                <el-input
                  v-model="form.serverAddr"
                  placeholder="例如：frp.example.com 或 192.168.1.100:9000"
                  size="default"
                >
                  <template #prepend>
                    <el-icon><Position /></el-icon>
                  </template>
                </el-input>
                <div class="form-help">
                  <el-icon><InfoFilled /></el-icon>
                  <span>输入服务端地址，格式：域名 或 IP:端口</span>
                </div>
              </el-form-item>

              <el-button
                type="default"
                size="default"
                class="test-connection-btn"
                :loading="testingConnection"
                @click="handleTestConnection"
              >
                <el-icon><Connection /></el-icon>
                <span>测试连接</span>
              </el-button>

              <div v-if="connectionTestResult" :class="['test-result', connectionTestResult.success ? 'test-result--success' : 'test-result--error']">
                <el-icon>
                  <component :is="connectionTestResult.success ? 'CircleCheckFilled' : 'CircleCloseFilled'" />
                </el-icon>
                <span>{{ connectionTestResult.message }}</span>
              </div>
            </div>
          </el-collapse-transition>
        </div>
      </div>

      <div class="login-footer">
        <p>FRP Client Panel v1.0.0</p>
        <p>安全、高效的内网穿透管理平台</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  User,
  Lock,
  Connection,
  ArrowDown,
  Position,
  InfoFilled,
  CircleCheckFilled,
  CircleCloseFilled,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import * as serverApi from '@/api/server'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const loading = ref(false)
const testingConnection = ref(false)
const showServerConfig = ref(false)
const connectionTestResult = ref<{ success: boolean; message: string } | null>(null)

const form = reactive({
  username: '',
  password: '',
  serverAddr: localStorage.getItem('frp_server_addr') || '',
  remember: false,
})

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 50, message: '用户名长度在 2 到 50 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 100, message: '密码长度在 6 到 100 个字符', trigger: 'blur' },
  ],
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    await authStore.login({
      username: form.username,
      password: form.password,
      serverAddr: form.serverAddr || undefined,
    })

    // Redirect to return URL or dashboard
    const redirect = route.query.redirect as string
    router.push(redirect || '/dashboard')
  } catch (error: any) {
    ElMessage.error(error.message || '登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}

async function handleTestConnection() {
  if (!form.serverAddr) {
    ElMessage.warning('请先输入服务器地址')
    return
  }

  testingConnection.value = true
  connectionTestResult.value = null

  try {
    const result = await serverApi.testConnection({
      addr: form.serverAddr,
    })

    if (result.success) {
      connectionTestResult.value = {
        success: true,
        message: `连接成功！延迟：${result.latency}ms`,
      }
    } else {
      connectionTestResult.value = {
        success: false,
        message: result.error || '连接失败',
      }
    }
  } catch (error: any) {
    connectionTestResult.value = {
      success: false,
      message: error.message || '连接测试失败',
    }
  } finally {
    testingConnection.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #F4F4F5;
  padding: 20px;
}

.login-container {
  width: 100%;
  max-width: 400px;
}

.login-card {
  background: #FFFFFF;
  border-radius: 12px;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.login-card__header {
  padding: 32px 32px 24px;
  text-align: center;
  border-bottom: 1px solid #E4E4E7;
}

.login-logo {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
}

.login-logo svg {
  width: 100%;
  height: 100%;
}

.login-title {
  font-size: 24px;
  font-weight: 700;
  color: #18181B;
  margin: 0 0 8px;
}

.login-subtitle {
  font-size: 14px;
  color: #71717A;
  margin: 0;
}

.login-form {
  padding: 24px 32px;
}

.login-options {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.login-forgot {
  font-size: 14px;
  color: #2563EB;
  text-decoration: none;
}

.login-forgot:hover {
  color: #1D4ED8;
}

.login-submit {
  width: 100%;
  height: 44px;
  font-size: 16px;
  font-weight: 600;
}

.login-server-config {
  padding: 0 32px 24px;
}

.server-config-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  background: #F4F4F5;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  color: #71717A;
  transition: all 0.2s ease;
}

.server-config-toggle:hover {
  background: #E4E4E7;
  color: #18181B;
}

.server-config-toggle .el-icon:last-child {
  transition: transform 0.2s ease;
}

.rotate-180 {
  transform: rotate(180deg);
}

.server-config-content {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #E4E4E7;
}

.form-help {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #A1A1AA;
}

.test-connection-btn {
  width: 100%;
  margin-top: 8px;
}

.test-result {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 12px;
  border-radius: 6px;
  font-size: 14px;
}

.test-result--success {
  background: #ECFDF5;
  color: #16A34A;
}

.test-result--error {
  background: #FEF2F2;
  color: #DC2626;
}

.login-footer {
  text-align: center;
  padding: 24px;
  color: #A1A1AA;
  font-size: 12px;
}

.login-footer p {
  margin: 4px 0;
}

/* Responsive */
@media (max-width: 480px) {
  .login-card__header {
    padding: 24px 20px 20px;
  }

  .login-form {
    padding: 20px;
  }

  .login-server-config {
    padding: 0 20px 20px;
  }
}
</style>