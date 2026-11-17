import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useEditor as realUseEditor } from '@tiptap/react'
import type { Editor } from '@tiptap/react'
import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'
import { RichTextDocumentRuntime } from './rich-text-document-runtime'
import { RichTextBitableProvider } from './rich-text-bitable-context'
import { createRichTextSlashCommands } from './rich-text-command-registry'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE as labels } from './rich-text.react-test-fixtures'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'

const captured = (() => ({ editor: null as Editor | null }))()
// bun 的 mock.module 会把导入绑定整体替换为 mock（live binding），
// 因此必须在注册前先捕获真实 useEditor，否则包装器会无限递归。
const realUseEditorImpl = realUseEditor
mock.module('@tiptap/react', () => ({
  useEditor: (...args: Parameters<typeof realUseEditorImpl>) => {
    const editor = realUseEditorImpl(...args); captured.editor = editor; return editor
  },
}))
const reference = { schemaVersion: 1 as const, resourceId: 'resource', tableId: 'table', viewId: 'view', height: 420 }
const toolbarLabels = { bold: 'Bold', italic: 'Italic', bulletList: 'Bullets', orderedList: 'Numbers', undo: 'Undo', redo: 'Redo' }
beforeEach(() => { captured.editor = null })
describe('async bitable picker serialization', () => {
  it('emits the inserted reference even while the picker still owns DOM focus', async () => {
    const onChange = vi.fn(), value = JSON.stringify({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: '/' }] }] })
    let resolve!: (value: typeof reference) => void
    const pick = () => new Promise<typeof reference>(yes => { resolve = yes })
    render(<RichTextBitableProvider value={{ pick, render: () => <div /> }}><RichTextDocumentRuntime value={value} onChange={onChange} minHeight={100} serialization="json" preset="document" documentSchemaVersion={1} documentLabels={labels} toolbarLabels={toolbarLabels} /></RichTextBitableProvider>)
    await waitFor(() => expect(captured.editor).toBeTruthy())
    const editor = captured.editor!
    expect(onChange).not.toHaveBeenCalled()
    expect(editor.isFocused).toBe(false)
    const command = createRichTextSlashCommands({ labels: labels.slashMenu.commands, schemaVersion: 3, pickBitable: pick }).find(entry => entry.id === 'bitable')!
    act(() => { command.run({ editor, range: { from: 1, to: 2 } }) })
    await act(async () => { resolve(reference); await Promise.resolve() })
    expect(onChange).toHaveBeenCalled()
    expect(JSON.parse(onChange.mock.calls[0][0]).content).toContainEqual({ type: 'bitable', attrs: reference })
  })
  it('opening legacy content with an enabled integration does not issue an onChange', async () => {
    const onChange = vi.fn()
    render(<RichTextBitableProvider value={{ render: () => <div /> }}><RichTextDocumentRuntime value="Legacy plain text" onChange={onChange} minHeight={100} serialization="json" preset="document" documentSchemaVersion={1} documentLabels={labels} toolbarLabels={toolbarLabels} /></RichTextBitableProvider>)
    await waitFor(() => expect(captured.editor?.getText()).toBe('Legacy plain text'))
    expect(onChange).not.toHaveBeenCalled()
  })
  it('height changes emit user-owned content while the resize handle owns focus', async () => {
    const onChange = vi.fn(), value = JSON.stringify({ type: 'doc', content: [{ type: 'bitable', attrs: reference }, { type: 'paragraph' }] })
    render(<RichTextBitableProvider value={{ render: () => <div /> }}><RichTextDocumentRuntime value={value} onChange={onChange} minHeight={100} serialization="json" preset="document" documentSchemaVersion={3} documentLabels={labels} toolbarLabels={toolbarLabels} /></RichTextBitableProvider>)
    const handle = await screen.findByRole('separator', { name: DOCUMENTS_COPY.bitable.resize })
    handle.focus()
    expect(captured.editor?.isFocused).toBe(false)
    fireEvent.keyDown(handle, { key: 'ArrowDown' })
    expect(onChange).toHaveBeenCalled()
    expect(JSON.parse(onChange.mock.calls[0][0]).content).toContainEqual({ type: 'bitable', attrs: { ...reference, height: 460 } })
  })
})
