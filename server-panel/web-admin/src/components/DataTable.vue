<template>
  <div class="data-table-wrapper">
    <!-- Table Header / Toolbar -->
    <div v-if="$slots.toolbar" class="table-toolbar">
      <slot name="toolbar" />
    </div>

    <!-- Table -->
    <el-table
      v-loading="loading"
      :data="data"
      :border="border"
      :stripe="stripe"
      :row-key="rowKey"
      :max-height="maxHeight"
      :highlight-current-row="highlightCurrentRow"
      :default-sort="defaultSort"
      style="width: 100%"
      @selection-change="handleSelectionChange"
      @sort-change="handleSortChange"
      @row-click="handleRowClick"
    >
      <!-- Selection Column -->
      <el-table-column v-if="selection" type="selection" width="50" align="center" />

      <!-- Index Column -->
      <el-table-column v-if="showIndex" type="index" width="60" label="#" align="center" />

      <!-- Dynamic Columns -->
      <el-table-column
        v-for="column in columns"
        :key="column.prop"
        :prop="column.prop"
        :label="column.label"
        :width="column.width"
        :min-width="column.minWidth"
        :fixed="column.fixed"
        :sortable="column.sortable"
        :show-overflow-tooltip="true"
        align="left"
      >
        <template #default="scope">
          <slot :name="column.slot || column.prop" :row="scope.row" :column="column" :$index="scope.$index">
            <span v-if="column.formatter">{{ column.formatter(scope.row) }}</span>
            <span v-else>{{ scope.row[column.prop] ?? '-' }}</span>
          </slot>
        </template>
      </el-table-column>

      <!-- Actions Column -->
      <el-table-column v-if="$slots.actions" label="操作" :width="actionsWidth" fixed="right" align="left">
        <template #default="scope">
          <slot name="actions" :row="scope.row" :$index="scope.$index" />
        </template>
      </el-table-column>

      <!-- Empty State -->
      <template #empty>
        <div class="empty-state">
          <el-icon :size="48" class="empty-icon"><Box /></el-icon>
          <p class="empty-title">{{ emptyText }}</p>
          <p v-if="emptyDescription" class="empty-description">{{ emptyDescription }}</p>
        </div>
      </template>
    </el-table>

    <!-- Pagination -->
    <div v-if="showPagination" class="table-pagination">
      <div class="pagination-info">
        <span v-if="showTotal">共 {{ total }} 条记录</span>
      </div>
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="currentPageSize"
        :page-sizes="pageSizes"
        :total="total"
        :layout="paginationLayout"
        :background="true"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { TableColumn } from '@/types'

interface Props {
  data: any[]
  columns: TableColumn[]
  loading?: boolean
  border?: boolean
  stripe?: boolean
  selection?: boolean
  showIndex?: boolean
  rowKey?: string
  maxHeight?: string | number
  highlightCurrentRow?: boolean
  defaultSort?: { prop: string; order: 'ascending' | 'descending' }
  // Pagination
  showPagination?: boolean
  total?: number
  page?: number
  pageSize?: number
  pageSizes?: number[]
  showTotal?: boolean
  paginationLayout?: string
  // Empty state
  emptyText?: string
  emptyDescription?: string
  // Actions
  actionsWidth?: number | string
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  border: false,
  stripe: true,
  selection: false,
  showIndex: false,
  rowKey: 'id',
  maxHeight: undefined,
  highlightCurrentRow: false,
  defaultSort: undefined,
  showPagination: true,
  total: 0,
  page: 1,
  pageSize: 20,
  pageSizes: () => [10, 20, 50, 100],
  showTotal: true,
  paginationLayout: 'total, sizes, prev, pager, next, jumper',
  emptyText: '暂无数据',
  emptyDescription: '',
  actionsWidth: 180,
})

const emit = defineEmits<{
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
  (e: 'selection-change', rows: any[]): void
  (e: 'sort-change', sort: { column: any; prop: string | null; order: string | null }): void
  (e: 'row-click', row: any): void
  (e: 'page-change', page: number): void
  (e: 'size-change', size: number): void
}>()

const currentPage = ref(props.page)
const currentPageSize = ref(props.pageSize)

watch(() => props.page, (val) => {
  currentPage.value = val
})

watch(() => props.pageSize, (val) => {
  currentPageSize.value = val
})

function handleSelectionChange(rows: any[]) {
  emit('selection-change', rows)
}

function handleSortChange(sort: { column: any; prop: string | null; order: string | null }) {
  emit('sort-change', sort)
}

function handleRowClick(row: any) {
  emit('row-click', row)
}

function handlePageChange(page: number) {
  emit('update:page', page)
  emit('page-change', page)
}

function handleSizeChange(size: number) {
  emit('update:pageSize', size)
  emit('size-change', size)
}
</script>

<style scoped>
.data-table-wrapper {
  background: var(--color-bg-primary);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
  overflow: hidden;
}

.table-toolbar {
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--color-border-light);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-md);
  flex-wrap: wrap;
}

.table-pagination {
  padding: var(--spacing-md) var(--spacing-lg);
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--color-border-light);
}

.pagination-info {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.empty-state {
  padding: var(--spacing-2xl) var(--spacing-lg);
  text-align: center;
}

.empty-icon {
  color: var(--color-text-disabled);
  margin-bottom: var(--spacing-md);
}

.empty-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-secondary);
  margin-bottom: var(--spacing-xs);
}

.empty-description {
  font-size: 13px;
  color: var(--color-text-placeholder);
}

:deep(.el-table) {
  --el-table-border-color: var(--color-border);
}

:deep(.el-table th.el-table__cell) {
  background: var(--color-bg-secondary) !important;
  font-weight: 600;
  font-size: 13px;
  color: var(--color-text-secondary);
}

:deep(.el-table td.el-table__cell) {
  font-size: 13px;
}

:deep(.el-pagination) {
  --el-pagination-bg-color: transparent;
}

:deep(.el-pagination .el-pager li) {
  border-radius: var(--radius-md);
}

:deep(.el-pagination .el-pager li.is-active) {
  background: var(--color-primary);
}
</style>
