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

  // Timers
  const talkUpdateTimer = ref(null)
  const statsUpdateTimer = ref(null)

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
      error.value = null
    } catch (err) {
      error.value = 'Failed to fetch repeaters'
      console.error('Error fetching repeaters:', err)
    }
  }

  async function fetchCurrentTalker() {
    try {
      console.log('Fetching current talker...')
      const response = await axios.get('/api/current-talker')
      console.log('Current talker response:', response.data)
      
      if (response.data.current_talker) {
        const talker = response.data.current_talker
        console.log('Setting current talker:', talker)
        
        // If it's a different talker, update currentTalker
        if (!currentTalker.value || 
            currentTalker.value.callsign !== talker.callsign ||
            currentTalker.value.address !== talker.address ||
            currentTalker.value.type !== talker.type) {
          currentTalker.value = {
            ...talker,
            talk_start_time: new Date(Date.now() - (talker.talk_duration * 1000))
          }
        } else {
          // Update existing talker's duration and other properties
          currentTalker.value = {
            ...currentTalker.value,
            ...talker,
            talk_start_time: currentTalker.value.talk_start_time // Keep original start time
          }
        }
        console.log('Current talker set to:', currentTalker.value)
      } else {
        console.log('No current talker in response, clearing currentTalker')
        currentTalker.value = null
      }

      error.value = null
    } catch (err) {
      error.value = 'Failed to fetch current talker'
      console.error('Error fetching current talker:', err)
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
        const index = repeaters.value.findIndex(r => r.callsign === data.data.callsign && r.address === data.data.address)
        if (index !== -1) {
          repeaters.value[index] = { ...repeaters.value[index], ...data.data }
        } else {
          repeaters.value.push(data.data)
        }
        break

      case 'repeater_connect':
        // Add new repeater if not already in the list
        const existingIndex = repeaters.value.findIndex(r => r.callsign === data.data.callsign && r.address === data.data.address)
        if (existingIndex === -1) {
          // Need to fetch the full repeater data since connect event only has callsign/address
          fetchRepeaters()
        }
        break

      case 'repeater_disconnect':
        const disconnectIndex = repeaters.value.findIndex(r => r.callsign === data.data.callsign && r.address === data.data.address)
        if (disconnectIndex !== -1) {
          repeaters.value.splice(disconnectIndex, 1)
        }
        break

      case 'talk_start':
        // Update repeater state if it's a repeater talker
        let startTalker
        if (data.data.address) {
          startTalker = repeaters.value.find(r => r.callsign === data.data.callsign && r.address === data.data.address)
        } else {
          startTalker = repeaters.value.find(r => r.callsign === data.data.callsign)
        }
        
        if (startTalker) {
          startTalker.is_talking = true
          startTalker.talk_start_time = new Date(data.data.timestamp)
        }

        // Fetch current talker from unified API to handle both repeaters and bridge talkers
        fetchCurrentTalker()
        startTalkUpdateTimer()
        startFastStatsTimer()
        break

      case 'talk_end':
        // Update repeater state if it's a repeater talker
        let endTalker
        if (data.data.address) {
          endTalker = repeaters.value.find(r => r.callsign === data.data.callsign && r.address === data.data.address)
        } else {
          endTalker = repeaters.value.find(r => r.callsign === data.data.callsign)
        }
        
        if (endTalker) {
          endTalker.is_talking = false
        }

        // Clear current talker if it was this one (handles both repeaters and bridge talkers)
        if (currentTalker.value && currentTalker.value.callsign === data.data.callsign) {
          // If address is available, also check address match
          if (!data.data.address || currentTalker.value.address === data.data.address) {
            currentTalker.value = null
          }
        }

        // Fetch current talker to check if someone else is still talking
        fetchCurrentTalker()

        // Stop timers if no one is talking (check after fetchCurrentTalker completes)
        setTimeout(() => {
          if (!currentTalker.value) {
            stopTalkUpdateTimer()
            startSlowStatsTimer()
          }
        }, 100)
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

  function startTalkUpdateTimer() {
    if (talkUpdateTimer.value) {
      clearInterval(talkUpdateTimer.value)
    }
    talkUpdateTimer.value = setInterval(() => {
      // Update current talker duration (works for both repeaters and bridge talkers)
      if (currentTalker.value && currentTalker.value.talk_start_time) {
        const now = new Date()
        const durationMs = now - currentTalker.value.talk_start_time
        currentTalker.value.talk_duration = Math.floor(durationMs / 1000)
      }
      
      // Update repeater talk durations for any active repeaters
      repeaters.value.forEach(repeater => {
        if (repeater.is_talking && repeater.talk_start_time) {
          const now = new Date()
          const durationMs = now - repeater.talk_start_time
          repeater.talk_duration = Math.floor(durationMs / 1000)
        }
      })
    }, 100) // Update every 100ms for smooth display
  }

  function stopTalkUpdateTimer() {
    if (talkUpdateTimer.value) {
      clearInterval(talkUpdateTimer.value)
      talkUpdateTimer.value = null
    }
  }

  function startFastStatsTimer() {
    if (statsUpdateTimer.value) {
      clearInterval(statsUpdateTimer.value)
    }
    statsUpdateTimer.value = setInterval(() => {
      fetchStats()
    }, 2000) // Update every 2 seconds when active
  }

  function startSlowStatsTimer() {
    if (statsUpdateTimer.value) {
      clearInterval(statsUpdateTimer.value)
    }
    statsUpdateTimer.value = setInterval(() => {
      fetchStats()
    }, 10000) // Update every 10 seconds when idle
  }

  function stopStatsTimer() {
    if (statsUpdateTimer.value) {
      clearInterval(statsUpdateTimer.value)
      statsUpdateTimer.value = null
    }
  }

  // Initialize
  function initialize() {
    fetchStats()
    fetchRepeaters()
    fetchCurrentTalker()
    fetchTalkLogs()
    connectWebSocket()
    startSlowStatsTimer() // Start with slow refresh when idle
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
    fetchCurrentTalker,
    fetchTalkLogs,
    connectWebSocket,
    disconnectWebSocket,
    startTalkUpdateTimer,
    stopTalkUpdateTimer,
    startFastStatsTimer,
    startSlowStatsTimer,
    stopStatsTimer,
    initialize
  }
})