export type DragPreview = ReturnType<typeof createDragPreview>
let previewSequence = 0
type PreviewEntry = { element: HTMLElement; box: { top: number; bottom: number; left: number; width: number; height: number } }
type NestedPreview = { styleScope: HTMLElement; select: (entries: PreviewEntry[], x: number, y: number) => PreviewEntry | undefined }

// Visual-only adapter for native DnD. It never reorders DOM or owns a mutation.
// Editor integrations can leave displacement off so ProseMirror retains hit testing.
export function createDragPreview(source: HTMLElement, event: DragEvent, siblings: () => HTMLElement[], displace = false, nativePreview = false, horizontalBounds?: () => { left: number; right: number }, nested?: NestedPreview, options: { cacheGeometry?: boolean; automaticTracking?: boolean } = {}) {
  const doc = source.ownerDocument
  const view = doc.defaultView!
  // ProseMirror observes attributes inside its content DOM. Keep editor-only
  // preview styles in a stylesheet so a visual move cannot trigger reparsing.
  const sheetElement = nativePreview ? doc.createElement('style') : null
  const scope = sheetElement ? nested?.styleScope ?? source.parentElement?.parentElement : null
  const scopeAttribute = 'data-drag-preview-scope'
  const previousScope = scope?.getAttribute(scopeAttribute)
  const scopeId = String(++previewSequence)
  scope?.setAttribute(scopeAttribute, scopeId)
  if (sheetElement) doc.head.append(sheetElement)
  const styles = new Map<HTMLElement, CSSStyleDeclaration>()
  const visualStyle = (element: HTMLElement) => {
    if (!sheetElement) return element.style
    const existing = styles.get(element)
    if (existing) return existing
    const path: string[] = []
    let current: Element | null = element
    while (current && current !== (scope ?? doc.documentElement)) {
      const parent: Element | null = current.parentElement
      if (!parent) break
      path.unshift(`:nth-child(${Array.from(parent.children).indexOf(current) + 1})`)
      current = parent
    }
    const sheet = sheetElement.sheet!
    const anchor = scope ? `[${scopeAttribute}="${scopeId}"]` : 'html'
    const index = sheet.insertRule(`${anchor} > ${path.join(' > ')} {}`, sheet.cssRules.length)
    const style = (sheet.cssRules[index] as CSSStyleRule).style
    styles.set(element, style)
    return style
  }
  const bounds = source.getBoundingClientRect()
  const overlay = doc.createElement('div')
  overlay.dataset.slot = 'drag-preview'
  overlay.className = 'pointer-events-none fixed z-[100] overflow-hidden rounded-md border border-primary/40 bg-card text-foreground shadow-lg'
  overlay.setAttribute('aria-hidden', 'true')
  overlay.inert = true
  Object.assign(overlay.style, { width: `${bounds.width}px`, maxHeight: '240px', top: '0', left: '0' })
  // The editor already supplies the native drag image. Cloning a large nested
  // table here would build a second, permanently hidden subtree.
  if (!nativePreview) {
    const clone = source.cloneNode(true) as HTMLElement
    for (const node of [clone, ...clone.querySelectorAll<HTMLElement>('[id]')]) node.removeAttribute('id')
    clone.style.transform = 'none'; clone.style.opacity = '1'
    if (source.tagName === 'TR') {
      const table = doc.createElement('table'), body = doc.createElement('tbody')
      table.className = 'w-full table-fixed border-collapse'
      Array.from(source.children).forEach((cell, index) => { (clone.children[index] as HTMLElement).style.width = `${cell.getBoundingClientRect().width}px` })
      body.append(clone); table.append(body); overlay.append(table)
    } else overlay.append(clone)
  }
  const marker = doc.createElement('div')
  marker.dataset.slot = 'drag-insertion-preview'
  marker.className = displace
    ? 'pointer-events-none fixed z-[99] rounded-md border border-dashed border-primary/50 bg-primary/5'
    : 'pointer-events-none fixed z-[99] h-0.5 rounded bg-primary'
  marker.setAttribute('aria-hidden', 'true')
  Object.assign(marker.style, { left: '0', top: '0', opacity: '0', height: displace ? `${bounds.height}px` : '2px', transform: `translate3d(${bounds.left}px, ${bounds.top}px, 0)`, transition: 'transform var(--design-motion-standard)' })
  doc.body.append(overlay, marker)
  const image = doc.createElement('span')
  image.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none'
  doc.body.append(image)
  if (nativePreview) overlay.style.display = 'none'
  else event.dataTransfer?.setDragImage?.(image, 0, 0)
  const originalOpacity = visualStyle(source).opacity
  visualStyle(source).opacity = '0'
  const grabX = Math.max(0, Math.min(bounds.width, event.clientX - bounds.left))
  const grabY = Math.max(0, Math.min(bounds.height, event.clientY - bounds.top))
  const altered = new Map<HTMLElement, { transform: string; transition: string; pointerEvents: string }>()
  let finished = false
  let target: { element: HTMLElement; direction: 'before' | 'after' } | null = null
  let geometry: PreviewEntry[] | null = null
  const invalidate = () => { geometry = null }
  const observer = options.cacheGeometry && typeof view.ResizeObserver === 'function' ? new view.ResizeObserver(invalidate) : null
  if (options.cacheGeometry) {
    doc.addEventListener('scroll', invalidate, true)
    doc.addEventListener('load', invalidate, true)
    view.addEventListener('resize', invalidate)
    if (scope) observer?.observe(scope)
  }
  const reset = () => {
    for (const [element, original] of altered) Object.assign(visualStyle(element), original)
    altered.clear()
  }
  const rect = (element: HTMLElement) => {
    const box = element.getBoundingClientRect()
    let offset = 0
    for (let current: HTMLElement | null = element; current; current = current.parentElement) {
      if (!altered.has(current)) continue
      const transform = view.getComputedStyle(current).transform
      if (transform.startsWith('matrix(')) offset += Number(transform.slice(7, -1).split(',')[5]) || 0
      else if (transform.startsWith('translateY(')) offset += parseFloat(transform.slice(11)) || 0
    }
    return { top: box.top - offset, bottom: box.bottom - offset, left: box.left, width: box.width, height: box.height }
  }
  const update = (x: number, y: number) => {
    if (finished) return null
    if (!source.isConnected) { finish(); return null }
    if (!options.cacheGeometry || !geometry) {
      geometry = siblings().filter(element => element.isConnected).map(element => ({ element, box: rect(element) }))
      target = null
    }
    const entries = geometry
    if (!nativePreview) overlay.style.transform = `translate3d(${x - grabX}px, ${y - grabY}px, 0)`
    const first = entries[0], last = entries.at(-1)
    const horizontal = horizontalBounds?.()
    if (!first || !last || x < (horizontal?.left ?? first.box.left - 48) || x > (horizontal?.right ?? first.box.left + first.box.width + 48) || y < first.box.top - 24 || y > last.box.bottom + 24) {
      target = null; marker.style.opacity = '0'; reset(); return null
    }
    const hit = nested ? nested.select(entries, x, y) : entries.find(entry => y < entry.box.bottom) ?? last
    if (!hit) { target = null; marker.style.opacity = '0'; reset(); return null }
    const direction = y < hit.box.top + hit.box.height / 2 ? 'before' : 'after'
    if (target?.element === hit.element && target.direction === direction) return target
    target = { element: hit.element, direction }
    marker.style.opacity = '1'
    marker.style.width = `${hit.box.width}px`
    const items = entries.map(entry => entry.element)
    const peers = nested ? items.filter(element => element.parentElement === source.parentElement) : items
    const canDisplace = displace && (!nested || hit.element.parentElement === source.parentElement)
    const from = peers.indexOf(source), to = peers.indexOf(hit.element) + (direction === 'after' ? 1 : 0)
    const boundary = direction === 'before' ? hit.box.top : hit.box.bottom
    const gapTop = canDisplace && from >= 0 && to > from ? boundary - bounds.height : boundary
    marker.style.height = canDisplace ? `${bounds.height}px` : '2px'
    marker.style.transform = `translate3d(${hit.box.left}px, ${gapTop}px, 0)`
    if (canDisplace) {
      peers.forEach((element, index) => {
        if (element === source) return
        const style = visualStyle(element)
        if (!altered.has(element)) altered.set(element, { transform: style.transform, transition: style.transition, pointerEvents: style.pointerEvents })
        // Moving rows must not keep changing the native browser drop target.
        // The stable list/editor container receives the gesture throughout.
        style.pointerEvents = 'none'
        const offset = from >= 0 && index > from && index < to ? -bounds.height : from >= 0 && index >= to && index < from ? bounds.height : 0
        style.transition = 'transform var(--design-motion-standard)'
        style.transform = `translateY(${offset}px)`
      })
    } else reset()
    return target
  }
  const onOver = (next: DragEvent) => { update(next.clientX, next.clientY) }
  const finish = () => {
    if (finished) return
    finished = true; reset(); visualStyle(source).opacity = originalOpacity
    sheetElement?.remove()
    if (scope) {
      if (previousScope == null) scope.removeAttribute(scopeAttribute)
      else scope.setAttribute(scopeAttribute, previousScope)
    }
    overlay.remove(); marker.remove(); image.remove()
    doc.removeEventListener('dragover', onOver)
    doc.removeEventListener('dragend', finish)
    doc.removeEventListener('drop', finish)
    view.removeEventListener('blur', finish)
    doc.removeEventListener('scroll', invalidate, true)
    doc.removeEventListener('load', invalidate, true)
    view.removeEventListener('resize', invalidate)
    observer?.disconnect()
  }
  if (options.automaticTracking !== false) doc.addEventListener('dragover', onOver)
  doc.addEventListener('dragend', finish)
  // Bubble cleanup runs after the owning row/editor's drop handler.
  doc.addEventListener('drop', finish)
  view.addEventListener('blur', finish)
  update(event.clientX, event.clientY)
  return { update, finish, get target() { return target } }
}
