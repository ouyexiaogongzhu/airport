import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock the API module
vi.mock('../api/index', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  },
}))

// Mock vue-router
const mockPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ path: '/' }),
  RouterLink: { template: '<a><slot /></a>' },
}))

describe('Portal Login.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockPush.mockClear()
  })

  it('renders the login form with heading', async () => {
    const Login = await import('../views/Login.vue')
    const wrapper = mount(Login.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })

    expect(wrapper.find('h1').text()).toContain('RFPlay')
    expect(wrapper.find('input[type="text"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.find('button[type="submit"]').text()).toContain('Sign In')
  })

  it('shows a link to register page', async () => {
    const Login = await import('../views/Login.vue')
    const wrapper = mount(Login.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })

    expect(wrapper.text()).toContain("Don't have an account?")
    expect(wrapper.text()).toContain('Register')
  })

  it('disables submit button while loading', async () => {
    const Login = await import('../views/Login.vue')
    const wrapper = mount(Login.default, {
      global: {
        plugins: [createPinia()],
        stubs: { 'router-link': { template: '<a><slot /></a>' } },
      },
    })

    await wrapper.find('input[type="text"]').setValue('user')
    await wrapper.find('input[type="password"]').setValue('pass')

    // Submit the form — loading is set synchronously before async call
    await wrapper.find('form').trigger('submit.prevent')
    // Wait for next tick so Vue re-renders with loading=true
    await wrapper.vm.$nextTick()

    const btn = wrapper.find('button[type="submit"]')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('has required username and password fields', () => {
    return import('../views/Login.vue').then(({ default: Login }) => {
      const wrapper = mount(Login, {
        global: {
          plugins: [createPinia()],
          stubs: { 'router-link': { template: '<a><slot /></a>' } },
        },
      })
      const usernameInput = wrapper.find('input[type="text"]')
      const passwordInput = wrapper.find('input[type="password"]')
      expect(usernameInput.attributes('required')).toBeDefined()
      expect(passwordInput.attributes('required')).toBeDefined()
    })
  })

  it('initial render shows Sign In button text', () => {
    return import('../views/Login.vue').then(({ default: Login }) => {
      const wrapper = mount(Login, {
        global: {
          plugins: [createPinia()],
          stubs: { 'router-link': { template: '<a><slot /></a>' } },
        },
      })
      expect(wrapper.find('button').text()).toBe('Sign In')
    })
  })
})
