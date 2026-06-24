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
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/nodes' }),
}))

const sampleNodes = [
  { id: 1, name: 'Node-US-1', type: 'xray', address: 'us1.example.com', port: 443, protocol: 'vmess', status: 'active', traffic_up: 1048576, traffic_down: 2097152, user_id: 1 },
  { id: 2, name: 'Node-EU-2', type: 'v2ray', address: 'eu2.example.com', port: 8443, protocol: 'trojan', status: 'inactive', traffic_up: 524288, traffic_down: 1048576, user_id: 2 },
  { id: 3, name: 'Node-AS-1', type: 'xray', address: 'as1.example.com', port: 443, protocol: 'shadowsocks', status: 'active', traffic_up: 2097152, traffic_down: 4194304, user_id: 3 },
]

describe('Admin Nodes.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockApi.get.mockReset()
  })

  it('renders loading state initially', async () => {
    mockApi.get.mockImplementationOnce(() => new Promise(() => {}))
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    // Wait for Vue to flush the reactive update from onMounted -> loading=true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.loading').exists()).toBe(true)
    expect(wrapper.find('.loading').text()).toContain('Loading nodes')
  })

  it('renders nodes table after loading', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleNodes })
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.table-wrap').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(3)
    expect(wrapper.text()).toContain('Node-US-1')
    expect(wrapper.text()).toContain('Node-EU-2')
  })

  it('shows the page heading', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleNodes })
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))
    // The sidebar has <h2 class="brand">RFPlay Admin</h2> and topbar has <h2>Nodes</h2>
    expect(wrapper.find('.topbar h2').text()).toBe('Nodes')
  })

  it('renders error state on API failure', async () => {
    mockApi.get.mockRejectedValueOnce({
      response: { data: { error: 'Server error' } },
    })
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('.error-msg').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Server error')
  })

  it('handles empty nodes list gracefully', async () => {
    mockApi.get.mockResolvedValueOnce({ data: [] })
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.find('tbody').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').length).toBe(0)
  })

  it('displays traffic data formatted correctly', async () => {
    mockApi.get.mockResolvedValueOnce({ data: sampleNodes })
    const Nodes = await import('../views/Nodes.vue')
    const wrapper = mount(Nodes.default, {
      global: { plugins: [createPinia()] },
    })
    await new Promise(r => setTimeout(r, 50))

    expect(wrapper.text()).toContain('MB')
  })
})
