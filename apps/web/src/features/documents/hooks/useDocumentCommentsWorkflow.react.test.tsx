import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { beforeEach, describe, expect, it, vi, mock, type Mock } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type { DocumentsClient } from '../api'
import type {
  DocumentCapabilities,
  DocumentCommentThreadPage,
  DocumentCommentThreadRecord,
} from '../model'
import {
  filterDocumentCommentsState,
  partitionDocumentCommentThreads,
} from '../model/comment'
import { useDocumentCommentsWorkflow } from './useDocumentCommentsWorkflow'

mock.module('@/shared/query', () => ({
  usePlatformUsersQuery: () => ({
    data: [{ id: 5, username: 'teo', display_name: 'Teo', avatar_url: '' }],
    query: '',
    setQuery: vi.fn(),
    hasMore: false,
    isLoadingMore: false,
    loadMore: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}))

const body = JSON.stringify({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: '同意' }] }] })

const thread: DocumentCommentThreadRecord = {
  id: 31,
  workspaceId: 7,
  documentId: 11,
  anchor: { kind: 'document', state: 'document' },
  status: 'open',
  createdBy: 5,
  createdAt: '2026-09-04T01:00:00Z',
  updatedAt: '2026-09-04T01:00:00Z',
  canResolve: true,
  canReopen: false,
  comments: [{
    id: 41,
    workspaceId: 7,
    documentId: 11,
    threadId: 31,
    authorUserId: 5,
    body,
    mentionedUserIds: [],
    canEdit: true,
    canDelete: true,
    createdAt: '2026-09-04T01:00:00Z',
    updatedAt: '2026-09-04T01:00:00Z',
  }],
}

const inlineThread: DocumentCommentThreadRecord = {
  ...thread,
  id: 32,
  anchor: {
    kind: 'selection_snapshot_v1',
    state: 'anchored',
    documentRevision: 3,
    from: 2,
    to: 4,
    quote: '决策',
  },
  comments: [{ ...thread.comments[0], id: 42, threadId: 32 }],
}

const capabilities: DocumentCapabilities = {
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
}

function setup(
  nextCapabilities = capabilities,
  listResult: DocumentCommentThreadPage | Error = { items: [thread], total: 1 },
) {
  const listCommentThreads = vi.fn<DocumentsClient['listCommentThreads']>()
  if (listResult instanceof Error) {
    listCommentThreads.mockRejectedValue(listResult)
  } else {
    listCommentThreads.mockResolvedValue(listResult)
  }
  const client = {
    listCommentThreads,
    createCommentThread: vi.fn<DocumentsClient['createCommentThread']>().mockResolvedValue(thread),
    replyComment: vi.fn<DocumentsClient['replyComment']>(),
    editComment: vi.fn<DocumentsClient['editComment']>(),
    deleteComment: vi.fn<DocumentsClient['deleteComment']>(),
    resolveCommentThread: vi.fn<DocumentsClient['resolveCommentThread']>(),
    reopenCommentThread: vi.fn<DocumentsClient['reopenCommentThread']>(),
  } as unknown as DocumentsClient
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useDocumentCommentsWorkflow({
    environment: { client, workspaceId: 7, sessionRevision: 5 },
    documentId: 11,
    documentCapabilities: nextCapabilities,
    actor: { id: 5, name: 'Teo', username: 'teo' },
  }), { wrapper })
  return { ...hook, client, queryClient }
}

