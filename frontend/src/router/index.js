import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import Repeaters from '@/views/Repeaters.vue'
import TalkLogs from '@/views/TalkLogs.vue'
import Settings from '@/views/Settings.vue'

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
    component: Settings
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router