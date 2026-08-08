import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

// Lightweight in-memory TTL cache for GET responses. Keyed by
// `METHOD url?params` so two requests with the same URL and serialized
// params reuse the same entry. Bounded by MAX_CACHE_ENTRIES.
interface CacheEntry {
  data: unknown
  expires: number
}

// Per-request cache options passed through the axios config, e.g.
// `api.get(url, { cache: { skipCache: true } })` or `{ cache: { cacheTtl: ms } }`.
export interface CacheOptions {
  skipCache?: boolean
  cacheTtl?: number
  skip?: boolean
  ttl?: number
}

declare module 'axios' {
  export interface AxiosRequestConfig {
    cache?: boolean | CacheOptions
  }
  export interface InternalAxiosRequestConfig {
    __cacheKey?: string
    __cacheTtl?: number
  }
}

export const DEFAULT_CACHE_TTL_MS = 15_000
const MAX_CACHE_ENTRIES = 200

const cache = new Map<string, CacheEntry>()

export function createCacheKey(method: string, url: string, params?: unknown): string {
  let paramsStr = ''
  if (params != null) {
    try {
      paramsStr = JSON.stringify(params)
    } catch {
      paramsStr = String(params)
    }
  }
  return `${method.toUpperCase()} ${url}?${paramsStr}`
}

export function getCached(key: string): { found: boolean; data: unknown } {
  const entry = cache.get(key)
  if (!entry) return { found: false, data: undefined }
  if (Date.now() > entry.expires) {
    cache.delete(key)
    return { found: false, data: undefined }
  }
  return { found: true, data: entry.data }
}

export function setCached(key: string, data: unknown, ttlMs: number = DEFAULT_CACHE_TTL_MS): void {
  if (cache.size >= MAX_CACHE_ENTRIES) {
    // Evict oldest entries (insertion order) to keep the map bounded.
    for (const oldestKey of cache.keys()) {
      cache.delete(oldestKey)
      if (cache.size < MAX_CACHE_ENTRIES) break
    }
  }
  cache.set(key, { data, expires: Date.now() + Math.max(0, ttlMs) })
}

export function clearApiCache(): void {
  cache.clear()
}

// Marker attached to the synthetic rejection used to short-circuit a request
// from inside the request interceptor when a cache entry is still fresh.
export interface CacheHitSignal {
  __rfplayCacheHit: true
  key: string
  data: unknown
  config: InternalAxiosRequestConfig
}

export function isCacheHit(err: unknown): err is CacheHitSignal {
  return (
    !!err &&
    typeof err === 'object' &&
    (err as Record<string, unknown>).__rfplayCacheHit === true
  )
}

export function resolveCacheOptions(config: { cache?: boolean | CacheOptions }): {
  enabled: boolean
  ttl: number
} {
  const c = config.cache
  if (c === false) return { enabled: false, ttl: DEFAULT_CACHE_TTL_MS }
  if (c && typeof c === 'object') {
    if (c.skipCache === true || c.skip === true) return { enabled: false, ttl: DEFAULT_CACHE_TTL_MS }
    return { enabled: true, ttl: c.cacheTtl ?? c.ttl ?? DEFAULT_CACHE_TTL_MS }
  }
  return { enabled: true, ttl: DEFAULT_CACHE_TTL_MS }
}

// Read path: called from the request interceptor. On a fresh cache entry a
// rejected promise carrying the cached data is returned; the response error
// interceptor turns it back into a resolved response.
export function applyGetCache(
  cfg: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig | Promise<never> {
  const method = (cfg.method || 'get').toLowerCase()
  if (method !== 'get') return cfg
  const options = resolveCacheOptions(cfg)
  if (!options.enabled) return cfg

  const key = createCacheKey(method, cfg.url || '', cfg.params)
  const hit = getCached(key)
  if (hit.found) {
    const signal: CacheHitSignal = { __rfplayCacheHit: true, key, data: hit.data, config: cfg }
    return Promise.reject(signal)
  }
  cfg.__cacheKey = key
  cfg.__cacheTtl = options.ttl
  return cfg
}

// Write path: called from the response success interceptor for 2xx GETs.
export function storeCachedResponse(
  cfg: InternalAxiosRequestConfig | undefined,
  data: unknown,
): void {
  if (cfg?.__cacheKey) {
    setCached(cfg.__cacheKey, data, cfg.__cacheTtl)
  }
}

export function buildCachedResponse(signal: CacheHitSignal): AxiosResponse {
  return {
    data: signal.data,
    status: 200,
    statusText: 'OK',
    headers: {},
    config: signal.config,
    request: {},
  }
}
