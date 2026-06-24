import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock the API module
const mockApi = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
}
vi.mock('../api/index', () => ({ default: mockApi }))

// Mock vue-router
const mockPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ path: '/products' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

const samplePlans = [
  { id: 'plan_1', name: 'Pro VPN', price: 1999, traffic_bytes: 107374182400, duration_days: 30, speed_limit_bps: 100000000, description: 'High-speed VPN' },
  { id: 'plan_2', name: 'Starter VPN', price: 999, traffic_bytes: 53687091200, duration_days: 30, speed_limit_bps: 50000000 },
  { id: 'plan_3', name: 'Unlimited', price: 4999, traffic_bytes: 0, duration_days: 365, speed_limit_bps: 0, description: 'No limits' },
]

describe('Portal Products.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockApi.get.mockReset()
    mockPush.mockClear()
  })

  it('renders loading state initially', async () => {
    mockApi.get.mockImplementationOnce(() => new Promise(() => {}))
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    expect(wrapper.find('.loading').exists()).toBe(true)
    expect(wrapper.find('.loading').text()).toContain('Loading plans')
  })

  it('renders plan cards after loading', async () => {
    mockApi.get.mockResolvedValueOnce({ data: samplePlans })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.plan-grid').exists()).toBe(true)
    const cards = wrapper.findAll('.plan-card')
    expect(cards.length).toBe(3)
    expect(wrapper.text()).toContain('Pro VPN')
    expect(wrapper.text()).toContain('Starter VPN')
  })

  it('shows the page heading', async () => {
    mockApi.get.mockResolvedValueOnce({ data: samplePlans })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.find('h2').text()).toContain('Plans')
  })

  it('renders error state on API failure', async () => {
    mockApi.get.mockRejectedValueOnce({
      response: { status: 500, data: { error: 'Internal error' } },
    })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Internal error')
  })

  it('shows empty state when no plans available', async () => {
    // Return empty array — first API call succeeds with empty data
    mockApi.get.mockResolvedValueOnce({ data: [] })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.empty').exists()).toBe(true)
    expect(wrapper.text()).toContain('No plans available')
  })

  it('displays plan price formatted correctly (cents to dollars)', async () => {
    mockApi.get.mockResolvedValueOnce({ data: samplePlans })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))

    // $19.99 for Pro VPN (1999 cents)
    expect(wrapper.text()).toContain('$19.99')
    expect(wrapper.text()).toContain('$9.99')
  })

  it('shows a Purchase button for each plan', async () => {
    mockApi.get.mockResolvedValueOnce({ data: samplePlans })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })
    await new Promise(r => setTimeout(r, 50))

    const buyButtons = wrapper.findAll('.buy-btn')
    expect(buyButtons.length).toBe(3)
    expect(buyButtons[0].text()).toBe('Purchase')
  })
})
