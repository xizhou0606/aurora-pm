import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY, copy } from '@/shared/copy'
import type { DetailResourceEntry, UserOptionsState } from '../../model/types'
import { CommentSection, resolveActiveMention } from './CommentSection'

const users: UserOptionsState = {
  status: 'ready',
  data: [
    { id: 101, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' },
    { id: 102, label: 'Alice', username: 'alice' },
  ],
}

const entry: DetailResourceEntry = {
  id: 'comments',
  title: WORK_ITEMS_COPY.detail.comments,
  emptyLabel: WORK_ITEMS_COPY.detail.noComments,
  state: { status: 'ready', data: [], refresh: 'idle' },
}

function renderComments(overrides: Partial<Parameters<typeof CommentSection>[0]> = {}) {
  const onComment = vi.fn().mockResolvedValue(true)
  const onUploadImage = vi.fn().mockResolvedValue('https://cdn.example.test/evidence.png')
  render(
    <CommentSection
      taskId={31}
      entry={entry}
      canComment
      canAttach
      canUploadBinary
      users={users}
      currentUserId={101}
      mutation={{ status: 'idle' }}
      onComment={onComment}
      onUploadImage={onUploadImage}
      onDownload={vi.fn().mockResolvedValue(null)}
      {...overrides}
    />,
  )
  return { onComment, onUploadImage }
}

afterEach(() => {
  vi.restoreAllMocks()
  Reflect.deleteProperty(URL, 'createObjectURL')
  Reflect.deleteProperty(URL, 'revokeObjectURL')
  Reflect.deleteProperty(Range.prototype, 'getClientRects')
  Reflect.deleteProperty(Range.prototype, 'getBoundingClientRect')
})

describe('work-item compact comment compose', () => {
  it('detects only the active @ token at the caret', () => {
    expect(resolveActiveMention('hello @te', 9)).toEqual({ start: 6, end: 9, query: 'te' })
    expect(resolveActiveMention('hello@example.test', 18)).toBeNull()
    expect(resolveActiveMention('hello @te done', 14)).toBeNull()
  })

  it('stores mention identity in rich text while rendering the member display name', async () => {
    vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue({
      length: 0,
      item: () => null,
      [Symbol.iterator]: function* iterator() {},
    } as DOMRectList)
    Object.defineProperty(Range.prototype, 'getClientRects', {
      configurable: true,
      value: () => ({
        length: 0,
        item: () => null,
        [Symbol.iterator]: function* iterator() {},
      } as DOMRectList),
    })
    Object.defineProperty(Range.prototype, 'getBoundingClientRect', {
      configurable: true,
      value: () => ({
        x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0,
        toJSON: () => ({}),
      } as DOMRect),
    })
    const { onComment } = renderComments()
    const editor = await screen.findByRole('textbox', { name: WORK_ITEMS_COPY.detail.addComment })

    fireEvent.focus(editor)
    editor.innerHTML = '<p>@te</p>'
    fireEvent.input(editor, { inputType: 'insertText', data: '@te' })
    expect(await screen.findByRole('listbox', { name: WORK_ITEMS_COPY.detail.mentionOptions })).not.toBeNull()
    fireEvent.keyDown(editor, { key: 'Enter' })
    expect(await screen.findByText('@Teo')).not.toBeNull()
    expect(onComment).not.toHaveBeenCalled()

    const send = screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.sendComment })
    await waitFor(() => expect(send.hasAttribute('disabled')).toBe(false))
    fireEvent.click(send)
    await waitFor(() => expect(onComment).toHaveBeenCalledTimes(1))
    expect(JSON.parse(onComment.mock.calls[0]?.[0])).toMatchObject({
      type: 'doc',
      content: [{
        type: 'paragraph',
        content: expect.arrayContaining([
          { type: 'mention', attrs: { id: '101', label: 'Teo', username: 'teo' } },
        ]),
      }],
    })
  })

  it('keeps an explicit empty mention state', async () => {
    renderComments()
    const editor = await screen.findByRole('textbox', { name: WORK_ITEMS_COPY.detail.addComment })
    fireEvent.focus(editor)
    editor.innerHTML = '<p>@nobody</p>'
    fireEvent.input(editor, { inputType: 'insertText', data: '@nobody' })
    expect(await screen.findByText(WORK_ITEMS_COPY.detail.noMentionMatches)).not.toBeNull()
  })

  it('previews an uploaded image and appends its attachment reference on send', async () => {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn().mockReturnValue('blob:comment-preview'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
    const { onComment, onUploadImage } = renderComments()
    const file = new File(['png'], 'evidence.png', { type: 'image/png', lastModified: 1 })

    // 编辑器懒加载后工具栏才渲染，图片 input 随工具栏出现
    const editor = await screen.findByRole('textbox', { name: WORK_ITEMS_COPY.detail.addComment })
    fireEvent.focus(editor)
    await screen.findByRole('button', { name: copy.ui.richText.toolbar.image })
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(fileInput, {
      target: { files: [file] },
    })
    await waitFor(() => expect(onUploadImage).toHaveBeenCalledWith(file))
    expect(await screen.findByRole('img', { name: 'evidence.png' })).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.sendComment }))

    await waitFor(() => expect(onComment).toHaveBeenCalledTimes(1))
    const body = JSON.parse(onComment.mock.calls[0]?.[0])
    expect(body).toMatchObject({ type: 'doc' })
    expect(JSON.stringify(body)).toContain('https://cdn.example.test/evidence.png')
  })

  it('renders stored rich text mentions by display name and keeps image preview interactive', async () => {
    const richEntry: DetailResourceEntry = {
      ...entry,
      state: {
        status: 'ready',
        refresh: 'idle',
        data: [{
          id: '1',
          primary: JSON.stringify({
            type: 'doc',
            content: [{
              type: 'paragraph',
              content: [
                { type: 'mention', attrs: { id: '101' } },
                { type: 'text', text: ' 请看' },
              ],
            }, { type: 'image', attrs: { src: 'https://cdn.example.test/proof.png', alt: 'proof.png' } }],
          }),
          secondary: 'Alice',
        }],
      },
    }
    renderComments({ entry: richEntry })

    expect(await screen.findByText('@Teo')).not.toBeNull()
    expect(screen.queryByText('@101')).toBeNull()
    fireEvent.click(await screen.findByRole('img', { name: 'proof.png' }))
    expect(await screen.findByRole('dialog')).not.toBeNull()
  })

  it('shows mutation errors and keeps empty sends disabled', () => {
    renderComments({ mutation: { status: 'error', action: 'comment', message: 'failed' } })
    expect(screen.getByRole('alert').textContent).toContain(WORK_ITEMS_COPY.detail.commentSubmitFailed)
    const send = screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.sendComment })
    expect(send.hasAttribute('disabled')).toBe(true)
  })
})
