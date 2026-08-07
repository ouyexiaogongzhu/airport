import { vi } from 'vitest'

// Mock window.location
Object.defineProperty(globalThis, 'location', {
  value: { href: '' },
  writable: true,
})

// Mock matchMedia
Object.defineProperty(globalThis, 'matchMedia', {
  value: vi.fn().mockImplementation(() => ({
    matches: false,
    addListener: vi.fn(),
    removeListener: vi.fn(),
  })),
})

// Mock ResizeObserver
class ResizeObserverMock {
  observe = vi.fn()
  unobserve = vi.fn()
  disconnect = vi.fn()
}
Object.defineProperty(globalThis, 'ResizeObserver', { value: ResizeObserverMock })

// Mock console.error to keep test output clean
vi.spyOn(console, 'error').mockImplementation(() => {})

// Mock Pinia
import { createPinia } from 'pinia'
export { createPinia }
