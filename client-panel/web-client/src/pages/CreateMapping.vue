<template>
  <div class="create-mapping">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">{{ isEdit ? '编辑映射' : '创建映射' }}</h1>
        <p class="page-header__description">{{ isEdit ? '修改端口映射配置' : '创建新的端口映射' }}</p>
      </div>
      <div class="page-header__actions">
        <el-button @click="router.back()">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEdit ? '保存修改' : '创建映射' }}
        </el-button>
      </div>
    </div>

    <div class="form-card">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
        label-position="left"
      >
        <!-- Basic Info -->
        <div class="form-section">
          <h3 class="form-section__title">基本信息</h3>

          <el-form-item label="映射名称" prop="name">
            <el-input
              v-model="form.name"
              placeholder="例如：我的Web服务"
              maxlength="50"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="协议类型" prop="protocol">
            <el-radio-group v-model="form.protocol">
              <el-radio-button value="tcp">
                <span class="protocol-label">
                  <span class="protocol-badge protocol-badge--tcp">TCP</span>
                  <span>TCP 端口转发</span>
                </span>
              </el-radio-button>
              <el-radio-button value="udp">
                <span class="protocol-label">
                  <span class="protocol-badge protocol-badge--udp">UDP</span>
                  <span>UDP 端口转发</span>
                </span>
              </el-radio-button>
              <el-radio-button value="http">
                <span class="protocol-label">
                  <span class="protocol-badge protocol-badge--http">HTTP</span>
                  <span>HTTP 域名访问</span>
                </span>
              </el-radio-button>
              <el-radio-button value="https">
                <span class="protocol-label">
                  <span class="protocol-badge protocol-badge--https">HTTPS</span>
                  <span>HTTPS 域名访问</span>
                </span>
              </el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>

        <!-- Local Service -->
        <div class="form-section">
          <h3 class="form-section__title">本地服务</h3>

          <el-form-item label="本地 IP" prop="localIp">
            <el-input
              v-model="form.localIp"
              placeholder="127.0.0.1"
            />
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>本地服务监听的 IP 地址，通常为 127.0.0.1 或 localhost</span>
            </div>
          </el-form-item>

          <el-form-item label="本地端口" prop="localPort">
            <el-input-number
              v-model="form.localPort"
              :min="1"
              :max="65535"
              placeholder="8080"
              style="width: 200px"
            />
            <el-button
              type="default"
              style="margin-left: 12px"
              :loading="healthChecking"
              @click="handleHealthCheck"
            >
              <el-icon><Connection /></el-icon>
              <span>健康检查</span>
            </el-button>
          </el-form-item>

          <div v-if="healthCheckResult" :class="['health-check-result', healthCheckResult.success ? 'health-check-result--success' : 'health-check-result--error']">
            <el-icon>
              <component :is="healthCheckResult.success ? 'CircleCheckFilled' : 'CircleCloseFilled'" />
            </el-icon>
            <span>{{ healthCheckResult.message }}</span>
          </div>
        </div>

        <!-- Remote Config -->
        <div class="form-section">
          <h3 class="form-section__title">远程配置</h3>

          <!-- TCP/UDP: Remote Port -->
          <el-form-item
            v-if="form.protocol === 'tcp' || form.protocol === 'udp'"
            label="远程端口"
            prop="remotePort"
          >
            <el-input-number
              v-model="form.remotePort"
              :min="1"
              :max="65535"
              placeholder="8080"
              style="width: 200px"
            />
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>用户通过此端口访问您的服务</span>
            </div>
          </el-form-item>

          <!-- HTTP/HTTPS: Domain -->
          <el-form-item
            v-if="form.protocol === 'http' || form.protocol === 'https'"
            label="访问域名"
            prop="customDomains"
          >
            <el-select
              v-model="form.customDomains"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="输入域名后回车"
              style="width: 100%"
            >
              <el-option
                v-for="domain in availableDomains"
                :key="domain"
                :label="domain"
                :value="domain"
              />
            </el-select>
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>用户通过这些域名访问您的服务</span>
            </div>
          </el-form-item>

          <el-form-item
            v-if="form.protocol === 'http' || form.protocol === 'https'"
            label="子域名"
            prop="subdomain"
          >
            <el-input
              v-model="form.subdomain"
              placeholder="例如：myapp"
            >
              <template #append>.frp.example.com</template>
            </el-input>
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>使用子域名访问您的服务，无需配置域名解析</span>
            </div>
          </el-form-item>
        </div>

        <!-- Advanced Options -->
        <div class="form-section">
          <h3 class="form-section__title">高级选项</h3>

          <el-form-item label="启用加密">
            <el-switch v-model="form.useEncryption" />
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>启用端到端加密，会略微影响性能</span>
            </div>
          </el-form-item>

          <el-form-item label="启用压缩">
            <el-switch v-model="form.useCompression" />
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>启用数据压缩，适合文本类服务</span>
            </div>
          </el-form-item>

          <el-form-item label="访问密钥">
            <el-input
              v-model="form.secretKey"
              placeholder="可选，用于身份验证"
              show-password
            />
            <div class="form-help">
              <el-icon><InfoFilled /></el-icon>
              <span>设置密钥后，访问时需要提供此密钥</span>
            </div>
          </el-form-item>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  InfoFilled,
  Connection,
  CircleCheckFilled,
  CircleCloseFilled,
} from '@element-plus/icons-vue'
import * as mappingsApi from '@/api/mappings'
import * as domainsApi from '@/api/domains'
import type { CreateMappingRequest, Protocol } from '@/types'
import { isValidIP, isValidPort } from '@/utils/format'

