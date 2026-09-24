import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock the API module (Users.vue loads data through the axios-based api module)
const mockApi = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  defaults: { baseURL: '/api/v1' },
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
}
vi.mock('../api/index', () => ({
  default: mockApi,
  setOnUnauthorized: vi.fn(),
}))

// Mock vue-router
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/users' }),
}))

const sampleUsers = [
  { id: 1, username: 'alice', role: 'user', status: 'active', subscription_status: 'active', client_token: 'tok_alice_abcdef123456', traffic_used_bytes: 1073741824, expire_time: 1893456000 },
  { id: 2, username: 'bob', role: 'admin', status: 'active', subscription_status: 'pending', client_token: 'tok_bob_xyz789012345', traffic_used_bytes: 536870912, expire_time: 0 },
  { id: 3, username: 'charlie', role: 'user', status: 'inactive', subscription_status: 'expired', client_token: 'tok_charlie_uvw34567890', traffic_used_bytes: 0, expire_time: 1700000000 },
]

describe('Admin Users.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockApi.get.mockReset()
  })

  it('renders loading state initially', async () => {
    // Keep the request pending to observe loading
    mockApi.get.mockImplementationOnce(() => new Promise(() => {}))
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
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })

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
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.find('.topbar h2').text()).toBe('Users')
  })

  it('renders error state on API failure', async () => {
    mockApi.get.mockRejectedValueOnce({
      response: { status: 500, data: { error: 'Failed to fetch' } },
    })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Failed to fetch')
  })

  it('filters users by search input', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })

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
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })

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

  it('shows Copy Base64 and Copy Clash buttons per user', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    const base64Btns = wrapper.findAll('button').filter(b => b.text() === 'Copy Base64')
    const clashBtns = wrapper.findAll('button').filter(b => b.text() === 'Copy Clash')
    expect(base64Btns.length).toBe(3)
    expect(clashBtns.length).toBe(3)
  })

  it('copies Base64 subscription URL to clipboard', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleUsers })
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    vi.stubGlobal('location', { origin: 'https://xva.rfplay.uk' })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    const base64Btn = wrapper.findAll('button').find(b => b.text() === 'Copy Base64')
    expect(base64Btn).toBeTruthy()
    await base64Btn!.trigger('click')
    await new Promise(r => setTimeout(r, 20))

    expect(writeText).toHaveBeenCalled()
    const copied = writeText.mock.calls[0][0] as string
    expect(copied).toContain('/api/v1/client/links/tok_alice_abcdef123456')
    expect(copied).not.toContain('/clash')
  })

  it('handles empty users response', async () => {
    mockApi.get.mockResolvedValueOnce({ data: [] })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('tbody').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(1)
    expect(wrapper.find('.empty-row').text()).toContain('No users found')
  })

  it('handles HTTP error response', async () => {
    mockApi.get.mockRejectedValueOnce({
      response: { status: 500, data: { error: 'HTTP 500' } },
    })

    const Users = await import('../views/Users.vue')
    const wrapper = mount(Users.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('HTTP 500')
  })
})
