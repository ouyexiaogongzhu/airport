import axios from 'axios'

// State-changing HTTP methods require a CSRF token header.
const UNSAFE_METHODS = ['post', 'put', 'patch', 'delete']

// Endpoints whose 401 responses are expected (e.g. while bootstrapping auth
// state at app startup) and must not trigger the global session-expired
// redirect to `/`.
const EXEMPT_401_URLS = ['/public/login', '/public/register', '/auth/csrf', '/auth/validate']

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
api.interceptors.request.use(cfg => {
  const method = (cfg.method || 'get').toLowerCase()
  if (UNSAFE_METHODS.includes(method)) {
    const csrfToken = readCookie('csrf')
    if (csrfToken) {
      cfg.headers.set('X-CSRF-Token', csrfToken)
    }
  }
  return cfg
})

api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401 && !isExempt401(err.config?.url)) {
      // Full page reload: in-memory auth state resets and the router guard /
      // auth init() run again, landing unauthenticated users on `/`.
      window.location.href = '/'
    }
    return Promise.reject(err)
  },
)

function isExempt401(url?: string): boolean {
  if (!url) return false
  return EXEMPT_401_URLS.some(path => url.includes(path))
}

export default api
