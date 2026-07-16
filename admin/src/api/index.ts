import axios from 'axios'

// TODO: CSRF protection — add a CSRF token header (e.g. X-CSRF-Token) for state-changing requests
// when the backend supports it. For now, the API relies on Bearer token auth.

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use(cfg => {
  const token = localStorage.getItem('admin_token')
  if (token) {
    cfg.headers.Authorization = `Bearer ${token}`
  }
  return cfg
})

api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      // Skip redirect for login endpoint — let auth.login() handle the error
      const url = err.config?.url || ''
      if (url.includes('/public/login')) {
        return Promise.reject(err)
      }
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_user')
      window.location.href = '/'
    }
    return Promise.reject(err)
  },
)

export default api
