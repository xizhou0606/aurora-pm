import { Editor } from '@tiptap/core'
import { EditorView } from '@tiptap/pm/view'
import StarterKit from '@tiptap/starter-kit'
import { EditorContent } from '@tiptap/react'
import { createEvent, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { RichTextEditorRuntime } from './rich-text-editor-runtime'
import { shouldShowRichTextInlineMenu } from './rich-text-editor-interactions'
import { createRichTextExtensions } from './rich-text-extensions'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE } from './rich-text.react-test-fixtures'
import { RichTextBlockDragHandle } from './rich-text-block-drag-handle'
import { updateRichTextCommentAnchors } from './rich-text-comment-anchors'

const toolbarLabels = {
  bold: 'Bold',
  italic: 'Italic',
  bulletList: 'Bullet list',
  orderedList: 'Ordered list',
  undo: 'Undo',
  redo: 'Redo',
}

async function hoverNativeBlock(target: HTMLElement) {
  // jsdom has no layout/hit testing; keep the real upstream DragHandle plugin.
  const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(40, 40, 400, 28))
  const previous = Object.getOwnPropertyDescriptor(document, 'elementsFromPoint')
  const position = vi.spyOn(EditorView.prototype, 'posAtCoords').mockReturnValue({ pos: 1, inside: 0 })
  Object.defineProperty(document, 'elementsFromPoint', { configurable: true, value: () => [target] })
  try {
    fireEvent.pointerMove(target)
    fireEvent.mouseMove(target, { clientX: 50, clientY: 50 })
    await screen.findByRole('button', { name: 'Block actions' })
  } finally {
    rect.mockRestore()
    position.mockRestore()
    if (previous) Object.defineProperty(document, 'elementsFromPoint', previous)
    else Reflect.deleteProperty(document, 'elementsFromPoint')
  }
}

