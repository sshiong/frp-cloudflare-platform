import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/pages/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: '登录',
    },
  },
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: '',
        redirect: '/dashboard',
      },
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/pages/DashboardView.vue'),
        meta: {
          title: '仪表盘',
          icon: 'Monitor',
        },
      },
      {
        path: 'mappings',
        name: 'Mappings',
        component: () => import('@/pages/MappingList.vue'),
        meta: {
          title: '端口映射',
          icon: 'Connection',
        },
      },
      {
        path: 'mappings/create',
        name: 'CreateMapping',
        component: () => import('@/pages/CreateMapping.vue'),
        meta: {
          title: '创建映射',
          icon: 'Plus',
          parent: 'Mappings',
        },
      },
      {
        path: 'mappings/:id/edit',
        name: 'EditMapping',
        component: () => import('@/pages/CreateMapping.vue'),
        meta: {
          title: '编辑映射',
          icon: 'Edit',
          parent: 'Mappings',
        },
      },
      {
        path: 'domains',
        name: 'Domains',
        component: () => import('@/pages/DomainList.vue'),
        meta: {
          title: '域名管理',
          icon: 'Globe',
        },
      },
      {
        path: 'frpc/logs',
        name: 'FrpLogs',
        component: () => import('@/pages/FrpLogs.vue'),
        meta: {
          title: 'FRPC 日志',
          icon: 'Document',
        },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/pages/Settings.vue'),
        meta: {
          title: '系统设置',
          icon: 'Setting',
        },
      },
      {
        path: 'server',
        name: 'ServerConfig',
        component: () => import('@/pages/ServerConfig.vue'),
        meta: {
          title: '服务器配置',
          icon: 'Server',
        },
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
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    }
    return { top: 0 }
  },
})

// Navigation guard
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  // Set page title
  const title = to.meta.title as string
  document.title = title ? `${title} - FRP Client Panel` : 'FRP Client Panel'

  // Check auth requirement
  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth !== false)

  if (requiresAuth && !authStore.isAuthenticated) {
    // Redirect to login with return URL
    next({
      path: '/login',
      query: { redirect: to.fullPath },
    })
  } else if (to.path === '/login' && authStore.isAuthenticated) {
    // Redirect to dashboard if already logged in
    next('/dashboard')
  } else {
    next()
  }
})

export default router