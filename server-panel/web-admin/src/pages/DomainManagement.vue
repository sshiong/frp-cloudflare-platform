<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">域名管理</h2>
        <p class="page-description">管理自定义域名</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">添加域名</el-button>
    </div>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="关键词">
          <el-input v-model="filters.keyword" placeholder="搜索域名" clearable @clear="handleSearch" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部" clearable @change="handleSearch">
            <el-option label="已验证" value="verified" />
            <el-option label="待验证" value="pending" />
            <el-option label="验证失败" value="error" />
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
      :data="domains"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :actions-width="240"
      @page-change="fetchDomains"
      @size-change="fetchDomains"
    >
      <template #domain="{ row }">
        <div class="domain-cell">
          <el-icon :size="16" class="domain-icon"><Link /></el-icon>
          <span class="domain-name">{{ row.domain }}</span>
        </div>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="row.status" dot />
      </template>

      <template #username="{ row }">
        {{ row.username || '-' }}
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #actions="{ row }">
        <el-button text type="primary" size="small" @click="handleVerify(row as any)" :loading="row._verifying">
          验证
        </el-button>
        <el-button text type="info" size="small" @click="handleViewDns(row as any)">DNS记录</el-button>
        <el-button text type="danger" size="small" @click="handleDelete(row as any)">删除</el-button>
      </template>
    </DataTable>

    <!-- Create Dialog -->
    <el-dialog v-model="dialogVisible" title="添加域名" width="450px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="80px">
        <el-form-item label="域名" prop="domain">
          <el-input v-model="form.domain" placeholder="example.com" />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            请确保域名已解析到服务器IP，并在Cloudflare中配置好DNS记录。
          </template>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">添加</el-button>
      </template>
    </el-dialog>

    <!-- DNS Records Dialog -->
    <el-dialog v-model="dnsDialogVisible" title="DNS 记录" width="550px" destroy-on-close>
      <el-table :data="dnsRecords" style="width: 100%" size="small" v-loading="dnsLoading">
        <el-table-column prop="type" label="类型" width="80" />
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column prop="value" label="值" min-width="200">
          <template #default="{ row }">
            <code class="font-mono">{{ row.value }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="ttl" label="TTL" width="80" />
      </el-table>
      <div v-if="dnsRecords.length === 0 && !dnsLoading" class="empty-state">
        <p>暂无DNS记录</p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { domainsApi } from '@/api/domains'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, extractErrorMessage } from '@/utils/format'
import type { Domain, DnsRecord, TableColumn } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const domains = ref<Domain[]>([])
const total = ref(0)
const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()

const dnsDialogVisible = ref(false)
const dnsLoading = ref(false)
const dnsRecords = ref<DnsRecord[]>([])

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  keyword: '',
  status: '',
})

const form = reactive({
  domain: '',
})

const formRules: FormRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9]([a-zA-Z0-9-]*\.)+[a-zA-Z]{2,}$/, message: '请输入有效的域名', trigger: 'blur' },
  ],
}

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'domain', label: '域名', minWidth: 200, slot: 'domain' },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'username', label: '用户', width: 120 },
  { prop: 'created_at', label: '创建时间', width: 180, slot: 'created_at' },
]

async function fetchDomains() {
  loading.value = true
  try {
    const res = await domainsApi.getList({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters,
    })
    if (res.code === 0) {
      domains.value = res.data.list
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
  fetchDomains()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  handleSearch()
}

function openCreateDialog() {
  form.domain = ''
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitLoading.value = true
    try {
      await domainsApi.create({ domain: form.domain })
      success('域名添加成功')
      dialogVisible.value = false
      fetchDomains()
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      submitLoading.value = false
    }
  })
}

async function handleVerify(domain: Domain) {
  const d = domain as any
  d._verifying = true
  try {
    await domainsApi.verify(domain.id)
    success('域名验证成功')
    fetchDomains()
  } catch (err) {
    error(extractErrorMessage(err))
  } finally {
    d._verifying = false
  }
}

async function handleViewDns(domain: Domain) {
  dnsDialogVisible.value = true
  dnsLoading.value = true
  dnsRecords.value = []
  try {
    const res = await domainsApi.getDnsRecords(domain.id)
    if (res.code === 0) {
      dnsRecords.value = res.data
    }
  } catch {
    // Error handled by interceptor
  } finally {
    dnsLoading.value = false
  }
}

async function handleDelete(domain: Domain) {
  const confirmed = await confirm(`确定要删除域名 ${domain.domain} 吗？`, '确认删除', 'warning')
  if (!confirmed) return

  try {
    await domainsApi.delete(domain.id)
    success('域名已删除')
    fetchDomains()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchDomains()
})
</script>

<style scoped>
.domain-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.domain-icon {
  color: var(--color-primary);
}

.domain-name {
  font-weight: 500;
}
</style>
