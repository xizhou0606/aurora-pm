import { flushSync } from 'react-dom'
import { useStore } from 'zustand'
import { createStore } from 'zustand/vanilla'

import type { AppearanceMode } from './theme-appearance-menu'

export const PM_APPEARANCE_STORAGE_KEY = 'pm-appearance'
export const PM_APPEARANCE_LEGACY_STORAGE_KEY = 'pm-landing-appearance'
export const PM_UI_MODE_DARK = 'dark-tech'

export type ResolvedAppearance = 'light' | 'dark'

type AppearanceState = {
  appearance: AppearanceMode
  resolved: ResolvedAppearance
  setAppearance: (next: AppearanceMode) => void
  syncSystem: () => void
  reset: () => void
}

export function prefersDarkScheme(): boolean {
  return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false
}

export function readStoredAppearance(): AppearanceMode {
  const stored =
    window.localStorage.getItem(PM_APPEARANCE_STORAGE_KEY) ||
    window.localStorage.getItem(PM_APPEARANCE_LEGACY_STORAGE_KEY)
  return stored === 'light' || stored === 'dark' || stored === 'system' ? stored : 'system'
}

export function resolveAppearance(
  mode: AppearanceMode,
  systemDark = prefersDarkScheme(),
): ResolvedAppearance {
  if (mode === 'system') return systemDark ? 'dark' : 'light'
  return mode
}

export function applyResolvedAppearance(resolved: ResolvedAppearance): void {
  const root = document.documentElement
  if (resolved === 'dark') {
    root.dataset.uiMode = PM_UI_MODE_DARK
    root.style.colorScheme = 'dark'
    return
  }
  delete root.dataset.uiMode
  root.style.colorScheme = 'light'
}

/** Attach an alpha channel to a `#rrggbb` color (canvas-safe rgba string). */
export function withAlpha(hex: string, alpha: number) {
  const value = hex.replace('#', '')
  if (value.length !== 6) return hex
  const r = Number.parseInt(value.slice(0, 2), 16)
  const g = Number.parseInt(value.slice(2, 4), 16)
  const b = Number.parseInt(value.slice(4, 6), 16)
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export function readThemePaint() {
  const styles = getComputedStyle(document.documentElement)
  const token = (name: string, fallback: string) => styles.getPropertyValue(name).trim() || fallback
  return {
    background: token('--background', '#ffffff'),
    card: token('--card', '#ffffff'),
    foreground: token('--foreground', '#151820'),
    muted: token('--muted-foreground', '#5d6675'),
    border: token('--border', 'rgba(20, 28, 42, 0.1)'),
    primary: token('--primary', '#2563eb'),
    // Chart semantic tokens (see design-tokens.css). Values are literal
    // colors so ECharts can consume them directly on canvas.
    chartGrid: token('--design-chart-grid', 'rgba(20, 28, 42, 0.07)'),
    chartSeries: [
      token('--design-chart-blue', '#4373d1'),
      token('--design-chart-green', '#1e9e63'),
      token('--design-chart-amber', '#bd8a1a'),
      token('--design-chart-violet', '#7466d1'),
      token('--design-chart-red', '#cd4f45'),
      token('--design-chart-slate', '#8a93a6'),
    ],
    chartPositive: token('--design-chart-positive', '#1e9e63'),
    chartNegative: token('--design-chart-negative', '#cd4f45'),
  }
}

function persistAppearance(mode: AppearanceMode): void {
  window.localStorage.setItem(PM_APPEARANCE_STORAGE_KEY, mode)
  window.localStorage.removeItem(PM_APPEARANCE_LEGACY_STORAGE_KEY)
}

function snapshot(mode: AppearanceMode): Pick<AppearanceState, 'appearance' | 'resolved'> {
  const resolved = resolveAppearance(mode)
  applyResolvedAppearance(resolved)
  return { appearance: mode, resolved }
}

const THEME_FADE_TRANSITION_TYPE = 'theme-fade'

type ViewTransitionFactory = (options: {
  types: string[]
  update: () => void
}) => unknown

// The theme attributes have no CSS transition, so a bare flip repaints the
// whole viewport in a single frame. Wrap the flip in a same-document view
// transition (type `theme-fade`, CSS in shared/styles.css); browsers without
// the API keep the instant swap. `latestThemeIntent` drops stale deferred
// callbacks when appearance changes again within the frame before a pending
// transition runs, so the last intent always wins even if it skipped the fade.
let latestThemeIntent = 0
let latestThemeTransition = 0

function startThemeFade(update: () => void, intent: number): void {
  const start = (
    document as Document & {
      startViewTransition?: ViewTransitionFactory
    }
  ).startViewTransition
  if (typeof start !== 'function') {
    update()
    return
  }
  // The content area keeps its own view-transition-name for route switches;
  // a theme-wide animation must move ONE snapshot, so the name is dropped
  // through the `data-theme-switch` attribute (CSS in shared/styles.css)
  // before the old state is captured and restored once the transition
  // settles. Without this, root and content layers animate their own
  // boundaries and the page tears into mismatched patches.
  const transitionId = ++latestThemeTransition
  document.documentElement.dataset.themeSwitch = ''
  const transition = start.call(document, {
    types: [THEME_FADE_TRANSITION_TYPE],
    update: () => {
      if (intent !== latestThemeIntent) return
      // React schedules store subscribers asynchronously; without a sync
      // flush the new-theme snapshot captures components that are still
      // painted in the old theme (stale cards, table bands) and they pop
      // only after the transition ends.
      flushSync(() => update())
    },
  }) as { finished?: Promise<void> } | undefined
  void Promise.allSettled([transition?.finished]).then(() => {
    if (transitionId === latestThemeTransition) {
      delete document.documentElement.dataset.themeSwitch
    }
  })
}

function commitAppearanceChange(
  current: ResolvedAppearance,
  next: AppearanceMode,
  apply: () => void,
): void {
  const intent = ++latestThemeIntent
  const reducedMotion =
    window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches ?? false
  if (resolveAppearance(next) === current || reducedMotion) {
    apply()
    return
  }
  startThemeFade(apply, intent)
}

export const appearanceStore = createStore<AppearanceState>((set, get) => ({
  appearance: 'system',
  resolved: 'light',
  setAppearance: (next) => {
    persistAppearance(next)
    commitAppearanceChange(get().resolved, next, () => set(snapshot(next)))
  },
  syncSystem: () => {
    const mode = get().appearance
    commitAppearanceChange(get().resolved, mode, () => set(snapshot(mode)))
  },
  reset: () => {
    window.localStorage.removeItem(PM_APPEARANCE_STORAGE_KEY)
    window.localStorage.removeItem(PM_APPEARANCE_LEGACY_STORAGE_KEY)
    commitAppearanceChange(get().resolved, 'system', () => set(snapshot('system')))
  },
}))

export function hydrateAppearance(): void {
  const appearance = readStoredAppearance()
  appearanceStore.setState(snapshot(appearance))
  persistAppearance(appearance)
}

export function bindAppearanceMedia(): () => void {
  const media = window.matchMedia?.('(prefers-color-scheme: dark)')
  if (!media) return () => undefined
  const sync = () => appearanceStore.getState().syncSystem()
  media.addEventListener('change', sync)
  return () => media.removeEventListener('change', sync)
}

export function useAppearance() {
  return useStore(appearanceStore)
}
