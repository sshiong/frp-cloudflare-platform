<template>
  <aside :class="['sidebar', { 'sidebar--collapsed': collapsed }]">
    <div class="sidebar__header">
      <div class="sidebar__logo">
        <div class="sidebar__logo-icon">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 2L2 7L12 12L22 7L12 2Z" fill="#2563EB"/>
            <path d="M2 17L12 22L22 17" stroke="#2563EB" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M2 12L12 17L22 12" stroke="#2563EB" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <span v-if="!collapsed" class="sidebar__logo-text">FRP Client</span>
      </div>
    </div>

    <nav class="sidebar__nav">
      <router-link
        v-for="item in menuItems"
        :key="item.path"
        :to="item.path"
        :class="['sidebar__item', { 'sidebar__item--active': isActive(item.path) }]"
        @click="handleMenuClick"
      >
        <el-icon :size="20">
          <component :is="item.icon" />
        </el-icon>
        <span v-if="!collapsed" class="sidebar__item-text">{{ item.label }}</span>
        <span v-if="!collapsed && item.badge" class="sidebar__item-badge">
          {{ item.badge }}
        </span>
      </router-link>
    </nav>

    <div class="sidebar__footer">
      <div class="sidebar__server-status">
        <div :class="['server-indicator', { 'server-indicator--connected': isConnected }]">
          <span class="server-indicator__dot" />
          <span v-if="!collapsed" class="server-indicator__text">
            {{ isConnected ? '服务器已连接' : '服务器未连接' }}
          </span>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Monitor,
  Connection,
  Plus,
  Globe,
  Document,
  Setting,
  Server,
} from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { useFrpcStore } from '@/stores/frpc'

interface Props {
  collapsed: boolean
}

const props = defineProps<Props>()

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const frpcStore = useFrpcStore()

const isConnected = computed(() => frpcStore.isConnected)

const menuItems = computed(() => [
  {
    path: '/dashboard',
    label: '仪表盘',
    icon: Monitor,
  },
  {
    path: '/mappings',
    label: '端口映射',
    icon: Connection,
    badge: frpcStore.status.proxyCount > 0 ? frpcStore.status.proxyCount : undefined,
  },
  {
    path: '/mappings/create',
    label: '创建映射',
    icon: Plus,
  },
  {
    path: '/domains',
    label: '域名管理',
    icon: Globe,
  },
  {
    path: '/frpc/logs',
    label: 'FRPC 日志',
    icon: Document,
  },
  {
    path: '/settings',
    label: '系统设置',
    icon: Setting,
  },
  {
    path: '/server',
    label: '服务器配置',
    icon: Server,
  },
])

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

function handleMenuClick() {
  // Close mobile sidebar when menu item is clicked
  if (appStore.isMobile) {
    appStore.closeMobileSidebar()
  }
}
</script>

<style scoped>
.sidebar {
  width: 240px;
  height: 100vh;
  background: #FFFFFF;
  border-right: 1px solid #E4E4E7;
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
  position: fixed;
  left: 0;
  top: 0;
  z-index: 200;
}

.sidebar--collapsed {
  width: 64px;
}

.sidebar__header {
  padding: 16px;
  border-bottom: 1px solid #E4E4E7;
}

.sidebar__logo {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
}

.sidebar__logo-icon {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
}

.sidebar__logo-icon svg {
  width: 100%;
  height: 100%;
}

.sidebar__logo-text {
  font-size: 16px;
  font-weight: 700;
  color: #18181B;
  white-space: nowrap;
}

.sidebar__nav {
  flex: 1;
  padding: 12px 8px;
  overflow-y: auto;
}

.sidebar__item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  margin-bottom: 4px;
  border-radius: 6px;
  color: #71717A;
  text-decoration: none;
  transition: all 0.2s ease;
  position: relative;
}

.sidebar__item:hover {
  background: #F4F4F5;
  color: #18181B;
}

.sidebar__item--active {
  background: #DBEAFE;
  color: #2563EB;
}

.sidebar__item--active:hover {
  background: #BFDBFE;
  color: #2563EB;
}

.sidebar__item-text {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
}

.sidebar__item-badge {
  margin-left: auto;
  padding: 2px 6px;
  background: #2563EB;
  color: #FFFFFF;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 600;
  min-width: 18px;
  text-align: center;
}

.sidebar__footer {
  padding: 16px;
  border-top: 1px solid #E4E4E7;
}

.sidebar__server-status {
  display: flex;
  align-items: center;
  justify-content: center;
}

.server-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  background: #F4F4F5;
  width: 100%;
}

.server-indicator--connected {
  background: #ECFDF5;
}

.server-indicator__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #71717A;
  flex-shrink: 0;
}

.server-indicator--connected .server-indicator__dot {
  background: #16A34A;
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.2);
}

.server-indicator__text {
  font-size: 12px;
  font-weight: 500;
  color: #71717A;
  white-space: nowrap;
}

.server-indicator--connected .server-indicator__text {
  color: #16A34A;
}

/* Collapsed state adjustments */
.sidebar--collapsed .sidebar__logo {
  justify-content: center;
  padding: 8px 0;
}

.sidebar--collapsed .sidebar__item {
  justify-content: center;
  padding: 10px;
}

.sidebar--collapsed .sidebar__server-status {
  justify-content: center;
}

.sidebar--collapsed .server-indicator {
  justify-content: center;
  padding: 8px;
}

/* Mobile overlay */
@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
    transition: transform 0.3s ease;
  }

  .sidebar--mobile-open {
    transform: translateX(0);
  }
}
</style>