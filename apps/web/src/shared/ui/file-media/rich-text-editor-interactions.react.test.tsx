import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { act, createEvent, fireEvent, render, screen } from '@testing-library/react'
import { forwardRef, type ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { RichTextInlineBubbleMenu } from './rich-text-editor-interactions'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE } from './rich-text.react-test-fixtures'

let inlineMenuShouldShow: ((input: Parameters<typeof import('./rich-text-editor-interactions').shouldShowRichTextInlineMenu>[0]) => boolean) | undefined

vi.mock('@tiptap/react/menus', () => ({
  BubbleMenu: forwardRef<HTMLDivElement, {
    children: ReactNode
    role?: string
    'aria-label'?: string
    shouldShow?: typeof inlineMenuShouldShow
  }>(function BubbleMenu({
    children,
    role,
    'aria-label': ariaLabel,
    shouldShow,
  }, ref) {
    inlineMenuShouldShow = shouldShow
    return <div ref={ref} role={role} aria-label={ariaLabel}>{children}</div>
  }),
}))

const editors: Editor[] = []

function createEditor() {
  const editor = new Editor({
    extensions: [StarterKit],
    content: '<p>format me</p>',
  })
  editors.push(editor)
  editor.commands.setTextSelection({ from: 1, to: 10 })
  return editor
}

afterEach(() => {
  for (const editor of editors.splice(0)) editor.destroy()
  inlineMenuShouldShow = undefined
})

describe('RichTextInlineBubbleMenu', () => {
  it('preserves the selection on pointer interaction and applies every inline mark', () => {
    const editor = createEditor()
    render(
      <RichTextInlineBubbleMenu
        editor={editor}
        labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.bubbleMenu}
      />,
    )

    expect(screen.getByRole('toolbar', { name: 'Inline formatting' })).not.toBeNull()
    for (const [label, mark] of [
      ['Bold', 'bold'],
      ['Italic', 'italic'],
      ['Strike', 'strike'],
      ['Code', 'code'],
    ] as const) {
      editor.commands.setContent('<p>format me</p>')
      editor.commands.setTextSelection({ from: 1, to: 10 })
      const button = screen.getByRole('button', { name: label })
      const mouseDown = createEvent.mouseDown(button)
      fireEvent(button, mouseDown)
      expect(mouseDown.defaultPrevented).toBe(true)
      fireEvent.click(button)
      expect(editor.state.selection).toMatchObject({ from: 1, to: 10 })
      expect(editor.getJSON()).toMatchObject({
        content: [{
          type: 'paragraph',
          content: [{
            type: 'text',
            text: 'format me',
            marks: expect.arrayContaining([{ type: mark }]),
          }],
        }],
      })
    }
  })

  it('emits a same-block selection snapshot for document comments', () => {
    const editor = createEditor()
    const onCommentSelection = vi.fn()
    editor.commands.setTextSelection({ from: 1, to: 7 })
    render(
      <RichTextInlineBubbleMenu
        editor={editor}
        labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.bubbleMenu}
        onCommentSelection={onCommentSelection}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))
    expect(onCommentSelection).toHaveBeenCalledWith({
      from: 1,
      to: 7,
      quote: 'format',
      suffix: ' me',
    })
  })

  it('reveals the comment action when a plain-text caret becomes a short selection', async () => {
    const editor = createEditor()
    const onCommentSelection = vi.fn()
    editor.commands.setTextSelection(2)
    render(
      <RichTextInlineBubbleMenu
        editor={editor}
        labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.bubbleMenu}
        onCommentSelection={onCommentSelection}
      />,
    )

    expect(screen.queryByRole('button', { name: 'Comment' })).toBeNull()
    await act(async () => {
      editor.commands.setTextSelection({ from: 2, to: 4 })
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Comment' }))
    expect(onCommentSelection).toHaveBeenCalledWith(expect.objectContaining({
      from: 2,
      to: 4,
      quote: 'or',
    }))
  })

  it('keeps the inline menu hidden until a primary-pointer text selection finishes', () => {
    const editor = createEditor()
    const dispatch = vi.spyOn(editor.view, 'dispatch')
    render(
      <RichTextInlineBubbleMenu
        editor={editor}
        labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.bubbleMenu}
      />,
    )
    const shouldShow = inlineMenuShouldShow
    expect(shouldShow).toBeTypeOf('function')
    const input = {
      editor,
      element: document.createElement('div'),
      from: 1,
      state: editor.state,
      to: 10,
      view: { ...editor.view, hasFocus: () => true } as Editor['view'],
    }

    const pointerDown = createEvent.pointerDown(editor.view.dom, {
      button: 0,
      pointerId: 7,
      pointerType: 'mouse',
    })
    fireEvent(editor.view.dom, pointerDown)

    expect(pointerDown.defaultPrevented).toBe(false)
    expect(dispatch).not.toHaveBeenCalled()
    expect(screen.getByRole('toolbar', { name: 'Inline formatting' }).classList.contains('invisible')).toBe(true)
    expect(shouldShow?.(input)).toBe(false)

    fireEvent.pointerUp(window, { button: 0, pointerId: 7, pointerType: 'mouse' })

    expect(shouldShow?.(input)).toBe(true)
    expect(editor.state.selection).toMatchObject({ from: 1, to: 10 })
  })

  it('allows a primary-pointer click to collapse the caret without restoring the old selection', () => {
    const editor = createEditor()
    render(
      <RichTextInlineBubbleMenu
        editor={editor}
        labels={RICH_TEXT_DOCUMENT_LABELS_FIXTURE.bubbleMenu}
      />,
    )
    const shouldShow = inlineMenuShouldShow

    fireEvent.pointerDown(editor.view.dom, {
      button: 0,
      pointerId: 9,
      pointerType: 'mouse',
    })
    editor.commands.setTextSelection(4)
    fireEvent.pointerUp(window, { button: 0, pointerId: 9, pointerType: 'mouse' })

    expect(editor.state.selection).toMatchObject({ from: 4, to: 4 })
    expect(shouldShow?.({
      editor,
      element: document.createElement('div'),
      from: 4,
      state: editor.state,
      to: 4,
      view: { ...editor.view, hasFocus: () => true } as Editor['view'],
    })).toBe(false)
  })
})
