import { Editor } from '@tiptap/core'
import { describe, expect, it } from 'bun:test'
import { createRichTextExtensions } from './rich-text-extensions'
import { transferRichTextBlock } from './rich-text-block-transfer'
import { DOCUMENT_RICH_TEXT_LABELS } from '@/shared/copy/documents'

function setup(content: string) {
  const editor = new Editor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 3, placeholder: '', documentLabels: DOCUMENT_RICH_TEXT_LABELS }), content })
  editor.view.dispatch(editor.state.tr)
  return editor
}
function position(editor: Editor, text: string, type?: string) {
  let found = -1
  editor.state.doc.descendants((node, pos) => {
    if (found < 0 && node.isBlock && node.textContent === text && (!type || node.type.name === type)) found = pos
  })
  return found
}
describe('format-preserving block transfers', () => {
  it('keeps H2 and inline marks when moved before H1, including undo', () => {
    const editor = setup('<h1>First</h1><h2><strong>Second</strong> <a href="https://example.com">link</a></h2><h1>Third</h1>')
    const before = editor.getJSON(), from = editor.state.doc.child(0).nodeSize
    const original = editor.state.doc.nodeAt(from)!
    const tr = transferRichTextBlock(editor.state, from, 0)!
    editor.view.dispatch(tr)
    expect(editor.state.doc.child(0).toJSON()).toEqual(original.toJSON())
    expect(editor.state.doc.child(0).attrs.level).toBe(2)
    expect(editor.commands.undo()).toBe(true)
    expect(editor.getJSON()).toEqual(before)
    editor.destroy()
  })
  it('moves a heading out of a table cell without losing its format or clearing other cell content', () => {
    const editor = setup('<p>Before</p><table><tbody><tr><td><h2><em>Nested</em></h2><p>Keep</p></td></tr></tbody></table><p>After</p>')
    const from = position(editor, 'Nested', 'heading'), node = editor.state.doc.nodeAt(from)!
    editor.view.dispatch(transferRichTextBlock(editor.state, from, 0)!)
    expect(editor.state.doc.firstChild?.toJSON()).toEqual(node.toJSON())
    expect(position(editor, 'Keep', 'paragraph')).toBeGreaterThan(0)
    editor.destroy()
  })
  it('retains required empty cell content when moving its only heading out', () => {
    const editor = setup('<p>Before</p><table><tbody><tr><td><h2>Nested</h2></td></tr></tbody></table>')
    editor.view.dispatch(transferRichTextBlock(editor.state, position(editor, 'Nested', 'heading'), 0)!)
    expect(editor.state.doc.firstChild?.attrs.level).toBe(2)
    let cell: unknown
    editor.state.doc.descendants(node => { if (node.type.name === 'tableCell') cell = node.firstChild?.type.name })
    expect(cell).toBe('paragraph')
    editor.destroy()
  })
  it('moves formatted paragraphs between nested containers', () => {
    const editor = setup('<blockquote><p><strong>Move</strong></p><p>Keep</p></blockquote><blockquote><p>Target</p></blockquote>')
    const from = position(editor, 'Move', 'paragraph'), original = editor.state.doc.nodeAt(from)!
    editor.view.dispatch(transferRichTextBlock(editor.state, from, position(editor, 'Target', 'paragraph'))!)
    expect(editor.state.doc.child(1).firstChild?.toJSON()).toEqual(original.toJSON())
    editor.destroy()
  })
  it('reorders list items without converting their parent list or inline formatting', () => {
    const editor = setup('<ol start="5"><li><p><strong>A</strong></p></li><li><p>B</p></li></ol>')
    const original = editor.state.doc.firstChild!.firstChild!
    editor.view.dispatch(transferRichTextBlock(editor.state, position(editor, 'A', 'listItem'), 1 + editor.state.doc.firstChild!.content.size)!)
    expect(editor.state.doc.firstChild!.lastChild!.toJSON()).toEqual(original.toJSON())
    expect(editor.state.doc.firstChild!.attrs.start).toBe(5)
    editor.destroy()
  })
  it('rejects incompatible or self-contained destinations instead of fitting the slice', () => {
    const editor = setup('<h2>Heading</h2><ul><li><p>Item</p></li></ul>')
    const before = editor.getJSON()
    expect(transferRichTextBlock(editor.state, 0, position(editor, 'Item', 'listItem'))).toBeNull()
    expect(transferRichTextBlock(editor.state, 0, 2)).toBeNull()
    expect(editor.getJSON()).toEqual(before)
    editor.destroy()
  })
})
