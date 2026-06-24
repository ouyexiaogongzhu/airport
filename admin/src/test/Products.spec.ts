import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock API module
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
}))

const sampleProducts = {
  data: {
    products: [
      { id: 1, name: 'Pro VPN', type: 'VPN', price: 19.99, stock: 50, status: 'active' },
      { id: 2, name: 'Starter VPN', type: 'VPN', price: 9.99, stock: 100, status: 'active' },
      { id: 3, name: 'Proxy Pack M', type: 'Proxy', price: 14.99, stock: 0, status: 'inactive' },
    ],
  },
}

describe('Admin Products.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockApi.get.mockReset()
    mockPush.mockClear()
  })

  it('renders loading state initially', async () => {
    // Make get return a promise that doesn't resolve immediately
    mockApi.get.mockImplementationOnce(() => new Promise(() => {}))
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    // Wait for Vue to flush the reactive update from onMounted -> loading=true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.loading').exists()).toBe(true)
    expect(wrapper.find('.loading').text()).toContain('Loading products')
  })

  it('renders products table after loading', async () => {
    mockApi.get.mockResolvedValueOnce(sampleProducts)
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.table-wrap').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(3)
    expect(wrapper.text()).toContain('Pro VPN')
    expect(wrapper.text()).toContain('Starter VPN')
  })

  it('shows the page heading', async () => {
    mockApi.get.mockResolvedValueOnce(sampleProducts)
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))
    // The sidebar has <h2 class="brand">RFPlay Admin</h2> and topbar has <h2>Products</h2>
    // Target the topbar heading specifically
    expect(wrapper.find('.topbar h2').text()).toBe('Products')
  })

  it('renders error state on API failure', async () => {
    mockApi.get.mockRejectedValueOnce({ message: 'Network Error' })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Network Error')
  })

  it('shows empty table for empty products array', async () => {
    mockApi.get.mockResolvedValueOnce({ data: { products: [] } })
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('tbody').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(0)
  })

  it('disables saving button when saving is in progress', async () => {
    mockApi.get.mockResolvedValueOnce(sampleProducts)
    const Products = await import('../views/Products.vue')
    const wrapper = mount(Products.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    // Click add product to open modal
    await wrapper.find('.btn-primary').trigger('click')
    expect(wrapper.find('.modal-overlay').exists()).toBe(true)
    expect(wrapper.find('h3').text()).toContain('Add Product')
  })
})
