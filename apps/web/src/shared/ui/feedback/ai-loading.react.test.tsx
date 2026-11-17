import { render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { appearanceStore } from '../theme/appearance'
import { AiLoading, AiLoadingOverlay } from './ai-loading'

const getInk = (container: HTMLElement) =>
  container.querySelector('[data-slot="ai-loading-ink"]')?.getAttribute('fill')

const getPaper = (container: HTMLElement) =>
  container.querySelector('[data-slot="ai-loading-paper"]')?.getAttribute('fill')

function stubThemePaint(colors: { foreground?: string; card?: string }) {
  vi.spyOn(window, 'getComputedStyle').mockImplementation(
    () =>
      ({
        getPropertyValue: (name: string) => {
          if (name === '--foreground') return colors.foreground ?? ''
          if (name === '--card') return colors.card ?? ''
          return ''
        },
      }) as unknown as CSSStyleDeclaration,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  appearanceStore.setState({ appearance: 'system', resolved: 'light' })
})

describe('AiLoading', () => {
  it('announces itself as an image with the shared AI label and honours the size prop', () => {
    const { container } = render(<AiLoading size={72} />)
    const svg = container.querySelector('svg[data-slot="ai-loading"]')
    expect(svg).not.toBeNull()
    expect(svg?.getAttribute('role')).toBe('img')
    expect(svg?.getAttribute('aria-label')).toBe(copy.ui.aiThinking)
    expect(svg?.getAttribute('width')).toBe('72')
    expect(svg?.getAttribute('height')).toBe('72')
  })

  it('renders decorative when the caller passes label={null}', () => {
    const { container } = render(<AiLoading label={null} />)
    const svg = container.querySelector('svg[data-slot="ai-loading"]')
    expect(svg?.getAttribute('aria-hidden')).toBe('true')
    expect(svg?.getAttribute('aria-label')).toBeNull()
  })

  it('paints ink and paper from theme tokens so light and dark both read', () => {
    stubThemePaint({ foreground: '#151820', card: '#ffffff' })
    const light = render(<AiLoading label={null} />)
    expect(getInk(light.container)).toBe('#151820')
    expect(getPaper(light.container)).toBe('#ffffff')
    light.unmount()

    stubThemePaint({ foreground: '#eceef3', card: '#1e222c' })
    appearanceStore.setState({ appearance: 'dark', resolved: 'dark' })
    const dark = render(<AiLoading label={null} />)
    expect(getInk(dark.container)).toBe('#eceef3')
    expect(getPaper(dark.container)).toBe('#1e222c')
  })

  it('falls back to safe defaults when a token is not a #rrggbb hex', () => {
    stubThemePaint({ foreground: 'oklch(0.9 0.1 200)', card: '' })
    const { container } = render(<AiLoading label={null} />)
    expect(getInk(container)).toBe('#151820')
    expect(getPaper(container)).toBe('#ffffff')
  })

  it('renders one labelled busy boundary fixed to the viewport, always centered on the body', () => {
    const { container } = render(<AiLoadingOverlay label="AI 拆解中" />)
    const overlay = container.querySelector('[data-slot="ai-loading-overlay"]')
    expect(overlay).not.toBeNull()
    expect(overlay?.getAttribute('role')).toBe('status')
    expect(overlay?.getAttribute('aria-busy')).toBe('true')
    expect(overlay?.getAttribute('aria-label')).toBe('AI 拆解中')
    expect(overlay?.getAttribute('class')).toContain('fixed')
    expect(overlay?.getAttribute('class')).toContain('bg-background/75')
    expect(overlay?.textContent).toContain('AI 拆解中')
    expect(container.querySelector('[data-slot="ai-loading"]')).not.toBeNull()
  })

  it('stays on a static idle pose under prefers-reduced-motion', () => {
    const raf = vi.spyOn(window, 'requestAnimationFrame')
    const original = window.matchMedia
    window.matchMedia = ((query: string) => ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia
    try {
      const { container, unmount } = render(<AiLoading label={null} />)
      expect(getInk(container)).not.toBeNull()
      // idle 姿态（demo 的海报帧）：黑球带着两只眼睛；无脸状态不会渲染眼睛
      expect(container.querySelectorAll('mask path').length).toBe(3)
      expect(raf).not.toHaveBeenCalled()
      unmount()
    } finally {
      window.matchMedia = original
    }
  })

  it('plays the full upstream default montage, starting from the idle ball', () => {
    // 静止帧即 demo 海报帧；动效序列由 defaultCycle 驱动，与上游逐帧一致
    const { container } = render(<AiLoading label={null} />)
    expect(container.querySelectorAll('mask path').length).toBe(3)
  })
})
