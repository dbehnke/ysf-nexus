<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Repeaters</h1>
        <p class="text-gray-600 dark:text-gray-400">Connected YSF repeaters and their status</p>
      </div>
      <div class="flex items-center space-x-3">
        <div class="flex items-center space-x-2">
          <span class="text-sm text-gray-600 dark:text-gray-400">{{ repeaters.length }} total</span>
          <span class="badge-success">{{ onlineRepeaters.length }} online</span>
        </div>
        <button @click="refreshData" :disabled="loading" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Repeaters Table -->
    <div class="card">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead class="table-header">
            <tr>
              <th class="table-header-cell">
                Status
              </th>
              <th class="table-header-cell">
                Callsign
              </th>
              <th class="table-header-cell">
                Address
              </th>
              <th class="table-header-cell">
                Connected
              </th>
              <th class="table-header-cell">
                Last Seen
              </th>
              <th class="table-header-cell">
                Packets
              </th>
              <th class="table-header-cell">
                Data Transfer
              </th>
            </tr>
          </thead>
          <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            <tr v-if="repeaters.length === 0">
              <td colspan="7" class="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                No repeaters connected
              </td>
            </tr>
            <tr v-for="repeater in sortedRepeaters" :key="`${repeater.callsign}-${repeater.address}`" class="table-row">
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center space-x-2">
                  <div :class="getStatusClass(repeater)"></div>
                  <span class="text-sm font-medium" :class="getStatusTextClass(repeater)">
                    {{ getStatusText(repeater) }}
                  </span>
                </div>
              </td>
              <td class="table-cell">
                <div class="flex items-center">
                  <div>
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ repeater.callsign }}</div>
                    <div v-if="repeater.is_talking" class="text-xs text-warning-600 font-medium">
                      🎙️ Talking ({{ formatTalkDuration(repeater.talk_duration || 0) }})
                    </div>
                  </div>
                </div>
              </td>
              <td class="table-cell">
                {{ repeater.address }}
              </td>
              <td class="table-cell">
                <div>
                  {{ formatDateTime(repeater.connected) }}
                </div>
                <div class="text-xs text-gray-400 dark:text-gray-500">
                  {{ formatTimeAgo(repeater.connected) }}
                </div>
              </td>
              <td class="table-cell">
                <div>
                  {{ formatDateTime(repeater.last_seen) }}
                </div>
                <div class="text-xs text-gray-400 dark:text-gray-500">
                  {{ formatTimeAgo(repeater.last_seen) }}
                </div>
              </td>
              <td class="table-cell">
                {{ repeater.packet_count.toLocaleString() }}
              </td>
              <td class="table-cell">
                <div>
                  <div>↓ {{ formatBytes(repeater.bytes_received) }}</div>
                  <div>↑ {{ formatBytes(repeater.bytes_transmitted) }}</div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Summary Cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-success-100 dark:bg-success-800/20 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-success-600 dark:text-success-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Online Repeaters</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ onlineRepeaters.length }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-warning-100 dark:bg-warning-800/20 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-warning-600 dark:text-warning-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Currently Talking</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ activeTalkers.length }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-primary-100 dark:bg-primary-800/20 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-primary-600 dark:text-primary-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Total Packets</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ totalPackets.toLocaleString() }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'

export default {
  name: 'Repeaters',
  setup() {
    const store = useDashboardStore()

    const sortedRepeaters = computed(() => {
      return [...store.repeaters].sort((a, b) => {
        // Talking repeaters first
        if (a.is_talking && !b.is_talking) return -1
        if (!a.is_talking && b.is_talking) return 1

        // Then online repeaters
        if (a.is_active && !b.is_active) return -1
        if (!a.is_active && b.is_active) return 1

        // Then by last seen (most recent first)
        return new Date(b.last_seen) - new Date(a.last_seen)
      })
    })

    const totalPackets = computed(() => {
      return store.repeaters.reduce((sum, r) => sum + r.packet_count, 0)
    })

    const getStatusClass = (repeater) => {
      if (repeater.is_talking) return 'status-talking'
      if (repeater.is_active) return 'status-online'
      return 'status-offline'
    }

    const getStatusTextClass = (repeater) => {
      if (repeater.is_talking) return 'text-warning-600'
      if (repeater.is_active) return 'text-success-600'
      return 'text-gray-400'
    }

    const getStatusText = (repeater) => {
      if (repeater.is_talking) return 'Talking'
      if (repeater.is_active) return 'Online'
      return 'Offline'
    }

    const formatTalkDuration = (seconds) => {
      if (seconds < 60) return `${seconds}s`
      const minutes = Math.floor(seconds / 60)
      const secs = seconds % 60
      return `${minutes}:${secs.toString().padStart(2, '0')}`
    }

    const formatDateTime = (timestamp) => {
      return new Date(timestamp).toLocaleString()
    }

    const formatTimeAgo = (timestamp) => {
      const now = new Date()
      const time = new Date(timestamp)
      const diff = Math.floor((now - time) / 1000)

      if (diff < 60) return `${diff}s ago`
      if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
      if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
      return `${Math.floor(diff / 86400)}d ago`
    }

    const refreshData = () => {
      store.fetchRepeaters()
    }

    onMounted(() => {
      if (!store.connected) {
        store.initialize()
      } else {
        store.fetchRepeaters()
      }
    })

    onUnmounted(() => {
      // Keep WebSocket connected for other views
    })

    return {
      // Store state
      repeaters: computed(() => store.repeaters),
      onlineRepeaters: computed(() => store.onlineRepeaters),
      activeTalkers: computed(() => store.activeTalkers),
      loading: computed(() => store.loading),

      // Computed
      sortedRepeaters,
      totalPackets,

      // Store methods
      formatBytes: computed(() => store.formatBytes),

      // Local methods
      getStatusClass,
      getStatusTextClass,
      getStatusText,
      formatTalkDuration,
      formatDateTime,
      formatTimeAgo,
      refreshData
    }
  }
}
</script>