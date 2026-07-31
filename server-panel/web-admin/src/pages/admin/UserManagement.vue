<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2 class="page-title">用户管理</h2>
        <p class="page-description">管理系统用户账户</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">创建用户</el-button>
    </div>

    <!-- Filters -->
    <div class="card mb-4">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="关键词">
          <el-input v-model="filters.keyword" placeholder="搜索用户名" clearable @clear="handleSearch" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="filters.role" placeholder="全部" clearable @change="handleSearch">
            <el-option label="管理员" value="admin" />
            <el-option label="普通用户" value="user" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部" clearable @change="handleSearch">
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="已锁定" value="locked" />
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
      :data="users"
      :columns="columns"
      :loading="loading"
      :total="total"
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      @page-change="fetchUsers"
      @size-change="fetchUsers"
    >
      <template #status="{ row }">
        <StatusBadge :status="row.status" />
      </template>

      <template #role="{ row }">
        <el-tag :type="row.role === 'admin' ? 'primary' : 'info'" size="small" effect="light">
          {{ formatRole(row.role) }}
        </el-tag>
      </template>

      <template #created_at="{ row }">
        {{ formatDate(row.created_at) }}
      </template>

      <template #last_login_at="{ row }">
        {{ row.last_login_at ? formatRelativeTime(row.last_login_at) : '从未登录' }}
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
      :title="isEditing ? '编辑用户' : '创建用户'"
      width="500px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" :disabled="isEditing" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码" :prop="isEditing ? '' : 'password'">
          <el-input v-model="form.password" type="password" show-password :placeholder="isEditing ? '留空则不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="form.role" placeholder="请选择角色">
            <el-option label="管理员" value="admin" />
            <el-option label="普通用户" value="user" />
          </el-select>
        </el-form-item>
        <el-form-item label="最大映射数" prop="max_mappings">
          <el-input-number v-model="form.max_mappings" :min="0" :max="1000" />
        </el-form-item>
        <el-form-item label="最大设备数" prop="max_devices">
          <el-input-number v-model="form.max_devices" :min="0" :max="100" />
        </el-form-item>
        <el-form-item label="备注" prop="note">
          <el-input v-model="form.note" type="textarea" :rows="3" placeholder="用户备注" />
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
import { usersApi } from '@/api/users'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNotification } from '@/composables/useNotification'
import { formatDate, formatRelativeTime, formatRole, extractErrorMessage } from '@/utils/format'
import type { User, TableColumn } from '@/types'
import type { FormInstance, FormRules } from 'element-plus'

const { success, error, confirm } = useNotification()

const loading = ref(false)
const users = ref<User[]>([])
const total = ref(0)
const dialogVisible = ref(false)
const isEditing = ref(false)
const submitLoading = ref(false)
const editingUser = ref<User | null>(null)
const formRef = ref<FormInstance>()

const pagination = reactive({
  page: 1,
  pageSize: 20,
})

const filters = reactive({
  keyword: '',
  role: '',
  status: '',
})

const form = reactive({
  username: '',
  password: '',
  role: 'user' as string,
  max_mappings: 10,
  max_devices: 5,
  note: '',
})

const formRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 32, message: '用户名长度在 2 到 32 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 位', trigger: 'blur' },
  ],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }],
}

const columns: TableColumn[] = [
  { prop: 'id', label: 'ID', width: 80 },
  { prop: 'username', label: '用户名', minWidth: 120 },
  { prop: 'role', label: '角色', width: 100, slot: 'role' },
  { prop: 'status', label: '状态', width: 100, slot: 'status' },
  { prop: 'max_mappings', label: '最大映射', width: 100 },
  { prop: 'max_devices', label: '最大设备', width: 100 },
  { prop: 'last_login_at', label: '最后登录', width: 150, slot: 'last_login_at' },
  { prop: 'created_at', label: '创建时间', width: 160, slot: 'created_at' },
]

async function fetchUsers() {
  loading.value = true
  try {
    const res = await usersApi.getList({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...filters,
    })
    if (res.code === 0) {
      users.value = res.data.list
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
  fetchUsers()
}

function handleReset() {
  filters.keyword = ''
  filters.role = ''
  filters.status = ''
  handleSearch()
}

function openCreateDialog() {
  isEditing.value = false
  editingUser.value = null
  form.username = ''
  form.password = ''
  form.role = 'user'
  form.max_mappings = 10
  form.max_devices = 5
  form.note = ''
  dialogVisible.value = true
}

function openEditDialog(user: User) {
  isEditing.value = true
  editingUser.value = user
  form.username = user.username
  form.password = ''
  form.role = user.role
  form.max_mappings = user.max_mappings || 10
  form.max_devices = user.max_devices || 5
  form.note = user.note || ''
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitLoading.value = true
    try {
      if (isEditing.value && editingUser.value) {
        const updateData: any = {
          role: form.role,
          max_mappings: form.max_mappings,
          max_devices: form.max_devices,
          note: form.note,
        }
        if (form.password) {
          updateData.password = form.password
        }
        await usersApi.update(editingUser.value.id, updateData)
        success('用户更新成功')
      } else {
        await usersApi.create({
          username: form.username,
          password: form.password,
          role: form.role as any,
          max_mappings: form.max_mappings,
          max_devices: form.max_devices,
          note: form.note,
        })
        success('用户创建成功')
      }
      dialogVisible.value = false
      fetchUsers()
    } catch (err) {
      error(extractErrorMessage(err))
    } finally {
      submitLoading.value = false
    }
  })
}

async function handleToggleStatus(user: User) {
  const action = user.status === 'active' ? '禁用' : '启用'
  const confirmed = await confirm(`确定要${action}用户 ${user.username} 吗？`)
  if (!confirmed) return

  try {
    if (user.status === 'active') {
      await usersApi.disable(user.id)
    } else {
      await usersApi.enable(user.id)
    }
    success(`用户已${action}`)
    fetchUsers()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

async function handleDelete(user: User) {
  const confirmed = await confirm(`确定要删除用户 ${user.username} 吗？此操作不可恢复。`, '确认删除', 'warning')
  if (!confirmed) return

  try {
    await usersApi.delete(user.id)
    success('用户已删除')
    fetchUsers()
  } catch (err) {
    error(extractErrorMessage(err))
  }
}

onMounted(() => {
  fetchUsers()
})
</script>
