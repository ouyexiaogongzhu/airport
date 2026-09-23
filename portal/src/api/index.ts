import axios from 'axios'
import {
  applyGetCache,
  buildCachedResponse,
  clearApiCache,
  isCacheHit,
  storeCachedResponse,
} from './cache'

// State-changing HTTP methods require a CSRF token header.
const UNSAFE_METHODS = ['post', 'put', 'patch', 'delete']

// Endpoints whose 401 responses are expected (e.g. while bootstrapping auth
// state at app startup) and must not trigger the global session-expired
// redirect to `/`.
const EXEMPT_401_URLS = ['/public/login', '/public/register', '/auth/csrf', '/auth/validate', '/auth/refresh', '/auth/logout']

// Endpoints whose 401 must not trigger a refresh-and-retry.
const NO_REFRESH_URLS = ['/public/login', '/public/register', '/auth/csrf', '/auth/refresh', '/auth/logout']

// Reads a non-httpOnly cookie (used for the CSRF double-submit token).
export function readCookie(name: string): string {
  const prefix = `${name}=`
  const cookies = document.cookie.split(';')
  for (const part of cookies) {
    const cookie = part.trim()
    if (cookie.startsWith(prefix)) {
      return decodeURIComponent(cookie.slice(prefix.length))
    }
  }
  return ''
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 10000,
  // Browser sessions are held in httpOnly cookies; send them with every request.
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})

// CSRF double-submit: echo the `csrf` cookie value in the X-CSRF-Token header
// on state-changing requests. The backend validates the header against the
// cookie and rejects with 403 on mismatch.
// 跨站前端（pages.dev）第三方 cookie 被丟棄 → localStorage Bearer 兜底
export const AUTH_TOKEN_KEY = 'auth_token'
// 跨站時 refresh cookie 同樣存不住 → refresh token 放 localStorage，401 時用它續期
export const REFRESH_TOKEN_KEY = 'refresh_token'

// Single-flight: concurrent 401s share one /auth/refresh call.
let refreshing: Promise<boolean> | null = null
function refreshSession(): Promise<boolean> {
  refreshing ??= api
    .post('/auth/refresh', { refresh_token: localStorage.getItem(REFRESH_TOKEN_KEY) || undefined })
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
    cfg.headers.set('Authorization', `Bearer ${bearer}`)
  }
  const method = (cfg.method || 'get').toLowerCase()
  if (UNSAFE_METHODS.includes(method)) {
    const csrfToken = readCookie('csrf')
    if (csrfToken) {
      cfg.headers.set('X-CSRF-Token', csrfToken)
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
    if (err.response?.status === 401 && cfg && !cfg._authRetried && !matches(NO_REFRESH_URLS, cfg.url)) {
      cfg._authRetried = true
      if (await refreshSession()) return api(cfg)
    }
    if (err.response?.status === 401 && !isExempt401(err.config?.url)) {
      // Full page reload: in-memory auth state resets and the router guard /
      // auth init() run again, landing unauthenticated users on `/`.
      clearApiCache()
      window.location.href = '/'
    }
    return Promise.reject(err)
  },
)

function matches(paths: string[], url?: string): boolean {
  if (!url) return false
  return paths.some(path => url.includes(path))
}

function isExempt401(url?: string): boolean {
  return matches(EXEMPT_401_URLS, url)
}

export default api
