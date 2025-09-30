<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Bridges</h1>
        <p class="text-gray-600 dark:text-gray-400">Inter-reflector bridge connections and status</p>
      </div>
      <div class="flex items-center space-x-3">
        <div class="flex items-center space-x-2">
          <span class="text-sm text-gray-600 dark:text-gray-400">{{ Object.keys(bridges).length }} total</span>
          <span class="badge-success">{{ connectedBridges.length }} connected</span>
          <span class="badge-warning">{{ scheduledBridges.length }} scheduled</span>
        </div>
        <button @click="refreshData" :disabled="loading" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Bridges Table -->
    <div class="card">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead class="table-header">
            <tr>
              <th class="table-header-cell">
                Status
              </th>
              <th class="table-header-cell">
                Name
              </th>
              <th class="table-header-cell">
                Remote Host
              </th>
              <th class="table-header-cell">
                Type
              </th>
              <th class="table-header-cell">
                Connected
              </th>
              <th class="table-header-cell">
                Next Schedule
              </th>
              <th class="table-header-cell">
                Packets
              </th>
              <th class="table-header-cell">
                Data Transfer
              </th>
              <th class="table-header-cell">
                Retry Count
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <tr v-for="(bridge, name) in bridges" :key="name" class="table-row">
              <!-- Status -->
              <td class="table-cell">
                <span :class="getStatusBadgeClass(bridge.state)" class="badge">
                  {{ getStatusText(bridge.state) }}
                </span>
              </td>
              <!-- Name -->
              <td class="table-cell">
                <div class="font-medium text-gray-900 dark:text-white">{{ name }}</div>
              </td>
              <!-- Remote Host -->
              <td class="table-cell">
                <div class="text-sm text-gray-900 dark:text-gray-300">{{ getRemoteHost(bridge) }}</div>
              </td>
              <!-- Type -->
              <td class="table-cell">
                <span :class="getTypeBadgeClass(bridge)" class="badge">
                  {{ getTypeText(bridge) }}
                </span>
              </td>
              <!-- Connected Time -->
              <td class="table-cell">
                <div v-if="bridge.connected_at" class="text-sm">
                  <div class="text-gray-900 dark:text-gray-300">{{ formatDateTime(bridge.connected_at) }}</div>
                  <div class="text-xs text-gray-500">{{ getUptime(bridge.connected_at) }}</div>
                </div>
                <div v-else-if="bridge.disconnected_at" class="text-sm text-gray-500">
                  Disconnected {{ formatDateTime(bridge.disconnected_at) }}
                </div>
                <div v-else class="text-sm text-gray-500">Never connected</div>
              </td>
              <!-- Next Schedule -->
              <td class="table-cell">
                <div v-if="bridge.next_schedule" class="text-sm text-gray-900 dark:text-gray-300">
                  {{ formatDateTime(bridge.next_schedule) }}
                </div>
                <div v-else class="text-sm text-gray-500">—</div>
              </td>
              <!-- Packets -->
              <td class="table-cell">
                <div class="text-sm">
                  <div class="text-gray-900 dark:text-gray-300">↓ {{ formatNumber(bridge.packets_rx) }}</div>
                  <div class="text-gray-900 dark:text-gray-300">↑ {{ formatNumber(bridge.packets_tx) }}</div>
                </div>
              </td>
              <!-- Data Transfer -->
              <td class="table-cell">
                <div class="text-sm">
                  <div class="text-gray-900 dark:text-gray-300">↓ {{ formatBytes(bridge.bytes_rx) }}</div>
                  <div class="text-gray-900 dark:text-gray-300">↑ {{ formatBytes(bridge.bytes_tx) }}</div>
                </div>
              </td>
              <!-- Retry Count -->
              <td class="table-cell">
                <div class="text-sm">
                  <span v-if="bridge.retry_count > 0" class="text-orange-600 dark:text-orange-400">
                    {{ bridge.retry_count }}
                  </span>
                  <span v-else class="text-gray-500">—</span>
                  <div v-if="bridge.last_error" class="text-xs text-red-500 mt-1" :title="bridge.last_error">
                    Error
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        
        <!-- Empty State -->
        <div v-if="Object.keys(bridges).length === 0" class="text-center py-12">
          <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
          </svg>
          <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">No bridges configured</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Configure bridge connections in your server configuration.</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const bridges = ref({})
const loading = ref(false)
let refreshInterval = null

const connectedBridges = computed(() => {
  return Object.values(bridges.value).filter(bridge => bridge.state === 'connected')
})

const scheduledBridges = computed(() => {
  return Object.values(bridges.value).filter(bridge => bridge.state === 'scheduled')
})

const refreshData = async () => {
  if (loading.value) return
  
  loading.value = true
  try {
    const response = await fetch('/api/bridges')
    const data = await response.json()
    bridges.value = data.bridges || {}
  } catch (error) {
    console.error('Failed to fetch bridge data:', error)
  } finally {
    loading.value = false
  }
}

const getStatusBadgeClass = (state) => {
  switch (state) {
    case 'connected': return 'badge-success'
    case 'connecting': return 'badge-warning'
    case 'scheduled': return 'badge-info'
    case 'failed': return 'badge-error'
    case 'disconnected': return 'badge-secondary'
    default: return 'badge-secondary'
  }
}

const getStatusText = (state) => {
  switch (state) {
    case 'connected': return 'Connected'
    case 'connecting': return 'Connecting'
    case 'scheduled': return 'Scheduled'
    case 'failed': return 'Failed'
    case 'disconnected': return 'Disconnected'
    default: return 'Unknown'
  }
}

const getTypeBadgeClass = (bridge) => {
  // Determine if permanent based on whether it has schedule info
  const isPermanent = !bridge.next_schedule
  return isPermanent ? 'badge-success' : 'badge-info'
}

const getTypeText = (bridge) => {
  const isPermanent = !bridge.next_schedule
  return isPermanent ? 'Permanent' : 'Scheduled'
}

const getRemoteHost = (bridge) => {
  // Extract host from bridge config (this would need to be provided by the API)
  // For now, show placeholder - the API should include host:port info
  return 'Remote Host'
}

const formatDateTime = (dateString) => {
  if (!dateString) return '—'
  const date = new Date(dateString)
  return date.toLocaleString()
}

const formatNumber = (num) => {
  if (!num && num !== 0) return '0'
  return num.toLocaleString()
}

const formatBytes = (bytes) => {
  if (!bytes && bytes !== 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0
  
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }
  
  return `${size.toFixed(1)} ${units[unitIndex]}`
}

const getUptime = (connectedAt) => {
  if (!connectedAt) return ''
  const now = new Date()
  const connected = new Date(connectedAt)
  const diff = now - connected
  
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  
  if (days > 0) return `${days}d ${hours % 24}h`
  if (hours > 0) return `${hours}h ${minutes % 60}m`
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`
  return `${seconds}s`
}

onMounted(() => {
  refreshData()
  // Refresh every 5 seconds
  refreshInterval = setInterval(refreshData, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>