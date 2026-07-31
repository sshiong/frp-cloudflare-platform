<template>
  <div class="dashboard">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">仪表盘</h1>
        <p class="page-header__description">FRP 客户端状态概览</p>
      </div>
      <div class="page-header__actions">
        <el-button @click="refreshAll">
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stats-card">
        <div class="stats-card__header">
          <span class="stats-card__label">代理数量</span>
          <div class="stats-card__icon stats-card__icon--blue">
            <el-icon><Connection /></el-icon>
          </div>
        </div>
        <div class="stats-card__value">{{ frpcStore.status.proxyCount }}</div>
        <div class="stats-card__footer">已配置的代理总数</div>
      </div>

      <div class="stats-card">
        <div class="stats-card__header">
          <span class="stats-card__label">运行状态</span>
          <div :class="['stats-card__icon', frpcStore.isRunning ? 'stats-card__icon--green' : 'stats-card__icon--gray']">
            <el-icon><Monitor /></el-icon>
          </div>
        </div>
        <div class="stats-card__value">{{ frpcStore.isRunning ? '运行中' : '已停止' }}</div>
        <div class="stats-card__footer">FRPC 客户端状态</div>
      </div>

      <div class="stats-card">
        <div class="stats-card__header">
          <span class="stats-card__label">服务器连接</span>
          <div :class="['stats-card__icon', frpcStore.isConnected ? 'stats-card__icon--green' : 'stats-card__icon--red']">
            <el-icon><Connection /></el-icon>
          </div>
        </div>
        <div class="stats-card__value">{{ frpcStore.isConnected ? '已连接' : '未连接' }}</div>
        <div class="stats-card__footer">{{ frpcStore.status.serverAddr || '未配置' }}</div>
      </div>

      <div class="stats-card">
        <div class="stats-card__header">
          <span class="stats-card__label">运行时长</span>
          <div class="stats-card__icon stats-card__icon--purple">
            <el-icon><Timer /></el-icon>
          </div>
        </div>
        <div class="stats-card__value">{{ frpcStore.uptimeFormatted }}</div>
        <div class="stats-card__footer">{{ frpcStore.status.startTime ? `启动于 ${formatDate(frpcStore.status.startTime, 'time')}` : '-' }}</div>
      </div>
    </div>

    <!-- Main Content -->
    <div class="dashboard-content">
      <!-- FRPC Status Card -->
      <div class="dashboard-section">
        <FrpcStatusCard
          :status="frpcStore.status"
          :action-loading="frpcStore.actionLoading"
          @start="handleStart"
          @stop="handleStop"
          @restart="handleRestart"
        />
      </div>

      <!-- Quick Actions -->
      <div class="dashboard-section">
        <div class="section-card">
          <div class="section-card__header">
            <h3 class="section-card__title">快捷操作</h3>
          </div>
          <div class="section-card__body">
            <div class="quick-actions">
              <el-button type="primary" @click="router.push('/mappings/create')">
                <el-icon><Plus /></el-icon>
                <span>创建映射</span>
              </el-button>
              <el-button @click="router.push('/mappings')">
                <el-icon><List /></el-icon>
                <span>查看映射</span>
              </el-button>
              <el-button @click="router.push('/frpc/logs')">
                <el-icon><Document /></el-icon>
                <span>查看日志</span>
              </el-button>
              <el-button @click="router.push('/settings')">
                <el-icon><Setting /></el-icon>
                <span>系统设置</span>
              </el-button>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Mappings -->
      <div class="dashboard-section">
        <div class="section-card">
          <div class="section-card__header">
            <h3 class="section-card__title">最近映射</h3>
            <el-button text type="primary" @click="router.push('/mappings')">
              查看全部
            </el-button>
          </div>
          <div class="section-card__body">
            <el-table
              :data="recentMappings"
              style="width: 100%"
              :header-cell-style="{ background: '#F4F4F5', color: '#18181B', fontWeight: '600' }"
            >
              <el-table-column prop="name" label="名称" min-width="120" />
              <el-table-column prop="protocol" label="协议" width="80">
                <template #default="{ row }">
                  <el-tag :type="getProtocolTagType(row.protocol)" size="small">
                    {{ row.protocol.toUpperCase() }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="localPort" label="本地端口" width="100" />
              <el-table-column prop="remotePort" label="远程端口" width="100">
                <template #default="{ row }">
                  {{ row.remotePort || '-' }}
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                  <StatusBadge :type="getStatusType(row.status)" :label="getStatusLabel(row.status)" />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button text type="primary" size="small" @click="router.push(`/mappings/${row.id}/edit`)">
                    编辑
                  </el-button>
                </template>
              </el-table-column>
            </el-table>

            <div v-if="recentMappings.length === 0" class="empty-state">
              <el-icon class="empty-state__icon"><Connection /></el-icon>
              <h4 class="empty-state__title">暂无映射</h4>
              <p class="empty-state__description">创建您的第一个端口映射开始使用</p>
              <el-button type="primary" @click="router.push('/mappings/create')">
                创建映射
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Refresh,
  Connection,
  Monitor,
  Timer,
  Plus,
  List,
  Document,
  Setting,
} from '@element-plus/icons-vue'
import FrpcStatusCard from '@/components/FrpcStatusCard.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useFrpcStore } from '@/stores/frpc'
import * as mappingsApi from '@/api/mappings'
import type { Mapping, MappingStatus, Protocol } from '@/types'
import { formatDate, getStatusLabel, getStatusType } from '@/utils/format'

