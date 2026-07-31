<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">映射管理</h2>
        <p class="page-description">管理端口映射规则</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">创建映射</el-button>
    </div>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="关键词">
          <el-input v-model="filters.keyword" placeholder="映射名称" clearable @clear="handleSearch" />
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="filters.protocol" placeholder="全部" clearable @change="handleSearch">
            <el-option label="TCP" value="tcp" />
            <el-option label="UDP" value="udp" />
            <el-option label="HTTP" value="http" />
            <el-option label="HTTPS" value="https" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部" clearable @change="handleSearch">
            <el-option label="运行中" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="异常" value="error" />
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
      :data="mappings"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :actions-width="280"
      @page-change="fetchMappings"
      @size-change="fetchMappings"
    >
      <template #protocol="{ row }">
        <el-tag :type="getProtocolTagType(row.protocol)" size="small" effect="plain">
          {{ formatProtocol(row.protocol) }}
        </el-tag>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="row.status" dot />
      </template>

      <template #local="{ row }">
        <code class="font-mono">{{ row.local_ip }}:{{ row.local_port }}</code>
      </template>

      <template #remote="{ row }">
        <code v-if="row.remote_port" class="font-mono">:{{ row.remote_port }}</code>
        <span v-else-if="row.custom_domain" class="font-mono">{{ row.custom_domain }}</span>
        <span v-else class="text-secondary">自动分配</span>
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #actions="{ row }">
        <el-button text type="primary" size="small" @click="openEditDialog(row)">编辑</el-button>
        <el-button
          text
          :type="row.status === 'active' ? 'warning' : 'success'"
          size="small"
          @click="handleToggleStatus(row)"
        >
          {{ row.status === 'active' ? '禁用' : '启用' }}
        </el-button>
        <el-button text type="danger" size="small" @click="handleDelete(row)">删除</el-button>
      </template>
    </DataTable>

    <!-- Create/Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑映射' : '创建映射'"
      width="550px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="110px">
        <el-form-item label="映射名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入映射名称" />
        </el-form-item>
        <el-form-item label="协议" prop="protocol">
          <el-radio-group v-model="form.protocol" :disabled="isEditing">
            <el-radio-button value="tcp">TCP</el-radio-button>
            <el-radio-button value="udp">UDP</el-radio-button>
            <el-radio-button value="http">HTTP</el-radio-button>
            <el-radio-button value="https">HTTPS</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="本地IP" prop="local_ip">
          <el-input v-model="form.local_ip" placeholder="127.0.0.1" />
        </el-form-item>
        <el-form-item label="本地端口" prop="local_port">
          <el-input-number v-model="form.local_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item v-if="form.protocol === 'tcp' || form.protocol === 'udp'" label="远程端口">
          <el-input-number v-model="form.remote_port" :min="1" :max="65535" placeholder="留空自动分配" />
        </el-form-item>
        <el-form-item v-if="form.protocol === 'http' || form.protocol === 'https'" label="自定义域名">
          <el-input v-model="form.custom_domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="带宽限制">
          <el-input-number v-model="form.bandwidth_limit" :min="0" :max="1000" />
          <span class="ml-2 text-secondary">MB/s (0为不限制)</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">
          {{ isEditing ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { mappingsApi } from '@/api/mappings'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, formatProtocol, extractErrorMessage } from '@/utils/format'
import type { Mapping, TableColumn } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const mappings = ref<Mapping[]>([])
const total = ref(0)
const dialogVisible = ref(false)
const isEditing = ref(false)
const submitLoading = ref(false)
const editingMapping = ref<Mapping | null>(null)
const formRef = ref<FormInstance>()

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  keyword: '',
  protocol: '',
  status: '',
})

const form = reactive({
  name: '',
  protocol: 'tcp' as string,
  local_ip: '127.0.0.1',
  local_port: 8080,
  remote_port: undefined as number | undefined,
  custom_domain: '',
  bandwidth_limit: 0,
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入映射名称', trigger: 'blur' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
  local_ip: [{ required: true, message: '请输入本地IP', trigger: 'blur' }],
  local_port: [{ required: true, message: '请输入本地端口', trigger: 'blur' }],
}

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'name', label: '名称', minWidth: 140 },
  { prop: 'protocol', label: '协议', width: 80, slot: 'protocol' },
  { prop: 'local', label: '本地地址', minWidth: 160, slot: 'local' },
  { prop: 'remote', label: '远程', minWidth: 140, slot: 'remote' },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'username', label: '用户', width: 100 },
  { prop: 'created_at', label: '创建时间', width: 160, slot: 'created_at' },
]

function getProtocolTagType(protocol: string): string {
  const map: Record<string, string> = {
    tcp: '',
    udp: 'warning',
    http: 'success',
    https: 'primary',
  }
  return map[protocol] || ''
}

async function fetchMappings() {
  loading.value = true
  try {
    const res = await mappingsApi.getList({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters,
    })
    if (res.code === 0) {
      mappings.value = res.data.list
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
  fetchMappings()
}

function handleReset() {
  filters.keyword = ''
  filters.protocol = ''
  filters.status = ''
  handleSearch()
}

function openCreateDialog() {
  isEditing.value = false
  editingMapping.value = null
  form.name = ''
  form.protocol = 'tcp'
  form.local_ip = '127.0.0.1'
  form.local_port = 8080
  form.remote_port = undefined
  form.custom_domain = ''
  form.bandwidth_limit = 0
  dialogVisible.value = true
}

function openEditDialog(mapping: Mapping) {
  isEditing.value = true
  editingMapping.value = mapping
  form.name = mapping.name
  form.protocol = mapping.protocol
  form.local_ip = mapping.local_ip
  form.local_port = mapping.local_port
  form.remote_port = mapping.remote_port
  form.custom_domain = mapping.custom_domain || ''
  form.bandwidth_limit = mapping.bandwidth_limit || 0
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitLoading.value = true
    try {
      if (isEditing.value && editingMapping.value) {
        await mappingsApi.update(editingMapping.value.id, {
          name: form.name,
          local_ip: form.local_ip,
          local_port: form.local_port,
          remote_port: form.remote_port,
          custom_domain: form.custom_domain || undefined,
          bandwidth_limit: form.bandwidth_limit || undefined,
        })
        success('映射更新成功')
      } else {
        await mappingsApi.create({
          name: form.name,
          protocol: form.protocol as any,
          local_ip: form.local_ip,
          local_port: form.local_port,
          remote_port: form.remote_port,
          custom_domain: form.custom_domain || undefined,
          bandwidth_limit: form.bandwidth_limit || undefined,
        })
        success('映射创建成功')
      }
      dialogVisible.value = false
      fetchMappings()
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      submitLoading.value = false
    }
  })
}

async function handleToggleStatus(mapping: Mapping) {
  const action = mapping.status === 'active' ? '禁用' : '启用'
  const confirmed = await confirm(`确定要${action}映射 ${mapping.name} 吗？`)
  if (!confirmed) return

  try {
    if (mapping.status === 'active') {
      await mappingsApi.disable(mapping.id)
    } else {
      await mappingsApi.enable(mapping.id)
    }
    success(`映射已${action}`)
    fetchMappings()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function handleDelete(mapping: Mapping) {
  const confirmed = await confirm(`确定要删除映射 ${mapping.name} 吗？`, '确认删除', 'warning')
  if (!confirmed) return

  try {
    await mappingsApi.delete(mapping.id)
    success('映射已删除')
    fetchMappings()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchMappings()
})
</script>
