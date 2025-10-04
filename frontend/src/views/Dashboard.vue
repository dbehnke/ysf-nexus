<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ systemInfo.name || 'YSF Nexus' }}</h1>
        <p class="text-sm text-gray-600 dark:text-gray-400">{{ systemInfo.description || 'YSF Reflector' }}</p>
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

    <!-- Current Talker Card - Compact -->
    <div class="card">
      <div v-if="currentTalker" class="flex items-center justify-between">
        <div class="flex items-center space-x-4">
          <div class="w-12 h-12 bg-warning-100 rounded-full flex items-center justify-center flex-shrink-0">
            <svg class="w-6 h-6 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          </div>
          <div>
            <div class="flex items-center space-x-2 mb-1">
              <h3 class="text-xl font-bold text-gray-900 dark:text-white">{{ currentTalker.callsign }}</h3>
              <span class="inline-flex items-center px-2 py-0.5 bg-blue-100 text-blue-800 rounded text-xs font-medium">
                {{ currentTalker.type === 'bridge' ? 'Bridge' : 'Repeater' }}
              </span>
            </div>
            <p class="text-sm text-gray-600 dark:text-gray-400">{{ currentTalker.address }}</p>
          </div>
        </div>
        <div class="flex items-center space-x-4">
          <div class="text-right">
            <div class="flex items-center space-x-2 mb-1">
              <div class="status-talking"></div>
              <span class="text-sm font-medium text-warning-600">On Air</span>
            </div>
            <div class="text-2xl font-bold text-warning-600">
              {{ formatTalkDuration(currentTalker.talk_duration || 0) }}
            </div>
          </div>
        </div>
      </div>

      <div v-else class="flex items-center justify-center py-6">
        <div class="flex items-center space-x-3">
          <div class="w-10 h-10 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center">
            <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          </div>
          <div>
            <h3 class="text-base font-medium text-gray-500 dark:text-gray-400">No Active Transmission</h3>
            <p class="text-sm text-gray-400 dark:text-gray-500">Waiting for someone to key up...</p>
          </div>
        </div>
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

    <!-- Recent Activity, Active Repeaters & Bridges -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
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

      <!-- Bridges -->
      <div class="card">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Bridges</h2>
          <span :class="activeBridges.length > 0 ? 'badge-success' : 'badge-secondary'">
            {{ activeBridges.length }} active
          </span>
        </div>

        <div class="space-y-3">
          <!-- Active bridges with countdown to end -->
          <div v-if="activeBridges.length > 0">
            <div
              v-for="bridge in activeBridges"
              :key="bridge.name"
              class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
            >
              <div class="flex items-center justify-between mb-2">
                <div class="flex items-center space-x-3">
                  <div class="status-online"></div>
                  <div>
                    <div class="flex items-center space-x-2">
                      <p class="font-medium text-gray-900 dark:text-white">{{ bridge.name }}</p>
                      <span :class="getBridgeTypeBadge(bridge.type)" class="badge text-xs">
                        {{ bridge.type === 'dmr' ? 'DMR' : 'YSF' }}
                      </span>
                    </div>
                    <p class="text-sm text-gray-500 dark:text-gray-400">
                      {{ getBridgeConnectionInfo(bridge) }}
                    </p>
                  </div>
                </div>
                <span class="badge-success">Active</span>
              </div>
              <!-- Countdown to bridge end -->
              <div v-if="getBridgeEndCountdown(bridge)" class="mt-2 pt-2 border-t border-gray-200 dark:border-gray-600">
                <div class="flex items-center justify-between text-sm">
                  <span class="text-gray-600 dark:text-gray-400">Ends in:</span>
                  <span class="font-semibold text-orange-600 dark:text-orange-400">
                    {{ getBridgeEndCountdown(bridge) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- No active bridges - show countdown to next -->
          <div v-else>
            <div v-if="nextScheduledBridge" class="text-center py-6">
              <div class="w-12 h-12 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-3">
                <svg class="w-6 h-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-2">Next bridge activation</p>
              <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ nextScheduledBridge.name }}</p>
              <p class="text-2xl font-bold text-blue-600 dark:text-blue-400 mt-2">{{ bridgeCountdown }}</p>
            </div>
            <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
              No bridges configured
            </div>
          </div>

          <div v-if="Object.keys(bridges).length > 0" class="text-center pt-2">
            <router-link to="/bridges" class="text-sm text-primary-600 hover:text-primary-700">
              View all bridges →
            </router-link>
          </div>
        </div>
      </div>
    </div>

    <!-- Footer -->
    <div class="text-center py-6 text-gray-500 dark:text-gray-400 text-sm">
      <p>
        YSF Nexus {{ systemInfo.version || 'dev' }} · Made with
        <svg class="inline w-4 h-4 text-red-500" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M3.172 5.172a4 4 0 015.656 0L10 6.343l1.172-1.171a4 4 0 115.656 5.656L10 17.657l-6.828-6.829a4 4 0 010-5.656z" clip-rule="evenodd" />
        </svg>
        in Macomb, MI
      </p>
    </div>
  </div>
</template>

