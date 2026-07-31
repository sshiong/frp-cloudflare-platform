<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">证书管理</h2>
        <p class="page-description">管理 SSL/TLS 证书</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openIssueDialog">申请证书</el-button>
    </div>

    <!-- Stats -->
    <el-row :gutter="16" class="mb-4">
      <el-col :span="6" v-for="stat in certStats" :key="stat.label">
        <div class="stat-card">
          <div class="stat-value" :style="{ color: stat.color }">{{ stat.value }}</div>
          <div class="stat-label">{{ stat.label }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- Table -->
    <DataTable
      :data="certificates"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      @page-change="fetchCertificates"
      @size-change="fetchCertificates"
    >
      <template #domain="{ row }">
        <div class="domain-cell">
          <el-icon :size="16" color="#16A34A"><Lock /></el-icon>
          <span>{{ row.domain }}</span>
        </div>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="row.status" dot />
      </template>

      <template #auto_renew="{ row }">
        <el-tag :type="row.auto_renew ? 'success' : 'info'" size="small" effect="light">
          {{ row.auto_renew ? '自动续签' : '手动续签' }}
        </el-tag>
      </template>

      <template #not_before="{ row }">
        {{ formatDate(row.not_before, 'YYYY-MM-DD') }}
      </template>

      <template #not_after="{ row }">
        <span :class="{ 'text-warning': isExpiringSoon(row.not_after), 'text-error': isExpired(row.not_after) }">
          {{ formatDate(row.not_after, 'YYYY-MM-DD') }}
        </span>
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #actions="{ row }">
        <el-button
          v-if="row.status === 'expired' || row.status === 'expiring_soon'"
          text
          type="primary"
          size="small"
          @click="handleRenew(row as any)"
          :loading="row._renewing"
        >
          续签
        </el-button>
      </template>
    </DataTable>

    <!-- Issue Dialog -->
    <el-dialog v-model="issueDialogVisible" title="申请证书" width="450px" destroy-on-close>
      <el-form ref="issueFormRef" :model="issueForm" :rules="issueRules" label-width="80px">
        <el-form-item label="域名" prop="domain">
          <el-input v-model="issueForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            证书将通过 Let's Encrypt 自动签发，请确保域名已正确解析到服务器。
          </template>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="issueDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="issueLoading" @click="handleIssue">申请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { certificatesApi } from '@/api/certificates'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, extractErrorMessage } from '@/utils/format'
import type { Certificate, TableColumn } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'
import dayjs from 'dayjs'

const { success, error } = useNotification()

const loading = ref(false)
const certificates = ref<Certificate[]>([])
const total = ref(0)
const issueDialogVisible = ref(false)
const issueLoading = ref(false)
const issueFormRef = ref<FormInstance>()

const stats = ref({ valid: 0, expired: 0, expiring_soon: 0, pending: 0, total: 0 })

const certStats = computed(() => [
  { label: '有效证书', value: stats.value.valid, color: '#16A34A' },
  { label: '即将过期', value: stats.value.expiring_soon, color: '#EAB308' },
  { label: '已过期', value: stats.value.expired, color: '#DC2626' },
  { label: '申请中', value: stats.value.pending, color: '#2563EB' },
])

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const issueForm = reactive({
  domain: '',
})

const issueRules: FormRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9]([a-zA-Z0-9-]*\.)+[a-zA-Z]{2,}$/, message: '请输入有效的域名', trigger: 'blur' },
  ],
}

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'domain', label: '域名', minWidth: 180, slot: 'domain' },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'issuer', label: '颁发者', width: 150 },
  { prop: 'auto_renew', label: '续签方式', width: 120, slot: 'auto_renew' },
  { prop: 'not_before', label: '生效时间', width: 120, slot: 'not_before' },
  { prop: 'not_after', label: '过期时间', width: 120, slot: 'not_after' },
  { prop: 'created_at', label: '创建时间', width: 160, slot: 'created_at' },
]

function isExpiringSoon(date: string): boolean {
  return dayjs(date).diff(dayjs(), 'day') <= 30
}

function isExpired(date: string): boolean {
  return dayjs(date).isBefore(dayjs())
}

async function fetchCertificates() {
  loading.value = true
  try {
    const res = await certificatesApi.getList({
      page: pagination.page,
      page_size: pagination.pageSize,
    })
    if (res.code === 0) {
      certificates.value = res.data.list
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
    const res = await certificatesApi.getStats()
    if (res.code === 0) {
      stats.value = res.data
    }
  } catch {
    // Ignore
  }
}

function openIssueDialog() {
  issueForm.domain = ''
  issueDialogVisible.value = true
}

async function handleIssue() {
  if (!issueFormRef.value) return
  await issueFormRef.value.validate(async (valid) => {
    if (!valid) return

    issueLoading.value = true
    try {
      await certificatesApi.issue({ domain: issueForm.domain })
      success('证书申请已提交')
      issueDialogVisible.value = false
      fetchCertificates()
      fetchStats()
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      issueLoading.value = false
    }
  })
}

async function handleRenew(cert: Certificate) {
  const c = cert as any
  c._renewing = true
  try {
    await certificatesApi.renew(cert.id)
    success('证书续签已提交')
    fetchCertificates()
    fetchStats()
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    c._renewing = false
  }
}

onMounted(() => {
  fetchCertificates()
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

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
}

.stat-label {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  margin-top: var(--spacing-xs);
}

.domain-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
