<template>
  <div class="settings">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">系统设置</h1>
        <p class="page-header__description">配置客户端面板和 FRPC 参数</p>
      </div>
      <div class="page-header__actions">
        <el-button type="primary" :loading="saving" @click="handleSave">
          <el-icon><Check /></el-icon>
          <span>保存设置</span>
        </el-button>
      </div>
    </div>

    <div v-loading="loading" class="settings-content">
      <!-- LAN Access -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">内网访问控制</h3>
            <p class="settings-section__description">配置内网访问白名单</p>
          </div>
        </div>
        <div class="settings-section__body">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="IP 白名单">
              <el-select
                v-model="form.lanWhitelist"
                multiple
                filterable
                allow-create
                default-first-option
                placeholder="输入 IP 后回车添加"
                style="width: 100%"
              />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>允许访问的 IP 地址列表，留空表示允许所有</span>
              </div>
            </el-form-item>

            <el-form-item label="Host 白名单">
              <el-select
                v-model="form.hostWhitelist"
                multiple
                filterable
                allow-create
                default-first-option
                placeholder="输入 Host 后回车添加"
                style="width: 100%"
              />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>允许访问的 Host 列表，留空表示允许所有</span>
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- FRPC Settings -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">FRPC 设置</h3>
            <p class="settings-section__description">配置 FRPC 客户端行为</p>
          </div>
        </div>
        <div class="settings-section__body">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="自动启动">
              <el-switch v-model="form.autoStart" />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>系统启动时自动运行 FRPC</span>
              </div>
            </el-form-item>

            <el-form-item label="自动重连">
              <el-switch v-model="form.autoReconnect" />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>连接断开时自动尝试重连</span>
              </div>
            </el-form-item>

            <el-form-item v-if="form.autoReconnect" label="重连间隔">
              <el-input-number
                v-model="form.reconnectInterval"
                :min="1"
                :max="300"
                :step="1"
              />
              <span class="form-unit">秒</span>
            </el-form-item>

            <el-form-item v-if="form.autoReconnect" label="最大重试次数">
              <el-input-number
                v-model="form.maxRetries"
                :min="0"
                :max="100"
                :step="1"
              />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>设置为 0 表示无限重试</span>
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- Log Settings -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">日志设置</h3>
            <p class="settings-section__description">配置日志记录参数</p>
          </div>
        </div>
        <div class="settings-section__body">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="日志级别">
              <el-select v-model="form.logLevel" style="width: 200px">
                <el-option label="Trace" value="trace" />
                <el-option label="Debug" value="debug" />
                <el-option label="Info" value="info" />
                <el-option label="Warn" value="warn" />
                <el-option label="Error" value="error" />
              </el-select>
            </el-form-item>

            <el-form-item label="日志保留天数">
              <el-input-number
                v-model="form.logMaxDays"
                :min="1"
                :max="365"
                :step="1"
              />
              <span class="form-unit">天</span>
            </el-form-item>

            <el-form-item label="日志文件大小">
              <el-input-number
                v-model="form.logMaxSize"
                :min="1"
                :max="1024"
                :step="1"
              />
              <span class="form-unit">MB</span>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- Notification -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">通知设置</h3>
            <p class="settings-section__description">配置系统通知</p>
          </div>
        </div>
        <div class="settings-section__body">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="启用通知">
              <el-switch v-model="form.notificationEnabled" />
              <div class="form-help">
                <el-icon><InfoFilled /></el-icon>
                <span>接收系统状态变更通知</span>
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- Appearance -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">外观设置</h3>
            <p class="settings-section__description">自定义界面外观</p>
          </div>
        </div>
        <div class="settings-section__body">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="主题">
              <el-radio-group v-model="form.theme">
                <el-radio-button value="light">浅色</el-radio-button>
                <el-radio-button value="dark">深色</el-radio-button>
                <el-radio-button value="system">跟随系统</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="语言">
              <el-select v-model="form.language" style="width: 200px">
                <el-option label="简体中文" value="zh-CN" />
                <el-option label="English" value="en-US" />
              </el-select>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- About -->
      <div class="settings-section">
        <div class="settings-section__header">
          <div>
            <h3 class="settings-section__title">关于</h3>
            <p class="settings-section__description">系统信息</p>
          </div>
        </div>
        <div class="settings-section__body">
          <div class="about-info">
            <div class="about-item">
              <span class="about-item__label">版本</span>
              <span class="about-item__value">v1.0.0</span>
            </div>
            <div class="about-item">
              <span class="about-item__label">构建日期</span>
              <span class="about-item__value">2024-01-01</span>
            </div>
            <div class="about-item">
              <span class="about-item__label">FRPC 版本</span>
              <span class="about-item__value">{{ frpcVersion }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Check, InfoFilled } from '@element-plus/icons-vue'
import * as settingsApi from '@/api/settings'
import { useFrpcStore } from '@/stores/frpc'
import { useNotification } from '@/composables/useNotification'
import type { ClientSettings } from '@/types'

const frpcStore = useFrpcStore()
const { success, error } = useNotification()

// State
const loading = ref(false)
const saving = ref(false)
const frpcVersion = ref('-')

const form = reactive<ClientSettings>({
  id: 0,
  lanWhitelist: [],
  hostWhitelist: [],
  autoStart: false,
  autoReconnect: true,
  reconnectInterval: 30,
  maxRetries: 0,
  notificationEnabled: true,
  logLevel: 'info',
  logMaxSize: 100,
  logMaxDays: 7,
  theme: 'light',
  language: 'zh-CN',
})

// Fetch settings
async function fetchSettings() {
  loading.value = true
  try {
    const settings = await settingsApi.getSettings()
    Object.assign(form, settings)
  } catch (err) {
    console.error('Failed to fetch settings:', err)
  } finally {
    loading.value = false
  }
}

// Fetch FRPC version
async function fetchFrpcVersion() {
  frpcVersion.value = await frpcStore.fetchVersion()
}

// Save settings
async function handleSave() {
  saving.value = true
  try {
    await settingsApi.updateSettings(form)
    success('设置已保存')
  } catch (err: any) {
    error(err.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// Initialize
onMounted(() => {
  fetchSettings()
  fetchFrpcVersion()
})
</script>

<style scoped>
.settings {
  max-width: 800px;
  margin: 0 auto;
}

.settings-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.settings-section {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.settings-section__header {
  padding: 20px 24px;
  border-bottom: 1px solid #E4E4E7;
}

.settings-section__title {
  font-size: 16px;
  font-weight: 600;
  color: #18181B;
  margin: 0 0 4px;
}

.settings-section__description {
  font-size: 14px;
  color: #71717A;
  margin: 0;
}

.settings-section__body {
  padding: 24px;
}

.form-help {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #A1A1AA;
}

.form-unit {
  margin-left: 8px;
  font-size: 14px;
  color: #71717A;
}

.about-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.about-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #F4F4F5;
}

.about-item:last-child {
  border-bottom: none;
}

.about-item__label {
  font-size: 14px;
  color: #71717A;
}

.about-item__value {
  font-size: 14px;
  font-weight: 500;
  color: #18181B;
}

/* Responsive */
@media (max-width: 768px) {
  .settings-section__body {
    padding: 16px;
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