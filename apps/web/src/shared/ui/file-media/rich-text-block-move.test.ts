import { Editor } from '@tiptap/core'
import { closeHistory } from '@tiptap/pm/history'
import { describe, expect, it } from 'bun:test'
import { createRichTextExtensions } from './rich-text-extensions'
import { updateRichTextCommentAnchors } from './rich-text-comment-anchors'
import { movedRichTextRange } from './rich-text-block-move'

describe('editorcn block drag comment mapping', () => {
  it('retains the moved range when React replays the same server anchor before and after autosave', () => {
    const editor = new Editor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }),
      content: '<h2>Alpha block</h2><p>Alpha annotation text.</p><h2>Beta block</h2><p>Beta body.</p>' })
    const anchor = { id: 'test', revision: 1, status: 'active' as const, label: 'test', selection: { from: 20, to: 30, quote: 'annotation' } }
    const replay = (currentRevision: number) => updateRichTextCommentAnchors(editor, { currentRevision, anchors: [{ ...anchor, selection: { ...anchor.selection } }] })
    replay(1)
    const paragraph = editor.state.doc.child(1)
    editor.view.dispatch(closeHistory(editor.state.tr).delete(13, 13 + paragraph.nodeSize).insert(25, paragraph))
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('annotation')
    replay(1)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('annotation')
    replay(2)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('annotation')
    expect(editor.commands.undo()).toBe(true)
    replay(2)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('annotation')
    editor.destroy()
  })
  it.each([false, true])('keeps annotations on reordered blocks through undo/redo (up=%s)', (up) => {
    const editor = new Editor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }),
      content: '<h2>Alpha</h2><p>Beta</p><ul><li><p>Gamma</p></li></ul>' })
    const from = up ? 7 : 0
    const node = editor.state.doc.nodeAt(from)!
    const target = up ? 0 : 6
    const selection = { from: from + 1, to: from + 4, quote: up ? 'Bet' : 'Alp' }
    updateRichTextCommentAnchors(editor, { currentRevision: 1, anchors: [{ id: 'test', revision: 1, status: 'active', label: 'test', selection }] })
    const original = editor.getJSON()
    // ProseMirror's native move emits one deletion and one insertion, not a JSON reset.
    const tr = closeHistory(editor.state.tr).delete(from, from + node.nodeSize).insert(target, node)
    expect(movedRichTextRange(tr, selection.from, selection.to)).toEqual({ from: target + 1, to: target + 4 })
    editor.view.dispatch(tr)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe(selection.quote)
    expect(editor.commands.undo()).toBe(true)
    expect(editor.getJSON()).toEqual(original)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe(selection.quote)
    expect(editor.commands.redo()).toBe(true)
    expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe(selection.quote)
    editor.destroy()
  })

  it('does not relocate a genuinely deleted range to unrelated inserted text', () => {
    const editor = new Editor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }), content: '<p>Alpha</p><p>Beta</p>' })
    const tr = editor.state.tr.delete(0, 7).insert(6, editor.state.schema.nodes.paragraph.create(null, editor.state.schema.text('Other')))
    expect(movedRichTextRange(tr, 1, 4)).toBeUndefined()
    editor.destroy()
  })
  it('moves a complete list without splitting list items and updates its heading positions', () => {
    const editor = new Editor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }), content: '<h2>Title</h2><ul><li><p>First</p></li><li><p>Second</p></li></ul><p>Last</p>' })
    const original = editor.getJSON()
    const list = editor.state.doc.child(1)
    const from = editor.state.doc.child(0).nodeSize
    const tr = closeHistory(editor.state.tr).delete(from, from + list.nodeSize).insert(0, list)
    editor.view.dispatch(tr)
    expect(editor.getJSON().content?.[0]).toEqual(original.content?.[1])
    expect(editor.state.doc.child(1).type.name).toBe('heading')
    expect(editor.commands.undo()).toBe(true)
    expect(editor.getJSON()).toEqual(original)
    editor.destroy()
  })
})