describe('document comments workflow', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 5 })
  })

  it('fails closed before the server grants comment view access', () => {
    const hook = setup({ ...capabilities, canViewComments: false })

    expect(hook.result.current.state).toEqual({ status: 'hidden' })
    expect(hook.client.listCommentThreads).not.toHaveBeenCalled()
    hook.unmount()
    hook.queryClient.clear()
  })

  it('shows historical admin authors without adding them to mention candidates', async () => {
    const historical = { ...thread, comments: [{ ...thread.comments[0], authorUserId: 109,
      author: { id: 109, name: 'Administrator' }, canEdit: false, canDelete: false }] }
    const hook = setup(capabilities, { items: [historical], total: 1 })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    expect(hook.result.current.state).toMatchObject({ threads: [{ comments: [{
      author: { id: 109, name: 'Administrator' }, canEdit: false, canDelete: false,
    }] }] })
    expect(hook.result.current.composer.mentions?.some((user) => user.id === 109)).toBe(false)
    hook.unmount()
    hook.queryClient.clear()
  })

  it('maps server threads and clears the selected anchor after creation', async () => {
    const hook = setup()
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    expect(hook.result.current.state).toMatchObject({
      threads: [{ comments: [{ author: { id: 5, name: 'Teo' }, body }] }],
    })

    const anchor = {
      kind: 'selection_snapshot_v1' as const,
      state: 'anchored' as const,
      documentRevision: 3,
      from: 2,
      to: 4,
      quote: '决策',
    }
    act(() => hook.result.current.control.selectAnchor(anchor))
    expect(hook.result.current.composer.anchor).toEqual(anchor)

    await act(async () => {
      await expect(hook.result.current.control.createThread({ anchor, body })).resolves.toBe(true)
    })
    expect(hook.client.createCommentThread).toHaveBeenCalledWith(11, { anchor, body })
    expect(hook.result.current.composer.anchor).toBeUndefined()
    hook.unmount()
    hook.queryClient.clear()
  })

  it('maps empty and failed comment pages to distinct view states', async () => {
    const empty = setup(capabilities, { items: [], total: 0 })
    await waitFor(() => expect(empty.result.current.state).toEqual({ status: 'empty' }))
    empty.unmount()
    empty.queryClient.clear()

    const failed = setup(capabilities, new Error('comment api unavailable'))
    await waitFor(() => expect(failed.result.current.state).toEqual({ status: 'error' }))
    failed.unmount()
    failed.queryClient.clear()
  })

  it('moves an inline thread to resolved immediately while resolve is pending', async () => {
    const hook = setup(capabilities, { items: [thread, inlineThread], total: 2 })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    let finishResolve: ((value: DocumentCommentThreadRecord) => void) | undefined
    ;((hook.client.resolveCommentThread) as unknown as Mock<typeof hook.client.resolveCommentThread>).mockImplementation(() => new Promise((resolve) => {
      finishResolve = resolve
    }))

    let resolving: Promise<boolean> | undefined
    act(() => {
      resolving = hook.result.current.control.resolve(inlineThread.id)
    })
    await waitFor(() => expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      threads: [{ id: thread.id }, { id: inlineThread.id, status: 'resolved' }],
    }))

    finishResolve?.({ ...inlineThread, status: 'resolved', canResolve: false, canReopen: true })
    await act(async () => { await resolving })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('partitions inline and document threads without creating another data source', async () => {
    const hook = setup(capabilities, { items: [thread, inlineThread], total: 2 })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))

    expect(hook.result.current.state).toMatchObject({
      threads: [{ id: 31 }, { id: 32 }],
    })
    if (hook.result.current.state.status !== 'ready') throw new Error('comments not ready')
    const groups = partitionDocumentCommentThreads(hook.result.current.state.threads)
    expect(groups.documentThreads.map(({ id }) => id)).toEqual([31])
    expect(groups.inlineThreads.map(({ id }) => id)).toEqual([32])
    expect(filterDocumentCommentsState(hook.result.current.state, 'document')).toMatchObject({
      status: 'ready',
      threads: [{ id: 31 }],
    })
    expect(filterDocumentCommentsState(hook.result.current.state, 'selection_snapshot_v1')).toMatchObject({
      status: 'ready',
      threads: [{ id: 32 }],
    })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('passes through non-ready states and maps an empty filtered result to empty', () => {
    const errorState = { status: 'error' as const }
    expect(filterDocumentCommentsState(errorState, 'document')).toBe(errorState)
    expect(filterDocumentCommentsState({
      status: 'ready',
      threads: [{
        id: 31,
        status: 'open',
        anchor: { kind: 'document', state: 'document' },
        comments: [],
        canResolve: true,
        canReopen: false,
      }],
    }, 'selection_snapshot_v1'))
      .toEqual({ status: 'empty' })
  })
})
