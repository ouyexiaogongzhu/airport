import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api, { setOnUnauthorized, AUTH_TOKEN_KEY, REFRESH_TOKEN_KEY } from '../api/index'
import { clearApiCache } from '../api/cache'

export const useAuthStore = defineStore('auth', () => {
  // Session state lives in memory only — the backend holds the actual session
  // in httpOnly cookies (admin_session/admin_refresh).
  const user = ref<Record<string, any> | null>(null)
  const role = ref<string | null>(null)

  const isLoggedIn = computed(() => !!user.value)
  const username = computed(() => user.value?.username || '')

  // Clear in-memory state when the API layer observes a 401.
  setOnUnauthorized(() => {
    clearApiCache()
    user.value = null
    role.value = null
  })

  async function login(username: string, password: string, turnstileToken?: string) {
    try {
      const body: Record<string, string> = { username, password }
      if (turnstileToken) body['cf-turnstile-response'] = turnstileToken
      const res = await api.post('/admin/auth/login', body)
      if (res.data.token) localStorage.setItem(AUTH_TOKEN_KEY, res.data.token)
      if (res.data.refresh_token) localStorage.setItem(REFRESH_TOKEN_KEY, res.data.refresh_token)
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
      // The refresh token lets the server revoke the session even if the access token expired.
      await api.post('/admin/auth/logout', { refresh_token: localStorage.getItem(REFRESH_TOKEN_KEY) || undefined })
    } catch {
      // Best-effort server-side session invalidation — clear local state regardless.
    } finally {
      localStorage.removeItem(AUTH_TOKEN_KEY)
      localStorage.removeItem(REFRESH_TOKEN_KEY)
      clearApiCache()
      user.value = null
      role.value = null
    }
  }

  async function init() {
    try {
      // Always hit the server: these bootstrap calls decide the auth state.
      await api.get('/auth/csrf', { cache: { skipCache: true } })
    } catch {
      // CSRF bootstrap failure should not prevent the validate attempt below.
    }
    try {
      // Admin-only check: reads admin_session (never the portal `session` cookie).
      const res = await api.get('/admin/auth/validate', { cache: { skipCache: true } })
      user.value = res.data.user ?? null
      role.value = res.data.role ?? res.data.user?.role ?? null
    } catch {
      user.value = null
      role.value = null
    }
  }

  return { user, role, isLoggedIn, username, login, logout, init }
})
