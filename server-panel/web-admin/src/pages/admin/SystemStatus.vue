<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">系统状态</h2>
        <p class="page-description">查看系统运行状态和信息</p>
      </div>
      <el-button :icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
    </div>

    <el-row :gutter="16">
      <!-- Server Info -->
      <el-col :xs="24" :lg="12">
        <div class="card mb-4">
          <div class="card-header">
            <h3 class="card-title">服务器信息</h3>
          </div>
          <div class="info-list">
            <div class="info-item">
              <span class="info-label">系统版本</span>
              <span class="info-value">{{ systemInfo?.version || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">运行时间</span>
              <span class="info-value">{{ systemInfo?.uptime || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">FRPS 版本</span>
              <span class="info-value">{{ systemInfo?.frps_version || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">FRPS 状态</span>
              <span class="info-value">
                <el-tag :type="systemInfo?.frps_status === 'running' ? 'success' : 'danger'" size="small">
                  {{ systemInfo?.frps_status === 'running' ? '运行中' : '已停止' }}
                </el-tag>
              </span>
            </div>
            <div class="info-item">
              <span class="info-label">路由器状态</span>
              <span class="info-value">
                <el-tag :type="systemInfo?.router_status === 'running' ? 'success' : 'danger'" size="small">
                  {{ systemInfo?.router_status === 'running' ? '运行中' : '已停止' }}
                </el-tag>
              </span>
            </div>
          </div>
        </div>
      </el-col>

      <!-- Port Info -->
      <el-col :xs="24" :lg="12">
        <div class="card mb-4">
          <div class="card-header">
            <h3 class="card-title">端口配置</h3>
          </div>
          <div class="info-list">
            <div class="info-item">
              <span class="info-label">服务端口</span>
              <span class="info-value font-mono">{{ systemInfo?.server_port || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">管理端口</span>
              <span class="info-value font-mono">{{ systemInfo?.dashboard_port || '-' }}</span>
            </div>
          </div>
        </div>
      </el-col>

      <!-- Database Info -->
      <el-col :xs="24" :lg="12">
        <div class="card mb-4">
          <div class="card-header">
            <h3 class="card-title">数据库信息</h3>
          </div>
          <div class="info-list">
            <div class="info-item">
              <span class="info-label">数据库类型</span>
              <span class="info-value">{{ systemInfo?.database_type || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">数据库大小</span>
              <span class="info-value">{{ systemInfo?.database_size || '-' }}</span>
            </div>
          </div>
        </div>
      </el-col>

      <!-- Statistics -->
      <el-col :xs="24" :lg="12">
        <div class="card mb-4">
          <div class="card-header">
            <h3 class="card-title">数据统计</h3>
          </div>
          <div class="info-list">
            <div class="info-item">
              <span class="info-label">总用户数</span>
              <span class="info-value">{{ systemInfo?.total_users || 0 }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">总设备数</span>
              <span class="info-value">{{ systemInfo?.total_devices || 0 }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">总映射数</span>
              <span class="info-value">{{ systemInfo?.total_mappings || 0 }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">总域名数</span>
              <span class="info-value">{{ systemInfo?.total_domains || 0 }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">在线客户端</span>
              <span class="info-value text-success font-bold">{{ systemInfo?.online_clients || 0 }}</span>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { adminApi } from '@/api/admin'
import type { SystemInfo } from '@/types'

const loading = ref(false)
const systemInfo = ref<SystemInfo | null>(null)

async function fetchData() {
  loading.value = true
  try {
    const res = await adminApi.getSystemInfo()
    if (res.code === 0) {
      systemInfo.value = res.data
    }
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.info-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.info-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--color-border-light);
}

.info-item:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.info-value {
  font-size: 0.875rem;
  color: var(--color-text-primary);
  font-weight: 500;
}
</style>
