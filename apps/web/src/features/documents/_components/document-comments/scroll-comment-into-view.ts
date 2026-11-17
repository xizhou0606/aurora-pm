/** Keep comment navigation inside its rail (or the body for full-document comments). */
export function scrollCommentIntoView(
  element: HTMLElement | null,
  block: 'start' | 'nearest' = 'start',
): void {
  if (element?.closest('[data-anchored-comments]')) return
  const container = element?.closest<HTMLElement>(
    '[data-slot="document-comments-scroll"], [data-slot="document-scroll"]',
  )
  if (!element || !container) return

  const target = element.getBoundingClientRect()
  const viewportTop = container.getBoundingClientRect().top + container.clientTop
  const start = target.top - viewportTop
  const end = target.bottom - viewportTop - container.clientHeight
  const offset = block === 'start' ? start
    : start < 0 && end > 0 ? 0
      : start < 0 ? start : end > 0 ? Math.min(start, end) : 0
  if (offset === 0) return
  container.scrollTop = Math.max(0, container.scrollTop + offset)
}
