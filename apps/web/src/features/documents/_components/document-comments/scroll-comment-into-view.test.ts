import { describe, expect, it, vi } from 'bun:test'

import { scrollCommentIntoView } from './scroll-comment-into-view'

function fixture(slot = 'document-comments-scroll') {
  const shell = document.createElement('article')
  const container = document.createElement('div')
  const target = document.createElement('div')
  container.dataset.slot = slot
  shell.append(container)
  container.append(target)
  shell.scrollTop = 0
  container.scrollTop = 100
  Object.defineProperty(container, 'clientHeight', { value: 400 })
  vi.spyOn(container, 'getBoundingClientRect').mockReturnValue({ top: 50 } as DOMRect)
  const bounds = vi.spyOn(target, 'getBoundingClientRect')
    .mockReturnValue({ top: 550, bottom: 650 } as DOMRect)
  const unboundedScroll = vi.fn()
  target.scrollIntoView = unboundedScroll
  return { shell, container, target, bounds, unboundedScroll }
}

describe('comment navigation scroll boundaries', () => {
  it.each(['document-comments-scroll', 'document-scroll'])(
    'aligns only the owning %s without moving the hidden shell', (slot) => {
      const { shell, container, target, unboundedScroll } = fixture(slot)
      scrollCommentIntoView(target)
      expect(container.scrollTop).toBe(600)
      expect(shell.scrollTop).toBe(0)
      expect(unboundedScroll).not.toHaveBeenCalled()
    },
  )

  it('reveals the composer with the minimum scroll and leaves visible content in place', () => {
    const { container, target, bounds } = fixture()
    scrollCommentIntoView(target, 'nearest')
    expect(container.scrollTop).toBe(300)
    bounds.mockReturnValue({ top: 100, bottom: 200 } as DOMRect)
    scrollCommentIntoView(target, 'nearest')
    expect(container.scrollTop).toBe(300)
    bounds.mockReturnValue({ top: 10, bottom: 80 } as DOMRect)
    scrollCommentIntoView(target, 'nearest')
    expect(container.scrollTop).toBe(260)
  })

  it('does not fall back to scrolling ancestors without a designated owner', () => {
    const { target, unboundedScroll } = fixture('unrelated')
    scrollCommentIntoView(target)
    scrollCommentIntoView(null)
    expect(unboundedScroll).not.toHaveBeenCalled()
  })
})
