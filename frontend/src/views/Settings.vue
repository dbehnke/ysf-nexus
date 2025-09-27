<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>
      <p class="text-gray-600 dark:text-gray-400">Configure your YSF Nexus reflector</p>
    </div>

    <!-- System Information -->
    <div class="card">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">System Information</h2>
      <dl class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Version</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.version || 'Loading...' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Build Time</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.buildTime || 'Loading...' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Server Host</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.host || 'Loading...' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Server Port</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.port || 'Loading...' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Max Connections</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.maxConnections || 'Loading...' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Timeout</dt>
          <dd class="text-sm text-gray-900 dark:text-gray-300">{{ systemInfo.timeout || 'Loading...' }}</dd>
        </div>
      </dl>
    </div>

    <!-- Server Configuration -->
    <div class="card">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Server Configuration</h2>
        <div class="flex space-x-2">
          <button @click="resetServerConfig" class="btn-secondary">Reset</button>
          <button @click="saveServerConfig" :disabled="saving" class="btn-primary">
            <svg v-if="saving" class="w-4 h-4 mr-2 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Save Changes
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <label class="form-label">Reflector Name</label>
          <input
            v-model="serverConfig.name"
            type="text"
            class="form-input"
            placeholder="YSF Nexus"
          />
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Maximum 16 characters</p>
        </div>

        <div>
          <label class="form-label">Description</label>
          <input
            v-model="serverConfig.description"
            type="text"
            class="form-input"
            placeholder="Go Reflector"
          />
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Maximum 14 characters</p>
        </div>

        <div>
          <label class="form-label">Max Connections</label>
          <input
            v-model.number="serverConfig.maxConnections"
            type="number"
            min="1"
            max="1000"
            class="form-input"
          />
        </div>

        <div>
          <label class="form-label">Connection Timeout (minutes)</label>
          <input
            v-model.number="serverConfig.timeoutMinutes"
            type="number"
            min="1"
            max="60"
            class="form-input"
          />
        </div>
      </div>
    </div>

    <!-- Blocklist Management -->
    <div class="card">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Blocklist Management</h2>
        <div class="flex items-center space-x-2">
          <label class="flex items-center">
            <input
              v-model="blocklistConfig.enabled"
              type="checkbox"
              class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
            <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Enable blocklist</span>
          </label>
        </div>
      </div>

      <div v-if="blocklistConfig.enabled" class="space-y-4">
        <div>
          <label class="form-label mb-2">Blocked Callsigns</label>
          <div class="space-y-2">
            <div
              v-for="(callsign, index) in blocklistConfig.callsigns"
              :key="index"
              class="flex items-center space-x-2"
            >
              <input
                v-model="blocklistConfig.callsigns[index]"
                type="text"
                class="flex-1 form-input"
                placeholder="Enter callsign"
              />
              <button
                @click="removeBlockedCallsign(index)"
                class="btn-danger"
              >
                Remove
              </button>
            </div>
          </div>
          <button @click="addBlockedCallsign" class="btn-secondary mt-2">
            Add Callsign
          </button>
        </div>

        <div class="flex space-x-2">
          <button @click="resetBlocklist" class="btn-secondary">Reset</button>
          <button @click="saveBlocklist" :disabled="saving" class="btn-primary">
            Save Blocklist
          </button>
        </div>
      </div>
    </div>

    <!-- Logging Configuration -->
    <div class="card">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Logging Configuration</h2>
        <div class="flex space-x-2">
          <button @click="resetLoggingConfig" class="btn-secondary">Reset</button>
          <button @click="saveLoggingConfig" :disabled="saving" class="btn-primary">
            Save Changes
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <label class="form-label">Log Level</label>
          <select v-model="loggingConfig.level" class="form-select">
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>

        <div>
          <label class="form-label">Log Format</label>
          <select v-model="loggingConfig.format" class="form-select">
            <option value="text">Text</option>
            <option value="json">JSON</option>
          </select>
        </div>

        <div>
          <label class="form-label">Log File</label>
          <input
            v-model="loggingConfig.file"
            type="text"
            class="form-input"
            placeholder="/var/log/ysf-nexus.log (optional)"
          />
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Leave empty to log to console only</p>
        </div>

        <div>
          <label class="form-label">Max File Size (MB)</label>
          <input
            v-model.number="loggingConfig.maxSize"
            type="number"
            min="1"
            max="1000"
            class="form-input"
          />
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="card">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">System Actions</h2>
      <div class="flex flex-wrap gap-4">
        <button @click="exportConfig" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          Export Config
        </button>
        <button @click="downloadLogs" class="btn-secondary">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          Download Logs
        </button>
        <button @click="restartServer" class="btn-warning">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Restart Server
        </button>
      </div>
    </div>

    <!-- Success/Error Messages -->
    <div v-if="message" :class="messageClass" class="rounded-md p-4">
      <div class="flex">
        <div class="flex-shrink-0">
          <svg v-if="message.type === 'success'" class="h-5 w-5 text-green-400" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
          </svg>
          <svg v-else class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="ml-3">
          <p class="text-sm font-medium">{{ message.text }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'

export default {
  name: 'Settings',
  setup() {
    // State
    const saving = ref(false)
    const message = ref(null)

    const systemInfo = reactive({
      version: '',
      buildTime: '',
      host: '',
      port: '',
      maxConnections: '',
      timeout: ''
    })

    const serverConfig = reactive({
      name: '',
      description: '',
      maxConnections: 200,
      timeoutMinutes: 5
    })

    const blocklistConfig = reactive({
      enabled: true,
      callsigns: []
    })

    const loggingConfig = reactive({
      level: 'info',
      format: 'text',
      file: '',
      maxSize: 100
    })

    // Computed
    const messageClass = computed(() => {
      if (!message.value) return ''
      return message.value.type === 'success'
        ? 'bg-green-50 border border-green-200 text-green-800'
        : 'bg-red-50 border border-red-200 text-red-800'
    })

    // Methods
    const showMessage = (text, type = 'success') => {
      message.value = { text, type }
      setTimeout(() => {
        message.value = null
      }, 5000)
    }

    const fetchSystemInfo = async () => {
      try {
        const response = await axios.get('/api/system/info')
        Object.assign(systemInfo, response.data)
      } catch (err) {
        console.error('Error fetching system info:', err)
      }
    }

    const fetchServerConfig = async () => {
      try {
        const response = await axios.get('/api/config/server')
        Object.assign(serverConfig, response.data)
      } catch (err) {
        console.error('Error fetching server config:', err)
      }
    }

    const fetchBlocklistConfig = async () => {
      try {
        const response = await axios.get('/api/config/blocklist')
        Object.assign(blocklistConfig, response.data)
      } catch (err) {
        console.error('Error fetching blocklist config:', err)
      }
    }

    const fetchLoggingConfig = async () => {
      try {
        const response = await axios.get('/api/config/logging')
        Object.assign(loggingConfig, response.data)
      } catch (err) {
        console.error('Error fetching logging config:', err)
      }
    }

    const saveServerConfig = async () => {
      try {
        saving.value = true
        await axios.put('/api/config/server', serverConfig)
        showMessage('Server configuration saved successfully')
      } catch (err) {
        showMessage('Failed to save server configuration', 'error')
        console.error('Error saving server config:', err)
      } finally {
        saving.value = false
      }
    }

    const saveBlocklist = async () => {
      try {
        saving.value = true
        const filteredCallsigns = blocklistConfig.callsigns.filter(c => c.trim() !== '')
        await axios.put('/api/config/blocklist', {
          ...blocklistConfig,
          callsigns: filteredCallsigns
        })
        showMessage('Blocklist saved successfully')
      } catch (err) {
        showMessage('Failed to save blocklist', 'error')
        console.error('Error saving blocklist:', err)
      } finally {
        saving.value = false
      }
    }

    const saveLoggingConfig = async () => {
      try {
        saving.value = true
        await axios.put('/api/config/logging', loggingConfig)
        showMessage('Logging configuration saved successfully')
      } catch (err) {
        showMessage('Failed to save logging configuration', 'error')
        console.error('Error saving logging config:', err)
      } finally {
        saving.value = false
      }
    }

    const addBlockedCallsign = () => {
      blocklistConfig.callsigns.push('')
    }

    const removeBlockedCallsign = (index) => {
      blocklistConfig.callsigns.splice(index, 1)
    }

    const resetServerConfig = () => {
      fetchServerConfig()
    }

    const resetBlocklist = () => {
      fetchBlocklistConfig()
    }

    const resetLoggingConfig = () => {
      fetchLoggingConfig()
    }

    const exportConfig = () => {
      // This would typically export the current configuration
      showMessage('Configuration export would be implemented here')
    }

    const downloadLogs = () => {
      // This would typically download log files
      showMessage('Log download would be implemented here')
    }

    const restartServer = async () => {
      if (confirm('Are you sure you want to restart the server? This will disconnect all repeaters temporarily.')) {
        try {
          await axios.post('/api/system/restart')
          showMessage('Server restart initiated')
        } catch (err) {
          showMessage('Failed to restart server', 'error')
          console.error('Error restarting server:', err)
        }
      }
    }

    // Initialize
    onMounted(() => {
      fetchSystemInfo()
      fetchServerConfig()
      fetchBlocklistConfig()
      fetchLoggingConfig()
    })

    return {
      // State
      saving,
      message,
      systemInfo,
      serverConfig,
      blocklistConfig,
      loggingConfig,

      // Computed
      messageClass,

      // Methods
      saveServerConfig,
      saveBlocklist,
      saveLoggingConfig,
      addBlockedCallsign,
      removeBlockedCallsign,
      resetServerConfig,
      resetBlocklist,
      resetLoggingConfig,
      exportConfig,
      downloadLogs,
      restartServer
    }
  }
}
</script>