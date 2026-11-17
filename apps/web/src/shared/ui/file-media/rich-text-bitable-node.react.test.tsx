import { act, render, screen, waitFor } from '@testing-library/react'
import { EditorContent, useEditor, type Editor } from '@tiptap/react'
import { describe, expect, it, vi } from 'bun:test'
import { createRichTextExtensions } from './rich-text-extensions'
import { RichTextBitableProvider } from './rich-text-bitable-context'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE as labels } from './rich-text.react-test-fixtures'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'

const reference = { schemaVersion: 1, resourceId: 'resource', tableId: 'table', viewId: 'view', height: 420 }
function Harness({ editable = true, ready }: { editable?: boolean; ready?: (editor: Editor) => void }) {
  const editor = useEditor({ extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 3, documentLabels: labels, placeholder: '' }),
    editable, content: { type: 'doc', content: [{ type: 'bitable', attrs: reference }, { type: 'paragraph' }] }, onCreate: ({ editor: current }) => ready?.(current) })
  return <EditorContent editor={editor} />
}
describe('injected bitable NodeView', () => {
  it('renders the same reference and immediately intersects document readonly with the surface', async () => {
    let editor!: Editor
    const renderSurface = vi.fn(({ readOnly }: { readOnly: boolean }) => <div>{readOnly ? 'readonly surface' : 'editable surface'}</div>)
    render(<RichTextBitableProvider value={{ render: renderSurface }}><Harness ready={(value) => { editor = value }} /></RichTextBitableProvider>)
    await screen.findByText('editable surface')
    await waitFor(() => expect(editor).toBeTruthy())
    expect(renderSurface).toHaveBeenLastCalledWith({ reference, readOnly: false })
    await act(async () => { editor.setEditable(false) })
    await screen.findByText('readonly surface')
    expect(screen.queryByRole('separator', { name: DOCUMENTS_COPY.bitable.resize })).toBeNull()
    expect(screen.getByRole('link', { name: DOCUMENTS_COPY.bitable.open }).getAttribute('href')).toContain('/workspace/bitables/resource')
  })
  it('without a provider preserves a visible unavailable reference rather than dropping the node', async () => {
    render(<Harness editable={false} />)
    expect(await screen.findByText(DOCUMENTS_COPY.bitable.unavailable)).toBeTruthy()
  })
})