const router = useRouter()
const route = useRoute()

const isEdit = computed(() => !!route.params.id)
const formRef = ref<FormInstance>()
const submitting = ref(false)
const healthChecking = ref(false)
const healthCheckResult = ref<{ success: boolean; message: string } | null>(null)
const availableDomains = ref<string[]>([])

const form = reactive<CreateMappingRequest & { enabled: boolean }>({
  name: '',
  protocol: 'tcp',
  localIp: '127.0.0.1',
  localPort: 8080,
  remotePort: undefined,
  customDomains: [],
  subdomain: '',
  useEncryption: false,
  useCompression: false,
  secretKey: '',
  enabled: true,
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入映射名称', trigger: 'blur' },
    { min: 2, max: 50, message: '名称长度在 2 到 50 个字符', trigger: 'blur' },
  ],
  protocol: [
    { required: true, message: '请选择协议类型', trigger: 'change' },
  ],
  localIp: [
    { required: true, message: '请输入本地 IP', trigger: 'blur' },
    {
      validator: (rule: any, value: string, callback: any) => {
        if (!isValidIP(value)) {
          callback(new Error('请输入有效的 IP 地址'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  localPort: [
    { required: true, message: '请输入本地端口', trigger: 'blur' },
    {
      validator: (rule: any, value: number, callback: any) => {
        if (!isValidPort(value)) {
          callback(new Error('端口范围为 1-65535'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  remotePort: [
    {
      validator: (rule: any, value: number | undefined, callback: any) => {
        if ((form.protocol === 'tcp' || form.protocol === 'udp') && !value) {
          callback(new Error('TCP/UDP 协议需要配置远程端口'))
        } else if (value && !isValidPort(value)) {
          callback(new Error('端口范围为 1-65535'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  customDomains: [
    {
      validator: (rule: any, value: string[], callback: any) => {
        if ((form.protocol === 'http' || form.protocol === 'https') && (!value || value.length === 0) && !form.subdomain) {
          callback(new Error('HTTP/HTTPS 协议需要配置域名或子域名'))
        } else {
          callback()
        }
      },
      trigger: 'change',
    },
  ],
}

// Fetch available domains
async function fetchDomains() {
  try {
    const domains = await domainsApi.getDomains()
    availableDomains.value = domains
      .filter((d) => d.status === 'active')
      .map((d) => d.domain)
  } catch (error) {
    console.error('Failed to fetch domains:', error)
  }
}

// Fetch mapping data for edit
async function fetchMapping() {
  if (!route.params.id) return

  try {
    const mapping = await mappingsApi.getMapping(Number(route.params.id))
    Object.assign(form, {
      name: mapping.name,
      protocol: mapping.protocol,
      localIp: mapping.localIp,
      localPort: mapping.localPort,
      remotePort: mapping.remotePort,
      customDomains: mapping.customDomains || [],
      subdomain: mapping.subdomain || '',
      enabled: mapping.enabled,
    })
  } catch (error) {
    console.error('Failed to fetch mapping:', error)
    ElMessage.error('获取映射信息失败')
    router.back()
  }
}

// Health check
async function handleHealthCheck() {
  healthChecking.value = true
  healthCheckResult.value = null

  try {
    const result = await mappingsApi.checkLocalService({
      localIp: form.localIp,
      localPort: form.localPort,
    })

    if (result.success) {
      healthCheckResult.value = {
        success: true,
        message: `服务可达！延迟：${result.latency}ms`,
      }
    } else {
      healthCheckResult.value = {
        success: false,
        message: result.error || '服务不可达',
      }
    }
  } catch (error: any) {
    healthCheckResult.value = {
      success: false,
      message: error.message || '健康检查失败',
    }
  } finally {
    healthChecking.value = false
  }
}

// Submit form
async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const data: CreateMappingRequest = {
      name: form.name,
      protocol: form.protocol,
      localIp: form.localIp,
      localPort: form.localPort,
      remotePort: form.remotePort,
      customDomains: form.customDomains?.length ? form.customDomains : undefined,
      subdomain: form.subdomain || undefined,
      useEncryption: form.useEncryption,
      useCompression: form.useCompression,
      secretKey: form.secretKey || undefined,
    }

    if (isEdit.value) {
      await mappingsApi.updateMapping(Number(route.params.id), data)
      ElMessage.success('映射已更新')
    } else {
      await mappingsApi.createMapping(data)
      ElMessage.success('映射已创建')
    }

    router.push('/mappings')
  } catch (error: any) {
    ElMessage.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

// Initialize
onMounted(() => {
  fetchDomains()
  if (isEdit.value) {
    fetchMapping()
  }
})
</script>

<style scoped>
.create-mapping {
  max-width: 800px;
  margin: 0 auto;
}

.form-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  padding: 24px;
}

.form-section {
  margin-bottom: 32px;
  padding-bottom: 24px;
  border-bottom: 1px solid #E4E4E7;
}

.form-section:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.form-section__title {
  font-size: 16px;
  font-weight: 600;
  color: #18181B;
  margin: 0 0 20px;
}

.protocol-label {
  display: flex;
  align-items: center;
  gap: 8px;
}

.protocol-badge {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}

.protocol-badge--tcp {
  background: #DBEAFE;
  color: #2563EB;
}

.protocol-badge--udp {
  background: #F3E8FF;
  color: #7C3AED;
}

.protocol-badge--http {
  background: #ECFDF5;
  color: #16A34A;
}

.protocol-badge--https {
  background: #FFFBEB;
  color: #92400E;
}

.form-help {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #A1A1AA;
}

.health-check-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-radius: 6px;
  margin-top: 12px;
  font-size: 14px;
}

.health-check-result--success {
  background: #ECFDF5;
  color: #16A34A;
}

.health-check-result--error {
  background: #FEF2F2;
  color: #DC2626;
}

/* Responsive */
@media (max-width: 768px) {
  .form-card {
    padding: 16px;
  }

  :deep(.el-form-item) {
    flex-direction: column;
  }

  :deep(.el-form-item__label) {
    text-align: left;
    margin-bottom: 8px;
  }
}
</style>