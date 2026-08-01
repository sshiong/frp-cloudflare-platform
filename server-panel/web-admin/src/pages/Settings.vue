<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">系统设置</h2>
        <p class="page-description">配置系统参数</p>
      </div>
    </div>

    <div class="card">
      <el-form
        ref="formRef"
        :model="settings"
        :rules="rules"
        label-width="160px"
        v-loading="loading"
        element-loading-text="加载中..."
      >
        <!-- Server Settings -->
        <div class="settings-section">
          <h3 class="section-title">服务器配置</h3>

          <el-form-item label="服务器名称" prop="server_name">
            <el-input v-model="settings.server_name" placeholder="FRP 云隧道服务器" />
          </el-form-item>

          <el-form-item label="FRPS 端口" prop="frps_port">
            <el-input-number v-model="settings.frps_port" :min="1" :max="65535" />
            <span class="form-tip">FRPC 连接端口 (默认 7000)</span>
          </el-form-item>

          <el-form-item label="HTTP 端口" prop="http_port">
            <el-input-number v-model="settings.http_port" :min="1" :max="65535" />
            <span class="form-tip">域名 HTTP 入口端口 (默认 80)</span>
          </el-form-item>

          <el-form-item label="HTTPS 端口" prop="https_port">
            <el-input-number v-model="settings.https_port" :min="1" :max="65535" />
            <span class="form-tip">域名 HTTPS 入口端口 (默认 443)</span>
          </el-form-item>
        </div>

        <!-- Limit Settings -->
        <div class="settings-section">
          <h3 class="section-title">资源限制</h3>

          <el-form-item label="最大用户数" prop="max_users">
            <el-input-number v-model="settings.max_users" :min="0" :max="10000" />
            <span class="ml-2 text-secondary">0 为不限制</span>
          </el-form-item>

          <el-form-item label="每用户最大映射数" prop="max_mappings_per_user">
            <el-input-number v-model="settings.max_mappings_per_user" :min="0" :max="1000" />
            <span class="ml-2 text-secondary">0 为不限制</span>
          </el-form-item>

          <el-form-item label="默认带宽限制" prop="default_bandwidth_limit">
            <el-input-number v-model="settings.default_bandwidth_limit" :min="0" :max="10000" />
            <span class="ml-2 text-secondary">MB/s，0 为不限制</span>
          </el-form-item>
        </div>

        <!-- Feature Settings -->
        <div class="settings-section">
          <h3 class="section-title">功能开关</h3>

          <el-form-item label="开放注册">
            <el-switch v-model="settings.registration_enabled" />
            <span class="ml-2 text-secondary">允许新用户自助注册</span>
          </el-form-item>

          <el-form-item label="邮件通知">
            <el-switch v-model="settings.email_notifications" />
            <span class="ml-2 text-secondary">启用邮件通知功能</span>
          </el-form-item>
        </div>

        <!-- Actions -->
        <div class="settings-actions">
          <el-button type="primary" :loading="saving" @click="handleSave">保存设置</el-button>
          <el-button @click="fetchSettings">重置</el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { adminApi } from '@/api/admin'
import { useNotification } from '@/composables/useNotification'
import { extractErrorMessage } from '@/utils/format'
import type { SystemSettings } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'

const { success, error } = useNotification()

const loading = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()

const settings = reactive<SystemSettings>({
  server_name: '',
  frps_port: 7000,
  http_port: 80,
  https_port: 443,
  max_users: 0,
  max_mappings_per_user: 10,
  default_bandwidth_limit: 0,
  registration_enabled: false,
  email_notifications: false,
})

const rules: FormRules = {
  server_name: [{ required: true, message: '请输入服务器名称', trigger: 'blur' }],
  frps_port: [{ required: true, message: '请输入FRPS端口', trigger: 'blur' }],
  http_port: [{ required: true, message: '请输入HTTP端口', trigger: 'blur' }],
  https_port: [{ required: true, message: '请输入HTTPS端口', trigger: 'blur' }],
}

async function fetchSettings() {
  loading.value = true
  try {
    const res = await adminApi.getSystemSettings()
    if (res.code === 0) {
      Object.assign(settings, res.data)
    }
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    saving.value = true
    try {
      await adminApi.updateSystemSettings(settings)
      success('设置保存成功')
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      saving.value = false
    }
  })
}

onMounted(() => {
  fetchSettings()
})
</script>

<style scoped>
.settings-section {
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-xl);
  border-bottom: 1px solid var(--color-border-light);
}

.settings-section:last-of-type {
  border-bottom: none;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: var(--spacing-lg);
}

.settings-actions {
  display: flex;
  gap: var(--spacing-sm);
  padding-top: var(--spacing-lg);
  border-top: 1px solid var(--color-border-light);
}

:deep(.el-form-item__label) {
  font-weight: 500;
}
</style>
