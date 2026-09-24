/// <reference types="vite/client" />

declare global {
  interface ImportMetaEnv {
    // Optional override for the subscription URL base (defaults to
    // VITE_API_BASE_URL-derived logic). See src/utils/subscriptionUrl.ts.
    readonly VITE_SUBSCRIPTION_BASE_URL?: string
    /** Cloudflare Turnstile site key（可選） */
    readonly VITE_TURNSTILE_SITE_KEY?: string
    /** API base，預設 /api/v1 */
    readonly VITE_API_BASE_URL?: string
    /** 有值則顯示 Continue with Google（須與 Worker GOOGLE_CLIENT_ID 對應） */
    readonly VITE_GOOGLE_CLIENT_ID?: string
  }
}

declare module '../stores/auth' {
  import { Store } from 'pinia'
  interface AuthStore {
    user: any
    isLoggedIn: boolean
    username: string
    init(): Promise<void>
    login(username: string, password: string, turnstileToken?: string): Promise<{ success: boolean; error?: string }>
    register(
      username: string,
      password: string,
      turnstileToken?: string,
      email?: string,
    ): Promise<{ success: boolean; error?: string }>
    logout(): Promise<void>
  }
  export function useAuthStore(): AuthStore
}

declare module '../api/index.js' {
  import { AxiosInstance } from 'axios'
  const api: AxiosInstance & { defaults: { baseURL?: string } }
  export default api
}
