<template>
  <div class="layout-container">
    <Sidebar />
    <div class="layout-main" :style="{ marginLeft: appStore.sidebarWidth + 'px' }">
      <Header />
      <div class="layout-content">
        <Breadcrumb />
        <div class="page-wrapper">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Sidebar from '@/components/Sidebar.vue'
import Header from '@/components/Header.vue'
import Breadcrumb from '@/components/Breadcrumb.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { onMounted } from 'vue'

const appStore = useAppStore()
const authStore = useAuthStore()

onMounted(async () => {
  // Initialize CSRF token and fetch user info
  await authStore.initCsrfToken()
  if (authStore.isAuthenticated) {
    await authStore.fetchUserInfo()
  }
})
</script>

<style scoped>
.layout-container {
  min-height: 100vh;
  background: var(--color-bg-secondary);
}

.layout-main {
  min-height: 100vh;
  transition: margin-left var(--transition-normal);
  display: flex;
  flex-direction: column;
}

.layout-content {
  flex: 1;
  padding: 0;
  overflow: hidden;
}

.page-wrapper {
  height: calc(100vh - var(--header-height) - 40px);
  overflow-y: auto;
  padding: 0 var(--spacing-lg) var(--spacing-lg);
}
</style>
