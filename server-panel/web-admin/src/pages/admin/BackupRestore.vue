<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">备份恢复</h2>
        <p class="page-description">创建备份和恢复系统数据</p>
      </div>
      <div class="header-actions">
        <el-button :loading="createLoading" @click="handleCreateBackup">
          <el-icon class="el-icon--left"><Plus /></el-icon>
          创建备份
        </el-button>
        <el-upload
          ref="uploadRef"
          :auto-upload="false"
          :show-file-list="false"
          accept=".zip,.tar.gz,.bak"
          :on-change="handleFileSelect"
        >
          <el-button type="primary">
            <el-icon class="el-icon--left"><Upload /></el-icon>
            上传备份
          </el-button>
        </el-upload>
      </div>
    </div>

    <!-- Backup List -->
    <DataTable
      :data="backups"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :actions-width="240"
      @page-change="fetchBackups"
      @size-change="fetchBackups"
    >
      <template #size="{ row }">
        {{ formatBytes(row.size) }}
      </template>

      <template #type="{ row }">
        <el-tag size="small" effect="plain">
          {{ row.type === 'full' ? '完整备份' : row.type === 'config' ? '配置备份' : '数据备份' }}
        </el-tag>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="row.status" />
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #actions="{ row }">
        <el-button text type="primary" size="small" @click="handleDownload(row as any)">下载</el-button>
        <el-button text type="warning" size="small" @click="handleRestore(row as any)">恢复</el-button>
      </template>
    </DataTable>

    <!-- Preflight Dialog -->
    <el-dialog v-model="preflightVisible" title="恢复确认" width="500px" destroy-on-close>
      <div v-if="preflightData" class="preflight-content">
        <el-alert
          v-if="preflightData.warnings.length > 0"
          type="warning"
          :closable="false"
          show-icon
          class="mb-3"
        >
          <template #title>
            <div v-for="warn in preflightData.warnings" :key="warn">{{ warn }}</div>
          </template>
        </el-alert>

        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="备份版本">{{ preflightData.version }}</el-descriptions-item>
          <el-descriptions-item label="备份大小">{{ formatBytes(preflightData.size) }}</el-descriptions-item>
          <el-descriptions-item label="包含表">{{ preflightData.tables.join(', ') }}</el-descriptions-item>
        </el-descriptions>

        <el-alert
          v-if="preflightData.errors.length > 0"
          type="error"
          :closable="false"
          show-icon
          class="mt-3"
        >
          <template #title>
            <div v-for="err in preflightData.errors" :key="err">{{ err }}</div>
          </template>
        </el-alert>
      </div>

      <el-alert type="warning" :closable="false" show-icon class="mt-3">
        <template #title>
          恢复操作将覆盖当前系统数据，此操作不可撤销！
        </template>
      </el-alert>

      <template #footer>
        <el-button @click="preflightVisible = false">取消</el-button>
        <el-button
          type="danger"
          :loading="restoreLoading"
          :disabled="preflightData && !preflightData.valid"
          @click="confirmRestore"
        >
          确认恢复
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Upload } from '@element-plus/icons-vue'
import { adminApi } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, formatBytes, extractErrorMessage } from '@/utils/format'
import type { Backup, BackupPreflightCheck, TableColumn } from '@/types'
import type { UploadFile } from 'element-plus'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const createLoading = ref(false)
const restoreLoading = ref(false)
const backups = ref<Backup[]>([])
const total = ref(0)
const preflightVisible = ref(false)
const preflightData = ref<BackupPreflightCheck | undefined>(undefined)
const selectedBackupId = ref<number | null>(null)

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'filename', label: '文件名', minWidth: 200 },
  { prop: 'size', label: '大小', width: 100, slot: 'size' },
  { prop: 'type', label: '类型', width: 100, slot: 'type' },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'created_at', label: '创建时间', width: 180, slot: 'created_at' },
]

async function fetchBackups() {
  loading.value = true
  try {
    const res = await adminApi.getBackups({
      page: pagination.page,
      page_size: pagination.pageSize,
    })
    if (res.code === 0) {
      backups.value = res.data.list
      total.value = res.data.total
    }
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}

async function handleCreateBackup() {
  createLoading.value = true
  try {
    await adminApi.createBackup()
    success('备份创建成功')
    fetchBackups()
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    createLoading.value = false
  }
}

async function handleDownload(backup: Backup) {
  try {
    await adminApi.downloadBackup(backup.id, backup.filename)
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function handleRestore(backup: Backup) {
  selectedBackupId.value = backup.id
  try {
    const res = await adminApi.preflightRestore(backup.id)
    if (res.code === 0) {
      preflightData.value = res.data
      preflightVisible.value = true
    }
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function confirmRestore() {
  if (!selectedBackupId.value) return

  const confirmed = await confirm('确定要恢复此备份吗？当前数据将被覆盖！', '确认恢复', 'warning')
  if (!confirmed) return

  restoreLoading.value = true
  try {
    await adminApi.restoreBackup(selectedBackupId.value)
    success('备份恢复成功')
    preflightVisible.value = false
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    restoreLoading.value = false
  }
}

function handleFileSelect(file: UploadFile) {
  if (file.raw) {
    handleUploadBackup(file.raw)
  }
}

async function handleUploadBackup(file: File) {
  try {
    const res = await adminApi.uploadBackup(file)
    if (res.code === 0) {
      preflightData.value = res.data
      preflightVisible.value = true
      success('备份上传成功')
      fetchBackups()
    }
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchBackups()
})
</script>

<style scoped>
.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

.preflight-content {
  margin-bottom: var(--spacing-md);
}
</style>
