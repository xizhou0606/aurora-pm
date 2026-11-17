import { DragHandle } from '@tiptap/extension-drag-handle-react'
import type { Editor } from '@tiptap/core'
import { closeHistory } from '@tiptap/pm/history'
import { useCallback, useEffect, useRef, useState } from 'react'
import { RichTextBlockHoverMenu } from './rich-text-editor-interactions'
import type { RichTextDocumentLabels } from './rich-text-types'
import type { DragPreview } from '@/shared/lib/drag-preview'
import { createRichTextBlockDragPreview, richTextDragSurface } from './rich-text-block-drag-preview'

const placement = { placement: 'left-start', strategy: 'absolute' } as const
const nestedOptions = { edgeDetection: 'none' } as const

// Same public DragHandle integration as editorcn's BlockEditor. PM owns only the
// menu appearance and top-level drop preview; upstream owns native drag startup.
export function RichTextBlockDragHandle({ editor, labels }: {
  editor: Editor
  labels: RichTextDocumentLabels['blockMenu']
}) {
  const [position, setPosition] = useState(0)
  const livePosition = useRef(0)
  const gesture = useRef(0)
  const dragged = useRef(false)
  const preview = useRef<DragPreview | null>(null)
  useEffect(() => () => { dragged.current = false; preview.current?.finish() }, [editor])
  useEffect(() => {
    const root = editor.view.dom
    const surface = richTextDragSurface(root)
    let selecting = false
    const hover = (event: MouseEvent) => {
      if (selecting || dragged.current || !editor.isEditable || !(event.target instanceof Element) || root.contains(event.target)) return
      if (event.target.closest('button, a, input, textarea, [role="dialog"], [data-slot="rich-text-block-tool"]')) return
      const bounds = root.getBoundingClientRect()
      if (event.clientY < bounds.top || event.clientY > bounds.bottom) return
      // Reuse the upstream block resolver, projecting gutter hover into the
      // content column. Do not create a second block-position state owner.
      root.dispatchEvent(new MouseEvent('mousemove', { bubbles: true, clientX: bounds.left + 8, clientY: event.clientY }))
    }
    const start = (event: PointerEvent) => {
      if (event.button !== 0 || editor.isDestroyed) return
      selecting = true
      editor.view.dispatch(editor.state.tr.setMeta('hideDragHandle', true))
      editor.view.dispatch(editor.state.tr.setMeta('lockDragHandle', true))
    }
    const finish = () => {
      if (!selecting || editor.isDestroyed) return
      selecting = false
      editor.view.dispatch(editor.state.tr.setMeta('lockDragHandle', false))
    }
    root.addEventListener('pointerdown', start)
    surface.addEventListener('mousemove', hover)
    window.addEventListener('pointerup', finish)
    window.addEventListener('pointercancel', finish)
    window.addEventListener('blur', finish)
    return () => {
      root.removeEventListener('pointerdown', start)
      surface.removeEventListener('mousemove', hover)
      window.removeEventListener('pointerup', finish)
      window.removeEventListener('pointercancel', finish)
      window.removeEventListener('blur', finish)
    }
  }, [editor])
  const onNodeChange = useCallback(({ pos }: { pos: number }) => {
    if (pos >= 0) { livePosition.current = pos; setPosition(pos) }
  }, [])
  const onStart = useCallback((event: DragEvent) => {
    if (!editor.isEditable) { event.preventDefault(); return }
    const ticket = ++gesture.current
    const sourcePosition = livePosition.current
    preview.current?.finish()
    preview.current = null
    dragged.current = true
    editor.view.focus()
    editor.view.dispatch(closeHistory(editor.state.tr))
    // Upstream first captures its native drag image and selection. Hide the source
    // only afterwards, otherwise that image inherits an invisible source style.
    queueMicrotask(() => {
      if (editor.isDestroyed || !dragged.current || ticket !== gesture.current) return
      const node = editor.state.doc.nodeAt(sourcePosition)
      const selection = editor.state.selection
      if (!node || selection.from < sourcePosition || selection.to > sourcePosition + node.nodeSize) return
      preview.current?.finish()
      preview.current = createRichTextBlockDragPreview(editor, sourcePosition, event)
    })
  }, [editor])
  const onEnd = useCallback(() => {
    dragged.current = false
    gesture.current++
    preview.current?.finish()
    preview.current = null
    if (!editor.isDestroyed) editor.view.dispatch(closeHistory(editor.state.tr))
  }, [editor])
  useEffect(() => {
    const doc = editor.view.dom.ownerDocument
    const finish = () => {
      const ticket = gesture.current
      // The drop transaction may replace the native source element, so its
      // dragend listener is not a reliable owner of gesture cleanup.
      queueMicrotask(() => { if (dragged.current && ticket === gesture.current) onEnd() })
    }
    doc.addEventListener('drop', finish, true)
    doc.addEventListener('dragend', finish, true)
    doc.defaultView?.addEventListener('blur', finish)
    return () => {
      doc.removeEventListener('drop', finish, true)
      doc.removeEventListener('dragend', finish, true)
      doc.defaultView?.removeEventListener('blur', finish)
    }
  }, [editor, onEnd])
  const onOpenChange = useCallback((open: boolean) => {
    if (!editor.isDestroyed) editor.view.dispatch(editor.state.tr.setMeta('lockDragHandle', open))
  }, [editor])
  return (
    <DragHandle
      editor={editor}
      nested={nestedOptions}
      className="z-30 pr-2 cursor-grab active:cursor-grabbing [&_button]:cursor-grab [&_button:active]:cursor-grabbing"
      computePositionConfig={placement}
      onNodeChange={onNodeChange}
      onElementDragStart={onStart}
      onElementDragEnd={onEnd}
    >
      <div
        data-slot="rich-text-block-tool"
        onPointerDownCapture={() => { dragged.current = false }}
        onClickCapture={(event) => {
          if (dragged.current) { event.preventDefault(); event.stopPropagation(); dragged.current = false }
        }}
      >
        <RichTextBlockHoverMenu editor={editor} labels={labels} position={position} onOpenChange={onOpenChange} />
      </div>
    </DragHandle>
  )
}
