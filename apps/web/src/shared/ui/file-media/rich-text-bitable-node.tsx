import { Node, mergeAttributes } from '@tiptap/core'
import { NodeViewWrapper, ReactNodeViewRenderer, useEditorState, type NodeViewProps } from '@tiptap/react'
import { useRef, useState } from 'react'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { buttonVariants } from '../primitives/button'
import { PhArrowSquareOut } from '../icons'
import { EmptyState } from '../feedback/empty-state'
import { useRichTextBitableIntegration } from './rich-text-bitable-context'
import { parseRichTextBitableReference, richTextBitablePath } from './rich-text-bitable-reference'
import { RICH_TEXT_USER_EDIT_META } from './rich-text-command-registry'

const copy = DOCUMENTS_COPY.bitable

function BitableNodeView({ node, editor, getPos }: NodeViewProps) {
  const integration = useRichTextBitableIntegration()
  const reference = parseRichTextBitableReference(node.attrs)
  const readOnly = useEditorState({ editor, selector: ({ editor: current }) => !current.isEditable })
  const [height, setHeight] = useState<number | null>(null)
  const drag = useRef<{ y: number; height: number } | null>(null)
  if (!reference) return <NodeViewWrapper contentEditable={false}><EmptyState title={copy.invalidReference} /></NodeViewWrapper>
  const commitHeight = (next: number) => {
    const position = getPos()
    if (readOnly || !editor.isEditable || typeof position !== 'number') return
    editor.chain().setMeta(RICH_TEXT_USER_EDIT_META, true).command(({ tr }) => {
      tr.setNodeMarkup(position, undefined, { ...node.attrs, height: Math.max(240, Math.min(900, Math.round(next))) })
      return true
    }).run()
  }
  return <NodeViewWrapper contentEditable={false} data-bitable-reference="true" className="relative my-4 overflow-hidden rounded-lg border border-border bg-background" onKeyDown={(event: React.KeyboardEvent) => event.stopPropagation()}>
    <a className={`${buttonVariants({ variant: 'ghost', size: 'icon-xs' })} absolute right-2 top-2 z-10 text-muted-foreground`} aria-label={copy.open} title={copy.open} href={richTextBitablePath(reference)} target="_blank" rel="noopener noreferrer"><PhArrowSquareOut className="size-3.5" aria-hidden="true" /></a>
    <div className="h-(--bitable-frame-height) min-h-0 min-w-0 overflow-hidden overscroll-contain" style={{ '--bitable-frame-height': `${height ?? reference.height}px` }} data-bitable-surface="true">
      {integration ? integration.render({ reference, readOnly }) : <EmptyState title={copy.unavailable} />}
    </div>
    {!readOnly ? <div role="separator" aria-orientation="horizontal" aria-label={copy.resize} aria-valuemin={240} aria-valuemax={900} aria-valuenow={height ?? reference.height} tabIndex={0}
      className="flex h-3 cursor-row-resize touch-none items-center justify-center border-t border-border bg-muted/20 hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
      onPointerDown={(event) => { event.preventDefault(); event.stopPropagation(); drag.current = { y: event.clientY, height: reference.height }; event.currentTarget.setPointerCapture(event.pointerId) }}
      onPointerMove={(event) => { if (drag.current) setHeight(Math.max(240, Math.min(900, Math.round(drag.current.height + event.clientY - drag.current.y)))) }}
      onPointerUp={(event) => { if (!drag.current) return; drag.current = null; if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId); if (height !== null) commitHeight(height); setHeight(null) }}
      onPointerCancel={() => { drag.current = null; setHeight(null) }}
      onKeyDown={(event) => { if (event.key === 'ArrowUp' || event.key === 'ArrowDown') { event.preventDefault(); commitHeight(reference.height + (event.key === 'ArrowDown' ? 40 : -40)) } }}><span className="h-1 w-8 rounded bg-border" /></div> : null}
  </NodeViewWrapper>
}

export const RichTextBitableNode = Node.create({
  name: 'bitable', group: 'block', atom: true, selectable: true, draggable: false, isolating: true,
  addAttributes() {
    return { schemaVersion: { default: 1 }, resourceId: { default: '' }, tableId: { default: '' }, viewId: { default: '' }, height: { default: 420 } }
  },
  parseHTML() {
    return [{ tag: 'div[data-bitable-reference]', getAttrs: (element) => {
      const source = { schemaVersion: Number(element.getAttribute('data-schema-version')), resourceId: element.getAttribute('data-resource-id'), tableId: element.getAttribute('data-table-id'), viewId: element.getAttribute('data-view-id'), height: Number(element.getAttribute('data-height')) }
      return parseRichTextBitableReference(source) ?? false
    } }]
  },
  renderHTML({ node }) {
    const reference = parseRichTextBitableReference(node.attrs)
    if (!reference) return ['div', { 'data-bitable-invalid': '' }, copy.invalidReference]
    return ['div', mergeAttributes({ 'data-bitable-reference': '', 'data-schema-version': 1, 'data-resource-id': reference.resourceId, 'data-table-id': reference.tableId, 'data-view-id': reference.viewId, 'data-height': reference.height }), ['a', { href: richTextBitablePath(reference) }, copy.title]]
  },
  renderText() { return copy.title },
  addNodeView() {
    return ReactNodeViewRenderer(BitableNodeView, { stopEvent: ({ event }) => {
      const target = event.target
      return target instanceof Element && Boolean(target.closest('[data-bitable-reference]'))
    } })
  },
})
