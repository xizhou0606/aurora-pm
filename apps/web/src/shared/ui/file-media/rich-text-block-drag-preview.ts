import type { Editor } from '@tiptap/core'
import { closeHistory } from '@tiptap/pm/history'
import { Fragment } from '@tiptap/pm/model'
import { createDragPreview } from '@/shared/lib/drag-preview'
import { transferRichTextBlock } from './rich-text-block-transfer'

const activePreviews = new WeakMap<Editor, () => void>()

export function richTextDragSurface(root: HTMLElement) {
  return root.closest<HTMLElement>('[data-slot="document-scroll"]')
    ?? root.closest<HTMLElement>('[data-slot="rich-text-editor-runtime"]')
    ?? root.parentElement!
}

// The visual gap and the transaction use the same nested block boundary.
// A concurrent document change cancels this gesture instead of using stale positions.
export function createRichTextBlockDragPreview(editor: Editor, position: number, event: DragEvent) {
  activePreviews.get(editor)?.()
  const root = editor.view.dom, doc = root.ownerDocument
  const surface = richTextDragSurface(root)
  const snapshot = editor.state.doc
  const source = editor.view.nodeDOM(position)
  const node = snapshot.nodeAt(position)
  if (!(source instanceof HTMLElement) || !node || !node.isBlock) return null
  const fragment = Fragment.from(node)
  const accepts = (boundary: number) => {
    const resolved = snapshot.resolve(boundary)
    return resolved.parent.canReplace(resolved.index(), resolved.index(), fragment)
  }
  const blocks = new Map<HTMLElement, { from: number; to: number; before: boolean; after: boolean }>()
  snapshot.descendants((block, from) => {
    if (!block.isBlock || ['tableRow', 'tableCell', 'tableHeader'].includes(block.type.name)) return
    if (from > position && from < position + node.nodeSize) return false
    const element = editor.view.nodeDOM(from)
    if (element instanceof HTMLElement) blocks.set(element, { from, to: from + block.nodeSize, before: accepts(from), after: accepts(from + block.nodeSize) })
  })
  const preview = createDragPreview(source, event, () => [...blocks.keys()], true, true, () => surface.getBoundingClientRect(), {
    styleScope: root.parentElement!,
    select: (entries, x, y) => {
      const distance = (entry: typeof entries[number]) => Math.max(entry.box.top - y, y - entry.box.bottom, 0)
      const horizontal = (entry: typeof entries[number]) => Math.max(entry.box.left - x, x - entry.box.left - entry.box.width, 0)
      let best: typeof entries[number] | undefined
      for (const entry of entries) {
        const range = blocks.get(entry.element)!
        if (!(y < entry.box.top + entry.box.height / 2 ? range.before : range.after)) continue
        if (!best || (distance(entry) - distance(best) || horizontal(entry) - horizontal(best) || entry.box.height - best.box.height) < 0) best = entry
      }
      return best
    },
  }, { cacheGeometry: true, automaticTracking: false })
  let finished = false
  let frame: number | null = null
  let pointer = { x: event.clientX, y: event.clientY }
  const finish = () => {
    if (finished) return
    finished = true
    if (frame !== null) doc.defaultView?.cancelAnimationFrame(frame)
    preview.finish()
    surface.removeEventListener('drop', drop, true)
    surface.removeEventListener('dragover', over, true)
    doc.removeEventListener('dragend', finish)
    doc.removeEventListener('drop', finish)
    doc.defaultView?.removeEventListener('blur', finish)
    editor.off('transaction', changed)
    if (activePreviews.get(editor) === finish) activePreviews.delete(editor)
  }
  const changed = () => {
    if (editor.state.doc !== snapshot) {
      // Do not let native DnD fall back to a selection that has changed remotely.
      editor.view.dragging = null
      preview.finish()
    }
  }
  const drop = (next: DragEvent) => {
    const hit = preview.update(next.clientX, next.clientY)
    const range = hit && blocks.get(hit.element)
    next.preventDefault(); next.stopImmediatePropagation()
    editor.view.dragging = null
    finish()
    if (!editor.isEditable || editor.state.doc !== snapshot || !range || !hit) return
    const boundary = hit.direction === 'before' ? range.from : range.to
    if (boundary >= position && boundary <= position + node.nodeSize) return
    const tr = transferRichTextBlock(editor.state, position, boundary)
    if (!tr) return
    editor.view.dispatch(tr)
    editor.view.focus()
    editor.view.dispatch(closeHistory(editor.state.tr))
  }
  const over = (next: DragEvent) => {
    // Accept the native drop synchronously; expensive preview work runs once per
    // frame. The drop handler always resolves the final coordinates itself.
    next.preventDefault()
    next.stopPropagation()
    if (next.dataTransfer) next.dataTransfer.dropEffect = 'move'
    pointer = { x: next.clientX, y: next.clientY }
    if (frame === null) frame = doc.defaultView!.requestAnimationFrame(() => {
      frame = null
      if (!finished) preview.update(pointer.x, pointer.y)
    })
  }
  surface.addEventListener('drop', drop, true)
  surface.addEventListener('dragover', over, true)
  doc.addEventListener('dragend', finish)
  doc.addEventListener('drop', finish)
  doc.defaultView?.addEventListener('blur', finish)
  editor.on('transaction', changed)
  activePreviews.set(editor, finish)
  return { ...preview, finish }
}