const router = useRouter()
const frpcStore = useFrpcStore()

const recentMappings = ref<Mapping[]>([])
const loading = ref(false)

// Fetch recent mappings
async function fetchRecentMappings() {
  loading.value = true
  try {
    const response = await mappingsApi.getMappings({
      page: 1,
      pageSize: 5,
    })
    recentMappings.value = response.items
  } catch (error) {
    console.error('Failed to fetch recent mappings:', error)
  } finally {
    loading.value = false
  }
}

// Refresh all data
async function refreshAll() {
  await Promise.all([
    frpcStore.fetchStatus(),
    fetchRecentMappings(),
  ])
}

// FRPC actions
async function handleStart() {
  await frpcStore.start()
}

async function handleStop() {
  await frpcStore.stop()
}

async function handleRestart() {
  await frpcStore.restart()
}

// Get protocol tag type
function getProtocolTagType(protocol: Protocol): 'primary' | 'success' | 'warning' | 'danger' | 'info' | undefined {
  const typeMap: Record<Protocol, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    tcp: '',
    udp: 'info',
    http: 'success',
    https: 'warning',
  }
  return typeMap[protocol] || 'primary'
}

// Initialize
onMounted(() => {
  refreshAll()
})
</script>

<style scoped>
.dashboard {
  max-width: 1200px;
  margin: 0 auto;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stats-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  padding: 20px;
  transition: all 0.2s ease;
}

.stats-card:hover {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1);
  transform: translateY(-1px);
}

.stats-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.stats-card__label {
  font-size: 12px;
  font-weight: 500;
  color: #71717A;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.stats-card__icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.stats-card__icon--blue {
  background: #DBEAFE;
  color: #2563EB;
}

.stats-card__icon--green {
  background: #ECFDF5;
  color: #16A34A;
}

.stats-card__icon--red {
  background: #FEF2F2;
  color: #DC2626;
}

.stats-card__icon--purple {
  background: #F3E8FF;
  color: #7C3AED;
}

.stats-card__icon--gray {
  background: #F4F4F5;
  color: #71717A;
}

.stats-card__value {
  font-size: 28px;
  font-weight: 700;
  color: #18181B;
  line-height: 1.2;
}

.stats-card__footer {
  margin-top: 8px;
  font-size: 12px;
  color: #71717A;
}

.dashboard-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.dashboard-section {
  width: 100%;
}

.section-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.section-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #E4E4E7;
}

.section-card__title {
  font-size: 14px;
  font-weight: 600;
  color: #18181B;
  margin: 0;
}

.section-card__body {
  padding: 20px;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.quick-actions .el-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
}

.empty-state__icon {
  font-size: 48px;
  color: #A1A1AA;
  margin-bottom: 16px;
}

.empty-state__title {
  font-size: 16px;
  font-weight: 600;
  color: #18181B;
  margin: 0 0 8px;
}

.empty-state__description {
  font-size: 14px;
  color: #71717A;
  margin: 0 0 24px;
  max-width: 400px;
}

/* Responsive */
@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .quick-actions {
    flex-direction: column;
  }

  .quick-actions .el-button {
    width: 100%;
    justify-content: center;
  }
}
</style>