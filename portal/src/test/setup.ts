import { vi } from 'vitest'

// Mock window.location
Object.defineProperty(globalThis, 'location', {
  value: { href: '' },
  writable: true,
})

// jsdom in this env exposes no localStorage — stub the Storage API surface
// used by the axios interceptors (auth Bearer fallback).
if (typeof globalThis.localStorage === 'undefined') {
  const store = new Map<string, string>()
  Object.defineProperty(globalThis, 'localStorage', {
    value: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, String(v)),
      removeItem: (k: string) => void store.delete(k),
      clear: () => void store.clear(),
    },
  })
}

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

// Mock HTMLCanvasElement.getContext for QR code rendering
HTMLCanvasElement.prototype.getContext = vi.fn() as any
