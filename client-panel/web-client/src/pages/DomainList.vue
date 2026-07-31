<template>
  <div class="domain-list">
    <div class="page-header">
      <div>
        <h1 class="page-header__title">域名管理</h1>
        <p class="page-header__description">管理您的域名和 SSL 证书</p>
      </div>
      <div class="page-header__actions">
        <el-button @click="fetchDomains">
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>
    </div>

    <!-- Domain Cards -->
    <div v-loading="loading" class="domain-grid">
      <div
        v-for="domain in domains"
        :key="domain.id"
        class="domain-card"
      >
        <div class="domain-card__header">
          <div class="domain-card__domain">
            <el-icon :size="20" color="#2563EB"><Compass /></el-icon>
            <span>{{ domain.domain }}</span>
          </div>
          <StatusBadge
            :type="getStatusType(domain.status)"
            :label="getStatusLabel(domain.status)"
          />
        </div>

        <div class="domain-card__body">
          <!-- DNS Status -->
          <div class="domain-info">
            <span class="domain-info__label">DNS 状态</span>
            <div class="domain-info__value">
              <el-tag
                v-for="record in domain.dnsRecords"
                :key="record.name"
                :type="record.status === 'active' ? 'success' : record.status === 'error' ? 'danger' : 'warning'"
                size="small"
                class="dns-tag"
              >
                {{ record.type }}: {{ record.name }}
              </el-tag>
              <span v-if="domain.dnsRecords.length === 0" class="text-muted">无记录</span>
            </div>
          </div>

          <!-- SSL Status -->
          <div class="domain-info">
            <span class="domain-info__label">SSL 证书</span>
            <div class="domain-info__value">
              <el-tag
                :type="getSslStatusType(domain.sslStatus)"
                size="small"
              >
                {{ getSslStatusLabel(domain.sslStatus) }}
              </el-tag>
              <span v-if="domain.sslExpiry" class="ssl-expiry">
                到期时间：{{ formatDate(domain.sslExpiry, 'date') }}
              </span>
            </div>
          </div>
        </div>

        <div class="domain-card__footer">
          <el-button
            type="primary"
            size="small"
            :loading="syncingIds.includes(domain.id)"
            @click="handleSyncDns(domain)"
          >
            <el-icon><Refresh /></el-icon>
            <span>同步 DNS</span>
          </el-button>
          <el-button
            v-if="domain.sslStatus === 'none' || domain.sslStatus === 'expired'"
            type="success"
            size="small"
            :loading="certIds.includes(domain.id)"
            @click="handleRequestCert(domain)"
          >
            <el-icon><Lock /></el-icon>
            <span>申请证书</span>
          </el-button>
          <el-button
            v-if="domain.sslStatus === 'active'"
            type="warning"
            size="small"
            :loading="certIds.includes(domain.id)"
            @click="handleRenewCert(domain)"
          >
            <el-icon><RefreshRight /></el-icon>
            <span>续期证书</span>
          </el-button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="!loading && domains.length === 0" class="empty-state">
        <el-icon class="empty-state__icon"><Compass /></el-icon>
        <h3 class="empty-state__title">暂无域名</h3>
        <p class="empty-state__description">域名将在创建 HTTP/HTTPS 映射时自动添加</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Compass, Refresh, Lock, RefreshRight } from '@element-plus/icons-vue'
import StatusBadge from '@/components/StatusBadge.vue'
import * as domainsApi from '@/api/domains'
import { useNotification } from '@/composables/useNotification'
import type { Domain, DomainStatus } from '@/types'
import { formatDate, getStatusLabel, getStatusType } from '@/utils/format'

const { success, error } = useNotification()

// State
const domains = ref<Domain[]>([])
const loading = ref(false)
const syncingIds = ref<number[]>([])
const certIds = ref<number[]>([])

// Fetch domains
async function fetchDomains() {
  loading.value = true
  try {
    domains.value = await domainsApi.getDomains()
  } catch (err) {
    console.error('Failed to fetch domains:', err)
  } finally {
    loading.value = false
  }
}

// Sync DNS
async function handleSyncDns(domain: Domain) {
  syncingIds.value.push(domain.id)
  try {
    await domainsApi.syncDomainDns(domain.id)
    success(`DNS 同步已启动：${domain.domain}`)
    await fetchDomains()
  } catch (err: any) {
    error(err.message || 'DNS 同步失败')
  } finally {
    syncingIds.value = syncingIds.value.filter((id) => id !== domain.id)
  }
}

// Request certificate
async function handleRequestCert(domain: Domain) {
  certIds.value.push(domain.id)
  try {
    await domainsApi.requestCertificate(domain.id)
    success(`证书申请已启动：${domain.domain}`)
    await fetchDomains()
  } catch (err: any) {
    error(err.message || '证书申请失败')
  } finally {
    certIds.value = certIds.value.filter((id) => id !== domain.id)
  }
}

// Renew certificate
async function handleRenewCert(domain: Domain) {
  certIds.value.push(domain.id)
  try {
    await domainsApi.renewCertificate(domain.id)
    success(`证书续期已启动：${domain.domain}`)
    await fetchDomains()
  } catch (err: any) {
    error(err.message || '证书续期失败')
  } finally {
    certIds.value = certIds.value.filter((id) => id !== domain.id)
  }
}

// Get SSL status type
function getSslStatusType(status: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const typeMap: Record<string, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    active: 'success',
    pending: 'warning',
    error: 'danger',
    expired: 'danger',
    none: 'info',
  }
  return typeMap[status] || 'info'
}

// Get SSL status label
function getSslStatusLabel(status: string): string {
  const labelMap: Record<string, string> = {
    active: '有效',
    pending: '申请中',
    error: '错误',
    expired: '已过期',
    none: '未配置',
  }
  return labelMap[status] || status
}

// Initialize
onMounted(() => {
  fetchDomains()
})
</script>

<style scoped>
.domain-list {
  max-width: 1200px;
  margin: 0 auto;
}

.domain-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 16px;
}

.domain-card {
  background: #FFFFFF;
  border: 1px solid #E4E4E7;
  border-radius: 8px;
  overflow: hidden;
  transition: all 0.2s ease;
}

.domain-card:hover {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1);
}

.domain-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #E4E4E7;
}

.domain-card__domain {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #18181B;
}

.domain-card__body {
  padding: 16px 20px;
}

.domain-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.domain-info:last-child {
  margin-bottom: 0;
}

.domain-info__label {
  font-size: 12px;
  font-weight: 500;
  color: #71717A;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.domain-info__value {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.dns-tag {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
  font-size: 11px;
}

.ssl-expiry {
  font-size: 12px;
  color: #71717A;
}

.domain-card__footer {
  display: flex;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid #E4E4E7;
  background: #FAFAFA;
}

.domain-card__footer .el-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.empty-state {
  grid-column: 1 / -1;
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
  margin: 0;
  max-width: 400px;
}

/* Responsive */
@media (max-width: 768px) {
  .domain-grid {
    grid-template-columns: 1fr;
  }

  .domain-card__footer {
    flex-wrap: wrap;
  }
}
</style>