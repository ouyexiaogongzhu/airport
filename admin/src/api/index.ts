import axios from 'axios'
import {
  applyGetCache,
  buildCachedResponse,
  clearApiCache,
  isCacheHit,
  storeCachedResponse,
} from './cache'

// Auth is cookie-based: session lives in httpOnly `admin_session`/`admin_refresh`
// cookies set by the backend, and CSRF protection uses double-submit via the
// JS-readable `admin_csrf` cookie.

export function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const match = document.cookie.match(new RegExp(`(?:^|;\\s*)${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : null
}

const UNSAFE_METHODS = new Set(['post', 'put', 'patch', 'delete'])

// The auth store registers a callback here so a 401 can clear in-memory state
// without creating an import cycle (auth store imports this module).
let onUnauthorized: (() => void) | null = null
export function setOnUnauthorized(handler: (() => void) | null) {
  onUnauthorized = handler
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
})

// 跨站前端（pages.dev）第三方 cookie 被丟棄 → localStorage Bearer 兜底
export const AUTH_TOKEN_KEY = 'auth_token'
// 跨站時 admin_refresh cookie 同樣存不住 → refresh token 放 localStorage，401 時用它續期
export const REFRESH_TOKEN_KEY = 'refresh_token'

// Endpoints whose 401 must not trigger a refresh-and-retry.
const NO_REFRESH_URLS = ['/admin/auth/login', '/auth/csrf', '/auth/refresh', '/auth/logout']

// Single-flight: concurrent 401s share one /admin/auth/refresh call.
let refreshing: Promise<boolean> | null = null
function refreshSession(): Promise<boolean> {
  refreshing ??= api
    .post('/admin/auth/refresh', { refresh_token: localStorage.getItem(REFRESH_TOKEN_KEY) || undefined })
    .then(res => {
      if (res.data?.token) localStorage.setItem(AUTH_TOKEN_KEY, res.data.token)
      return true
    })
    .catch(() => false)
    .finally(() => {
      refreshing = null
    })
  return refreshing
}

api.interceptors.request.use(cfg => {
  const bearer = localStorage.getItem(AUTH_TOKEN_KEY)
  if (bearer) {
    cfg.headers['Authorization'] = `Bearer ${bearer}`
  }
  const method = (cfg.method || 'get').toLowerCase()
  if (UNSAFE_METHODS.has(method)) {
    const csrfToken = readCookie('admin_csrf')
    if (csrfToken) {
      cfg.headers['X-CSRF-Token'] = csrfToken
    }
    // Any state change invalidates previously cached GET data.
    clearApiCache()
  }
  return cfg
})

// Serve fresh GET responses from the in-memory TTL cache before hitting the
// network. Cached responses resolve through the error path below.
api.interceptors.request.use(cfg => applyGetCache(cfg))

api.interceptors.response.use(
  res => {
    if (res.status >= 200 && res.status < 300) {
      storeCachedResponse(res.config, res.data)
    }
    return res
  },
  async err => {
    if (isCacheHit(err)) {
      return Promise.resolve(buildCachedResponse(err))
    }
    const cfg = err.config as (typeof err.config & { _authRetried?: boolean }) | undefined
    if (
      err.response?.status === 401 &&
      cfg &&
      !cfg._authRetried &&
      !NO_REFRESH_URLS.some(path => (cfg.url || '').includes(path))
    ) {
      cfg._authRetried = true
      if (await refreshSession()) return api(cfg)
    }
    if (err.response?.status === 401) {
      const url = err.config?.url || ''
      const excluded =
        url.includes('/admin/auth/login') ||
        url.includes('/auth/csrf') ||
        url.includes('/auth/validate') ||
        url.includes('/auth/refresh') ||
        url.includes('/auth/logout')
      if (!excluded) {
        clearApiCache()
        onUnauthorized?.()
        window.location.href = '/'
      }
    }
    return Promise.reject(err)
  },
)

export default api
