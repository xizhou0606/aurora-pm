import type { Editor } from '@tiptap/core'
import { TextSelection } from '@tiptap/pm/state'

import type { RichTextDocumentPositionOptions } from './rich-text-types'

type HeadingTarget = {
  nodePosition: number
  selectionPosition: number
}

export function scrollRichTextElementWithinDocument(
  editor: Editor,
  element: HTMLElement,
  options: {
    behavior?: ScrollBehavior
    block?: 'start' | 'center'
  } = {},
): void {
  const container = editor.view.dom.closest<HTMLElement>('[data-slot="document-scroll"]')
  if (!container || typeof container.scrollTo !== 'function') {
    element.scrollIntoView?.({
      behavior: options.behavior ?? 'smooth',
      block: options.block ?? 'start',
      inline: 'nearest',
    })
    return
  }

  const elementRect = element.getBoundingClientRect()
  const containerRect = container.getBoundingClientRect()
  const offset = elementRect.top - containerRect.top
  const centeredOffset = options.block === 'center'
    ? offset - ((container.clientHeight - elementRect.height) / 2)
    : offset
  container.scrollTo({
    top: Math.max(0, container.scrollTop + centeredOffset),
    behavior: options.behavior ?? 'smooth',
  })
}

function resolveHeadingTarget(editor: Editor, position: number): HeadingTarget | undefined {
  const { doc } = editor.state
  if (!Number.isSafeInteger(position) || position < 0 || position > doc.content.size) return undefined

  const directNode = doc.nodeAt(position)
  if (directNode?.type.name === 'heading') {
    return { nodePosition: position, selectionPosition: position + 1 }
  }

  try {
    const resolved = doc.resolve(position)
    for (let depth = resolved.depth; depth > 0; depth -= 1) {
      if (resolved.node(depth).type.name !== 'heading') continue
      return {
        nodePosition: resolved.before(depth),
        selectionPosition: Math.max(resolved.start(depth), position),
      }
    }
  } catch {
    return undefined
  }
  return undefined
}

export function focusRichTextDocumentPosition(
  editor: Editor,
  position: number,
  options: RichTextDocumentPositionOptions = {},
): boolean {
  if (editor.isDestroyed) return false
  const target = resolveHeadingTarget(editor, position)
  if (!target) return false

  editor.view.dispatch(
    editor.state.tr
      .setSelection(TextSelection.create(editor.state.doc, target.selectionPosition))
      .setMeta('addToHistory', false),
  )

  const node = editor.view.nodeDOM(target.nodePosition)
  if (node instanceof HTMLElement) {
    scrollRichTextElementWithinDocument(editor, node, {
      behavior: options.scrollBehavior,
      block: 'start',
    })
  }
  if (options.focus !== false) editor.view.focus()
  return true
}
