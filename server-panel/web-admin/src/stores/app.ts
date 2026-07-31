import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAppStore = defineStore('app', () => {
  // Sidebar
  const sidebarCollapsed = ref(false)
  const sidebarWidth = computed(() => sidebarCollapsed.value ? 64 : 240)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
    localStorage.setItem('sidebar_collapsed', String(sidebarCollapsed.value))
  }

  function setSidebarCollapsed(collapsed: boolean) {
    sidebarCollapsed.value = collapsed
    localStorage.setItem('sidebar_collapsed', String(collapsed))
  }

  // Restore sidebar state
  const savedCollapsed = localStorage.getItem('sidebar_collapsed')
  if (savedCollapsed === 'true') {
    sidebarCollapsed.value = true
  }

  // Breadcrumb
  const breadcrumbs = ref<Array<{ title: string; path?: string }>>([])

  function setBreadcrumbs(items: Array<{ title: string; path?: string }>) {
    breadcrumbs.value = items
  }

  // Loading
  const globalLoading = ref(false)
  const globalLoadingText = ref('')

  function showLoading(text: string = '加载中...') {
    globalLoading.value = true
    globalLoadingText.value = text
  }

  function hideLoading() {
    globalLoading.value = false
    globalLoadingText.value = ''
  }

  // Page title
  const pageTitle = ref('FRP 云隧道管理平台')

  function setPageTitle(title: string) {
    pageTitle.value = title ? `${title} - FRP 云隧道管理平台` : 'FRP 云隧道管理平台'
    document.title = pageTitle.value
  }

  return {
    sidebarCollapsed,
    sidebarWidth,
    toggleSidebar,
    setSidebarCollapsed,
    breadcrumbs,
    setBreadcrumbs,
    globalLoading,
    globalLoadingText,
    showLoading,
    hideLoading,
    pageTitle,
    setPageTitle,
  }
})
