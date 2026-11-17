import { cleanup } from '@testing-library/react'
import { gsap } from 'gsap'
import axios from 'axios'
import { afterEach, vi } from 'bun:test'

import { PM_ACTION_CODES } from '@/generated/access-catalog'
import { appearanceStore } from '@/shared/ui/theme/appearance'

function createMemoryStorage(): Storage {
  const values = new Map<string, string>()

  return {
    get length() {
      return values.size
    },
    clear: () => values.clear(),
    getItem: (key) => values.get(String(key)) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => {
      values.delete(String(key))
    },
    setItem: (key, value) => {
      values.set(String(key), String(value))
    },
  }
}

const memoryLocalStorage = createMemoryStorage()

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: memoryLocalStorage,
  writable: true,
})

Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: memoryLocalStorage,
  writable: true,
})

const { coreApi } = await import('@/shared/api')
const defaultCoreAdapter = axios.getAdapter(coreApi.defaults.adapter)

// Legacy feature tests run as an access administrator unless they explicitly
// seed or mock the action-permission query. This keeps the production hook
// fail-closed while making permission context a stable part of the test base.
coreApi.defaults.adapter = async (config) => {
  if (String(config.url ?? '').endsWith('/access/me/action-permissions')) {
    return {
      data: { data: { action_codes: PM_ACTION_CODES } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  return defaultCoreAdapter(config)
}

Object.defineProperty(window, 'scrollTo', {
  configurable: true,
  value: vi.fn(),
  writable: true,
})

Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }),
  writable: true,
})

class TestResizeObserver implements ResizeObserver {
  disconnect() {}
  observe() {}
  unobserve() {}
}

Object.defineProperty(globalThis, 'ResizeObserver', {
  configurable: true,
  value: TestResizeObserver,
  writable: true,
})

afterEach(() => {
  cleanup()
  // GSAP 官方清理：阻止跨用例的补间/时间轴泄漏（BI Screen 动画）。
  gsap.globalTimeline.clear()
  memoryLocalStorage.clear()
  appearanceStore.getState().reset()
})

afterEach(() => {
})
