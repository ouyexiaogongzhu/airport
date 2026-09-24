/** Google OAuth start URL — Worker redirect flow（非 GIS one-tap） */

function apiV1Base(): string {
  const raw = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim() || '/api/v1'
  if (raw.startsWith('http://') || raw.startsWith('https://')) {
    return raw.replace(/\/$/, '')
  }
  // 相對路徑（本地 vite proxy）：用當前 origin
  if (typeof window !== 'undefined') {
    const path = raw.startsWith('/') ? raw : `/${raw}`
    return `${window.location.origin}${path.replace(/\/$/, '')}`
  }
  return raw.replace(/\/$/, '')
}

/** 設了 VITE_GOOGLE_CLIENT_ID 才顯示按鈕（與 Worker secrets 配對；僅作 UI 開關） */
export function isGoogleSignInEnabled(): boolean {
  return !!(import.meta.env.VITE_GOOGLE_CLIENT_ID as string | undefined)?.trim()
}

export function googleOAuthStartUrl(): string {
  return `${apiV1Base()}/public/oauth/google/start`
}
