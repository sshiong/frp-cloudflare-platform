import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { DashboardStats } from '@/types'

export const useAppStore = defineStore('app', () => {
  // State
  const sidebarCollapsed = ref(false)
  const loading = ref(false)
  const globalError = ref<string | null>(null)
  const dashboardStats = ref<DashboardStats | null>(null)

  // UI State
  const isMobile = ref(window.innerWidth < 768)
  const showMobileSidebar = ref(false)

  // Getters
  const sidebarWidth = computed(() => {
    if (isMobile.value) return '0px'
    return sidebarCollapsed.value ? '64px' : '240px'
  })

  // Actions
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setSidebarCollapsed(collapsed: boolean) {
    sidebarCollapsed.value = collapsed
  }

  function toggleMobileSidebar() {
    showMobileSidebar.value = !showMobileSidebar.value
  }

  function closeMobileSidebar() {
    showMobileSidebar.value = false
  }

  function setLoading(value: boolean) {
    loading.value = value
  }

  function setGlobalError(error: string | null) {
    globalError.value = error
  }

  function clearGlobalError() {
    globalError.value = null
  }

  function updateDashboardStats(stats: DashboardStats) {
    dashboardStats.value = stats
  }

  function updateMobileState() {
    isMobile.value = window.innerWidth < 768
    if (!isMobile.value) {
      showMobileSidebar.value = false
    }
  }

  // Initialize resize listener
  function initResizeListener() {
    window.addEventListener('resize', updateMobileState)
  }

  return {
    // State
    sidebarCollapsed,
    loading,
    globalError,
    dashboardStats,
    isMobile,
    showMobileSidebar,

    // Getters
    sidebarWidth,

    // Actions
    toggleSidebar,
    setSidebarCollapsed,
    toggleMobileSidebar,
    closeMobileSidebar,
    setLoading,
    setGlobalError,
    clearGlobalError,
    updateDashboardStats,
    updateMobileState,
    initResizeListener,
  }
})