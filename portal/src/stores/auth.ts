import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  // Session lives server-side in an httpOnly cookie — no token/user is kept
  // in browser storage. The user object is held in memory only.
  const user = ref<any>(null)

  const isLoggedIn = computed(() => !!user.value)
  const username = computed(() => user.value?.username || '')

  // Bootstrap auth state: ensure the csrf cookie exists, then restore the
  // session from the httpOnly `session` cookie. Safe to call on every page
  // load — a 401 from /auth/validate simply leaves the user logged out.
  async function init() {
    try {
      await api.get('/auth/csrf')
    } catch {
      // Non-fatal — /auth/validate below determines the real auth state.
    }
    try {
      const res = await api.get('/auth/validate')
      user.value = res.data.user
    } catch {
      user.value = null
    }
  }

  async function login(username: string, password: string) {
    try {
      const res = await api.post('/public/login', { username, password })
      user.value = res.data.user
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Login failed'
      return { success: false, error: apiError }
    }
  }

  async function register(username: string, email: string, password: string, _captchaToken?: string, _captchaAnswer?: string) {
    try {
      const res = await api.post('/public/register', { username, email, password })
      user.value = res.data.user
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Registration failed'
      return { success: false, error: apiError }
    }
  }

  async function logout() {
    try {
      // State-changing request; CSRF header is attached by the api interceptor.
      await api.post('/auth/logout')
    } catch {
      // Clear local state regardless of the server response.
    }
    user.value = null
  }

  return { user, isLoggedIn, username, init, login, register, logout }
})
