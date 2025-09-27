<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Talk Logs</h1>
        <p class="text-gray-600">History of voice transmissions</p>
      </div>
      <div class="flex items-center space-x-3">
        <select v-model="selectedLimit" @change="fetchLogs" class="rounded-md border-gray-300 text-sm">
          <option value="50">50 logs</option>
          <option value="100">100 logs</option>
          <option value="250">250 logs</option>
          <option value="500">500 logs</option>
        </select>
        <button @click="fetchLogs" :disabled="loading" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </button>
        <button @click="exportLogs" class="btn-primary">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          Export
        </button>
      </div>
    </div>

    <!-- Statistics Cards -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-6">
      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-primary-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Total Transmissions</p>
            <p class="text-xl font-semibold text-gray-900">{{ talkLogs.length }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-success-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Total Duration</p>
            <p class="text-xl font-semibold text-gray-900">{{ formatDuration(totalDuration) }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-warning-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Average Duration</p>
            <p class="text-xl font-semibold text-gray-900">{{ formatDuration(averageDuration) }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center">
          <div class="w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center">
            <svg class="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
          </div>
          <div class="ml-4">
            <p class="text-sm font-medium text-gray-600">Unique Callsigns</p>
            <p class="text-xl font-semibold text-gray-900">{{ uniqueCallsigns.size }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Filters -->
    <div class="card">
      <div class="flex flex-wrap items-center gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Filter by Callsign</label>
          <input
            v-model="callsignFilter"
            type="text"
            placeholder="Enter callsign..."
            class="rounded-md border-gray-300 text-sm w-48"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Time Range</label>
          <select v-model="timeFilter" class="rounded-md border-gray-300 text-sm">
            <option value="all">All time</option>
            <option value="today">Today</option>
            <option value="yesterday">Yesterday</option>
            <option value="week">This week</option>
            <option value="month">This month</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Min Duration</label>
          <select v-model="durationFilter" class="rounded-md border-gray-300 text-sm">
            <option value="0">All durations</option>
            <option value="5">5+ seconds</option>
            <option value="10">10+ seconds</option>
            <option value="30">30+ seconds</option>
            <option value="60">1+ minute</option>
          </select>
        </div>
        <div class="flex items-end">
          <button @click="clearFilters" class="btn-secondary">
            Clear Filters
          </button>
        </div>
      </div>
    </div>

    <!-- Talk Logs Table -->
    <div class="card">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Callsign
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Start Time
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Duration
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Time Ago
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr v-if="filteredLogs.length === 0">
              <td colspan="4" class="px-6 py-12 text-center text-gray-500">
                {{ talkLogs.length === 0 ? 'No talk logs available' : 'No logs match your filters' }}
              </td>
            </tr>
            <tr v-for="log in paginatedLogs" :key="log.id" class="hover:bg-gray-50">
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                  <div class="w-2 h-2 bg-success-500 rounded-full mr-3"></div>
                  <div class="text-sm font-medium text-gray-900">{{ log.callsign }}</div>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ formatDateTime(log.timestamp) }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="badge-gray">{{ formatDuration(log.duration) }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-400">
                {{ formatTimeAgo(log.timestamp) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div v-if="filteredLogs.length > pageSize" class="flex items-center justify-between px-6 py-3 border-t border-gray-200 bg-gray-50">
        <div class="text-sm text-gray-700">
          Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, filteredLogs.length) }} of {{ filteredLogs.length }} results
        </div>
        <div class="flex space-x-2">
          <button
            @click="currentPage = Math.max(1, currentPage - 1)"
            :disabled="currentPage === 1"
            class="btn-secondary"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage === 1 }"
          >
            Previous
          </button>
          <button
            @click="currentPage = Math.min(totalPages, currentPage + 1)"
            :disabled="currentPage === totalPages"
            class="btn-secondary"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage === totalPages }"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'

export default {
  name: 'TalkLogs',
  setup() {
    const store = useDashboardStore()

    // State
    const selectedLimit = ref(100)
    const callsignFilter = ref('')
    const timeFilter = ref('all')
    const durationFilter = ref('0')
    const currentPage = ref(1)
    const pageSize = ref(25)

    // Computed
    const filteredLogs = computed(() => {
      let logs = [...store.talkLogs]

      // Callsign filter
      if (callsignFilter.value) {
        const filter = callsignFilter.value.toLowerCase()
        logs = logs.filter(log => log.callsign.toLowerCase().includes(filter))
      }

      // Time filter
      if (timeFilter.value !== 'all') {
        const now = new Date()
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
        const yesterday = new Date(today.getTime() - 86400000)
        const weekAgo = new Date(now.getTime() - 7 * 86400000)
        const monthAgo = new Date(now.getTime() - 30 * 86400000)

        logs = logs.filter(log => {
          const logDate = new Date(log.timestamp)
          switch (timeFilter.value) {
            case 'today':
              return logDate >= today
            case 'yesterday':
              return logDate >= yesterday && logDate < today
            case 'week':
              return logDate >= weekAgo
            case 'month':
              return logDate >= monthAgo
            default:
              return true
          }
        })
      }

      // Duration filter
      if (durationFilter.value !== '0') {
        const minDuration = parseInt(durationFilter.value)
        logs = logs.filter(log => log.duration >= minDuration)
      }

      return logs
    })

    const paginatedLogs = computed(() => {
      const start = (currentPage.value - 1) * pageSize.value
      const end = start + pageSize.value
      return filteredLogs.value.slice(start, end)
    })

    const totalPages = computed(() => {
      return Math.ceil(filteredLogs.value.length / pageSize.value)
    })

    const totalDuration = computed(() => {
      return filteredLogs.value.reduce((sum, log) => sum + log.duration, 0)
    })

    const averageDuration = computed(() => {
      return filteredLogs.value.length > 0 ? Math.round(totalDuration.value / filteredLogs.value.length) : 0
    })

    const uniqueCallsigns = computed(() => {
      return new Set(filteredLogs.value.map(log => log.callsign))
    })

    // Methods
    const fetchLogs = () => {
      store.fetchTalkLogs(selectedLimit.value)
    }

    const clearFilters = () => {
      callsignFilter.value = ''
      timeFilter.value = 'all'
      durationFilter.value = '0'
      currentPage.value = 1
    }

    const exportLogs = () => {
      const data = filteredLogs.value.map(log => ({
        Callsign: log.callsign,
        'Start Time': formatDateTime(log.timestamp),
        'Duration (seconds)': log.duration,
        'Duration (formatted)': store.formatDuration(log.duration)
      }))

      const csv = [
        Object.keys(data[0]).join(','),
        ...data.map(row => Object.values(row).map(val => `"${val}"`).join(','))
      ].join('\n')

      const blob = new Blob([csv], { type: 'text/csv' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `ysf-talk-logs-${new Date().toISOString().split('T')[0]}.csv`
      link.click()
      URL.revokeObjectURL(url)
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

    // Watch for filter changes to reset pagination
    watch([callsignFilter, timeFilter, durationFilter], () => {
      currentPage.value = 1
    })

    onMounted(() => {
      if (!store.connected) {
        store.initialize()
      } else {
        fetchLogs()
      }
    })

    return {
      // Store state
      talkLogs: computed(() => store.talkLogs),
      loading: computed(() => store.loading),

      // Local state
      selectedLimit,
      callsignFilter,
      timeFilter,
      durationFilter,
      currentPage,
      pageSize,

      // Computed
      filteredLogs,
      paginatedLogs,
      totalPages,
      totalDuration,
      averageDuration,
      uniqueCallsigns,

      // Store methods
      formatDuration: computed(() => store.formatDuration),

      // Local methods
      fetchLogs,
      clearFilters,
      exportLogs,
      formatDateTime,
      formatTimeAgo
    }
  }
}
</script>