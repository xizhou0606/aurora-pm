import { useLayoutEffect, useRef } from 'react'

/** Position the desktop comment rail in the document's scrolling coordinate space. */
export function useCommentAnchorLayout(enabled: boolean, documentId: number) {
  const ref = useRef<HTMLElement>(null)
  useLayoutEffect(() => {
    const panel = ref.current
    const editor = panel?.closest('[data-slot="document-editor"]')
    const body = editor?.querySelector<HTMLElement>('[data-slot="document-scroll"]')
    if (!enabled || !panel || !editor || !body) return
    let frame = 0
    const layout = () => {
      frame = 0
      const rail = panel.querySelector<HTMLElement>('[data-slot="document-comments-scroll"]')
      if (!rail) return
      const anchors = new Map<string, number>()
      for (const marker of body.querySelectorAll<HTMLElement>('[data-rich-text-comment-anchor-ids]')) {
        const ids: unknown = JSON.parse(marker.dataset.richTextCommentAnchorIds || '[]')
        if (!Array.isArray(ids)) continue
        for (const id of ids) {
          if (!anchors.has(String(id))) anchors.set(String(id), marker.getBoundingClientRect().top - rail.getBoundingClientRect().top + body.scrollTop)
        }
      }
      const cards = Array.from(rail.querySelectorAll<HTMLElement>('[data-thread-id]'))
      cards.sort((a, b) => (anchors.get(a.dataset.threadId!) ?? Infinity) - (anchors.get(b.dataset.threadId!) ?? Infinity))
      let bottom = 12
      for (const card of cards) {
        const top = Math.max(bottom, anchors.get(card.dataset.threadId!) ?? bottom)
        card.style.position = 'absolute'
        card.style.left = '12px'
        card.style.right = '12px'
        card.style.width = 'auto'
        card.style.top = `${top - body.scrollTop}px`
        bottom = top + card.getBoundingClientRect().height + 12
      }
    }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(layout) }
    const resize = new ResizeObserver(schedule)
    resize.observe(body)
    resize.observe(panel)
    const observeCards = () => {
      panel.querySelectorAll('[data-thread-id]').forEach(card => resize.observe(card))
      schedule()
    }
    // Observe content/anchor changes, not our own layout style writes.
    const mutation = new MutationObserver(observeCards)
    mutation.observe(editor, { childList: true, subtree: true, characterData: true })
    body.addEventListener('scroll', schedule, { passive: true })
    window.addEventListener('resize', schedule)
    observeCards()
    return () => {
      cancelAnimationFrame(frame)
      resize.disconnect()
      mutation.disconnect()
      body.removeEventListener('scroll', schedule)
      window.removeEventListener('resize', schedule)
      panel.querySelectorAll<HTMLElement>('[data-thread-id]').forEach(card => {
        for (const key of ['position', 'left', 'right', 'width', 'top']) card.style.removeProperty(key)
      })
    }
  }, [enabled, documentId])
  return ref
}
