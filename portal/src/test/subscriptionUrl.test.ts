import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import api from '../api'
import { buildSubscriptionUrl } from '../utils/subscriptionUrl'

describe('buildSubscriptionUrl', () => {
  beforeEach(() => {
    vi.unstubAllEnvs()
    // setup.ts replaces window.location with a bare { href: '' } stub, so give
    // it an origin for the relative-base assertions below.
    Object.defineProperty(globalThis, 'location', {
      value: { href: '', origin: 'https://portal.rfplay.uk' },
      writable: true,
    })
  })

  afterEach(() => {
    api.defaults.baseURL = '/api/v1'
  })

  it('uses VITE_SUBSCRIPTION_BASE_URL and appends /api/v1', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', 'https://api.rfplay.uk')
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('derives from VITE_API_BASE_URL absolute URL and appends /api/v1', () => {
    api.defaults.baseURL = 'https://api.rfplay.uk'
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('derives from a relative /api/v1 base and resolves against the origin', () => {
    api.defaults.baseURL = '/api/v1'
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://portal.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('does not duplicate /api/v1 when the base already contains it', () => {
    api.defaults.baseURL = 'https://api.rfplay.uk/api/v1'
    expect(buildSubscriptionUrl('tok_1')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1',
    )
  })

  it('returns an empty string for an empty token', () => {
    expect(buildSubscriptionUrl('')).toBe('')
  })

  it('appends /clash for the clash format', () => {
    vi.stubEnv('VITE_SUBSCRIPTION_BASE_URL', 'https://api.rfplay.uk')
    expect(buildSubscriptionUrl('tok_1', 'clash')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1/clash',
    )
  })

  it('resolves the clash format against the origin for a relative base', () => {
    api.defaults.baseURL = '/api/v1'
    expect(buildSubscriptionUrl('tok_1', 'clash')).toBe(
      'https://portal.rfplay.uk/api/v1/client/links/tok_1/clash',
    )
  })

  it('does not duplicate /api/v1 for the clash format either', () => {
    api.defaults.baseURL = 'https://api.rfplay.uk/api/v1'
    expect(buildSubscriptionUrl('tok_1', 'clash')).toBe(
      'https://api.rfplay.uk/api/v1/client/links/tok_1/clash',
    )
  })

  it('returns an empty string for an empty token in clash format', () => {
    expect(buildSubscriptionUrl('', 'clash')).toBe('')
  })
})
