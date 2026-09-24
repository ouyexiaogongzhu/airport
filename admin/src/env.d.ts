/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  // Absolute API origin for clipboard subscription URLs (Clash / Base64).
  // See src/utils/subscriptionUrl.ts.
  readonly VITE_SUBSCRIPTION_BASE_URL?: string
  // Cloudflare Turnstile sitekey (admin login). Same widget as portal.
  readonly VITE_TURNSTILE_SITE_KEY?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

