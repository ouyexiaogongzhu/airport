/// <reference types="vite/client" />

declare global {
  interface ImportMetaEnv {
    // Optional override for the subscription URL base (defaults to
    // VITE_API_BASE_URL-derived logic). See src/utils/subscriptionUrl.ts.
    readonly VITE_SUBSCRIPTION_BASE_URL?: string
  }
}

declare module '../stores/auth' {
  import { Store } from 'pinia'
  interface AuthStore {
    user: any
    isLoggedIn: boolean
    username: string
    init(): Promise<void>
    login(username: string, password: string): Promise<{ success: boolean; error?: string }>
    register(username: string, password: string, turnstileToken?: string): Promise<{ success: boolean; error?: string }>
    logout(): Promise<void>
  }
  export function useAuthStore(): AuthStore
}

declare module '../api/index.js' {
  import { AxiosInstance } from 'axios'
  const api: AxiosInstance & { defaults: { baseURL?: string } }
  export default api
}
