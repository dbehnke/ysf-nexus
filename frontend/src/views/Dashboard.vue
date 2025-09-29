<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h1>
        <p class="text-gray-600 dark:text-gray-400">YSF Nexus Reflector Status</p>
      </div>
      <div class="flex items-center space-x-3">
        <div class="flex items-center space-x-2">
          <div :class="connected ? 'status-online' : 'status-offline'"></div>
          <span class="text-sm text-gray-600 dark:text-gray-400">
            {{ connected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>
        <button
          @click="refreshData"
          :disabled="loading"
          class="btn-secondary"
        >
          <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Current Talker Card -->
    <div class="card">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Current Talker</h2>
        <div v-if="currentTalker" class="flex items-center space-x-2">
          <div class="status-talking"></div>
          <span class="text-sm font-medium text-warning-600">On Air</span>
        </div>
      </div>

      <div v-if="currentTalker" class="text-center py-8">
        <div class="w-24 h-24 bg-warning-100 rounded-full flex items-center justify-center mx-auto mb-4">
          <svg class="w-12 h-12 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
          </svg>
        </div>
        <h3 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">{{ currentTalker.callsign }}</h3>
        <p class="text-gray-600 dark:text-gray-400 mb-2">{{ currentTalker.address }}</p>
        <div class="inline-flex items-center px-2 py-1 bg-blue-100 text-blue-800 rounded text-xs font-medium mb-4">
          {{ currentTalker.type === 'bridge' ? 'Bridge Talker' : 'Repeater' }}
        </div>
        <div class="inline-flex items-center px-3 py-1 bg-warning-100 text-warning-800 rounded-full text-sm font-medium">
          <svg class="w-4 h-4 mr-1 animate-pulse" fill="currentColor" viewBox="0 0 20 20">
            <circle cx="10" cy="10" r="3"/>
          </svg>
          {{ formatTalkDuration(currentTalker.talk_duration || 0) }}
        </div>
      </div>

      <div v-else class="text-center py-12">
        <div class="w-16 h-16 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center mx-auto mb-4">
          <svg class="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
          </svg>
        </div>
        <h3 class="text-lg font-medium text-gray-500 dark:text-gray-400 mb-2">No Active Transmission</h3>
        <p class="text-gray-400 dark:text-gray-500">Waiting for someone to key up...</p>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-primary-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Uptime</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ formatDuration(stats.uptime) }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-success-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Active Repeaters</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.activeRepeaters }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-warning-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Total Packets</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.totalPackets.toLocaleString() }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Data Transfer</p>
            <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ formatBytes(stats.bytesReceived + stats.bytesSent) }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Active Repeaters & Recent Activity -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Active Repeaters -->
      <div class="card">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Active Repeaters</h2>
          <span class="badge-success">{{ onlineRepeaters.length }} online</span>
        </div>

        <div class="space-y-3">
          <div v-if="onlineRepeaters.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
            No active repeaters
          </div>
          <div
            v-for="repeater in onlineRepeaters.slice(0, 5)"
            :key="repeater.callsign"
            class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
          >
            <div class="flex items-center space-x-3">
              <div :class="repeater.is_talking ? 'status-talking' : 'status-online'"></div>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ repeater.callsign }}</p>
                <p class="text-sm text-gray-500 dark:text-gray-400">{{ repeater.address }}</p>
              </div>
            </div>
            <div class="text-right">
              <p class="text-sm text-gray-600 dark:text-gray-300">{{ formatDuration(Math.floor((Date.now() - new Date(repeater.connected)) / 1000)) }}</p>
              <p class="text-xs text-gray-400 dark:text-gray-500">{{ repeater.packet_count }} packets</p>
            </div>
          </div>
          <div v-if="onlineRepeaters.length > 5" class="text-center">
            <router-link to="/repeaters" class="text-sm text-primary-600 hover:text-primary-700">
              View all {{ onlineRepeaters.length }} repeaters →
            </router-link>
          </div>
        </div>
      </div>

      <!-- Recent Talk Activity -->
      <div class="card">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Recent Activity</h2>
          <router-link to="/logs" class="text-sm text-primary-600 hover:text-primary-700">
            View all logs →
          </router-link>
        </div>

        <div class="space-y-3">
          <div v-if="talkLogs.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
            No recent activity
          </div>
          <div
            v-for="log in talkLogs.slice(0, 5)"
            :key="log.id"
            class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
          >
            <div class="flex items-center space-x-3">
              <div class="w-2 h-2 bg-success-500 rounded-full"></div>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ log.callsign }}</p>
                <p class="text-sm text-gray-500 dark:text-gray-400">{{ formatTimeAgo(log.timestamp) }}</p>
              </div>
            </div>
            <div class="text-right">
              <span class="badge-gray">{{ formatDuration(log.duration) }}</span>
            </div>
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
  name: 'Dashboard',
  setup() {
    const store = useDashboardStore()

    const formatTalkDuration = (seconds) => {
      if (seconds < 60) return `${seconds}s`
      const minutes = Math.floor(seconds / 60)
      const secs = seconds % 60
      return `${minutes}:${secs.toString().padStart(2, '0')}`
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
      store.fetchStats()
      store.fetchRepeaters()
      store.fetchCurrentTalker()
      store.fetchTalkLogs()
    }

    let updateInterval

    onMounted(() => {
      store.initialize()

      // Start periodic current talker updates to keep duration accurate
      updateInterval = setInterval(() => {
        if (store.currentTalker) {
          store.fetchCurrentTalker()
        }
      }, 2000) // Update every 2 seconds
    })

    onUnmounted(() => {
      if (updateInterval) {
        clearInterval(updateInterval)
      }
      store.disconnectWebSocket()
    })

    return {
      // Store state
      stats: computed(() => store.stats),
      repeaters: computed(() => store.repeaters),
      currentTalker: computed(() => store.currentTalker),
      talkLogs: computed(() => store.talkLogs),
      connected: computed(() => store.connected),
      loading: computed(() => store.loading),
      onlineRepeaters: computed(() => store.onlineRepeaters),

      // Store methods
      formatBytes: computed(() => store.formatBytes),
      formatDuration: computed(() => store.formatDuration),

      // Local methods
      formatTalkDuration,
      formatTimeAgo,
      refreshData
    }
  }
}
</script>