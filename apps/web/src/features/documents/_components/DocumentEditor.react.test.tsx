import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { DOCUMENT_RICH_TEXT_LABELS, DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { LazyRichTextFieldProps } from '@/shared/ui'
import type { DocumentCommentsWorkflow, DocumentEditorControl, DocumentEditorViewState } from '../hooks'
import { DocumentEditor } from './DocumentEditor'

const probes = (() => ({ richText: vi.fn() }))()

mock.module('@/shared/ui', () => ({
    LazyRichTextField: (props: LazyRichTextFieldProps) => {
      probes.richText(props)
      return <div data-testid="document-rich-text" />
    },
  }))

const control: DocumentEditorControl = {
  changeContent: vi.fn(),
  flush: vi.fn().mockResolvedValue(undefined),
  retrySave: vi.fn(),
  loadLatest: vi.fn().mockResolvedValue(undefined),
  copyLocalDraft: vi.fn().mockResolvedValue(undefined),
  deferConflict: vi.fn(),
  pasteFiles: vi.fn().mockResolvedValue([]),
  resolveImage: vi.fn().mockResolvedValue('blob:test'),
}

beforeEach(() => {
  probes.richText.mockClear()
})

describe('DocumentEditor', () => {
  it('re-derives outline positions after edits and emits a request for every navigation click', () => {
    probes.richText.mockClear()
    const createState = (content: string): DocumentEditorViewState => ({
      status: 'ready',
      documentId: 11,
      content,
      revision: 3,
      schemaVersion: 1,
      updatedAt: '2026-09-03T01:00:00Z',
      refresh: 'idle',
      saveStatus: 'saved',
      copyStatus: 'idle',
      pasteStatus: 'idle',
      capabilities: {
        canView: true,
        canExport: false,
        canUpdate: true,
        canViewAssets: false,
        canUploadAssets: false,
        canDownloadAssets: false,
        canDeleteAssets: false,
        canViewComments: false,
        canCreateComments: false,
        canReplyComments: false,
      },
    })
    const initialContent = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph' },
        { type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: '目标标题' }] },
      ],
    })
    const editedContent = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph' },
        { type: 'paragraph' },
        { type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: '目标标题' }] },
      ],
    })
    const { container, rerender } = render(
      <DocumentEditor state={createState(initialContent)} control={control} />,
    )

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.outline.navigate('目标标题') }))
    expect(probes.richText.mock.calls.at(-1)?.[0]).toMatchObject({
      activeDocumentPosition: 2,
      activeDocumentPositionOptions: { scrollBehavior: 'instant' },
      documentPositionRequestId: 1,
    })

    rerender(<DocumentEditor state={createState(editedContent)} control={control} />)
    const target = screen.getByRole('button', { name: DOCUMENTS_COPY.outline.navigate('目标标题') })
    fireEvent.click(target)
    expect(probes.richText.mock.calls.at(-1)?.[0]).toMatchObject({
      activeDocumentPosition: 4,
      documentPositionRequestId: 2,
    })
    fireEvent.click(target)
    expect(probes.richText.mock.calls.at(-1)?.[0]).toMatchObject({
      activeDocumentPosition: 4,
      documentPositionRequestId: 3,
    })

    expect(container.querySelector('article')?.classList.contains('overflow-clip')).toBe(true)
    expect(container.querySelector('[data-slot="document-scroll"]')?.classList.contains('overflow-y-auto')).toBe(true)
    expect(container.querySelector('aside[aria-label="文章大纲"]')?.classList.contains('overflow-hidden')).toBe(true)
  })

  it('wires the server schema and typed document labels into the shared editor', () => {
    const state: DocumentEditorViewState = {
      status: 'ready',
      documentId: 11,
      content: '{"type":"doc","content":[{"type":"paragraph"}]}',
      revision: 3,
      schemaVersion: 1,
      updatedAt: '2026-09-03T01:00:00Z',
      refresh: 'idle',
      saveStatus: 'saved',
      copyStatus: 'idle',
      pasteStatus: 'idle',
      capabilities: {
        canView: true,
        canExport: false,
        canUpdate: true,
        canViewAssets: false,
        canUploadAssets: true,
        canDownloadAssets: true,
      canDeleteAssets: false,
      canViewComments: false,
      canCreateComments: false,
      canReplyComments: false,
      },
    }

    render(<DocumentEditor state={state} control={control} />)

    expect(probes.richText).toHaveBeenCalledTimes(1)
    expect(probes.richText.mock.calls[0][0]).toMatchObject({
      preset: 'document',
      serialization: 'json',
      documentSchemaVersion: 1,
      documentLabels: DOCUMENT_RICH_TEXT_LABELS,
      readOnly: false,
      onPasteFiles: control.pasteFiles,
      onResolveImage: control.resolveImage,
    })
  })

  it('preserves inline title cancel, Enter, and blur editing interactions', async () => {
    const rename = vi.fn().mockResolvedValue(true)
    const share = vi.fn()
    const state: DocumentEditorViewState = {
      status: 'ready',
      documentId: 11,
      content: '{"type":"doc","content":[{"type":"paragraph"}]}',
      revision: 3,
      schemaVersion: 1,
      updatedAt: new Date().toISOString(),
      refresh: 'idle',
      saveStatus: 'saved',
      copyStatus: 'idle',
      pasteStatus: 'idle',
      capabilities: {
        canView: true,
        canExport: false,
        canUpdate: true,
        canViewAssets: false,
        canUploadAssets: false,
        canDownloadAssets: false,
      canDeleteAssets: false,
      canViewComments: false,
      canCreateComments: false,
      canReplyComments: false,
      },
    }

    render(
      <DocumentEditor
        state={state}
        control={control}
        node={{
          id: 19,
          workspaceId: 7,
          spaceId: 2,
          type: 'document',
          title: '旧标题',
          sortOrder: 1024,
          documentId: 11,
          systemManaged: false,
          favorited: false,
          createdBy: 109,
          creator: { id: 109, name: 'Administrator' },
          updatedBy: 3,
          createdAt: '2026-09-03T01:00:00Z',
          updatedAt: '2026-09-03T01:00:00Z',
        }}
        actor={{ id: 3, name: '铁奥' }}
        canRenameTitle
        onRenameTitle={rename}
        onShare={share}
      />,
    )

    const title = screen.getByRole('textbox', { name: DOCUMENTS_COPY.editor.titleLabel })
    fireEvent.focus(title)
    fireEvent.change(title, { target: { value: '取消的标题' } })
    fireEvent.keyDown(title, { key: 'Escape' })
    expect((title as HTMLInputElement).value).toBe('旧标题')
    expect(rename).not.toHaveBeenCalled()

    fireEvent.focus(title)
    fireEvent.change(title, { target: { value: '新的 ADR 标题' } })
    fireEvent.keyDown(title, { key: 'Enter' })

    await waitFor(() => expect(rename).toHaveBeenCalledWith('新的 ADR 标题'))
    await waitFor(() => expect((title as HTMLInputElement).readOnly).toBe(false))
    fireEvent.change(title, { target: { value: '失焦保存标题' } })
    fireEvent.blur(title)
    await waitFor(() => expect(rename).toHaveBeenLastCalledWith('失焦保存标题'))
    expect(screen.getByLabelText('Administrator')).not.toBeNull()
    expect(screen.queryByText('成员 #109')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.tree.share }))
    expect(share).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.tree.copyLink })).toBeNull()
    expect(screen.getByText(DOCUMENTS_COPY.editor.modifiedToday)).not.toBeNull()
  })

  it('keeps selection comments in the side panel and document comments at the article bottom', async () => {
    probes.richText.mockClear()
    const state: DocumentEditorViewState = {
      status: 'ready',
      documentId: 11,
      content: JSON.stringify({
        type: 'doc',
        content: [{ type: 'paragraph', content: [{ type: 'text', text: '需要评审的正文' }] }],
      }),
      revision: 7,
      schemaVersion: 1,
      updatedAt: '2026-09-03T01:00:00Z',
      refresh: 'idle',
      saveStatus: 'saved',
      copyStatus: 'idle',
      pasteStatus: 'idle',
      capabilities: {
        canView: true,
        canExport: false,
        canUpdate: true,
        canViewAssets: false,
        canUploadAssets: false,
        canDownloadAssets: false,
        canDeleteAssets: false,
        canViewComments: true,
        canCreateComments: true,
        canReplyComments: true,
      },
    }
    const commentsControl: DocumentCommentsWorkflow['control'] = {
      retry: vi.fn(),
      selectAnchor: vi.fn(),
      createThread: vi.fn().mockResolvedValue(true),
      reply: vi.fn().mockResolvedValue(true),
      edit: vi.fn().mockResolvedValue(true),
      remove: vi.fn().mockResolvedValue(true),
      resolve: vi.fn().mockResolvedValue(true),
      reopen: vi.fn().mockResolvedValue(true),
    }
    const inlineThread = {
      id: 21,
      status: 'open' as const,
      anchor: {
        kind: 'selection_snapshot_v1' as const,
        state: 'anchored' as const,
        documentRevision: 7,
        from: 1,
        to: 7,
        quote: '需要评审',
      },
      comments: [{
        id: 101,
        author: { id: 3, name: '行内作者' },
        body: '{"type":"doc","content":[]}',
        canEdit: false,
        canDelete: false,
      }],
      canResolve: true,
      canReopen: false,
    }
    const documentThread = {
      id: 22,
      status: 'open' as const,
      anchor: { kind: 'document' as const, state: 'document' as const },
      comments: [{
        id: 102,
        author: { id: 4, name: '全文作者' },
        body: '{"type":"doc","content":[]}',
        canEdit: false,
        canDelete: false,
      }],
      canResolve: true,
      canReopen: false,
    }
    const resolvedInlineThread = {
      ...inlineThread,
      id: 23,
      status: 'resolved' as const,
      comments: [{
        ...inlineThread.comments[0],
        id: 103,
        author: { id: 5, name: '已解决作者' },
      }],
      canResolve: false,
      canReopen: true,
    }
    const comments: Omit<DocumentCommentsWorkflow, 'control'> = {
      state: { status: 'ready', threads: [inlineThread, resolvedInlineThread, documentThread] },
      mutation: { status: 'idle' },
      capabilities: { canCreate: true, canReply: true },
      composer: { author: { id: 9, name: '当前成员' } },
    }

    const view = render(
      <DocumentEditor
        state={state}
        control={control}
        comments={comments}
        commentsControl={commentsControl}
      />,
    )

    const editorProps = probes.richText.mock.calls
      .map(([props]) => props)
      .find((props) => props.preset === 'document')
    expect(editorProps).toMatchObject({
      currentRevision: 7,
      commentAnchors: [{ id: 21, revision: 7, status: 'active' }],
    })
    expect(editorProps?.commentAnchors).toHaveLength(1)
    expect(screen.getByText(DOCUMENTS_COPY.comments.documentTitle)).not.toBeNull()
    expect(screen.getByText('全文作者')).not.toBeNull()
    expect(screen.queryByText('行内作者')).toBeNull()

    act(() => editorProps?.onCommentAnchorSelect?.(21))
    expect(await screen.findByText(DOCUMENTS_COPY.comments.inlineTitle)).not.toBeNull()
    expect(screen.queryByText(DOCUMENTS_COPY.comments.resolved)).toBeNull()
    expect(screen.getByText('行内作者')).not.toBeNull()

    view.rerender(
      <DocumentEditor
        state={state}
        control={control}
        comments={comments}
        commentsControl={commentsControl}
        commentThreadId={23}
      />,
    )
    expect(document.querySelector('[data-thread-id="23"]')).toBeNull()
  })
})
