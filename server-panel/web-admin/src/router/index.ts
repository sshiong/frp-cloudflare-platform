import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/LoginView.vue'),
    meta: { requiresAuth: false, title: '登录' },
  },
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/dashboard',
      },
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/pages/DashboardView.vue'),
        meta: { title: '仪表盘', icon: 'Odometer' },
      },
      {
        path: 'admin/users',
        name: 'UserManagement',
        component: () => import('@/pages/admin/UserManagement.vue'),
        meta: { title: '用户管理', icon: 'User', requiresAdmin: true },
      },
      {
        path: 'admin/audit',
        name: 'AuditLog',
        component: () => import('@/pages/admin/AuditLog.vue'),
        meta: { title: '审计日志', icon: 'Document', requiresAdmin: true },
      },
      {
        path: 'admin/system',
        name: 'SystemStatus',
        component: () => import('@/pages/admin/SystemStatus.vue'),
        meta: { title: '系统状态', icon: 'Monitor', requiresAdmin: true },
      },
      {
        path: 'admin/backups',
        name: 'BackupRestore',
        component: () => import('@/pages/admin/BackupRestore.vue'),
        meta: { title: '备份恢复', icon: 'FolderOpened', requiresAdmin: true },
      },
      {
        path: 'devices',
        name: 'DeviceManagement',
        component: () => import('@/pages/DeviceManagement.vue'),
        meta: { title: '设备管理', icon: 'Cpu' },
      },
      {
        path: 'mappings',
        name: 'MappingManagement',
        component: () => import('@/pages/MappingManagement.vue'),
        meta: { title: '映射管理', icon: 'Connection' },
      },
      {
        path: 'domains',
        name: 'DomainManagement',
        component: () => import('@/pages/DomainManagement.vue'),
        meta: { title: '域名管理', icon: 'Link' },
      },
      {
        path: 'cloudflare',
        name: 'CloudflareSettings',
        component: () => import('@/pages/CloudflareSettings.vue'),
        meta: { title: 'Cloudflare 配置', icon: 'Cloudy' },
      },
      {
        path: 'certificates',
        name: 'CertificateManagement',
        component: () => import('@/pages/CertificateManagement.vue'),
        meta: { title: '证书管理', icon: 'Lock' },
      },
      {
        path: 'operations',
        name: 'OperationHistory',
        component: () => import('@/pages/OperationHistory.vue'),
        meta: { title: '操作历史', icon: 'List' },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/pages/Settings.vue'),
        meta: { title: '系统设置', icon: 'Setting', requiresAdmin: true },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Navigation guard
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Set page title
  const title = to.meta.title as string
  if (title) {
    document.title = `${title} - FRP 云隧道管理平台`
  }

  // Check if route requires authentication
  if (to.meta.requiresAuth !== false) {
    if (!authStore.isAuthenticated) {
      next({ path: '/login', query: { redirect: to.fullPath } })
      return
    }

    // Check admin requirement
    if (to.meta.requiresAdmin && !authStore.isAdmin) {
      next({ path: '/dashboard' })
      return
    }
  }

  // If authenticated and going to login, redirect to dashboard
  if (to.path === '/login' && authStore.isAuthenticated) {
    next({ path: '/dashboard' })
    return
  }

  next()
})

export default router
