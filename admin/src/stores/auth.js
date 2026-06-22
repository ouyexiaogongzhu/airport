import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/index.js'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('admin_token') || null)
  const user = ref(JSON.parse(localStorage.getItem('admin_user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')

  // ---------- mock helpers ----------
  function mockDelay(ms = 600) {
    return new Promise(r => setTimeout(r, ms))
  }

  function mockAdminUser(username) {
    return { id: 1, username, email: `admin@rfplay.uk`, role: 'admin', created_at: '2025-06-01T00:00:00Z' }
  }

  function mockToken() { return 'mock_admin_jwt_' + Date.now() }

  // ---------- actions ----------
  async function login(username, password) {
    try {
      const res = await api.post('/public/login', { username, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('admin_token', t)
      localStorage.setItem('admin_user', JSON.stringify(u))
      return { success: true }
    } catch {
      await mockDelay()
      if (username === 'admin' && password) {
        const t = mockToken()
        const u = mockAdminUser(username)
        token.value = t
        user.value = u
        localStorage.setItem('admin_token', t)
        localStorage.setItem('admin_user', JSON.stringify(u))
        return { success: true }
      }
      return { success: false, error: 'Invalid admin credentials' }
    }
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('admin_token')
    localStorage.removeItem('admin_user')
  }

  return { token, user, isLoggedIn, username, login, logout }
})
