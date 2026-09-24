import { describe, it, expect, vi, afterEach } from 'vitest'
import { googleOAuthStartUrl } from '../utils/googleAuth'

describe('googleOAuthStartUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('uses VITE_SUBSCRIPTION_BASE_URL absolute host for OAuth', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', 'https://api.rfplay.uk')
    vi.stubEnv('VITE_API_BASE_URL', '/api/v1')
    expect(googleOAuthStartUrl()).toBe(
      'https://api.rfplay.uk/api/v1/public/oauth/google/start',
    )
  })

  it('falls back to public API when VITE_API_BASE_URL is relative', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', '')
    vi.stubEnv('VITE_API_BASE_URL', '/api/v1')
    expect(googleOAuthStartUrl()).toBe(
      'https://api.rfplay.uk/api/v1/public/oauth/google/start',
    )
  })

  it('keeps absolute VITE_API_BASE_URL when subscription unset', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', '')
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.rfplay.uk/api/v1')
    expect(googleOAuthStartUrl()).toBe(
      'https://api.rfplay.uk/api/v1/public/oauth/google/start',
    )
  })
})
