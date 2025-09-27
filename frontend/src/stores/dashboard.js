import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

export const useDashboardStore = defineStore('dashboard', () => {
  // State
  const stats = ref({
    uptime: 0,
    activeRepeaters: 0,
    totalConnections: 0,
    totalPackets: 0,
    bytesReceived: 0,
    bytesSent: 0
  })

  const repeaters = ref([])
  const currentTalker = ref(null)
  const talkLogs = ref([])
  const connected = ref(false)
  const loading = ref(false)
  const error = ref(null)

  // WebSocket connection
  const ws = ref(null)

  // Computed
  const activeTalkers = computed(() => {
    return repeaters.value.filter(r => r.is_talking)
  })

  const onlineRepeaters = computed(() => {
    return repeaters.value.filter(r => r.is_active)
  })

  const formatBytes = computed(() => (bytes) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  })

  const formatDuration = computed(() => (seconds) => {
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = seconds % 60
    return `${hours}h ${minutes}m ${secs}s`
  })

  // Actions
  async function fetchStats() {
    try {
      loading.value = true
      const response = await axios.get('/api/stats')
      stats.value = response.data
      error.value = null
    } catch (err) {
      error.value = 'Failed to fetch stats'
      console.error('Error fetching stats:', err)
    } finally {
      loading.value = false
    }
  }

  async function fetchRepeaters() {
    try {
      const response = await axios.get('/api/repeaters')
      repeaters.value = response.data.repeaters || []

      // Update current talker
      const talking = repeaters.value.find(r => r.is_talking)
      if (talking && (!currentTalker.value || currentTalker.value.callsign !== talking.callsign)) {
        currentTalker.value = talking
      } else if (!talking) {
        currentTalker.value = null
      }

      error.value = null
    } catch (err) {
      error.value = 'Failed to fetch repeaters'
      console.error('Error fetching repeaters:', err)
    }
  }

  async function fetchTalkLogs(limit = 100) {
    try {
      const response = await axios.get(`/api/logs/talk?limit=${limit}`)
      talkLogs.value = response.data.logs || []
      error.value = null
    } catch (err) {
      error.value = 'Failed to fetch talk logs'
      console.error('Error fetching talk logs:', err)
    }
  }

  function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    ws.value = new WebSocket(wsUrl)

    ws.value.onopen = () => {
      connected.value = true
      console.log('WebSocket connected')
    }

    ws.value.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleWebSocketMessage(data)
      } catch (err) {
        console.error('Error parsing WebSocket message:', err)
      }
    }

    ws.value.onclose = () => {
      connected.value = false
      console.log('WebSocket disconnected')
      // Attempt to reconnect after 3 seconds
      setTimeout(connectWebSocket, 3000)
    }

    ws.value.onerror = (err) => {
      console.error('WebSocket error:', err)
      connected.value = false
    }
  }

  function handleWebSocketMessage(data) {
    switch (data.type) {
      case 'stats_update':
        stats.value = { ...stats.value, ...data.data }
        break

      case 'repeater_update':
        const index = repeaters.value.findIndex(r => r.callsign === data.data.callsign)
        if (index !== -1) {
          repeaters.value[index] = { ...repeaters.value[index], ...data.data }
        } else {
          repeaters.value.push(data.data)
        }
        break

      case 'repeater_disconnect':
        const disconnectIndex = repeaters.value.findIndex(r => r.callsign === data.data.callsign)
        if (disconnectIndex !== -1) {
          repeaters.value.splice(disconnectIndex, 1)
        }
        break

      case 'talk_start':
        const startTalker = repeaters.value.find(r => r.callsign === data.data.callsign)
        if (startTalker) {
          startTalker.is_talking = true
          startTalker.talk_duration = 0
          currentTalker.value = startTalker
        }
        break

      case 'talk_end':
        const endTalker = repeaters.value.find(r => r.callsign === data.data.callsign)
        if (endTalker) {
          endTalker.is_talking = false
          endTalker.talk_duration = 0
        }

        // Add to talk logs
        talkLogs.value.unshift({
          id: Date.now(),
          callsign: data.data.callsign,
          duration: data.data.duration,
          timestamp: new Date().toISOString()
        })

        // Keep only last 100 logs
        if (talkLogs.value.length > 100) {
          talkLogs.value = talkLogs.value.slice(0, 100)
        }

        // Clear current talker if it was this one
        if (currentTalker.value && currentTalker.value.callsign === data.data.callsign) {
          currentTalker.value = null
        }
        break

      case 'event':
        // Handle other events as needed
        console.log('Event received:', data.data)
        break
    }
  }

  function disconnectWebSocket() {
    if (ws.value) {
      ws.value.close()
      ws.value = null
    }
    connected.value = false
  }

  // Initialize
  function initialize() {
    fetchStats()
    fetchRepeaters()
    fetchTalkLogs()
    connectWebSocket()
  }

  return {
    // State
    stats,
    repeaters,
    currentTalker,
    talkLogs,
    connected,
    loading,
    error,

    // Computed
    activeTalkers,
    onlineRepeaters,
    formatBytes,
    formatDuration,

    // Actions
    fetchStats,
    fetchRepeaters,
    fetchTalkLogs,
    connectWebSocket,
    disconnectWebSocket,
    initialize
  }
})