<template>
  <div class="layout">
    <Sidebar :collapsed="appStore.sidebarCollapsed" />

    <div
      :class="['layout__main', { 'layout__main--collapsed': appStore.sidebarCollapsed }]"
    >
      <Header />

      <main class="layout__content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>

    <!-- Mobile overlay -->
    <div
      v-if="appStore.isMobile && !appStore.sidebarCollapsed"
      class="layout__overlay"
      @click="appStore.toggleSidebar()"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import Sidebar from '@/components/Sidebar.vue'
import Header from '@/components/Header.vue'
import { useAppStore } from '@/stores/app'
import { useFrpcStore } from '@/stores/frpc'
import { useWebSocket } from '@/composables/useWebSocket'

const appStore = useAppStore()
const frpcStore = useFrpcStore()

// Initialize WebSocket connection
const { isConnected: wsConnected } = useWebSocket({
  onConnected: () => {
    console.log('[Layout] WebSocket connected')
  },
  onDisconnected: () => {
    console.log('[Layout] WebSocket disconnected')
  },
})

// Fetch initial FRPC status
onMounted(async () => {
  await frpcStore.fetchStatus()
})

// Handle resize
function handleResize() {
  appStore.updateMobileState()
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.layout {
  display: flex;
  min-height: 100vh;
  background: #F4F4F5;
}

.layout__main {
  flex: 1;
  margin-left: 240px;
  transition: margin-left 0.2s ease;
  display: flex;
  flex-direction: column;
}

.layout__main--collapsed {
  margin-left: 64px;
}

.layout__content {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}

.layout__overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 150;
}

/* Fade transition */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Mobile styles */
@media (max-width: 768px) {
  .layout__main {
    margin-left: 0;
  }

  .layout__main--collapsed {
    margin-left: 0;
  }

  .layout__content {
    padding: 16px;
  }
}
</style>