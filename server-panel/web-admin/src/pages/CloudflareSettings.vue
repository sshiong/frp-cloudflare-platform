<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">Cloudflare 配置</h2>
        <p class="page-description">管理 Cloudflare API 令牌和域名区域</p>
      </div>
    </div>

    <!-- Token Status -->
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">API 令牌状态</h3>
        <el-button
          v-if="tokenStatus.status !== 'not_configured'"
          text
          type="primary"
          size="small"
          @click="handleVerifyToken"
          :loading="verifyLoading"
        >
          验证令牌
        </el-button>
      </div>

      <div class="token-status-content">
        <div class="token-status-info">
          <StatusBadge :status="tokenStatus.status" dot size="large" />
          <div v-if="tokenStatus.account_id" class="token-detail">
            <span class="detail-label">账户ID:</span>
            <code class="font-mono">{{ tokenStatus.account_id }}</code>
          </div>
          <div v-if="tokenStatus.zone_count !== undefined" class="token-detail">
            <span class="detail-label">区域数量:</span>
            <span>{{ tokenStatus.zone_count }}</span>
          </div>
          <div v-if="tokenStatus.verified_at" class="token-detail">
            <span class="detail-label">验证时间:</span>
            <span>{{ formatDate(tokenStatus.verified_at) }}</span>
          </div>
        </div>

        <div class="token-actions">
          <el-button type="primary" @click="openUploadDialog">
            {{ tokenStatus.status === 'not_configured' ? '上传令牌' : '替换令牌' }}
          </el-button>
          <el-button
            v-if="tokenStatus.status !== 'not_configured'"
            type="danger"
            plain
            @click="handleClearToken"
            :loading="clearLoading"
          >
            清除令牌
          </el-button>
        </div>
      </div>
    </div>

    <!-- Zone List -->
    <div class="card">
      <div class="card-header">
        <h3 class="card-title">域名区域列表</h3>
        <el-button text type="primary" size="small" @click="fetchZones" :loading="zonesLoading">
          刷新
        </el-button>
      </div>

      <el-table :data="zones" style="width: 100%" v-loading="zonesLoading" stripe>
        <el-table-column prop="name" label="域名" min-width="200">
          <template #default="{ row }">
            <div class="zone-name">
              <el-icon :size="16" color="#F58000"><Cloudy /></el-icon>
              <span>{{ row.name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="id" label="Zone ID" min-width="250">
          <template #default="{ row }">
            <code class="font-mono text-secondary">{{ row.id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'warning'" size="small">
              {{ row.status === 'active' ? '活跃' : row.status }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="zones.length === 0 && !zonesLoading" class="empty-state">
        <el-icon :size="48"><Cloudy /></el-icon>
        <p>请先配置 Cloudflare API 令牌</p>
      </div>
    </div>

    <!-- Upload Dialog -->
    <el-dialog v-model="uploadDialogVisible" title="上传 API 令牌" width="500px" destroy-on-close>
      <el-form ref="tokenFormRef" :model="tokenForm" :rules="tokenRules" label-width="100px">
        <el-form-item label="API 令牌" prop="api_token">
          <el-input
            v-model="tokenForm.api_token"
            type="password"
            show-password
            placeholder="请输入 Cloudflare API 令牌"
          />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            请在 Cloudflare 控制面板创建具有 Zone:DNS:Edit 和 Zone:Zone:Read 权限的 API 令牌。
          </template>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="uploadDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploadLoading" @click="handleUploadToken">上传</el-button>
      </template>
    </el-dialog>

    <!-- Clear Confirmation Dialog -->
    <ConfirmDialog
      v-model="clearConfirmVisible"
      title="清除 API 令牌"
      message="确定要清除 Cloudflare API 令牌吗？清除后将无法使用 Cloudflare 相关功能。此操作不可撤销。"
      confirm-text="确认清除"
      confirm-type="danger"
      icon="Warning"
      icon-type="warning"
      :loading="clearLoading"
      :countdown="5"
      @confirm="confirmClearToken"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { cloudflareApi } from '@/api/cloudflare'
import StatusBadge from '@/components/StatusBadge.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, extractErrorMessage } from '@/utils/format'
import type { CloudflareToken, CloudflareZone } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'

const { success, error, confirm } = useNotification()

const verifyLoading = ref(false)
const uploadLoading = ref(false)
const clearLoading = ref(false)
const zonesLoading = ref(false)
const uploadDialogVisible = ref(false)
const clearConfirmVisible = ref(false)

const tokenStatus = ref<CloudflareToken>({ status: 'not_configured' })
const zones = ref<CloudflareZone[]>([])

const tokenFormRef = ref<FormInstance>()
const tokenForm = reactive({
  api_token: '',
})

const tokenRules: FormRules = {
  api_token: [{ required: true, message: '请输入 API 令牌', trigger: 'blur' }],
}

async function fetchTokenStatus() {
  try {
    const res = await cloudflareApi.getTokenStatus()
    if (res.code === 0) {
      tokenStatus.value = res.data
    }
  } catch {
    // Error handled by interceptor
  }
}

async function fetchZones() {
  zonesLoading.value = true
  try {
    const res = await cloudflareApi.getZones()
    if (res.code === 0) {
      zones.value = res.data
    }
  } catch {
    // Error handled by interceptor
  } finally {
    zonesLoading.value = false
  }
}

function openUploadDialog() {
  tokenForm.api_token = ''
  uploadDialogVisible.value = true
}

async function handleUploadToken() {
  if (!tokenFormRef.value) return
  await tokenFormRef.value.validate(async (valid) => {
    if (!valid) return

    uploadLoading.value = true
    try {
      await cloudflareApi.uploadToken({ api_token: tokenForm.api_token })
      success('令牌上传成功')
      uploadDialogVisible.value = false
      fetchTokenStatus()
      fetchZones()
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      uploadLoading.value = false
    }
  })
}

async function handleVerifyToken() {
  verifyLoading.value = true
  try {
    const res = await cloudflareApi.verifyToken()
    if (res.code === 0 && res.data.valid) {
      success('令牌验证成功')
    } else {
      error('令牌验证失败')
    }
    fetchTokenStatus()
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    verifyLoading.value = false
  }
}

function handleClearToken() {
  clearConfirmVisible.value = true
}

async function confirmClearToken() {
  clearLoading.value = true
  try {
    await cloudflareApi.clearToken()
    success('令牌已清除')
    clearConfirmVisible.value = false
    tokenStatus.value = { status: 'not_configured' }
    zones.value = []
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    clearLoading.value = false
  }
}

onMounted(() => {
  fetchTokenStatus()
  fetchZones()
})
</script>

<style scoped>
.token-status-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--spacing-lg);
  flex-wrap: wrap;
}

.token-status-info {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.token-detail {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: 0.875rem;
}

.detail-label {
  color: var(--color-text-secondary);
  min-width: 80px;
}

.token-actions {
  display: flex;
  gap: var(--spacing-sm);
}

.zone-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}
</style>
