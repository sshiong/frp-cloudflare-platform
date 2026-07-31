<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">审计日志</h2>
        <p class="page-description">查看系统操作审计记录</p>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="操作类型">
          <el-select v-model="filters.action" placeholder="全部" clearable @change="handleSearch">
            <el-option label="登录" value="login" />
            <el-option label="创建" value="create" />
            <el-option label="更新" value="update" />
            <el-option label="删除" value="delete" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户">
          <el-input v-model="filters.username" placeholder="搜索用户名" clearable @clear="handleSearch" />
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="filters.dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            @change="handleSearch"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- Table -->
    <DataTable
      :data="logs"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :show-index="true"
      @page-change="fetchLogs"
      @size-change="fetchLogs"
    >
      <template #action="{ row }">
        <el-tag size="small" effect="plain">{{ row.action }}</el-tag>
      </template>

      <template #detail="{ row }">
        <el-tooltip v-if="row.detail" :content="row.detail" placement="top" :show-after="500">
          <span class="detail-text">{{ row.detail }}</span>
        </el-tooltip>
        <span v-else class="text-secondary">-</span>
      </template>

      <template #ip="{ row }">
        <code v-if="row.ip" class="font-mono">{{ row.ip }}</code>
        <span v-else class="text-secondary">-</span>
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { adminApi } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { formatDate } from '@/utils/format'
import type { AuditLog, TableColumn } from '@/types'

const loading = ref(false)
const logs = ref<AuditLog[]>([])
const total = ref(0)

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  action: '',
  username: '',
  dateRange: null as string[] | null,
})

const columns: TableColumn[] = [
  { prop: 'username', label: '用户', width: 120 },
  { prop: 'action', label: '操作', width: 120, slot: 'action' },
  { prop: 'target_type', label: '目标类型', width: 120 },
  { prop: 'target_id', label: '目标ID', width: 100 },
  { prop: 'detail', label: '详情', minWidth: 200, slot: 'detail' },
  { prop: 'ip', label: 'IP地址', width: 140, slot: 'ip' },
  { prop: 'created_at', label: '时间', width: 180, slot: 'created_at' },
]

async function fetchLogs() {
  loading.value = true
  try {
    const params: any = {
      page: pagination.page,
      page_size: pagination.pageSize,
    }
    if (filters.action) params.action = filters.action
    if (filters.username) params.user_id = filters.username
    if (filters.dateRange) {
      params.start_date = filters.dateRange[0]
      params.end_date = filters.dateRange[1]
    }

    const res = await adminApi.getAuditLogs(params)
    if (res.code === 0) {
      logs.value = res.data.list
      total.value = res.data.total
    }
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  fetchLogs()
}

function handleReset() {
  filters.action = ''
  filters.username = ''
  filters.dateRange = null
  handleSearch()
}

onMounted(() => {
  fetchLogs()
})
</script>

<style scoped>
.detail-text {
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  color: var(--color-text-secondary);
}

.detail-text:hover {
  color: var(--color-primary);
}
</style>
