import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api, { AUTH_TOKEN_KEY, REFRESH_TOKEN_KEY } from '../api/index'
import { clearApiCache } from '../api/cache'

export const useAuthStore = defineStore('auth', () => {
  // Session lives server-side in an httpOnly cookie — no token/user is kept
  // in browser storage. The user object is held in memory only.
  const user = ref<any>(null)
  const bootstrapped = ref(false)

  let resolveReady: () => void
  const authReady = new Promise<void>((resolve) => {
    resolveReady = resolve
  })

  const isLoggedIn = computed(() => !!user.value)
  const username = computed(() => user.value?.username || '')

  // Bootstrap auth state: ensure the csrf cookie exists, then restore the
  // session from the httpOnly `session` cookie. Safe to call on every page
  // load — a 401 from /auth/validate simply leaves the user logged out.
  async function init() {
    try {
      // Always hit the server: these bootstrap calls decide the auth state.
      await api.get('/auth/csrf', { cache: { skipCache: true } })
    } catch {
      // Non-fatal — /auth/validate below determines the real auth state.
    }
    try {
      const res = await api.get('/auth/validate', { cache: { skipCache: true } })
      user.value = res.data.user
    } catch {
      user.value = null
    } finally {
      bootstrapped.value = true
      resolveReady()
    }
  }

  async function login(username: string, password: string, turnstileToken?: string) {
    try {
      const body: Record<string, string> = { username, password }
      // Always include the field when a token is present so siteverify can run.
      if (turnstileToken) body['cf-turnstile-response'] = turnstileToken
      const res = await api.post('/public/login', body)
      storeTokens(res.data)
      user.value = res.data.user
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Login failed'
      return { success: false, error: apiError }
    }
  }

  async function register(username: string, password: string, turnstileToken?: string, email?: string) {
    try {
      const body: Record<string, string> = { username, password }
      if (email) body.email = email
      if (turnstileToken) body['cf-turnstile-response'] = turnstileToken
      const res = await api.post('/public/register', body)
      // Backend always returns a token (parity with login) — store it so the
      // Bearer fallback works on cross-site frontends (pages.dev).
      storeTokens(res.data)
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
      // The refresh token lets the server revoke the session even if the access token expired.
      await api.post('/auth/logout', { refresh_token: localStorage.getItem(REFRESH_TOKEN_KEY) || undefined })
    } catch {
      // Clear local state regardless of the server response.
    }
    clearApiCache()
    localStorage.removeItem(AUTH_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    user.value = null
  }

  function storeTokens(data: { token?: string; refresh_token?: string }) {
    if (data.token) localStorage.setItem(AUTH_TOKEN_KEY, data.token)
    if (data.refresh_token) localStorage.setItem(REFRESH_TOKEN_KEY, data.refresh_token)
  }

  return {
    user,
    isLoggedIn,
    username,
    bootstrapped,
    authReady,
    init,
    login,
    register,
    logout,
  }
})
