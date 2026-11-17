import { Editor } from '@tiptap/core'
import { fireEvent } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import {
  resolveRichTextCommentAnchor,
  focusRichTextCommentAnchor,
  updateRichTextCommentAnchors,
} from './rich-text-comment-anchors'
import { createRichTextExtensions } from './rich-text-extensions'
import type { RichTextCommentAnchor } from './rich-text-types'

function createEditor() {
  return new Editor({
    extensions: createRichTextExtensions({
      preset: 'document',
      schemaVersion: 1,
      placeholder: '',
    }),
    content: '<p>Alpha beta gamma</p>',
  })
}

function anchor(overrides: Partial<RichTextCommentAnchor> = {}): RichTextCommentAnchor {
  return {
    id: 'thread-17',
    revision: 8,
    status: 'active',
    label: 'Open comment thread',
    selection: {
      from: 7,
      to: 11,
      quote: 'beta',
      prefix: 'Alpha ',
      suffix: ' gamma',
    },
    ...overrides,
  }
}

describe('rich text comment anchors', () => {
  it('accepts a server-validated non-empty range from the current revision', () => {
    const editor = createEditor()

    expect(resolveRichTextCommentAnchor(editor.state.doc, 8, anchor()).status).toBe('active')
    expect(resolveRichTextCommentAnchor(editor.state.doc, 9, anchor()).status).toBe('orphan')
    expect(resolveRichTextCommentAnchor(editor.state.doc, 8, anchor({
      selection: { ...anchor().selection, quote: 'original snapshot text' },
    })).status).toBe('active')
    expect(resolveRichTextCommentAnchor(editor.state.doc, 8, anchor({ status: 'orphan' })).status)
      .toBe('orphan')

    editor.destroy()
  })

  it('renders active and resolved markers without positioning orphaned snapshots', () => {
    const editor = createEditor()
    updateRichTextCommentAnchors(editor, {
      currentRevision: 8,
      anchors: [
        anchor(),
        anchor({
          id: 'thread-18',
          status: 'resolved',
          selection: { from: 12, to: 17, quote: 'gamma', prefix: 'Alpha beta ' },
        }),
        anchor({ id: 'thread-19', status: 'orphan' }),
        anchor({ id: 'thread-20', revision: 7 }),
      ],
    })

    expect(editor.view.dom.querySelectorAll('[data-rich-text-comment-anchor]')).toHaveLength(2)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor-id="thread-17"]')
      ?.getAttribute('data-comment-anchor-state')).toBe('active')
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor-id="thread-18"]')
      ?.getAttribute('data-comment-anchor-state')).toBe('resolved')
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor-id="thread-19"]')).toBeNull()
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor-id="thread-20"]')).toBeNull()

    editor.destroy()
  })

  it('uses one marker for identical ranges and selects the controlled thread from that range', () => {
    const editor = createEditor()
    const onSelect = vi.fn()
    updateRichTextCommentAnchors(editor, {
      currentRevision: 8,
      activeAnchorId: 'thread-18',
      anchors: [anchor(), anchor({ id: 'thread-18', status: 'resolved' })],
      onSelect,
    })

    const markers = editor.view.dom.querySelectorAll<HTMLElement>('[data-rich-text-comment-anchor]')
    expect(markers).toHaveLength(1)
    expect(markers[0]?.dataset.richTextCommentAnchorIds).toBe('["thread-17","thread-18"]')
    expect(focusRichTextCommentAnchor(editor, 'thread-18')).toBe(true)
    fireEvent.click(markers[0] as HTMLElement)
    expect(onSelect).toHaveBeenCalledWith('thread-18')

    editor.destroy()
  })

  it('clears stale focus styling when another comment becomes active', () => {
    const editor = createEditor()
    const anchors = [
      anchor(),
      anchor({
        id: 'thread-18',
        status: 'resolved',
        selection: { from: 12, to: 17, quote: 'gamma', prefix: 'Alpha beta ' },
      }),
    ]
    updateRichTextCommentAnchors(editor, {
      currentRevision: 8,
      activeAnchorId: 'thread-17',
      anchors,
    })
    expect(editor.view.dom.querySelectorAll('[data-comment-anchor-focused]')).toHaveLength(1)

    updateRichTextCommentAnchors(editor, {
      currentRevision: 8,
      activeAnchorId: 'thread-18',
      anchors,
    })
    const focused = editor.view.dom.querySelectorAll<HTMLElement>('[data-comment-anchor-focused]')
    expect(focused).toHaveLength(1)
    expect(focused[0]?.dataset.richTextCommentAnchorId).toBe('thread-18')

    editor.destroy()
  })

  it('activates a thread and maps its range until the selected content is fully deleted', () => {
    const editor = createEditor()
    const onSelect = vi.fn()
    updateRichTextCommentAnchors(editor, {
      currentRevision: 8,
      anchors: [anchor()],
      activeAnchorId: 'thread-17',
      onSelect,
    })
    const marker = editor.view.dom.querySelector<HTMLElement>('[data-rich-text-comment-anchor]')
    expect(marker).not.toBeNull()
    expect(marker?.getAttribute('role')).toBe('button')
    expect(marker?.getAttribute('tabindex')).toBe('0')
    expect(marker?.hasAttribute('data-comment-anchor-focused')).toBe(true)
    const focus = vi.spyOn(marker as HTMLElement, 'focus')
    expect(focusRichTextCommentAnchor(editor, 'thread-17')).toBe(true)
    expect(focus).not.toHaveBeenCalled()

    fireEvent.click(marker as HTMLElement)
    fireEvent.keyDown(marker as HTMLElement, { key: 'Enter' })
    fireEvent.keyDown(marker as HTMLElement, { key: ' ' })
    expect(onSelect).toHaveBeenNthCalledWith(1, 'thread-17')
    expect(onSelect).toHaveBeenNthCalledWith(2, 'thread-17')
    expect(onSelect).toHaveBeenNthCalledWith(3, 'thread-17')

    editor.commands.setTextSelection({ from: 7, to: 11 })
    fireEvent.click(marker as HTMLElement)
    expect(onSelect).toHaveBeenCalledTimes(3)

    editor.commands.insertContentAt(1, 'Changed ')
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('beta')

    editor.commands.insertContentAt({ from: 15, to: 19 }, 'delta')
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('delta')

    editor.commands.insertContentAt(20, ' extended')
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('delta extended')

    editor.commands.deleteRange({ from: 20, to: 29 })
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('delta')

    editor.commands.deleteRange({ from: 15, to: 20 })
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')).toBeNull()

    editor.destroy()
  })
})
