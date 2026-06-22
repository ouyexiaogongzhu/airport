declare module '../stores/auth' {
  import { Store } from 'pinia'
  interface AuthStore {
    token: string | null
    user: any
    isLoggedIn: boolean
    username: string
    login(username: string, password: string): Promise<{ success: boolean; error?: string }>
    register(username: string, email: string, password: string): Promise<{ success: boolean; error?: string }>
    logout(): void
  }
  export function useAuthStore(): AuthStore
}

declare module '../api/index.js' {
  import { AxiosInstance } from 'axios'
  const api: AxiosInstance & { defaults: { baseURL?: string } }
  export default api
}
