<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">设备管理</h2>
        <p class="page-description">管理客户端设备连接</p>
      </div>
    </div>

    <!-- Stats -->
    <el-row :gutter="16" class="mb-4">
      <el-col :span="8">
        <div class="stat-card">
          <div class="stat-label">在线设备</div>
          <div class="stat-value text-success">{{ deviceStats.online }}</div>
        </div>
      </el-col>
      <el-col :span="8">
        <div class="stat-card">
          <div class="stat-label">离线设备</div>
          <div class="stat-value">{{ deviceStats.offline }}</div>
        </div>
      </el-col>
      <el-col :span="8">
        <div class="stat-card">
          <div class="stat-label">总设备数</div>
          <div class="stat-value text-primary">{{ deviceStats.total }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="关键词">
          <el-input v-model="filters.keyword" placeholder="设备名称/ID" clearable @clear="handleSearch" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部" clearable @change="handleSearch">
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- Table -->
    <DataTable
      :data="devices"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      @page-change="fetchDevices"
      @size-change="fetchDevices"
    >
      <template #status="{ row }">
        <StatusBadge :status="row.status" dot />
      </template>

      <template #device_name="{ row }">
        <div>
          <div class="device-name">{{ row.device_name }}</div>
          <div class="device-id font-mono text-secondary">{{ row.device_id }}</div>
        </div>
      </template>

      <template #ip="{ row }">
        <code class="font-mono">{{ row.ip || '-' }}</code>
      </template>

      <template #last_seen_at="{ row }">
        {{ row.last_seen_at ? formatRelativeTime(row.last_seen_at) : '从未连接' }}
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #actions="{ row }">
        <el-button text type="danger" size="small" @click="handleRevoke(row)">撤销</el-button>
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { devicesApi } from '@/api/devices'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, formatRelativeTime, extractErrorMessage } from '@/utils/format'
import type { Device, TableColumn } from '@/types'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const devices = ref<Device[]>([])
const total = ref(0)
const deviceStats = ref({ online: 0, offline: 0, total: 0 })

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  keyword: '',
  status: '',
})

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'device_name', label: '设备名称', minWidth: 180, slot: 'device_name' },
  { prop: 'username', label: '用户', width: 120 },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'os', label: '操作系统', width: 120 },
  { prop: 'frpc_version', label: 'FRPC版本', width: 120 },
  { prop: 'ip', label: 'IP地址', width: 140, slot: 'ip' },
  { prop: 'last_seen_at', label: '最后在线', width: 150, slot: 'last_seen_at' },
  { prop: 'created_at', label: '创建时间', width: 160, slot: 'created_at' },
]

async function fetchDevices() {
  loading.value = true
  try {
    const res = await devicesApi.getList({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters,
    })
    if (res.code === 0) {
      devices.value = res.data.list
      total.value = res.data.total
    }
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}

async function fetchStats() {
  try {
    const res = await devicesApi.getStats()
    if (res.code === 0) {
      deviceStats.value = res.data
    }
  } catch {
    // Ignore
  }
}

function handleSearch() {
  pagination.page = 1
  fetchDevices()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  handleSearch()
}

async function handleRevoke(device: Device) {
  const confirmed = await confirm(`确定要撤销设备 ${device.device_name} 吗？该设备将被强制断开连接。`)
  if (!confirmed) return

  try {
    await devicesApi.revoke(device.id)
    success('设备已撤销')
    fetchDevices()
    fetchStats()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchDevices()
  fetchStats()
})
</script>

<style scoped>
.stat-card {
  background: var(--color-bg-primary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  text-align: center;
}

.stat-card .stat-label {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  margin-bottom: var(--spacing-xs);
}

.stat-card .stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-text-primary);
}

.device-name {
  font-weight: 500;
  color: var(--color-text-primary);
}

.device-id {
  font-size: 12px;
  margin-top: 2px;
}
</style>
