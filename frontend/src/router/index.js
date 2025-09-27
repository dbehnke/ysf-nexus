import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Repeaters from '@/views/Repeaters.vue'
import TalkLogs from '@/views/TalkLogs.vue'
import Settings from '@/views/Settings.vue'
import Login from '@/views/Login.vue'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard
  },
  {
    path: '/repeaters',
    name: 'Repeaters',
    component: Repeaters
  },
  {
    path: '/logs',
    name: 'TalkLogs',
    component: TalkLogs
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings,
    meta: { requiresAuth: true }
  },
  {
    path: '/login',
    name: 'Login',
    component: Login
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Navigation guard for authentication
router.beforeEach(async (to, from, next) => {
  // Import auth store dynamically to avoid circular dependency
  const { useAuthStore } = await import('@/stores/auth')
  const authStore = useAuthStore()

  // Check auth status on first navigation
  if (!authStore.authRequired) {
    await authStore.checkAuthStatus()
  }

  // Handle routes that require authentication
  if (to.meta.requiresAuth) {
    if (authStore.needsAuth) {
      // Redirect to login with return path
      next({
        name: 'Login',
        query: { redirect: to.fullPath }
      })
      return
    }
  }

  // Don't allow access to login page if already authenticated
  if (to.name === 'Login' && authStore.isAuthenticated) {
    next({ name: 'Dashboard' })
    return
  }

  next()
})

export default router