<script>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import axios from 'axios'

export default {
  name: 'Dashboard',
  setup() {
    const store = useDashboardStore()
    const bridges = ref({})
    const bridgeCountdown = ref('')
    const systemInfo = ref({
      name: '',
      description: '',
      version: 'dev'
    })

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

    const fetchBridges = async () => {
      try {
        const response = await axios.get('/api/bridges')
        bridges.value = response.data.bridges || {}
      } catch (error) {
        console.error('Failed to fetch bridges:', error)
      }
    }

    const fetchSystemInfo = async () => {
      try {
        const response = await axios.get('/api/system/info')
        systemInfo.value = {
          name: response.data.name || 'YSF Nexus',
          description: response.data.description || 'YSF Reflector',
          version: response.data.version || 'dev'
        }
      } catch (error) {
        console.error('Failed to fetch system info:', error)
      }
    }

    const activeBridges = computed(() => {
      return Object.values(bridges.value).filter(bridge =>
        bridge.state === 'connected' || bridge.state === 'connecting'
      )
    })

    const nextScheduledBridge = computed(() => {
      const scheduled = Object.values(bridges.value)
        .filter(bridge => bridge.next_schedule)
        .sort((a, b) => new Date(a.next_schedule) - new Date(b.next_schedule))
      return scheduled[0] || null
    })

    const updateCountdown = () => {
      if (!nextScheduledBridge.value || !nextScheduledBridge.value.next_schedule) {
        bridgeCountdown.value = ''
        return
      }

      const now = new Date()
      const scheduleTime = new Date(nextScheduledBridge.value.next_schedule)
      const diff = scheduleTime - now

      if (diff <= 0) {
        bridgeCountdown.value = 'Starting soon...'
        return
      }

      const days = Math.floor(diff / (1000 * 60 * 60 * 24))
      const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
      const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
      const seconds = Math.floor((diff % (1000 * 60)) / 1000)

      if (days > 0) {
        bridgeCountdown.value = `${days}d ${hours}h ${minutes}m`
      } else if (hours > 0) {
        bridgeCountdown.value = `${hours}h ${minutes}m ${seconds}s`
      } else if (minutes > 0) {
        bridgeCountdown.value = `${minutes}m ${seconds}s`
      } else {
        bridgeCountdown.value = `${seconds}s`
      }
    }

    const getBridgeEndCountdown = (bridge) => {
      if (!bridge.connected_at || !bridge.duration) {
        return null
      }

      const now = new Date()
      const connectedAt = new Date(bridge.connected_at)

      // Duration comes as nanoseconds from Go, convert to milliseconds
      const durationMs = bridge.duration / 1000000
      const endTime = new Date(connectedAt.getTime() + durationMs)
      const diff = endTime - now

      if (diff <= 0) {
        return 'Ending...'
      }

      const minutes = Math.floor(diff / (1000 * 60))
      const seconds = Math.floor((diff % (1000 * 60)) / 1000)

      if (minutes > 0) {
        return `${minutes}m ${seconds}s`
      } else {
        return `${seconds}s`
      }
    }

    const getBridgeTypeBadge = (type) => {
      return type === 'dmr' ? 'badge-success' : 'badge-primary'
    }

    const getBridgeConnectionInfo = (bridge) => {
      if (bridge.type === 'dmr' && bridge.metadata) {
        const network = bridge.metadata.dmr_network || 'DMR'
        const tg = bridge.metadata.talk_group || '?'
        return `${network} TG${tg}`
      }
      return 'Connected'
    }

    const refreshData = () => {
      store.fetchStats()
      store.fetchRepeaters()
      store.fetchCurrentTalker()
      store.fetchTalkLogs()
      fetchBridges()
      fetchSystemInfo()
    }

    let updateInterval
    let countdownInterval

    onMounted(() => {
      store.initialize()
      fetchBridges()
      fetchSystemInfo()

      // Start periodic current talker updates to keep duration accurate
      updateInterval = setInterval(() => {
        // store.currentTalker is a Pinia ref; check .value so we only poll when someone is active
        if (store.currentTalker && store.currentTalker.value) {
          store.fetchCurrentTalker()
        }
      }, 2000) // Update every 2 seconds

      // Update bridge countdown every second
      countdownInterval = setInterval(() => {
        updateCountdown()
      }, 1000)

      // Fetch bridges every 10 seconds
      setInterval(fetchBridges, 10000)

      // Initial countdown update
      updateCountdown()
    })

    onUnmounted(() => {
      if (updateInterval) {
        clearInterval(updateInterval)
      }
      if (countdownInterval) {
        clearInterval(countdownInterval)
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

      // Bridge state
      bridges,
      activeBridges,
      nextScheduledBridge,
      bridgeCountdown,

      // System info
      systemInfo,

      // Store methods
      formatBytes: computed(() => store.formatBytes),
      formatDuration: computed(() => store.formatDuration),

      // Local methods
      formatTalkDuration,
      formatTimeAgo,
      refreshData,
      getBridgeEndCountdown,
      getBridgeTypeBadge,
      getBridgeConnectionInfo
    }
  }
}
</script>