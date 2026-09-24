import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

vi.mock('../api/index', () => ({
  default: { defaults: { baseURL: '/api/v1' } },
}))

import { buildSubscriptionUrl } from '../utils/subscriptionUrl'

describe('buildSubscriptionUrl (admin)', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', '')
    vi.stubGlobal('location', { origin: 'https://xva.rfplay.uk' })
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('uses VITE_SUBSCRIPTION_BASE_URL and appends /api/v1', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', 'https://api.rfplay.uk')
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('appends /clash for the clash format', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', 'https://api.rfplay.uk')
    expect(buildSubscriptionUrl('tok_1', 'clash')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1/clash',
    )
  })

  it('resolves relative base against page origin', () => {
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://xva.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('returns empty string for empty token', () => {
    expect(buildSubscriptionUrl('')).toBe('')
    expect(buildSubscriptionUrl('', 'clash')).toBe('')
  })
})
