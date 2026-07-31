<template>
  <div class="mapping-list">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">端口映射</h1>
        <p class="page-header__description">管理您的端口映射配置</p>
      </div>
      <div class="page-header__actions">
        <el-button type="primary" @click="router.push('/mappings/create')">
          <el-icon><Plus /></el-icon>
          <span>创建映射</span>
        </el-button>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索映射名称..."
        clearable
        style="width: 240px"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <el-select
        v-model="filterProtocol"
        placeholder="协议类型"
        clearable
        style="width: 120px"
        @change="handleFilter"
      >
        <el-option label="TCP" value="tcp" />
        <el-option label="UDP" value="udp" />
        <el-option label="HTTP" value="http" />
        <el-option label="HTTPS" value="https" />
      </el-select>

      <el-select
        v-model="filterStatus"
        placeholder="状态"
        clearable
        style="width: 120px"
        @change="handleFilter"
      >
        <el-option label="运行中" value="active" />
        <el-option label="已停止" value="inactive" />
        <el-option label="错误" value="error" />
      </el-select>

      <el-button @click="fetchMappings">
        <el-icon><Refresh /></el-icon>
        <span>刷新</span>
      </el-button>
    </div>

    <!-- Table -->
    <div class="table-card">
      <el-table
        v-loading="loading"
        :data="mappings"
        style="width: 100%"
        :header-cell-style="{ background: '#F4F4F5', color: '#18181B', fontWeight: '600' }"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="50" />

        <el-table-column prop="name" label="名称" min-width="150">
          <template #default="{ row }">
            <div class="mapping-name">
              <span class="mapping-name__text">{{ row.name }}</span>
              <span class="mapping-name__id">#{{ row.id }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="protocol" label="协议" width="80">
          <template #default="{ row }">
            <el-tag :type="getProtocolTagType(row.protocol)" size="small">
              {{ row.protocol.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="本地地址" min-width="150">
          <template #default="{ row }">
            <span class="address">{{ row.localIp }}:{{ row.localPort }}</span>
          </template>
        </el-table-column>

        <el-table-column label="远程端口/域名" min-width="150">
          <template #default="{ row }">
            <span v-if="row.remotePort">{{ row.remotePort }}</span>
            <span v-else-if="row.customDomains">{{ row.customDomains.join(', ') }}</span>
            <span v-else-if="row.subdomain">{{ row.subdomain }}</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <StatusBadge :type="getStatusType(row.status)" :label="getStatusLabel(row.status)" />
          </template>
        </el-table-column>

        <el-table-column label="启用" width="80">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="handleToggleEnabled(row)"
            />
          </template>
        </el-table-column>

        <el-table-column label="流量" width="120">
          <template #default="{ row }">
            <div class="traffic-info">
              <span class="traffic-in">↓ {{ formatBytes(row.trafficIn || 0) }}</span>
              <span class="traffic-out">↑ {{ formatBytes(row.trafficOut || 0) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="router.push(`/mappings/${row.id}/edit`)">
              编辑
            </el-button>
            <el-button text type="danger" size="small" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="table-footer">
        <div class="table-footer__info">
          <span>已选择 {{ selectedIds.length }} 项</span>
          <el-button
            v-if="selectedIds.length > 0"
            type="danger"
            text
            size="small"
            @click="handleBatchDelete"
          >
            批量删除
          </el-button>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="fetchMappings"
          @current-change="fetchMappings"
        />
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="!loading && mappings.length === 0" class="empty-state">
      <el-icon class="empty-state__icon"><Connection /></el-icon>
      <h3 class="empty-state__title">暂无映射</h3>
      <p class="empty-state__description">创建您的第一个端口映射开始使用</p>
      <el-button type="primary" @click="router.push('/mappings/create')">
        创建映射
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Search, Refresh, Connection } from '@element-plus/icons-vue'
import StatusBadge from '@/components/StatusBadge.vue'
import * as mappingsApi from '@/api/mappings'
import { useNotification } from '@/composables/useNotification'
import type { Mapping, MappingStatus, Protocol } from '@/types'
import { formatBytes, getStatusLabel, getStatusType } from '@/utils/format'

const router = useRouter()
const { confirmDelete, success, error } = useNotification()

// State
const mappings = ref<Mapping[]>([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchKeyword = ref('')
const filterProtocol = ref('')
const filterStatus = ref('')
const selectedIds = ref<number[]>([])

// Fetch mappings
async function fetchMappings() {
  loading.value = true
  try {
    const response = await mappingsApi.getMappings({
      page: currentPage.value,
      pageSize: pageSize.value,
      protocol: filterProtocol.value || undefined,
      status: filterStatus.value || undefined,
      keyword: searchKeyword.value || undefined,
    })
    mappings.value = response.items
    total.value = response.total
  } catch (err) {
    console.error('Failed to fetch mappings:', err)
  } finally {
    loading.value = false
  }
}

// Handle search
function handleSearch() {
  currentPage.value = 1
  fetchMappings()
}

// Handle filter
function handleFilter() {
  currentPage.value = 1
  fetchMappings()
}

// Handle selection change
function handleSelectionChange(selection: Mapping[]) {
  selectedIds.value = selection.map((item) => item.id)
}

// Handle toggle enabled
async function handleToggleEnabled(mapping: Mapping) {
  try {
    await mappingsApi.toggleMapping(mapping.id, mapping.enabled)
    success(`映射已${mapping.enabled ? '启用' : '禁用'}`)
  } catch (err) {
    // Revert on error
    mapping.enabled = !mapping.enabled
    error('操作失败')
  }
}

// Handle delete
async function handleDelete(mapping: Mapping) {
  const confirmed = await confirmDelete(`映射 "${mapping.name}"`)
  if (!confirmed) return

  try {
    await mappingsApi.deleteMapping(mapping.id)
    success('删除成功')
    fetchMappings()
  } catch (err) {
    error('删除失败')
  }
}

// Handle batch delete
async function handleBatchDelete() {
  const confirmed = await confirmDelete(`选中的 ${selectedIds.value.length} 个映射`)
  if (!confirmed) return

  try {
    await mappingsApi.batchDeleteMappings(selectedIds.value)
    success('批量删除成功')
    selectedIds.value = []
    fetchMappings()
  } catch (err) {
    error('批量删除失败')
  }
}

// Get protocol tag type
function getProtocolTagType(protocol: Protocol): '' | 'success' | 'warning' | 'danger' | 'info' {
  const typeMap: Record<Protocol, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    tcp: '',
    udp: 'info',
    http: 'success',
    https: 'warning',
  }
  return typeMap[protocol] || ''
}

// Initialize
onMounted(() => {
  fetchMappings()
})
</script>

<style scoped>
.mapping-list {
  max-width: 1200px;
  margin: 0 auto;
}

.filters {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.table-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
}

.mapping-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mapping-name__text {
  font-weight: 500;
  color: #18181B;
}

.mapping-name__id {
  font-size: 12px;
  color: #A1A1AA;
}

.address {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
  font-size: 13px;
  color: #3F3F46;
}

.traffic-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}

.traffic-in {
  color: #16A34A;
}

.traffic-out {
  color: #2563EB;
}

.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-top: 1px solid #E4E4E7;
}

.table-footer__info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
  color: #71717A;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 24px;
  text-align: center;
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
}

.empty-state__icon {
  font-size: 64px;
  color: #A1A1AA;
  margin-bottom: 24px;
}

.empty-state__title {
  font-size: 18px;
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
@media (max-width: 768px) {
  .filters {
    flex-direction: column;
    align-items: stretch;
  }

  .filters .el-input,
  .filters .el-select {
    width: 100% !important;
  }

  .table-footer {
    flex-direction: column;
    gap: 16px;
  }
}
</style>