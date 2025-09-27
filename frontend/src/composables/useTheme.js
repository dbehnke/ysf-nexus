import { ref, watch, onMounted } from 'vue'

const isDark = ref(false)

export function useTheme() {
  const setTheme = (dark) => {
    isDark.value = dark
    if (dark) {
      document.documentElement.classList.add('dark')
      localStorage.setItem('theme', 'dark')
    } else {
      document.documentElement.classList.remove('dark')
      localStorage.setItem('theme', 'light')
    }
  }

  const toggleTheme = () => {
    setTheme(!isDark.value)
  }

  const initTheme = () => {
    // Check localStorage first
    const savedTheme = localStorage.getItem('theme')

    if (savedTheme) {
      setTheme(savedTheme === 'dark')
    } else {
      // Default to system preference
      const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      setTheme(systemDark)
    }

    // Listen for system theme changes
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', (e) => {
      // Only update if user hasn't manually set a preference
      if (!localStorage.getItem('theme')) {
        setTheme(e.matches)
      }
    })
  }

  onMounted(() => {
    initTheme()
  })

  return {
    isDark,
    setTheme,
    toggleTheme,
    initTheme
  }
}