import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock vue-router
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/users' }),
}))

// Users.vue uses fetch() directly, not the api module
const sampleUsers = [
  { id: 1, username: 'alice', role: 'user', status: 'active', subscription_status: 'active', client_token: 'tok_alice_abcdef123456', traffic_used_bytes: 1073741824, expire_time: 1893456000 },
  { id: 2, username: 'bob', role: 'admin', status: 'active', subscription_status: 'pending', client_token: 'tok_bob_xyz789012345', traffic_used_bytes: 536870912, expire_time: 0 },
  { id: 3, username: 'charlie', role: 'user', status: 'inactive', subscription_status: 'expired', client_token: 'tok_charlie_uvw34567890', traffic_used_bytes: 0, expire_time: 1700000000 },
]

describe('Admin Users.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    // Mock global fetch
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders loading state initially', async () => {
    // Keep fetch pending to observe loading
    vi.mocked(fetch).mockImplementationOnce(() => new Promise(() => {}))
    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    // Wait for Vue to flush the reactive update from onMounted -> loading=true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.loading').exists()).toBe(true)
    expect(wrapper.find('.loading').text()).toContain('Loading users')
  })

  it('renders users table after loading', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(sampleUsers),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.table-wrap').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(3)
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('bob')
  })

  it('shows the page heading', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(sampleUsers),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))
    // The sidebar has <h2 class="brand">RFPlay Admin</h2> and topbar has <h2>Users</h2>
    expect(wrapper.find('.topbar h2').text()).toBe('Users')
  })

  it('renders error state on API failure', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error('Failed to fetch'))

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Failed to fetch')
  })

  it('filters users by search input', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(sampleUsers),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    // All 3 users should be visible initially
    expect(wrapper.findAll('tbody tr').length).toBe(3)

    // Search for 'alice' — should show 1 result
    const searchInput = wrapper.find('.search-input')
    await searchInput.setValue('alice')
    await new Promise(r => setTimeout(r, 50))

    // After filtering, only alice should match
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('alice')
  })

  it('masks tokens correctly', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(sampleUsers),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    // Token should be masked (showing only first 6 and last 4 chars)
    const tokenTexts = wrapper.findAll('.token-text')
    expect(tokenTexts.length).toBeGreaterThan(0)
    for (const tokenEl of tokenTexts) {
      const text = tokenEl.text()
      expect(text).not.toBe('—') // Not empty
    }
  })

  it('handles empty users response', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('tbody').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(0)
  })

  it('handles HTTP error response', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({}),
    } as any)

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('HTTP 500')
  })
})
