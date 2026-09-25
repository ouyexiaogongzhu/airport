import api from '../api'

const API_V1_PREFIX = '/api/v1'
const PUBLIC_API_ORIGIN = 'https://api.rfplay.uk'

// Builds a fully-qualified subscription URL for import into external proxy
// apps (V2rayNG, Shadowrocket, Clash Verge, Stash, etc.).
//
// Base resolution order:
//   1. VITE_SUBSCRIPTION_BASE_URL (explicit override, e.g. https://api.rfplay.uk)
//   2. api.defaults.baseURL (VITE_API_BASE_URL) — in dev this is often a
//      relative path like "/api/v1" that proxy apps cannot resolve on their own.
//   3. "/api/v1" fallback.
//
// If the base does not already end with "/api/v1" the prefix is appended, so
// the result always targets the real manager endpoint
// /api/v1/client/links/{token}[{/clash}]. Absolute http(s) bases are kept as-is
// (including local dev). Relative bases are prefixed with the public API
// origin so clipboard URLs never follow the page host (xv / xva).
export type SubscriptionFormat = 'base64' | 'clash'

export function buildSubscriptionUrl(token: string, format: SubscriptionFormat = 'base64'): string {
  if (!token) return ''
  const base = (
    import.meta.env.VITE_SUBSCRIPTION_BASE_URL || api.defaults.baseURL || API_V1_PREFIX
  )
    .trim()
    .replace(/\/+$/, '')
  const withPrefix = base.endsWith(API_V1_PREFIX) ? base : `${base}${API_V1_PREFIX}`
  const suffix = format === 'clash' ? '/clash' : ''
  const url = `${withPrefix}/client/links/${token}${suffix}`
  if (/^https?:\/\//i.test(url)) return url
  return `${PUBLIC_API_ORIGIN}${url}`
}
