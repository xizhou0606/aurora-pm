import { Editor } from '@tiptap/core'
import { describe, expect, it, vi } from 'bun:test'

import { focusRichTextDocumentPosition } from './rich-text-document-navigation'
import { updateRichTextCommentAnchors } from './rich-text-comment-anchors'
import { createRichTextExtensions } from './rich-text-extensions'

function createEditor() {
  return new Editor({
    extensions: createRichTextExtensions({
      preset: 'document',
      schemaVersion: 1,
      placeholder: '',
    }),
    content: '<h2>Architecture</h2><p>Decision</p>',
  })
}

describe('rich text document position navigation', () => {
  it('places the selection inside a heading and scrolls only the document body container', () => {
    const editor = createEditor()
    const heading = editor.view.nodeDOM(0) as HTMLElement
    const scrollIntoView = vi.fn()
    Object.defineProperty(heading, 'scrollIntoView', { configurable: true, value: scrollIntoView })
    vi.spyOn(heading, 'getBoundingClientRect').mockReturnValue({
      top: 180,
      height: 32,
    } as DOMRect)
    const container = document.createElement('div')
    container.dataset.slot = 'document-scroll'
    container.scrollTop = 40
    vi.spyOn(container, 'getBoundingClientRect').mockReturnValue({ top: 80 } as DOMRect)
    const scrollTo = vi.fn()
    Object.defineProperty(container, 'scrollTo', { configurable: true, value: scrollTo })
    container.append(editor.view.dom)
    const focus = vi.spyOn(editor.view, 'focus')
    const contentBefore = editor.getJSON()
    updateRichTextCommentAnchors(editor, {
      currentRevision: 5,
      anchors: [{
        id: 'thread-1',
        revision: 5,
        status: 'active',
        label: 'Open comment',
        selection: { from: 1, to: 13, quote: 'Architecture' },
      }],
    })

    expect(focusRichTextDocumentPosition(editor, 0)).toBe(true)
    expect(editor.state.selection).toMatchObject({ from: 1, to: 1 })
    expect(scrollTo).toHaveBeenCalledWith({
      behavior: 'smooth',
      top: 140,
    })
    expect(scrollIntoView).not.toHaveBeenCalled()
    expect(focus).toHaveBeenCalledTimes(1)
    expect(editor.getJSON()).toEqual(contentBefore)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor-id="thread-1"]')).not.toBeNull()

    editor.destroy()
  })

  it('does not add outline navigation to the content undo history', () => {
    const editor = createEditor()
    const contentBefore = editor.getJSON()
    editor.commands.setTextSelection(1)
    editor.commands.insertContent('Updated ')
    expect(editor.getJSON()).not.toEqual(contentBefore)

    expect(focusRichTextDocumentPosition(editor, 0)).toBe(true)
    expect(editor.commands.undo()).toBe(true)
    expect(editor.getJSON()).toEqual(contentBefore)

    editor.destroy()
  })

  it('accepts a position inside a heading and supports scroll without focus', () => {
    const editor = createEditor()
    const heading = editor.view.nodeDOM(0) as HTMLElement
    const scrollIntoView = vi.fn()
    Object.defineProperty(heading, 'scrollIntoView', { configurable: true, value: scrollIntoView })
    const focus = vi.spyOn(editor.view, 'focus')

    expect(focusRichTextDocumentPosition(editor, 4, {
      focus: false,
      scrollBehavior: 'auto',
    })).toBe(true)
    expect(editor.state.selection).toMatchObject({ from: 4, to: 4 })
    expect(scrollIntoView).toHaveBeenCalledWith({
      behavior: 'auto',
      block: 'start',
      inline: 'nearest',
    })
    expect(focus).not.toHaveBeenCalled()

    editor.destroy()
  })

  it('rejects out-of-range and non-heading positions without changing content or selection', () => {
    const editor = createEditor()
    const selectionBefore = editor.state.selection
    const contentBefore = editor.getJSON()
    const paragraphPosition = editor.state.doc.child(0).nodeSize

    expect(focusRichTextDocumentPosition(editor, -1)).toBe(false)
    expect(focusRichTextDocumentPosition(editor, editor.state.doc.content.size + 1)).toBe(false)
    expect(focusRichTextDocumentPosition(editor, paragraphPosition)).toBe(false)
    expect(editor.state.selection.eq(selectionBefore)).toBe(true)
    expect(editor.getJSON()).toEqual(contentBefore)

    editor.destroy()
  })
})
