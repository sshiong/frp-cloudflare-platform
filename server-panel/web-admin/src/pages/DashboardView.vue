<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">仪表盘</h2>
        <p class="page-description">系统概览和关键指标</p>
      </div>
      <el-button :icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
    </div>

    <!-- Stats Cards -->
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="12" :sm="6" v-for="stat in statsCards" :key="stat.title">
        <div class="stat-card hover-card">
          <div class="stat-icon" :style="{ background: stat.bgColor }">
            <el-icon :size="24" :color="stat.color">
              <component :is="stat.icon" />
            </el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stat.value }}</div>
            <div class="stat-label">{{ stat.title }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Middle Row -->
    <el-row :gutter="16" class="content-row">
      <!-- Online Clients -->
      <el-col :xs="24" :sm="12" :lg="8">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">客户端状态</h3>
          </div>
          <div class="client-stats">
            <div class="client-stat-item">
              <div class="client-stat-circle online">
                <span>{{ stats?.online_clients || 0 }}</span>
              </div>
              <div class="client-stat-label">在线</div>
            </div>
            <div class="client-stat-item">
              <div class="client-stat-circle offline">
                <span>{{ stats?.offline_clients || 0 }}</span>
              </div>
              <div class="client-stat-label">离线</div>
            </div>
            <div class="client-stat-item">
              <div class="client-stat-circle total">
                <span>{{ (stats?.online_clients || 0) + (stats?.offline_clients || 0) }}</span>
              </div>
              <div class="client-stat-label">总计</div>
            </div>
          </div>
        </div>
      </el-col>

      <!-- Quick Stats -->
      <el-col :xs="24" :sm="12" :lg="8">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">运行状态</h3>
          </div>
          <div class="quick-stats">
            <div class="quick-stat-item">
              <span class="quick-stat-label">活跃映射</span>
              <span class="quick-stat-value text-success">{{ stats?.active_mappings || 0 }}</span>
            </div>
            <div class="quick-stat-item">
              <span class="quick-stat-label">有效证书</span>
              <span class="quick-stat-value text-primary">{{ stats?.valid_certificates || 0 }}</span>
            </div>
            <div class="quick-stat-item">
              <span class="quick-stat-label">总用户数</span>
              <span class="quick-stat-value">{{ stats?.total_users || 0 }}</span>
            </div>
            <div class="quick-stat-item">
              <span class="quick-stat-label">总域名数</span>
              <span class="quick-stat-value">{{ stats?.total_domains || 0 }}</span>
            </div>
          </div>
        </div>
      </el-col>

      <!-- System Health -->
      <el-col :xs="24" :sm="24" :lg="8">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">系统健康</h3>
          </div>
          <div class="health-items">
            <div class="health-item">
              <div class="health-info">
                <el-icon :size="16"><Monitor /></el-icon>
                <span>FRPS 服务</span>
              </div>
              <el-tag type="success" size="small" effect="light">运行中</el-tag>
            </div>
            <div class="health-item">
              <div class="health-info">
                <el-icon :size="16"><Connection /></el-icon>
                <span>路由器</span>
              </div>
              <el-tag type="success" size="small" effect="light">正常</el-tag>
            </div>
            <div class="health-item">
              <div class="health-info">
                <el-icon :size="16"><Coin /></el-icon>
                <span>数据库</span>
              </div>
              <el-tag type="success" size="small" effect="light">正常</el-tag>
            </div>
            <div class="health-item">
              <div class="health-info">
                <el-icon :size="16"><Timer /></el-icon>
                <span>运行时间</span>
              </div>
              <span class="health-value">{{ systemInfo?.uptime || '-' }}</span>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Recent Operations -->
    <div class="card">
      <div class="card-header">
        <h3 class="card-title">最近操作</h3>
        <el-button text type="primary" size="small" @click="$router.push('/operations')">
          查看全部
          <el-icon class="el-icon--right"><ArrowRight /></el-icon>
        </el-button>
      </div>
      <el-table :data="recentOperations" style="width: 100%" size="default" stripe>
        <el-table-column prop="type" label="操作类型" min-width="120">
          <template #default="{ row }">
            {{ formatOperationType(row.type) }}
          </template>
        </el-table-column>
        <el-table-column prop="target" label="目标" min-width="150" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <StatusBadge :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="created_at" label="时间" width="160">
          <template #default="{ row }">
            {{ formatRelativeTime(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Refresh, ArrowRight } from '@element-plus/icons-vue'
import { adminApi } from '@/api/admin'
import StatusBadge from '@/components/StatusBadge.vue'
import { formatOperationType, formatRelativeTime } from '@/utils/format'
import type { DashboardStats, SystemInfo, Operation } from '@/types'

const loading = ref(false)
const stats = ref<DashboardStats | null>(null)
const systemInfo = ref<SystemInfo | null>(null)
const recentOperations = ref<Operation[]>([])

const statsCards = computed(() => [
  {
    title: '总用户数',
    value: stats.value?.total_users || 0,
    icon: 'User',
    color: '#2563EB',
    bgColor: '#EFF6FF',
  },
  {
    title: '在线设备',
    value: stats.value?.online_clients || 0,
    icon: 'Cpu',
    color: '#16A34A',
    bgColor: '#F0FDF4',
  },
  {
    title: '活跃映射',
    value: stats.value?.active_mappings || 0,
    icon: 'Connection',
    color: '#8B5CF6',
    bgColor: '#F5F3FF',
  },
  {
    title: '域名总数',
    value: stats.value?.total_domains || 0,
    icon: 'Link',
    color: '#F59E0B',
    bgColor: '#FFFBEB',
  },
])

async function fetchData() {
  loading.value = true
  try {
    const [statsRes, sysRes] = await Promise.allSettled([
      adminApi.getDashboardStats(),
      adminApi.getSystemInfo(),
    ])

    if (statsRes.status === 'fulfilled' && statsRes.value.code === 0) {
      stats.value = statsRes.value.data
      recentOperations.value = statsRes.value.data.recent_operations || []
    }

    if (sysRes.status === 'fulfilled' && sysRes.value.code === 0) {
      systemInfo.value = sysRes.value.data
    }
  } catch {
    // Errors handled by interceptors
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.stats-row {
  margin-bottom: var(--spacing-lg);
}

.stat-card {
  background: var(--color-bg-primary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-md);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1;
}

.stat-label {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  margin-top: 4px;
}

.content-row {
  margin-bottom: var(--spacing-lg);
}

.content-row .card {
  height: 100%;
}

/* Client Stats */
.client-stats {
  display: flex;
  justify-content: space-around;
  padding: var(--spacing-md) 0;
}

.client-stat-item {
  text-align: center;
}

.client-stat-circle {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto var(--spacing-sm);
  font-size: 1.25rem;
  font-weight: 700;
}

.client-stat-circle.online {
  background: var(--color-success-bg);
  color: var(--color-success);
  border: 2px solid var(--color-success);
}

.client-stat-circle.offline {
  background: var(--color-bg-secondary);
  color: var(--color-text-secondary);
  border: 2px solid var(--color-border);
}

.client-stat-circle.total {
  background: var(--color-primary-bg);
  color: var(--color-primary);
  border: 2px solid var(--color-primary);
}

.client-stat-label {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  font-weight: 500;
}

/* Quick Stats */
.quick-stats {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.quick-stat-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--color-border-light);
}

.quick-stat-item:last-child {
  border-bottom: none;
}

.quick-stat-label {
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.quick-stat-value {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

/* Health Items */
.health-items {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.health-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--color-border-light);
}

.health-item:last-child {
  border-bottom: none;
}

.health-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: 0.875rem;
  color: var(--color-text-primary);
}

.health-value {
  font-size: 0.875rem;
  color: var(--color-text-secondary);
  font-family: var(--font-mono);
}
</style>
