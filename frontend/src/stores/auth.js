import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

export const useAuthStore = defineStore('auth', () => {
  // State
  const token = ref(localStorage.getItem('auth_token') || null)
  const authRequired = ref(false)
  const isLoading = ref(false)
  const error = ref(null)

  // Computed
  const isAuthenticated = computed(() => {
    return !authRequired.value || !!token.value
  })

  const needsAuth = computed(() => {
    return authRequired.value && !token.value
  })

  // Actions
  const checkAuthStatus = async () => {
    try {
      isLoading.value = true
      error.value = null

      const response = await axios.get('/api/auth/status')
      authRequired.value = response.data.auth_required

      // If auth is required but we're not authenticated, clear any stale token
      if (authRequired.value && !response.data.authenticated) {
        token.value = null
        localStorage.removeItem('auth_token')
      }

      return response.data
    } catch (err) {
      console.error('Error checking auth status:', err)
      error.value = 'Failed to check authentication status'
      return null
    } finally {
      isLoading.value = false
    }
  }

  const login = async (username, password) => {
    try {
      isLoading.value = true
      error.value = null

      const response = await axios.post('/api/auth/login', {
        username,
        password
      })

      if (response.data.success) {
        token.value = response.data.token
        localStorage.setItem('auth_token', response.data.token)
        setupAxiosInterceptor()
        return true
      } else {
        error.value = 'Login failed'
        return false
      }
    } catch (err) {
      console.error('Login error:', err)
      error.value = err.response?.data || 'Login failed'
      return false
    } finally {
      isLoading.value = false
    }
  }

  const logout = async () => {
    try {
      // Call logout endpoint if we have a token
      if (token.value) {
        await axios.post('/api/auth/logout')
      }
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      // Clear local state regardless of API call success
      token.value = null
      localStorage.removeItem('auth_token')
      setupAxiosInterceptor()
    }
  }

  const setupAxiosInterceptor = () => {
    // Remove any existing interceptor
    axios.interceptors.request.eject(0)

    // Add request interceptor to include auth token
    axios.interceptors.request.use(
      (config) => {
        if (token.value) {
          config.headers.Authorization = `Bearer ${token.value}`
        }
        return config
      },
      (error) => {
        return Promise.reject(error)
      }
    )

    // Add response interceptor to handle auth errors
    axios.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401 && token.value) {
          // Token is invalid/expired, clear it
          token.value = null
          localStorage.removeItem('auth_token')
        }
        return Promise.reject(error)
      }
    )
  }

  const clearError = () => {
    error.value = null
  }

  // Initialize interceptor on store creation
  setupAxiosInterceptor()

  return {
    // State
    token,
    authRequired,
    isLoading,
    error,

    // Computed
    isAuthenticated,
    needsAuth,

    // Actions
    checkAuthStatus,
    login,
    logout,
    clearError
  }
})