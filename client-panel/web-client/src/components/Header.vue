<template>
  <header class="header">
    <div class="header__left">
      <el-button
        class="header__menu-btn"
        :icon="Expand"
        text
        @click="appStore.toggleSidebar()"
      />
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/dashboard' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item v-if="currentRoute.meta.parent">
          {{ currentRoute.meta.parent }}
        </el-breadcrumb-item>
        <el-breadcrumb-item>{{ currentRoute.meta.title }}</el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="header__right">
      <div class="header__status">
        <div class="status-item" :class="{ 'status-item--connected': frpcStore.isConnected }">
          <span class="status-dot" />
          <span class="status-text">{{ frpcStore.isConnected ? '已连接' : '未连接' }}</span>
        </div>
      </div>

      <el-dropdown trigger="click" @command="handleCommand">
        <div class="header__user">
          <el-avatar :size="32" class="header__avatar">
            {{ userInitial }}
          </el-avatar>
          <span class="header__username">{{ authStore.username }}</span>
          <el-icon><ArrowDown /></el-icon>
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="settings">
              <el-icon><Setting /></el-icon>
              <span>系统设置</span>
            </el-dropdown-item>
            <el-dropdown-item command="profile">
              <el-icon><User /></el-icon>
              <span>个人信息</span>
            </el-dropdown-item>
            <el-dropdown-item divided command="logout">
              <el-icon><SwitchButton /></el-icon>
              <span>退出登录</span>
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Expand, ArrowDown, Setting, User, SwitchButton } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useFrpcStore } from '@/stores/frpc'
import { useNotification } from '@/composables/useNotification'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const frpcStore = useFrpcStore()
const { confirm } = useNotification()

const currentRoute = computed(() => route)

const userInitial = computed(() => {
  const username = authStore.username
  return username ? username.charAt(0).toUpperCase() : 'U'
})

async function handleCommand(command: string) {
  switch (command) {
    case 'settings':
      router.push('/settings')
      break
    case 'profile':
      // Navigate to profile or show profile dialog
      break
    case 'logout':
      const confirmed = await confirm('确定要退出登录吗？', '退出登录')
      if (confirmed) {
        authStore.logout()
      }
      break
  }
}
</script>

<style scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
  background: #FFFFFF;
  border-bottom: 1px solid #E4E4E7;
  position: sticky;
  top: 0;
  z-index: 100;
}

.header__left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header__menu-btn {
  font-size: 20px;
  color: #71717A;
}

.header__right {
  display: flex;
  align-items: center;
  gap: 24px;
}

.header__status {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  background: #F4F4F5;
  font-size: 12px;
  color: #71717A;
}

.status-item--connected {
  background: #ECFDF5;
  color: #16A34A;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #71717A;
}

.status-item--connected .status-dot {
  background: #16A34A;
  box-shadow: 0 0 0 2px rgba(22, 163, 74, 0.2);
}

.status-text {
  font-weight: 500;
}

.header__user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  transition: background-color 0.2s;
}

.header__user:hover {
  background: #F4F4F5;
}

.header__avatar {
  background: #2563EB;
  color: #FFFFFF;
  font-weight: 600;
}

.header__username {
  font-size: 14px;
  font-weight: 500;
  color: #18181B;
}

@media (max-width: 768px) {
  .header {
    padding: 0 16px;
  }

  .header__username {
    display: none;
  }
}
</style>