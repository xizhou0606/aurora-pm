import { afterEach, describe, expect, it } from 'bun:test'

import {
  PM_APPEARANCE_LEGACY_STORAGE_KEY,
  PM_APPEARANCE_STORAGE_KEY,
  PM_UI_MODE_DARK,
  appearanceStore,
  hydrateAppearance,
  readStoredAppearance,
  readThemePaint,
  resolveAppearance,
} from './appearance'

type RecordedViewTransition = {
  types?: string[]
  update: () => void
}

function installStartViewTransition() {
  const calls: RecordedViewTransition[] = []
  const original = (document as { startViewTransition?: unknown }).startViewTransition
  Object.defineProperty(document, 'startViewTransition', {
    configurable: true,
    value: (options: RecordedViewTransition) => {
      calls.push(options)
      return { ready: Promise.resolve(), finished: Promise.resolve() }
    },
    writable: true,
  })
  return {
    calls,
    restore: () => {
      if (original === undefined) {
        delete (document as { startViewTransition?: unknown }).startViewTransition
        return
      }
      Object.defineProperty(document, 'startViewTransition', {
        configurable: true,
        value: original,
        writable: true,
      })
    },
  }
}

function installReducedMotion() {
  const original = window.matchMedia
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => ({
      matches: query.includes('prefers-reduced-motion'),
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
  return () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: original,
      writable: true,
    })
  }
}

describe('appearance runtime', () => {
  afterEach(() => {
    appearanceStore.getState().reset()
  })

  it('resolves system appearance from the color-scheme preference', () => {
    expect(resolveAppearance('light', true)).toBe('light')
    expect(resolveAppearance('dark', false)).toBe('dark')
    expect(resolveAppearance('system', true)).toBe('dark')
    expect(resolveAppearance('system', false)).toBe('light')
  })

  it('migrates the landing appearance key onto the shared store', () => {
    window.localStorage.setItem(PM_APPEARANCE_LEGACY_STORAGE_KEY, 'dark')
    expect(readStoredAppearance()).toBe('dark')

    hydrateAppearance()
    appearanceStore.getState().setAppearance('light')

    expect(document.documentElement.dataset.uiMode).toBeUndefined()
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(window.localStorage.getItem(PM_APPEARANCE_STORAGE_KEY)).toBe('light')
    expect(window.localStorage.getItem(PM_APPEARANCE_LEGACY_STORAGE_KEY)).toBeNull()
  })

  it('applies the PM dark UI mode to the document', () => {
    appearanceStore.getState().setAppearance('dark')

    expect(document.documentElement.dataset.uiMode).toBe(PM_UI_MODE_DARK)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('cross-fades the swap through the view transition API when supported', () => {
    appearanceStore.getState().setAppearance('light')
    const transition = installStartViewTransition()
    try {
      appearanceStore.getState().setAppearance('dark')

      expect(transition.calls).toHaveLength(1)
      expect(transition.calls[0]?.types).toContain('theme-fade')
      // The DOM flip happens inside the update callback so the old snapshot is
      // captured with the previous paint.
      expect(document.documentElement.dataset.uiMode).toBeUndefined()
      expect(appearanceStore.getState().resolved).toBe('light')

      transition.calls[0]?.update()

      expect(document.documentElement.dataset.uiMode).toBe(PM_UI_MODE_DARK)
      expect(document.documentElement.style.colorScheme).toBe('dark')
      expect(appearanceStore.getState().resolved).toBe('dark')
    } finally {
      transition.restore()
    }
  })

  it('skips the view transition when the resolved theme does not change', () => {
    const transition = installStartViewTransition()
    try {
      appearanceStore.getState().setAppearance('system')

      expect(transition.calls).toHaveLength(0)
      expect(document.documentElement.dataset.uiMode).toBeUndefined()
      expect(appearanceStore.getState().resolved).toBe('light')
    } finally {
      transition.restore()
    }
  })

  it('swaps instantly when the user prefers reduced motion', () => {
    const restoreMatchMedia = installReducedMotion()
    const transition = installStartViewTransition()
    try {
      appearanceStore.getState().setAppearance('dark')

      expect(transition.calls).toHaveLength(0)
      expect(document.documentElement.dataset.uiMode).toBe(PM_UI_MODE_DARK)
    } finally {
      transition.restore()
      restoreMatchMedia()
    }
  })

  it('drops a stale deferred swap when the appearance changes again immediately', () => {
    appearanceStore.getState().setAppearance('light')
    const transition = installStartViewTransition()
    try {
      appearanceStore.getState().setAppearance('dark')
      expect(transition.calls).toHaveLength(1)

      // Resolved is still 'light' until the pending callback runs, so this
      // swap skips the fade and supersedes the stale dark callback.
      appearanceStore.getState().setAppearance('light')
      expect(transition.calls).toHaveLength(1)

      transition.calls[0]?.update()

      expect(document.documentElement.dataset.uiMode).toBeUndefined()
      expect(appearanceStore.getState().resolved).toBe('light')
    } finally {
      transition.restore()
    }
  })

  it('exposes the current semantic primary paint for canvas consumers', () => {
    document.documentElement.style.setProperty('--primary', '#3794ff')

    expect(readThemePaint().primary).toBe('#3794ff')

    document.documentElement.style.removeProperty('--primary')
  })
})
