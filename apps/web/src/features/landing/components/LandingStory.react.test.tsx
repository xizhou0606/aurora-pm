import { act, render, screen, within } from '@testing-library/react'
import { expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { LandingStory } from './LandingStory'

it('keeps the current chapter semantics and visual state together when the visible scene changes', () => {
  const OriginalObserver = globalThis.IntersectionObserver
  let sceneObserver: IntersectionObserver | undefined
  let onSceneChange: IntersectionObserverCallback | undefined
  globalThis.IntersectionObserver = class extends OriginalObserver {
    constructor(callback: IntersectionObserverCallback, options?: IntersectionObserverInit) {
      super(callback, options)
      if (Array.isArray(options?.threshold)) {
        sceneObserver = this
        onSceneChange = callback
      }
    }
  }

  const view = render(<LandingStory workspaceHref="/workspace/projects" />)
  try {
    const navigation = screen.getByRole('navigation', { name: copy.landing.chapterNavigation })
    const stops = within(navigation).getAllByRole('button')
    expect(stops[0].getAttribute('aria-current')).toBe('step')
    expect(stops[0].dataset.active).toBe('true')

    const scene = view.container.querySelector('[data-mobile-scene="2"]')!
    expect(onSceneChange).toBeDefined()
    act(() => onSceneChange!([
      { target: scene, isIntersecting: true, intersectionRatio: 0.8 } as IntersectionObserverEntry,
    ], sceneObserver!))

    expect(stops[2].getAttribute('aria-current')).toBe('step')
    expect(stops[2].dataset.active).toBe('true')
    expect(navigation.querySelectorAll('[aria-current="step"]')).toHaveLength(1)
    expect(navigation.querySelectorAll('[data-active="true"]')).toHaveLength(1)
    expect(stops[0].hasAttribute('aria-current')).toBe(false)
    expect(stops[0].dataset.active).toBe('false')
  } finally {
    view.unmount()
    globalThis.IntersectionObserver = OriginalObserver
  }
})
