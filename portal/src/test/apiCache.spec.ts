import { describe, it, expect, vi, beforeEach } from 'vitest'
import api from '../api/index'
import { clearApiCache } from '../api/cache'

// The api module is a singleton (imported above); the cache is module-level,
// so reset it between tests to avoid cross-test pollution.
beforeEach(() => {
  clearApiCache()
})

// Custom adapter that never touches the network, so we can observe exactly
// how many times a request actually reaches the transport layer.
const adapterCalls: string[] = []
function countingAdapter(config: any) {
  adapterCalls.push(`${config.method}:${config.url}:${JSON.stringify(config.params ?? {})}`)
  return Promise.resolve({
    data: { url: config.url, params: config.params },
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
    request: {},
  })
}

describe('portal GET cache', () => {
  beforeEach(() => {
    adapterCalls.length = 0
  })

  it('serves the same URL from cache on the second call', async () => {
    const first = await api.get('/cached', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)

    const second = await api.get('/cached', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)
    expect(second.data).toEqual(first.data)
  })

  it('keeps distinct cache entries per serialized params', async () => {
    await api.get('/list', { params: { a: 1 }, adapter: countingAdapter })
    await api.get('/list', { params: { a: 1 }, adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)

    await api.get('/list', { params: { a: 2 }, adapter: countingAdapter })
    expect(adapterCalls.length).toBe(2)
  })

  it('re-fetches after the cache TTL expires', async () => {
    await api.get('/ttl', { cache: { cacheTtl: 40 }, adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)

    await new Promise(r => setTimeout(r, 80))
    await api.get('/ttl', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(2)
  })

  it('honors skipCache and always hits the transport', async () => {
    await api.get('/skip', { cache: { skipCache: true }, adapter: countingAdapter })
    await api.get('/skip', { cache: { skipCache: true }, adapter: countingAdapter })
    expect(adapterCalls.length).toBe(2)
  })

  it('honors cache: false as a full opt-out', async () => {
    await api.get('/off', { cache: false, adapter: countingAdapter })
    await api.get('/off', { cache: false, adapter: countingAdapter })
    expect(adapterCalls.length).toBe(2)
  })

  it('clears the cache on clearApiCache()', async () => {
    await api.get('/cleared', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)

    clearApiCache()
    await api.get('/cleared', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(2)
  })

  it('invalidates cached GET data when a mutation is dispatched', async () => {
    await api.get('/mut', { adapter: countingAdapter })
    expect(adapterCalls.length).toBe(1)

    await api.post('/mut', { x: 1 }, { adapter: countingAdapter })
    await api.get('/mut', { adapter: countingAdapter })
    // get + post + get again (cache was cleared by the POST)
    expect(adapterCalls.length).toBe(3)
  })
})
