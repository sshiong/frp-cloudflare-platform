<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">操作历史</h2>
        <p class="page-description">查看系统操作记录</p>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="操作类型">
          <el-select v-model="filters.type" placeholder="全部" clearable @change="handleSearch">
            <el-option label="创建映射" value="mapping_create" />
            <el-option label="更新映射" value="mapping_update" />
            <el-option label="删除映射" value="mapping_delete" />
            <el-option label="添加域名" value="domain_create" />
            <el-option label="验证域名" value="domain_verify" />
            <el-option label="签发证书" value="certificate_issue" />
            <el-option label="续签证书" value="certificate_renew" />
            <el-option label="上传令牌" value="token_upload" />
            <el-option label="验证令牌" value="token_verify" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部" clearable @change="handleSearch">
            <el-option label="待处理" value="pending" />
            <el-option label="执行中" value="running" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
            <el-option label="已取消" value="cancelled" />
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
      :data="operations"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :actions-width="200"
      @page-change="fetchOperations"
      @size-change="fetchOperations"
    >
      <template #type="{ row }">
        <el-tag size="small" effect="plain">{{ formatOperationType(row.type) }}</el-tag>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="row.status" dot />
      </template>

      <template #detail="{ row }">
        <el-tooltip v-if="row.detail" :content="row.detail" placement="top" :show-after="500">
          <span class="detail-text">{{ row.detail }}</span>
        </el-tooltip>
        <span v-else class="text-secondary">-</span>
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #completed_at="{ row }">
        {{ row.completed_at ? formatDate(row.completed_at) : '-' }}
      </template>

      <template #actions="{ row }">
        <el-button
          v-if="row.status === 'pending' || row.status === 'running'"
          text
          type="warning"
          size="small"
          @click="handleCancel(row as Operation)"
        >
          取消
        </el-button>
        <el-button
          v-if="row.status === 'failed'"
          text
          type="primary"
          size="small"
          @click="handleRetry(row as Operation)"
        >
          重试
        </el-button>
        <el-button
          v-if="row.status === 'running'"
          text
          type="danger"
          size="small"
          @click="handleForceComplete(row as Operation)"
        >
          强制完成
        </el-button>
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { adminApi } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, formatOperationType, extractErrorMessage } from '@/utils/format'
import type { Operation, TableColumn } from '@/types'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const operations = ref<Operation[]>([])
const total = ref(0)

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  type: '',
  status: '',
})

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'type', label: '操作类型', width: 120, slot: 'type' },
  { prop: 'target', label: '目标', minWidth: 180 },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'username', label: '操作人', width: 100 },
  { prop: 'detail', label: '详情', minWidth: 200, slot: 'detail' },
  { prop: 'created_at', label: '创建时间', width: 160, slot: 'created_at' },
  { prop: 'completed_at', label: '完成时间', width: 160, slot: 'completed_at' },
]

async function fetchOperations() {
  loading.value = true
  try {
    const res = await adminApi.getOperations({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters,
    })
    if (res.code === 0) {
      operations.value = res.data.list
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
  fetchOperations()
}

function handleReset() {
  filters.type = ''
  filters.status = ''
  handleSearch()
}

async function handleCancel(operation: Operation) {
  const confirmed = await confirm('确定要取消此操作吗？')
  if (!confirmed) return

  try {
    await adminApi.cancelOperation(operation.id)
    success('操作已取消')
    fetchOperations()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function handleRetry(operation: Operation) {
  try {
    await adminApi.retryOperation(operation.id)
    success('操作已重新提交')
    fetchOperations()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function handleForceComplete(operation: Operation) {
  const confirmed = await confirm('确定要强制完成此操作吗？这可能导致数据不一致。', '强制完成', 'warning')
  if (!confirmed) return

  try {
    await adminApi.forceCompleteOperation(operation.id)
    success('操作已强制完成')
    fetchOperations()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchOperations()
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
