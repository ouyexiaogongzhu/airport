/** Google OAuth start URL — Worker redirect flow（非 GIS one-tap） */

const PUBLIC_API_V1 = 'https://api.rfplay.uk/api/v1'

/**
 * OAuth redirect URIs are registered on api.rfplay.uk — always prefer that
 * absolute host. Do not use the Pages same-origin `/api` proxy for OAuth
 * (Google callback must hit the Worker custom domain).
 */
function apiV1Base(): string {
  const sub = (import.meta.env.VITE_SUBSCRIPTION_BASE_URL as string | undefined)?.trim()
  if (sub) {
    const host = sub.replace(/\/$/, '')
    return host.endsWith('/api/v1') ? host : `${host}/api/v1`
  }
  const raw = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim() || '/api/v1'
  if (raw.startsWith('http://') || raw.startsWith('https://')) {
    return raw.replace(/\/$/, '')
  }
  // Relative VITE_API_BASE_URL (Pages /api proxy): public API host for OAuth
  return PUBLIC_API_V1
}

/** 設了 VITE_GOOGLE_CLIENT_ID 才顯示按鈕（與 Worker secrets 配對；僅作 UI 開關） */
export function isGoogleSignInEnabled(): boolean {
  return !!(import.meta.env.VITE_GOOGLE_CLIENT_ID as string | undefined)?.trim()
}

export function googleOAuthStartUrl(): string {
  return `${apiV1Base()}/public/oauth/google/start`
}
