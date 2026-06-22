import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/index.js'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('portal_token') || null)
  const user = ref(JSON.parse(localStorage.getItem('portal_user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')

  // ---------- mock helpers ----------
  function mockDelay(ms = 600) {
    return new Promise(r => setTimeout(r, ms))
  }

  function mockUser(username, role = 'user') {
    return { id: 1, username, email: `${username}@rfplay.uk`, role, created_at: '2026-01-15T08:00:00Z' }
  }

  function mockToken() { return 'mock_jwt_' + Date.now() }

  // ---------- actions ----------
  async function login(username, password) {
    try {
      const res = await api.post('/public/login', { username, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('portal_token', t)
      localStorage.setItem('portal_user', JSON.stringify(u))
      return { success: true }
    } catch {
      // fallback to mock
      await mockDelay()
      if (username && password) {
        const t = mockToken()
        const u = mockUser(username)
        token.value = t
        user.value = u
        localStorage.setItem('portal_token', t)
        localStorage.setItem('portal_user', JSON.stringify(u))
        return { success: true }
      }
      return { success: false, error: 'Invalid credentials' }
    }
  }

  async function register(username, email, password) {
    try {
      const res = await api.post('/public/register', { username, email, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('portal_token', t)
      localStorage.setItem('portal_user', JSON.stringify(u))
      return { success: true }
    } catch {
      await mockDelay()
      if (username && email && password) {
        const t = mockToken()
        const u = mockUser(username)
        token.value = t
        user.value = u
        localStorage.setItem('portal_token', t)
        localStorage.setItem('portal_user', JSON.stringify(u))
        return { success: true }
      }
      return { success: false, error: 'Registration failed' }
    }
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('portal_token')
    localStorage.removeItem('portal_user')
  }

  return { token, user, isLoggedIn, username, login, register, logout }
})
