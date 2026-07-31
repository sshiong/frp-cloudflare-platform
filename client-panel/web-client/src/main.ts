import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router'
import './styles/global.css'

// Create app instance
const app = createApp(App)

// Create Pinia instance
const pinia = createPinia()

// Use plugins
app.use(pinia)
app.use(router)
app.use(ElementPlus, {
  locale: undefined, // Use default Chinese locale
})

// Register all Element Plus icons
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

// Mount app
app.mount('#app')

// Initialize auth store after app is mounted
import { useAuthStore } from './stores/auth'
import { useAppStore } from './stores/app'

const authStore = useAuthStore()
const appStore = useAppStore()

// Initialize auth state from localStorage
authStore.initAuth()

// Initialize resize listener
appStore.initResizeListener()

// Log app info
console.log('%c FRP Client Panel %c v1.0.0 ', 'background:#2563EB;color:#fff;border-radius:4px 0 0 4px;padding:4px 8px', 'background:#18181B;color:#fff;border-radius:0 4px 4px 0;padding:4px 8px')