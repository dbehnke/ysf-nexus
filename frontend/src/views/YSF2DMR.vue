<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">YSF2DMR Bridge</h1>
        <p class="text-sm text-gray-600 dark:text-gray-400">Cross-mode YSF ↔ DMR voice bridge</p>
      </div>
      <div class="flex items-center space-x-3">
        <div class="flex items-center space-x-2">
          <div :class="status.enabled && status.dmr_connected ? 'status-online' : 'status-offline'"></div>
          <span class="text-sm text-gray-600 dark:text-gray-400">
            {{ status.enabled && status.dmr_connected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>
        <button @click="fetchStatus" :disabled="loading" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Not Enabled State -->
    <div v-if="!status.enabled" class="card text-center py-12">
      <div class="w-16 h-16 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center mx-auto mb-4">
        <svg class="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636a9 9 0 010 12.728m0 0l-2.829-2.829m2.829 2.829L21 21M15.536 8.464a5 5 0 010 7.072m0 0l-2.829-2.829m-4.243 2.829a4.978 4.978 0 01-1.414-2.83m-1.414 5.658a9 9 0 01-2.167-9.238m7.824 2.167a1 1 0 111.414 1.414m-1.414-1.414L3 3m8.293 8.293l1.414 1.414" />
        </svg>
      </div>
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">YSF2DMR Bridge Not Enabled</h3>
      <p class="text-gray-600 dark:text-gray-400">The YSF2DMR bridge is not configured or disabled in settings.</p>
    </div>

    <!-- Enabled State -->
    <div v-else>
      <!-- Connection Status Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- YSF Side -->
        <div class="card">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">YSF Interface</h2>
            <span :class="status.ysf_listening ? 'badge-success' : 'badge-secondary'">
              {{ status.ysf_listening ? 'Listening' : 'Offline' }}
            </span>
          </div>
          <div class="space-y-3">
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">Callsign:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.ysf_callsign || 'N/A' }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">YSF Packets RX:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.stats?.ysf_packets_rx?.toLocaleString() || 0 }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">YSF Packets TX:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.stats?.ysf_packets_tx?.toLocaleString() || 0 }}</span>
            </div>
          </div>
        </div>

        <!-- DMR Side -->
        <div class="card">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">DMR Network</h2>
            <span :class="status.dmr_connected ? 'badge-success' : 'badge-secondary'">
              {{ status.dmr_connected ? 'Connected' : 'Disconnected' }}
            </span>
          </div>
          <div class="space-y-3">
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">Network:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.dmr_network || 'N/A' }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">DMR ID:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.dmr_id || 'N/A' }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">Talk Group:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.talk_group || 'N/A' }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">DMR Packets RX:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.stats?.dmr_packets_rx?.toLocaleString() || 0 }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-gray-600 dark:text-gray-400">DMR Packets TX:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ status.stats?.dmr_packets_tx?.toLocaleString() || 0 }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Active Call -->
      <div class="card">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Active Call</h2>
        <div v-if="status.active_call" class="bg-warning-50 dark:bg-warning-900/20 border border-warning-200 dark:border-warning-800 rounded-lg p-4">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">Direction</p>
              <div class="flex items-center space-x-2">
                <span class="font-medium text-gray-900 dark:text-white">
                  {{ status.active_call.direction === 'ysf_to_dmr' ? 'YSF → DMR' : 'DMR → YSF' }}
                </span>
              </div>
            </div>
            <div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">YSF Callsign</p>
              <p class="font-medium text-gray-900 dark:text-white">{{ status.active_call.ysf_callsign || 'N/A' }}</p>
            </div>
            <div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">DMR ID</p>
              <p class="font-medium text-gray-900 dark:text-white">{{ status.active_call.dmr_id || 'N/A' }}</p>
            </div>
            <div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">Talk Group</p>
              <p class="font-medium text-gray-900 dark:text-white">{{ status.active_call.talk_group || 'N/A' }}</p>
            </div>
            <div>
              <p class="text-sm text-gray-600 dark:text-gray-400 mb-1">Duration</p>
              <p class="font-medium text-gray-900 dark:text-white">{{ formatCallDuration(status.active_call.start_time) }}</p>
            </div>
          </div>
        </div>
        <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
          No active calls
        </div>
      </div>

      <!-- Statistics -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div class="card">
          <div class="flex items-center">
            <div class="w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center">
              <svg class="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Total Calls</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ status.stats?.total_calls?.toLocaleString() || 0 }}</p>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center">
            <div class="w-8 h-8 bg-green-100 rounded-lg flex items-center justify-center">
              <svg class="w-5 h-5 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6" />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-600 dark:text-gray-400">YSF → DMR</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ status.stats?.ysf_to_dmr_calls?.toLocaleString() || 0 }}</p>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center">
            <div class="w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center">
              <svg class="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 17l-5-5m0 0l5-5m-5 5h12" />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-600 dark:text-gray-400">DMR → YSF</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ status.stats?.dmr_to_ysf_calls?.toLocaleString() || 0 }}</p>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center">
            <div class="w-8 h-8 bg-orange-100 rounded-lg flex items-center justify-center">
              <svg class="w-5 h-5 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-600 dark:text-gray-400">Conversion Errors</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ status.stats?.conversion_errors?.toLocaleString() || 0 }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'

export default {
  name: 'YSF2DMR',
  setup() {
    const status = ref({
      enabled: false,
      dmr_connected: false,
      ysf_listening: false,
      stats: {}
    })
    const loading = ref(false)

    const fetchStatus = async () => {
      loading.value = true
      try {
        const response = await axios.get('/api/ysf2dmr/status')
        if (response.data.enabled && response.data.status) {
          status.value = response.data.status
        } else {
          status.value = { enabled: false }
        }
      } catch (error) {
        console.error('Failed to fetch YSF2DMR status:', error)
        status.value = { enabled: false }
      } finally {
        loading.value = false
      }
    }

    const formatCallDuration = (startTime) => {
      if (!startTime) return 'N/A'
      const start = new Date(startTime)
      const now = new Date()
      const diff = Math.floor((now - start) / 1000)

      if (diff < 60) return `${diff}s`
      const minutes = Math.floor(diff / 60)
      const seconds = diff % 60
      return `${minutes}:${seconds.toString().padStart(2, '0')}`
    }

    let updateInterval

    onMounted(() => {
      fetchStatus()
      // Update status every 2 seconds
      updateInterval = setInterval(fetchStatus, 2000)
    })

    onUnmounted(() => {
      if (updateInterval) {
        clearInterval(updateInterval)
      }
    })

    return {
      status,
      loading,
      fetchStatus,
      formatCallDuration
    }
  }
}
</script>
