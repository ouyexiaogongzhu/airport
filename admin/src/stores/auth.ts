import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api, { setOnUnauthorized } from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  // Session state lives in memory only — the backend holds the actual session
  // in httpOnly cookies (admin_session/admin_refresh).
  const user = ref<Record<string, any> | null>(null)
  const role = ref<string | null>(null)

  const isLoggedIn = computed(() => !!user.value)
  const username = computed(() => user.value?.username || '')

  // Clear in-memory state when the API layer observes a 401.
  setOnUnauthorized(() => {
    user.value = null
    role.value = null
  })

  async function login(username: string, password: string) {
    try {
      const res = await api.post('/admin/auth/login', { username, password })
      user.value = res.data.user ?? null
      role.value = res.data.role ?? null
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Login failed'
      return { success: false, error: apiError }
    }
  }

  async function logout() {
    try {
      await api.post('/admin/auth/logout')
    } catch {
      // Best-effort server-side session invalidation — clear local state regardless.
    } finally {
      user.value = null
      role.value = null
    }
  }

  async function init() {
    try {
      await api.get('/auth/csrf')
    } catch {
      // CSRF bootstrap failure should not prevent the validate attempt below.
    }
    try {
      const res = await api.get('/auth/validate')
      user.value = res.data.user ?? null
      role.value = res.data.role ?? null
    } catch {
      user.value = null
      role.value = null
    }
  }

  return { user, role, isLoggedIn, username, login, logout, init }
})
