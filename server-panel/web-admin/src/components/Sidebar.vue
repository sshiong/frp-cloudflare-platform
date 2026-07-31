<template>
  <div class="sidebar" :class="{ collapsed: appStore.sidebarCollapsed }">
    <!-- Logo -->
    <div class="sidebar-logo">
      <div class="logo-icon">
        <el-icon :size="24"><Connection /></el-icon>
      </div>
      <transition name="fade">
        <span v-show="!appStore.sidebarCollapsed" class="logo-text">FRP 管理平台</span>
      </transition>
    </div>

    <!-- Navigation -->
    <el-scrollbar class="sidebar-menu-scrollbar">
      <el-menu
        :default-active="activeMenu"
        :collapse="appStore.sidebarCollapsed"
        :collapse-transition="false"
        router
        background-color="transparent"
        text-color="#A1A1AA"
        active-text-color="#FFFFFF"
        class="sidebar-menu"
      >
        <!-- Main Navigation -->
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <template #title>仪表盘</template>
        </el-menu-item>

        <!-- User Section -->
        <el-menu-item index="/devices">
          <el-icon><Cpu /></el-icon>
          <template #title>设备管理</template>
        </el-menu-item>

        <el-menu-item index="/mappings">
          <el-icon><Connection /></el-icon>
          <template #title>映射管理</template>
        </el-menu-item>

        <el-menu-item index="/domains">
          <el-icon><Link /></el-icon>
          <template #title>域名管理</template>
        </el-menu-item>

        <el-menu-item index="/cloudflare">
          <el-icon><Cloudy /></el-icon>
          <template #title>Cloudflare</template>
        </el-menu-item>

        <el-menu-item index="/certificates">
          <el-icon><Lock /></el-icon>
          <template #title>证书管理</template>
        </el-menu-item>

        <el-menu-item index="/operations">
          <el-icon><List /></el-icon>
          <template #title>操作历史</template>
        </el-menu-item>

        <!-- Admin Section -->
        <template v-if="authStore.isAdmin">
          <div class="menu-divider">
            <transition name="fade">
              <span v-show="!appStore.sidebarCollapsed" class="divider-label">系统管理</span>
            </transition>
          </div>

          <el-menu-item index="/admin/users">
            <el-icon><User /></el-icon>
            <template #title>用户管理</template>
          </el-menu-item>

          <el-menu-item index="/admin/audit">
            <el-icon><Document /></el-icon>
            <template #title>审计日志</template>
          </el-menu-item>

          <el-menu-item index="/admin/system">
            <el-icon><Monitor /></el-icon>
            <template #title>系统状态</template>
          </el-menu-item>

          <el-menu-item index="/admin/backups">
            <el-icon><FolderOpened /></el-icon>
            <template #title>备份恢复</template>
          </el-menu-item>

          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <template #title>系统设置</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-scrollbar>

    <!-- Collapse Button -->
    <div class="sidebar-footer" @click="appStore.toggleSidebar">
      <el-icon :size="18">
        <Fold v-if="!appStore.sidebarCollapsed" />
        <Expand v-else />
      </el-icon>
      <transition name="fade">
        <span v-show="!appStore.sidebarCollapsed" class="collapse-text">收起菜单</span>
      </transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

const activeMenu = computed(() => {
  return route.path
})
</script>

<style scoped>
.sidebar {
  position: fixed;
  left: 0;
  top: 0;
  bottom: 0;
  width: var(--sidebar-width);
  background: var(--sidebar-bg);
  display: flex;
  flex-direction: column;
  transition: width var(--transition-normal);
  z-index: 100;
  overflow: hidden;
}

.sidebar.collapsed {
  width: var(--sidebar-collapsed-width);
}

.sidebar-logo {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
  overflow: hidden;
}

.logo-icon {
  width: 32px;
  height: 32px;
  background: var(--color-primary);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.logo-text {
  margin-left: 12px;
  font-size: 15px;
  font-weight: 600;
  color: #FFFFFF;
  white-space: nowrap;
  letter-spacing: 0.02em;
}

.sidebar-menu-scrollbar {
  flex: 1;
  overflow: hidden;
}

.sidebar-menu {
  border-right: none;
  padding: 8px;
}

.sidebar-menu:not(.el-menu--collapse) {
  width: 100%;
}

.sidebar-menu .el-menu-item {
  height: 40px;
  line-height: 40px;
  margin-bottom: 2px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
}

.sidebar-menu .el-menu-item:hover {
  background: var(--sidebar-hover) !important;
}

.sidebar-menu .el-menu-item.is-active {
  background: var(--sidebar-active) !important;
  color: #FFFFFF !important;
}

.sidebar-menu .el-menu-item .el-icon {
  font-size: 18px;
  margin-right: 10px;
}

.menu-divider {
  padding: 16px 16px 8px;
  overflow: hidden;
}

.divider-label {
  font-size: 11px;
  font-weight: 600;
  color: rgba(161, 161, 170, 0.6);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  white-space: nowrap;
}

.sidebar-footer {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  cursor: pointer;
  color: var(--sidebar-text);
  transition: all var(--transition-fast);
  flex-shrink: 0;
  gap: 8px;
}

.sidebar-footer:hover {
  color: #FFFFFF;
  background: var(--sidebar-hover);
}

.collapse-text {
  font-size: 13px;
  white-space: nowrap;
}

/* Collapse menu adjustments */
.sidebar.collapsed .sidebar-menu .el-menu-item {
  padding: 0 !important;
  display: flex;
  justify-content: center;
}

.sidebar.collapsed .sidebar-menu .el-menu-item .el-icon {
  margin-right: 0;
}

.sidebar.collapsed .sidebar-footer {
  justify-content: center;
}
</style>
