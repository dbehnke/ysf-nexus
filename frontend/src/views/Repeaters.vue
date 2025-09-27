<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Repeaters</h1>
        <p class="text-gray-600">Connected YSF repeaters and their status</p>
      </div>
      <div class="flex items-center space-x-3">
        <div class="flex items-center space-x-2">
          <span class="text-sm text-gray-600">{{ repeaters.length }} total</span>
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
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Callsign
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Address
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Connected
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Last Seen
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Packets
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Data Transfer
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr v-if="repeaters.length === 0">
              <td colspan="7" class="px-6 py-12 text-center text-gray-500">
                No repeaters connected
              </td>
            </tr>
            <tr v-for="repeater in sortedRepeaters" :key="repeater.callsign" class="hover:bg-gray-50">
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center space-x-2">
                  <div :class="getStatusClass(repeater)"></div>
                  <span class="text-sm font-medium" :class="getStatusTextClass(repeater)">
                    {{ getStatusText(repeater) }}
                  </span>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                  <div>
                    <div class="text-sm font-medium text-gray-900">{{ repeater.callsign }}</div>
                    <div v-if="repeater.is_talking" class="text-xs text-warning-600 font-medium">
                      🎙️ Talking ({{ formatTalkDuration(repeater.talk_duration || 0) }})
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ repeater.address }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                <div>
                  {{ formatDateTime(repeater.connected) }}
                </div>
                <div class="text-xs text-gray-400">
                  {{ formatTimeAgo(repeater.connected) }}
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                <div>
                  {{ formatDateTime(repeater.last_seen) }}
                </div>
                <div class="text-xs text-gray-400">
                  {{ formatTimeAgo(repeater.last_seen) }}
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ repeater.packet_count.toLocaleString() }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
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
          <div class="w-8 h-8 bg-success-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Online Repeaters</p>
            <p class="text-xl font-semibold text-gray-900">{{ onlineRepeaters.length }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-warning-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Currently Talking</p>
            <p class="text-xl font-semibold text-gray-900">{{ activeTalkers.length }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-primary-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Total Packets</p>
            <p class="text-xl font-semibold text-gray-900">{{ totalPackets.toLocaleString() }}</p>
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