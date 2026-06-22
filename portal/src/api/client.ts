import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'https://api.rfplay.uk',
  withCredentials: true,
})

api.interceptors.request.use(cfg => {
  if (['post', 'put', 'patch', 'delete'].includes(cfg.method || '')) {
    const csrf = getCookie('csrf')
    if (csrf) cfg.headers['X-CSRF-Token'] = csrf
  }
  return cfg
})

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : null
}

export default api
