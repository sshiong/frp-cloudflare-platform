<template>
  <div class="server-config">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">服务器配置</h1>
        <p class="page-header__description">管理 FRP 服务器连接配置</p>
      </div>
    </div>

    <div v-loading="loading" class="config-content">
      <!-- Current Server -->
      <div class="config-section">
        <div class="config-section__header">
          <div>
            <h3 class="config-section__title">当前服务器</h3>
            <p class="config-section__description">显示当前连接的服务器信息</p>
          </div>
          <StatusBadge
            :type="serverConfig.connected ? 'success' : 'danger'"
            :label="serverConfig.connected ? '已连接' : '未连接'"
          />
        </div>
        <div class="config-section__body">
          <div class="server-info">
            <div class="info-item">
              <span class="info-item__label">服务器地址</span>
              <span class="info-item__value">{{ serverConfig.addr || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-item__label">服务器端口</span>
              <span class="info-item__value">{{ serverConfig.port || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-item__label">TLS 加密</span>
              <span class="info-item__value">
                <el-tag :type="serverConfig.tlsEnabled ? 'success' : 'info'" size="small">
                  {{ serverConfig.tlsEnabled ? '已启用' : '未启用' }}
                </el-tag>
              </span>
            </div>
            <div class="info-item">
              <span class="info-item__label">TLS 验证</span>
              <span class="info-item__value">
                <el-tag :type="serverConfig.tlsVerify ? 'success' : 'warning'" size="small">
                  {{ serverConfig.tlsVerify ? '严格验证' : '宽松验证' }}
                </el-tag>
              </span>
            </div>
            <div v-if="serverConfig.lastConnected" class="info-item">
              <span class="info-item__label">最后连接</span>
              <span class="info-item__value">{{ formatDate(serverConfig.lastConnected) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Modify Server -->
      <div class="config-section">
        <div class="config-section__header">
          <div>
            <h3 class="config-section__title">修改服务器配置</h3>
            <p class="config-section__description">更新服务器连接参数</p>
          </div>
        </div>
        <div class="config-section__body">
          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            label-width="120px"
            label-position="left"
          >
            <el-form-item label="服务器地址" prop="addr">
              <el-input
                v-model="form.addr"
                placeholder="frp.example.com"
              />
            </el-form-item>

            <el-form-item label="服务器端口" prop="port">
              <el-input-number
                v-model="form.port"
                :min="1"
                :max="65535"
                style="width: 200px"
              />
            </el-form-item>

            <el-form-item label="认证令牌">
              <el-input
                v-model="form.token"
                placeholder="可选"
                show-password
              />
            </el-form-item>

            <el-form-item label="TLS 加密">
              <el-switch v-model="form.tlsEnabled" />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>启用 TLS 加密传输</span>
              </div>
            </el-form-item>

            <el-form-item v-if="form.tlsEnabled" label="TLS 验证">
              <el-switch v-model="form.tlsVerify" />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>严格验证服务器证书，自签名证书请关闭</span>
              </div>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" :loading="saving" @click="handleSave">
                保存配置
              </el-button>
              <el-button :loading="testing" @click="handleTest">
                测试连接
              </el-button>
            </el-form-item>
          </el-form>

          <div v-if="testResult" :class="['test-result', testResult.success ? 'test-result--success' : 'test-result--error']">
            <el-icon>
              <component :is="testResult.success ? 'CircleCheckFilled' : 'CircleCloseFilled'" />
            </el-icon>
            <span>{{ testResult.message }}</span>
          </div>
        </div>
      </div>

      <!-- Danger Zone -->
      <div class="config-section config-section--danger">
        <div class="config-section__header">
          <div>
            <h3 class="config-section__title">危险操作</h3>
            <p class="config-section__description">请谨慎操作以下功能</p>
          </div>
        </div>
        <div class="config-section__body">
          <div class="danger-actions">
            <div class="danger-action">
              <div class="danger-action__info">
                <h4 class="danger-action__title">切换服务器</h4>
                <p class="danger-action__description">切换到其他 FRP 服务器，当前配置将被覆盖</p>
              </div>
              <el-button type="warning" @click="handleSwitchServer">
                切换服务器
              </el-button>
            </div>

            <div class="danger-action">
              <div class="danger-action__info">
                <h4 class="danger-action__title">解绑设备</h4>
                <p class="danger-action__description">解除当前设备与服务器的绑定关系</p>
              </div>
              <el-button type="danger" @click="handleUnbind">
                解绑设备
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { type FormInstance, type FormRules } from 'element-plus'
import {
  InfoFilled,
  CircleCheckFilled,
  CircleCloseFilled,
} from '@element-plus/icons-vue'
import StatusBadge from '@/components/StatusBadge.vue'
import * as serverApi from '@/api/server'
import { useNotification } from '@/composables/useNotification'
import { formatDate } from '@/utils/format'
import type { ServerConfig } from '@/types'

const { success, error, confirm, prompt } = useNotification()

// State
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const formRef = ref<FormInstance>()
const testResult = ref<{ success: boolean; message: string } | null>(null)

const serverConfig = ref<ServerConfig>({
  id: 0,
  addr: '',
  port: 7000,
  tlsEnabled: false,
  tlsVerify: false,
  connected: false,
})

const form = reactive({
  addr: '',
  port: 7000,
  token: '',
  tlsEnabled: false,
  tlsVerify: false,
})

const rules: FormRules = {
  addr: [
    { required: true, message: '请输入服务器地址', trigger: 'blur' },
  ],
  port: [
    { required: true, message: '请输入服务器端口', trigger: 'blur' },
  ],
}

// Fetch server config
async function fetchServerConfig() {
  loading.value = true
  try {
    const config = await serverApi.getServerConfig()
    serverConfig.value = config
    Object.assign(form, {
      addr: config.addr,
      port: config.port,
      token: config.token || '',
      tlsEnabled: config.tlsEnabled,
      tlsVerify: config.tlsVerify,
    })
  } catch (err) {
    console.error('Failed to fetch server config:', err)
  } finally {
    loading.value = false
  }
}

// Save config
async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await serverApi.updateServerConfig(form)
    success('服务器配置已更新')
    await fetchServerConfig()
  } catch (err: any) {
    error(err.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// Test connection
async function handleTest() {
  testing.value = true
  testResult.value = null

  try {
    const result = await serverApi.testConnection({
      addr: form.addr,
      port: form.port,
      token: form.token || undefined,
    })

    if (result.success) {
      testResult.value = {
        success: true,
        message: `连接成功！延迟：${result.latency}ms`,
      }
    } else {
      testResult.value = {
        success: false,
        message: result.error || '连接失败',
      }
    }
  } catch (err: any) {
    testResult.value = {
      success: false,
      message: err.message || '连接测试失败',
    }
  } finally {
    testing.value = false
  }
}

// Switch server
async function handleSwitchServer() {
  const confirmed = await confirm(
    '切换服务器将覆盖当前配置，确定要继续吗？',
    '切换服务器',
    { type: 'warning' }
  )
  if (!confirmed) return

  const addr = await prompt('请输入新服务器地址', '切换服务器', {
    inputPlaceholder: 'frp.example.com',
  })
  if (!addr) return

  const portStr = await prompt('请输入服务器端口', '切换服务器', {
    inputPlaceholder: '7000',
    inputValue: '7000',
  })
  if (!portStr) return

  const port = parseInt(portStr)
  if (isNaN(port) || port < 1 || port > 65535) {
    error('请输入有效的端口号')
    return
  }

  try {
    await serverApi.switchServer({ addr, port })
    success('服务器切换成功')
    await fetchServerConfig()
  } catch (err: any) {
    error(err.message || '切换失败')
  }
}

// Unbind device
async function handleUnbind() {
  const confirmed = await confirm(
    '解绑设备后需要重新绑定才能使用，确定要继续吗？',
    '解绑设备',
    { type: 'error' }
  )
  if (!confirmed) return

  try {
    await serverApi.unbindDevice()
    success('设备已解绑')
  } catch (err: any) {
    error(err.message || '解绑失败')
  }
}

// Initialize
onMounted(() => {
  fetchServerConfig()
})
</script>

<style scoped>
.server-config {
  max-width: 800px;
  margin: 0 auto;
}

.config-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.config-section {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.config-section--danger {
  border-color: #FECACA;
}

.config-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid #E4E4E7;
}

.config-section--danger .config-section__header {
  background: #FEF2F2;
}

.config-section__title {
  font-size: 16px;
  font-weight: 600;
  color: #18181B;
  margin: 0 0 4px;
}

.config-section__description {
  font-size: 14px;
  color: #71717A;
  margin: 0;
}

.config-section__body {
  padding: 24px;
}

.server-info {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-item__label {
  font-size: 12px;
  font-weight: 500;
  color: #71717A;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.info-item__value {
  font-size: 14px;
  font-weight: 500;
  color: #18181B;
}

.form-help {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #A1A1AA;
}

.test-result {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  padding: 12px 16px;
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

.danger-actions {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.danger-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border: 1px solid #E4E4E7;
  border-radius: 6px;
}

.danger-action__title {
  font-size: 14px;
  font-weight: 600;
  color: #18181B;
  margin: 0 0 4px;
}

.danger-action__description {
  font-size: 12px;
  color: #71717A;
  margin: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .server-info {
    grid-template-columns: 1fr;
  }

  .danger-action {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  :deep(.el-form-item) {
    flex-direction: column;
  }

  :deep(.el-form-item__label) {
    text-align: left;
    margin-bottom: 8px;
  }
}
</style>