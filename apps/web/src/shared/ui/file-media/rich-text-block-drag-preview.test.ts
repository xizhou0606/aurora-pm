import { Editor } from '@tiptap/core'
import { afterEach, describe, expect, it, vi } from 'bun:test'
import { useJsdomEnvironment } from '@/shared/testing/jsdom-environment'
import { createRichTextExtensions } from './rich-text-extensions'
import { createRichTextBlockDragPreview } from './rich-text-block-drag-preview'
import { DOCUMENT_RICH_TEXT_LABELS } from '@/shared/copy/documents'

// Keep native CSSOM rule evaluation for displacement assertions.
useJsdomEnvironment()

const cleanups: (() => void)[] = []
afterEach(() => {
  for (const cleanup of cleanups.splice(0).reverse()) cleanup()
})

function setup() {
  const element = document.createElement('div'); document.body.append(element)
  element.getBoundingClientRect = () => ({ top: 0, bottom: 120, left: -100, right: 400, width: 500, height: 120, x: -100, y: 0, toJSON() {} })
  const editor = new Editor({ element, editorProps: { handleScrollToSelection: () => true }, extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }), content: '<p>Alpha</p><p>Beta</p><p>Gamma</p>' })
  const blocks = Array.from(editor.view.dom.children) as HTMLElement[]
  blocks.forEach((block, i) => {
    block.getBoundingClientRect = () => {
      const offset = parseFloat(getComputedStyle(block).transform.slice(11)) || 0
      return { top: i * 40 + offset, bottom: (i + 1) * 40 + offset, left: 0, right: 200, width: 200, height: 40, x: 0, y: i * 40 + offset, toJSON() {} }
    }
  })
  const cleanup = () => {
    document.dispatchEvent(new Event('dragend'))
    if (!editor.isDestroyed) editor.destroy()
    element.remove()
  }
  cleanups.push(cleanup)
  return { editor, blocks, cleanup }
}
function drop(root: HTMLElement, y: number) {
  root.dispatchEvent(new MouseEvent('drop', { bubbles: true, cancelable: true, clientX: 20, clientY: y }))
}
describe('document block placeholder', () => {
  it('coalesces dragover work and uses the drop coordinates before the pending frame', () => {
    const { editor, cleanup } = setup()
    const frame = vi.spyOn(window, 'requestAnimationFrame').mockReturnValue(42)
    const cancel = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
    const preview = createRichTextBlockDragPreview(editor, 0, { clientX: 10, clientY: 10 } as DragEvent)!
    try {
      for (let i = 0; i < 50; i++) editor.view.dom.dispatchEvent(new MouseEvent('dragover', { bubbles: true, cancelable: true, clientX: 20, clientY: 45 }))
      expect(frame).toHaveBeenCalledTimes(1)
      drop(editor.view.dom, 115)
      expect(editor.state.doc.textContent).toBe('BetaGammaAlpha')
      expect(cancel).toHaveBeenCalledWith(42)
    } finally { preview.finish(); frame.mockRestore(); cancel.mockRestore(); cleanup() }
  })
  it('replaces an unfinished gesture so its capture listener cannot swallow the next drop', () => {
    const { editor, cleanup } = setup()
    const old = createRichTextBlockDragPreview(editor, 0, { clientX: 10, clientY: 10 } as DragEvent)!
    editor.view.dispatch(editor.state.tr.insertText('New ', 1))
    const last = editor.state.doc.child(0).nodeSize + editor.state.doc.child(1).nodeSize
    const next = createRichTextBlockDragPreview(editor, last, { clientX: 10, clientY: 90 } as DragEvent)!
    old.finish()
    drop(editor.view.dom, 5)
    expect(editor.state.doc.textContent).toBe('GammaNew AlphaBeta')
    expect(document.querySelector('[data-slot="drag-insertion-preview"]')).toBeNull()
    next.finish(); cleanup()
  })
  it('selects a nested heading instead of its table and transfers its exact node to the root', () => {
    const element = document.createElement('div'); document.body.append(element)
    element.getBoundingClientRect = () => ({ left: 0, right: 500, top: 0, bottom: 200 } as DOMRect)
    const editor = new Editor({ element, editorProps: { handleScrollToSelection: () => true }, extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 3, placeholder: '', documentLabels: DOCUMENT_RICH_TEXT_LABELS }), content: '<h1>First</h1><table><tbody><tr><td><h2><strong>Nested</strong></h2><p>Keep</p></td></tr></tbody></table><p>Last</p>' })
    let source = -1
    editor.state.doc.descendants((node, pos) => {
      const dom = editor.view.nodeDOM(pos)
      if (!(dom instanceof HTMLElement)) return
      const top = node.textContent === 'First' ? 0 : node.textContent === 'Nested' ? 60 : node.textContent === 'Keep' ? 100 : node.textContent === 'Last' ? 160 : 40
      const height = node.type.name === 'table' ? 120 : 40
      dom.getBoundingClientRect = () => ({ left: 0, right: 200, top, bottom: top + height, width: 200, height } as DOMRect)
      if (node.type.name === 'heading' && node.attrs.level === 2) source = pos
    })
    const original = editor.state.doc.nodeAt(source)!.toJSON()
    const preview = createRichTextBlockDragPreview(editor, source, { clientX: 20, clientY: 65 } as DragEvent)!
    expect(preview).not.toBeNull()
    expect(preview.update(20, 5)?.element.tagName).toBe('H1')
    drop(editor.view.dom, 5)
    expect(editor.state.doc.firstChild!.toJSON()).toEqual(original)
    expect(editor.state.doc.textContent).toBe('NestedFirstKeepLast')
    expect(element.querySelector('[data-drag-preview-scope]')).toBeNull()
    preview.finish(); editor.destroy(); element.remove()
  })
  it('shows an animated gap and moves to that exact boundary with undo support', () => {
    const { editor, blocks, cleanup } = setup()
    const original = editor.getJSON()
    const preview = createRichTextBlockDragPreview(editor, 0, { clientX: 10, clientY: 10 } as DragEvent)!
    preview.update(20, 115)
    const marker = document.querySelector<HTMLElement>('[data-slot="drag-insertion-preview"]')!
    expect(marker.style.transform).toContain('0px, 80px')
    expect(marker.style.transition).toContain('transform')
    expect(getComputedStyle(blocks[1]).transform).toBe('translateY(-40px)')
    expect(blocks.every(block => !block.hasAttribute('style'))).toBe(true)
    drop(editor.view.dom, 115)
    expect(editor.state.doc.textContent).toBe('BetaGammaAlpha')
    expect(document.querySelector('[data-slot="drag-insertion-preview"]')).toBeNull()
    expect(editor.commands.undo()).toBe(true)
    expect(editor.getJSON()).toEqual(original)
    preview.finish(); cleanup()
  })
  it('cancels a stale drag after a concurrent content update instead of overwriting it', () => {
    const { editor, cleanup } = setup()
    const preview = createRichTextBlockDragPreview(editor, 0, { clientX: 10, clientY: 10 } as DragEvent)!
    preview.update(20, 115)
    editor.view.dispatch(editor.state.tr.insertText('New ', 1))
    const updated = editor.getJSON()
    drop(editor.view.dom, 115)
    expect(editor.getJSON()).toEqual(updated)
    expect(document.querySelector('[data-slot="drag-insertion-preview"]')).toBeNull()
    preview.finish(); cleanup()
  })
  it('moves a block upward and restores preview styles after dropping', () => {
    const { editor, blocks, cleanup } = setup()
    const position = editor.state.doc.child(0).nodeSize + editor.state.doc.child(1).nodeSize
    const preview = createRichTextBlockDragPreview(editor, position, { clientX: 10, clientY: 90 } as DragEvent)!
    preview.update(20, 5)
    expect(getComputedStyle(blocks[0]).transform).toBe('translateY(40px)')
    drop(editor.view.dom, 5)
    expect(editor.state.doc.textContent).toBe('GammaAlphaBeta')
    expect(document.querySelector('[data-drag-preview-scope]')).toBeNull()
    preview.finish(); cleanup()
  })
  it('accepts drops in the outer gutter and rejects positions beyond the document surface', () => {
    const { editor, cleanup } = setup()
    const preview = createRichTextBlockDragPreview(editor, 0, { clientX: 10, clientY: 10 } as DragEvent)!
    expect(preview.update(450, 115)).toBeNull()
    expect(preview.update(350, 115)).not.toBeNull()
    editor.view.dom.parentElement!.dispatchEvent(new MouseEvent('drop', { bubbles: true, cancelable: true, clientX: 350, clientY: 115 }))
    expect(editor.state.doc.textContent).toBe('BetaGammaAlpha')
    preview.finish(); cleanup()
  })
})