describe('RichTextEditorRuntime JSON compatibility', () => {
  it('keeps compact inputs editable with image upload but without formatting toolbar buttons', async () => {
    render(<RichTextEditorRuntime value="<p>Comment</p>" compact minHeight={32}
      ariaLabel="Compact comment" onChange={vi.fn()} onUploadImage={vi.fn()}
      toolbarLabels={{ ...toolbarLabels, image: 'Upload image' }} />)
    expect(await screen.findByRole('textbox', { name: 'Compact comment' })).not.toBeNull()
    expect(screen.queryByRole('button', { name: 'Bold' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Undo' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Upload image' })).not.toBeNull()
  })
  it('lets a collaboration adapter own content and disables controlled hydration and local history', async () => {
    const onChange = vi.fn()
    const adapter = { key: 'room:1', extensions: [] }
    const { rerender } = render(
      <RichTextEditorRuntime
        value="<p>legacy snapshot</p>"
        onChange={onChange}
        collaboration={adapter}
        minHeight={100}
        serialization="html"
        preset="document"
        toolbarLabels={toolbarLabels}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="Collaborative document"
      />,
    )
    const root = await screen.findByRole('textbox', { name: 'Collaborative document' })
    expect(root.textContent).not.toContain('legacy snapshot')
    fireEvent.focus(root)
    root.innerHTML = '<p>remote owned content</p>'
    fireEvent.input(root, { inputType: 'insertText', data: 'remote owned content' })
    expect(onChange).not.toHaveBeenCalled()
    rerender(
      <RichTextEditorRuntime
        value="<p>late HTTP snapshot</p>"
        onChange={onChange}
        collaboration={adapter}
        minHeight={100}
        serialization="html"
        preset="document"
        toolbarLabels={toolbarLabels}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="Collaborative document"
      />,
    )
    expect(root.textContent).toContain('remote owned content')

    const extensions = createRichTextExtensions({
      preset: 'document', schemaVersion: 1, placeholder: '', collaboration: true,
    })
    expect(extensions.map((extension) => extension.name)).not.toContain('richTextPhysicalControlHistoryShortcuts')
  })
  it('installs authoritative annotations after controlled document hydration without mapping a whole-document deletion', async () => {
    const props = { serialization: 'html' as const, preset: 'document' as const, toolbarLabels,
      documentLabels: RICH_TEXT_DOCUMENT_LABELS_FIXTURE, minHeight: 100, ariaLabel: 'Hydration document' }
    const { rerender } = render(<RichTextEditorRuntime {...props} value="<p>Loading</p>" />)
    await screen.findByRole('textbox', { name: 'Hydration document' })
    rerender(<RichTextEditorRuntime {...props}
      value="<h2>Alpha block</h2><p>Alpha annotation text.</p><h2>Beta block</h2><p>Beta body.</p>"
      currentRevision={4} commentAnchors={[{ id: 5, revision: 4, status: 'active', label: 'test',
        selection: { from: 20, to: 30, quote: 'annotation' } }]} />)
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Hydration document' })
      .querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('annotation'))
  })
  it('moves a block using the upstream drag handle and native drop, saves and undoes with its comment', async () => {
    const onUpdate = vi.fn()
    const editor = new Editor({
      extensions: createRichTextExtensions({ preset: 'document', schemaVersion: 1, placeholder: '' }),
      content: '<h2>Alpha</h2><p>Beta</p>', onUpdate,
    })
    updateRichTextCommentAnchors(editor, { currentRevision: 1, anchors: [{ id: 1, revision: 1, status: 'active', label: 'test', selection: { from: 1, to: 4, quote: 'Alp' } }] })
    const { unmount } = render(<><EditorContent editor={editor} /><RichTextBlockDragHandle editor={editor} labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.blockMenu} /></>)
    const heading = editor.view.dom.querySelector('h2')!
    await hoverNativeBlock(heading)
    const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(40, 40, 400, 28))
    const coords = vi.spyOn(editor.view, 'posAtCoords').mockReturnValue({ pos: 1, inside: 0 })
    const previous = Object.getOwnPropertyDescriptor(document, 'elementsFromPoint')
    Object.defineProperty(document, 'elementsFromPoint', { configurable: true, value: () => [heading] })
    const transfer = { files: [], types: [], clearData: vi.fn(), setDragImage: vi.fn(), setData: vi.fn(), getData: () => '' }
    const scroll = vi.spyOn(window, 'scrollBy').mockImplementation(() => undefined)
    try {
      const grip = screen.getByRole('button', { name: 'Block actions' }).closest('[draggable="true"]')!
      const start = createEvent.dragStart(grip, { dataTransfer: transfer })
      Object.defineProperties(start, { clientX: { value: 50 }, clientY: { value: 50 } })
      fireEvent(grip, start)
      expect(editor.view.dragging?.slice.content.firstChild?.type.name).toBe('heading')
      coords.mockReturnValue({ pos: editor.state.doc.content.size, inside: -1 })
      fireEvent.drop(editor.view.dom, { dataTransfer: transfer, clientX: 50, clientY: 120 })
      // StarterKit keeps an editable trailing paragraph after a terminal heading.
      expect(editor.getJSON().content?.map((node) => node.type)).toEqual(['paragraph', 'heading', 'paragraph'])
      expect(editor.getJSON().content?.[2].content).toBeUndefined()
      expect(onUpdate).toHaveBeenCalled()
      expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('Alp')
      expect(editor.commands.undo()).toBe(true)
      expect(editor.getJSON().content?.map((node) => node.type)).toEqual(['heading', 'paragraph'])
      expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('Alp')
      expect(editor.commands.redo()).toBe(true)
      expect(editor.view.dom.querySelector('[data-rich-text-comment-anchor]')?.textContent).toBe('Alp')
    } finally {
      fireEvent.dragEnd(document)
      rect.mockRestore(); coords.mockRestore(); scroll.mockRestore()
      if (previous) Object.defineProperty(document, 'elementsFromPoint', previous)
      else Reflect.deleteProperty(document, 'elementsFromPoint')
      unmount(); editor.destroy()
    }
  })
  it('does not expose a drag grip or accept drops for a read-only document', async () => {
    const onChange = vi.fn()
    render(<RichTextEditorRuntime value='<p>Read only</p>' serialization="html" preset="document"
      readOnly onChange={onChange} minHeight={100} toolbarLabels={toolbarLabels}
      documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE} ariaLabel="Read only document" />)
    const root = await screen.findByRole('textbox', { name: 'Read only document' })
    expect(screen.queryByRole('button', { name: 'Block actions', hidden: true })).toBeNull()
    fireEvent.drop(root, { dataTransfer: { files: [], getData: () => '<p>Injected</p>' } })
    expect(root.textContent).toBe('Read only')
    expect(onChange).not.toHaveBeenCalled()
  })
  it('keeps Shift+Enter as a serialized line break and reserves plain Enter for submit', async () => {
    const onChange = vi.fn()
    const onSubmit = vi.fn()
    render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [{ type: 'paragraph', content: [{ type: 'text', text: '第一行' }] }],
        })}
        onChange={onChange}
        onSubmit={onSubmit}
        minHeight={80}
        serialization="json"
        ariaLabel="Multiline comment"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Multiline comment' })
    fireEvent.focus(editor)

    const newline = createEvent.keyDown(editor, { key: 'Enter', shiftKey: true })
    fireEvent(editor, newline)
    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(JSON.parse(onChange.mock.calls.at(-1)?.[0] as string)).toMatchObject({
      type: 'doc',
      content: expect.arrayContaining([
        expect.objectContaining({
          type: 'paragraph',
          content: expect.arrayContaining([{ type: 'hardBreak' }]),
        }),
      ]),
    })
    expect(onSubmit).not.toHaveBeenCalled()

    const submit = createEvent.keyDown(editor, { key: 'Enter' })
    fireEvent(editor, submit)
    expect(submit.defaultPrevented).toBe(true)
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('repeats outline navigation when a new request targets the same position', async () => {
    const baseProps = {
      value: JSON.stringify({
        type: 'doc',
        content: [{
          type: 'heading',
          attrs: { level: 2 },
          content: [{ type: 'text', text: 'Architecture' }],
        }],
      }),
      onChange: vi.fn(),
      minHeight: 120,
      serialization: 'json' as const,
      preset: 'document' as const,
      documentSchemaVersion: 1 as const,
      documentLabels: RICH_TEXT_DOCUMENT_LABELS_FIXTURE,
      ariaLabel: 'ADR body',
      toolbarLabels,
    }
    const { rerender } = render(<RichTextEditorRuntime {...baseProps} />)
    const heading = await screen.findByRole('heading', { name: 'Architecture' })
    const scrollIntoView = vi.fn()
    Object.defineProperty(heading, 'scrollIntoView', { configurable: true, value: scrollIntoView })

    rerender(
      <RichTextEditorRuntime
        {...baseProps}
        activeDocumentPosition={0}
        documentPositionRequestId={1}
      />,
    )
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(1))

    rerender(
      <RichTextEditorRuntime
        {...baseProps}
        activeDocumentPosition={0}
        documentPositionRequestId={2}
      />,
    )
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(2))
  })

  it('handles physical Ctrl+Z undo and Ctrl+Shift+Z or Ctrl+Y redo through the editor DOM', () => {
    const editor = new Editor({
      extensions: createRichTextExtensions({
        preset: 'document',
        schemaVersion: 1,
        placeholder: '',
      }),
      content: '<p>Decision</p>',
    })
    editor.commands.setTextSelection(9)
    editor.commands.insertContent(' updated')
    expect(editor.getText()).toBe('Decision updated')

    const undo = createEvent.keyDown(editor.view.dom, {
      key: 'z',
      code: 'KeyZ',
      ctrlKey: true,
    })
    fireEvent(editor.view.dom, undo)
    expect(undo.defaultPrevented).toBe(true)
    expect(editor.getText()).toBe('Decision')

    const redoWithShift = createEvent.keyDown(editor.view.dom, {
      key: 'Z',
      code: 'KeyZ',
      ctrlKey: true,
      shiftKey: true,
    })
    fireEvent(editor.view.dom, redoWithShift)
    expect(redoWithShift.defaultPrevented).toBe(true)
    expect(editor.getText()).toBe('Decision updated')

    fireEvent.keyDown(editor.view.dom, { key: 'z', code: 'KeyZ', ctrlKey: true })
    const redoWithY = createEvent.keyDown(editor.view.dom, {
      key: 'y',
      code: 'KeyY',
      ctrlKey: true,
    })
    fireEvent(editor.view.dom, redoWithY)
    expect(redoWithY.defaultPrevented).toBe(true)
    expect(editor.getText()).toBe('Decision updated')
    expect(editor.extensionManager.extensions.filter(({ name }) => name === 'undoRedo')).toHaveLength(1)
    expect(editor.extensionManager.extensions.filter(
      ({ name }) => name === 'richTextPhysicalControlHistoryShortcuts',
    )).toHaveLength(1)

    editor.destroy()
  })

  it('keeps the shared physical Ctrl undo/redo adapter active for the field preset', async () => {
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value="Decision"
        onChange={onChange}
        minHeight={120}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Description' })
    fireEvent.focus(editor)
    editor.innerHTML = '<p>Decision updated</p>'
    fireEvent.input(editor, { inputType: 'insertText', data: ' updated' })
    await waitFor(() => expect(editor.textContent).toBe('Decision updated'))

    const undo = createEvent.keyDown(editor, {
      key: 'z',
      code: 'KeyZ',
      ctrlKey: true,
    })
    fireEvent(editor, undo)
    expect(undo.defaultPrevented).toBe(true)
    await waitFor(() => expect(editor.textContent).toBe('Decision'))

    const redo = createEvent.keyDown(editor, {
      key: 'y',
      code: 'KeyY',
      ctrlKey: true,
    })
    fireEvent(editor, redo)
    expect(redo.defaultPrevented).toBe(true)
    await waitFor(() => expect(editor.textContent).toBe('Decision updated'))
  })

  it.each([
    ['legacy HTML', '<p><strong>Legacy HTML</strong></p>', 'Legacy HTML'],
    ['legacy plain text', 'First line\nSecond line', 'First line'],
    ['Tiptap JSON', JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'JSON content' }] }],
    }), 'JSON content'],
  ])('renders %s without emitting a rewrite on mount', async (_name, value, expected) => {
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value={value}
        onChange={onChange}
        minHeight={120}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )

    expect(await screen.findByText(expected)).not.toBeNull()
    expect(screen.getByRole('textbox', { name: 'Description' })).not.toBeNull()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('emits canonical JSON after an actual focused edit', async () => {
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value="Legacy plain text"
        onChange={onChange}
        minHeight={120}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Description' })
    fireEvent.focus(editor)
    editor.innerHTML = '<p>Updated text</p>'
    fireEvent.input(editor, { inputType: 'insertText', data: 'Updated text' })

    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(JSON.parse(onChange.mock.calls.at(-1)?.[0] as string)).toMatchObject({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Updated text' }] }],
    })
  })

  it('focuses the editor once for every explicit focus request', async () => {
    const { rerender } = render(
      <RichTextEditorRuntime
        value=""
        onChange={vi.fn()}
        minHeight={120}
        serialization="json"
        ariaLabel="Comment draft"
        toolbarLabels={toolbarLabels}
        focusRequestId={1}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Comment draft' })
    await waitFor(() => expect(document.activeElement).toBe(editor))

    const outside = document.createElement('button')
    document.body.append(outside)
    outside.focus()
    rerender(
      <RichTextEditorRuntime
        value=""
        onChange={vi.fn()}
        minHeight={120}
        serialization="json"
        ariaLabel="Comment draft"
        toolbarLabels={toolbarLabels}
        focusRequestId={1}
      />,
    )
    expect(document.activeElement).toBe(outside)

    rerender(
      <RichTextEditorRuntime
        value=""
        onChange={vi.fn()}
        minHeight={120}
        serialization="json"
        ariaLabel="Comment draft"
        toolbarLabels={toolbarLabels}
        focusRequestId={2}
      />,
    )
    await waitFor(() => expect(document.activeElement).toBe(editor))
    outside.remove()
  })

  it('extends the editable surface across the configured field height', async () => {
    render(
      <RichTextEditorRuntime
        value=""
        onChange={vi.fn()}
        minHeight={180}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )

    const editor = await screen.findByRole('textbox', { name: 'Description' })
    expect(editor.parentElement?.classList.contains('[&_.tiptap]:min-h-[inherit]')).toBe(true)
    expect(editor.parentElement?.classList.contains('min-h-(--rich-text-min-height)')).toBe(true)
    expect(editor.parentElement?.style.getPropertyValue('--rich-text-min-height')).toBe('180px')
  })

  it('gives document inline marks explicit visual treatments', async () => {
    render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [{
            type: 'paragraph',
            content: [
              { type: 'text', text: 'Bold', marks: [{ type: 'bold' }] },
              { type: 'text', text: 'Italic', marks: [{ type: 'italic' }] },
              { type: 'text', text: 'Strike', marks: [{ type: 'strike' }] },
              { type: 'text', text: 'Code', marks: [{ type: 'code' }] },
            ],
          }],
        })}
        onChange={vi.fn()}
        minHeight={180}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )

    const editor = await screen.findByRole('textbox', { name: 'ADR body' })
    const content = editor.parentElement
    expect(editor.querySelector('strong')?.textContent).toBe('Bold')
    expect(editor.querySelector('em')?.textContent).toBe('Italic')
    expect(editor.querySelector('s')?.textContent).toBe('Strike')
    expect(editor.querySelector('code')?.textContent).toBe('Code')
    expect(content?.classList.contains('[&_.tiptap_strong]:font-bold')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap_em]:italic')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap_s]:line-through')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap_code]:bg-muted')).toBe(true)
  })

  it('keeps long editable content inside a fixed scroll region', async () => {
    render(
      <RichTextEditorRuntime
        value="Long content"
        onChange={vi.fn()}
        minHeight={120}
        height={320}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )

    const editor = await screen.findByRole('textbox', { name: 'Description' })
    const content = editor.parentElement
    expect(content?.classList.contains('h-(--rich-text-height)')).toBe(true)
    expect(content?.style.getPropertyValue('--rich-text-height')).toBe('320px')
    expect(content?.classList.contains('overflow-y-auto')).toBe(true)
    expect(content?.classList.contains('overscroll-contain')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap]:min-h-full')).toBe(true)
  })

  it('constrains read-only comment images to the thumbnail surface', async () => {
    render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [{ type: 'image', attrs: { src: 'https://cdn.example.test/comment.png' } }],
        })}
        readOnly
        imageDisplay="thumbnail"
        minHeight={0}
        serialization="json"
        ariaLabel="Comment"
        toolbarLabels={toolbarLabels}
      />,
    )

    const editor = await screen.findByRole('textbox', { name: 'Comment' })
    const content = editor.parentElement
    expect(content?.classList.contains('[&_.tiptap_img]:max-h-40')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap_img]:max-w-64')).toBe(true)
    expect(content?.classList.contains('[&_.tiptap_img]:max-h-[32rem]')).toBe(false)
  })

  it('uploads a pasted image and inserts it into the document', async () => {
    const onUploadImage = vi.fn().mockResolvedValue('https://cdn.example.test/shot.png')
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value=""
        onChange={onChange}
        onUploadImage={onUploadImage}
        minHeight={120}
        serialization="html"
        ariaLabel="Opinion"
        toolbarLabels={{ ...toolbarLabels, image: 'Insert image', imageInput: 'Choose image' }}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Opinion' })
    const image = new File(['png'], 'shot.png', { type: 'image/png' })
    fireEvent.paste(editor, {
      clipboardData: {
        files: [image],
        getData: () => '',
        types: [],
      },
    })
    await waitFor(() => expect(onUploadImage).toHaveBeenCalledWith(image))
    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(String(onChange.mock.calls.at(-1)?.[0])).toContain('https://cdn.example.test/shot.png')
  })

  it('uploads pasted files and inserts only returned images into JSON content', async () => {
    const onChange = vi.fn()
    const image = new File(['image'], 'proof.png', { type: 'image/png' })
    const document = new File(['rows'], 'tasks.csv', { type: 'text/csv' })
    const onPasteFiles = vi.fn().mockResolvedValue([
      { src: 'https://cdn.example.test/proof.png', alt: 'proof.png', assetId: 31 },
    ])
    render(
      <RichTextEditorRuntime
        value=""
        onChange={onChange}
        onPasteFiles={onPasteFiles}
        minHeight={120}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Description' })
    fireEvent.focus(editor)
    fireEvent.paste(editor, {
      clipboardData: { files: [image, document], getData: () => '' },
    })

    await waitFor(() => expect(onPasteFiles).toHaveBeenCalledWith([image, document]))
    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(JSON.stringify(JSON.parse(onChange.mock.calls.at(-1)?.[0] as string))).toContain(
      'https://cdn.example.test/proof.png',
    )
    const pastedDocument = JSON.parse(onChange.mock.calls.at(-1)?.[0] as string)
    expect(pastedDocument.content.find((node: { type?: string }) => node.type === 'image')).toMatchObject({
      attrs: { assetId: 31 },
    })
    expect(JSON.stringify(JSON.parse(onChange.mock.calls.at(-1)?.[0] as string))).not.toContain(
      'tasks.csv',
    )
  })

  it.each(['field', 'document'] as const)('syncs pasted images after upload when %s editor cannot regain focus', async (preset) => {
    const onChange = vi.fn()
    let finishUpload!: (images: { src: string; alt: string }[]) => void
    const onPasteFiles = vi.fn(() => new Promise<{ src: string; alt: string }[]>((resolve) => {
      finishUpload = resolve
    }))
    render(<RichTextEditorRuntime
      value="" onChange={onChange} onPasteFiles={onPasteFiles}
      minHeight={120} serialization="json" preset={preset}
      documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
      ariaLabel="Async note" toolbarLabels={toolbarLabels}
    />)
    const editor = await screen.findByRole('textbox', { name: 'Async note' })
    fireEvent.focus(editor)
    fireEvent.paste(editor, {
      clipboardData: { files: [new File(['image'], 'proof.png', { type: 'image/png' })], getData: () => '' },
    })
    await waitFor(() => expect(onPasteFiles).toHaveBeenCalledTimes(1))
    fireEvent.blur(editor)
    // Upload can finish while the tab is inactive or the pending field is disabled.
    const focus = vi.spyOn(EditorView.prototype, 'focus').mockImplementation(() => {})
    try {
      finishUpload([{ src: 'https://cdn.example.test/proof.png', alt: 'proof.png' }])
      await screen.findByRole('img', { name: 'proof.png' })
      await waitFor(() => expect(onChange.mock.calls.at(-1)?.[0]).toContain('https://cdn.example.test/proof.png'))
    } finally {
      focus.mockRestore()
    }
  })

  it('leaves a clipboard payload with only non-image files to the native paste path', async () => {
    const onPasteFiles = vi.fn().mockResolvedValue([])
    render(
      <RichTextEditorRuntime
        value=""
        onChange={vi.fn()}
        onPasteFiles={onPasteFiles}
        minHeight={120}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Description' })
    const attachment = new File(['rows'], 'tasks.csv', { type: 'text/csv' })
    const paste = createEvent.paste(editor, {
      clipboardData: { files: [attachment], getData: () => '', types: ['Files'] },
    })

    fireEvent(editor, paste)

    expect(onPasteFiles).not.toHaveBeenCalled()
    expect(paste.defaultPrevented).toBe(false)
  })

  it('keeps mixed attachment upload while only inserting returned images in the default field preset', async () => {
    const image = new File(['image'], 'proof.png', { type: 'image/png' })
    const attachment = new File(['rows'], 'tasks.csv', { type: 'text/csv' })
    const onPasteFiles = vi.fn().mockResolvedValue([
      { src: 'https://cdn.example.test/proof.png', alt: 'proof.png' },
    ])
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value=""
        onChange={onChange}
        onPasteFiles={onPasteFiles}
        minHeight={120}
        serialization="json"
        ariaLabel="Description"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'Description' })
    fireEvent.focus(editor)
    fireEvent.paste(editor, {
      clipboardData: { files: [image, attachment], getData: () => '' },
    })

    await waitFor(() => expect(onPasteFiles).toHaveBeenCalledWith([image, attachment]))
    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(onChange.mock.calls.at(-1)?.[0]).toContain('https://cdn.example.test/proof.png')
  })

  it('enables the v2 document preset only when explicitly selected', async () => {
    const { container } = render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [
            { type: 'callout', attrs: { tone: 'info' }, content: [{ type: 'text', text: 'Decision' }] },
            { type: 'futureDiagram', attrs: { source: 'a --> b' } },
          ],
        })}
        onChange={vi.fn()}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={2}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )

    expect(await screen.findByText('Decision')).not.toBeNull()
    expect(await screen.findByText('Unsupported block: futureDiagram')).not.toBeNull()
    expect(container.querySelector('[data-preset="document"]')?.getAttribute('data-schema-version'))
      .toBe('2')
    await hoverNativeBlock(screen.getByText('Decision'))
    expect(screen.getByRole('button', { name: 'Block actions' })).not.toBeNull()
    expect(container.querySelector('[data-slot="rich-text-block-highlight"]')).not.toBeNull()
    expect(screen.queryByRole('toolbar')).toBeNull()
  })

  it('opens the typed slash menu and inserts a v2 table by keyboard', async () => {
    const onChange = vi.fn()
    render(
      <RichTextEditorRuntime
        value=""
        onChange={onChange}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={2}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )
    const editor = await screen.findByRole('textbox', { name: 'ADR body' })
    fireEvent.focus(editor)
    editor.innerHTML = '<p>/table</p>'
    fireEvent.input(editor, { inputType: 'insertText', data: '/table' })

    expect(await screen.findByRole('listbox', { name: 'Insert block' })).not.toBeNull()
    expect(screen.getByRole('option', { name: 'Table' })).not.toBeNull()
    fireEvent.keyDown(editor, { key: 'Enter' })

    await waitFor(() => expect(onChange).toHaveBeenCalled())
    expect(JSON.stringify(JSON.parse(onChange.mock.calls.at(-1)?.[0] as string))).toContain('tableHeader')
  })

  it('keeps basic block actions inside the shared document adapter', async () => {
    const onChange = vi.fn()
    const { container } = render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Decision' }] }],
        })}
        onChange={onChange}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )
    await hoverNativeBlock(await screen.findByText('Decision'))
    fireEvent.click(screen.getByRole('button', { name: 'Block actions' }))
    const copy = await screen.findByRole('menuitem', { name: 'Copy block' })
    fireEvent.pointerLeave(container.querySelector('[data-slot="rich-text-editor-runtime"]') as HTMLElement)
    expect(screen.getByRole('menuitem', { name: 'Copy block' })).not.toBeNull()
    fireEvent.click(copy)

    await waitFor(() => expect(onChange).toHaveBeenCalled())
    const document = JSON.parse(onChange.mock.calls.at(-1)?.[0] as string)
    expect(document.content).toHaveLength(2)
    expect(document.content[1]).toMatchObject({
      type: 'paragraph',
      content: [{ type: 'text', text: 'Decision' }],
    })
  })

  it('does not mount block-hover UI while a primary-pointer text selection is in progress', async () => {
    render(
      <RichTextEditorRuntime
        value={JSON.stringify({
          type: 'doc',
          content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Decision' }] }],
        })}
        onChange={vi.fn()}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )
    const text = await screen.findByText('Decision')
    fireEvent.pointerDown(text, { button: 0, pointerId: 12, pointerType: 'mouse' })
    fireEvent.pointerMove(text, { pointerId: 12, pointerType: 'mouse' })
    expect(screen.queryByRole('button', { name: 'Block actions' })).toBeNull()

    fireEvent.pointerUp(window, { button: 0, pointerId: 12, pointerType: 'mouse' })
    await hoverNativeBlock(text)
    expect(screen.getByRole('button', { name: 'Block actions' })).not.toBeNull()
  })

  it('renders and activates a verified current-revision inline comment marker', async () => {
    const onCommentAnchorSelect = vi.fn()
    const content = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Decision' }] }],
    })
    const { rerender } = render(
      <RichTextEditorRuntime
        value={content}
        onChange={vi.fn()}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        currentRevision={12}
        activeCommentAnchorId={31}
        commentAnchors={[{
          id: 31,
          revision: 12,
          status: 'active',
          label: 'Open inline comment',
          selection: { from: 1, to: 9, quote: 'Decision' },
        }]}
        onCommentAnchorSelect={onCommentAnchorSelect}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )

    const marker = await screen.findByRole('button', { name: 'Open inline comment' })
    expect(marker.textContent).toBe('Decision')
    expect(marker.hasAttribute('data-comment-anchor-focused')).toBe(true)
    fireEvent.click(marker)
    expect(onCommentAnchorSelect).toHaveBeenCalledWith(31)

    rerender(
      <RichTextEditorRuntime
        value={content}
        onChange={vi.fn()}
        minHeight={240}
        serialization="json"
        preset="document"
        documentSchemaVersion={1}
        documentLabels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE}
        currentRevision={13}
        activeCommentAnchorId={31}
        commentAnchors={[{
          id: 31,
          revision: 12,
          status: 'active',
          label: 'Open inline comment',
          selection: { from: 1, to: 9, quote: 'Decision' },
        }]}
        onCommentAnchorSelect={onCommentAnchorSelect}
        ariaLabel="ADR body"
        toolbarLabels={toolbarLabels}
      />,
    )
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Open inline comment' }).textContent).toBe('Decision')
    })
  })

  it('shows inline formatting only for a focused non-empty text selection', () => {
    const editor = new Editor({
      extensions: [StarterKit],
      content: '<p>Selectable text</p>',
    })
    const element = document.createElement('div')
    editor.commands.setTextSelection({ from: 1, to: 11 })

    expect(shouldShowRichTextInlineMenu({
      editor,
      element,
      from: 1,
      state: editor.state,
      to: 11,
      view: { ...editor.view, hasFocus: () => true } as Editor['view'],
    })).toBe(true)

    editor.commands.setTextSelection(1)
    expect(shouldShowRichTextInlineMenu({
      editor,
      element,
      from: 1,
      state: editor.state,
      to: 1,
      view: { ...editor.view, hasFocus: () => true } as Editor['view'],
    })).toBe(false)
    editor.destroy()
  })
})
