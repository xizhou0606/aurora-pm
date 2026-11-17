import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError } from '@/shared/api'
import {
  clearResourceAccessCredential,
  enterResourceAccessEntry,
  expectedResourceAccessId,
  retainResourceAccessGrant,
} from '@/features/resource-access'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import type { DocumentsClient } from '../api'
import type { DocumentCommentThreadRecord, DocumentRecord } from '../model'
import { useDocumentCommentThreadsQuery, useDocumentQuery } from './useDocumentQueries'

function wrapperWith(client: QueryClient) {
  return ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

function record(workspaceId: number): DocumentRecord {
  return {
    id: 11,
    workspaceId,
    nodeId: 19,
    content: '{"type":"doc"}',
    schemaVersion: 1,
    revision: 1,
    contentHash: 'hash',
    plainText: '',
    ownerUserId: 3,
    createdBy: 3,
    updatedBy: 3,
    createdAt: '2026-09-03T01:00:00Z',
    updatedAt: '2026-09-03T01:00:00Z',
    allowedActions: [PM_ACTION.DocumentsContentView],
  }
}

function commentThread(workspaceId: number, documentId = 11): DocumentCommentThreadRecord {
  return {
    id: 31,
    workspaceId,
    documentId,
    anchor: { kind: 'document', state: 'document' },
    status: 'open',
    createdBy: 3,
    createdAt: '2026-09-03T01:00:00Z',
    updatedAt: '2026-09-03T01:10:00Z',
    comments: [{
      id: 41,
      workspaceId,
      documentId,
      threadId: 31,
      authorUserId: 3,
      body: '{"type":"doc"}',
      mentionedUserIds: [],
      canEdit: true,
      canDelete: true,
      createdAt: '2026-09-03T01:00:00Z',
      updatedAt: '2026-09-03T01:10:00Z',
    }],
    canResolve: true,
    canReopen: false,
  }
}

describe('document queries', () => {
  it.each([401, 403, 404])('settles HTTP %s immediately without QueryClient default retries', async (status) => {
    const error = new PMSafeApiError('DOCUMENT_FORBIDDEN', status)
    const document = vi.fn().mockRejectedValue(error)
    const queryClient = new QueryClient()
    const hook = renderHook(() => useDocumentQuery({
      client: { document } as unknown as DocumentsClient, workspaceId: 7, sessionRevision: 3,
    }, 11), { wrapper: wrapperWith(queryClient) })
    await waitFor(() => expect(hook.result.current.status).toBe('error'))
    expect(hook.result.current).toMatchObject({ error })
    expect(document).toHaveBeenCalledTimes(1)
    hook.unmount()
    queryClient.clear()
  })
  afterEach(() => {
    clearResourceAccessCredential()
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 0 })
  })

  it('fails closed when the response belongs to another workspace', async () => {
    const document = vi.fn().mockResolvedValue(record(8))
    const client = { document } as unknown as DocumentsClient
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentQuery({
      client,
      workspaceId: 7,
      sessionRevision: 3,
    }, 11), { wrapper: wrapperWith(queryClient) })

    await waitFor(() => expect(hook.result.current.status).toBe('error'))
    expect(document).toHaveBeenCalledTimes(1)
    hook.unmount()
    queryClient.clear()
  })

  it('does not request a document while the page capability gate is closed', () => {
    const document = vi.fn()
    const client = { document } as unknown as DocumentsClient
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentQuery({
      client,
      workspaceId: 0,
      sessionRevision: 3,
    }, 11), { wrapper: wrapperWith(queryClient) })

    expect(hook.result.current.status).toBe('loading')
    expect(document).not.toHaveBeenCalled()
    hook.unmount()
    queryClient.clear()
  })

  it('fails closed and clears link access when the document projection has another resource id', async () => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    const entry = { workspaceId: 7, sessionRevision: 3, kind: 'document' as const, key: '11' }
    enterResourceAccessEntry(entry)
    retainResourceAccessGrant(entry, {
      resourceId: 'expected-resource', accessSecret: 'access-secret',
      level: 'viewer', expiresAt: '2099-09-09T00:00:00Z',
    })
    const document = vi.fn().mockResolvedValue({ ...record(7), resourceId: 'other-resource' })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentQuery({
      client: { document } as unknown as DocumentsClient,
      workspaceId: 7,
      sessionRevision: 3,
    }, 11), { wrapper: wrapperWith(queryClient) })

    await waitFor(() => expect(hook.result.current.status).toBe('error'))
    expect(expectedResourceAccessId('document')).toBeUndefined()
    hook.unmount()
    queryClient.clear()
  })

  it('returns an empty comment page as ready data', async () => {
    const listCommentThreads = vi.fn().mockResolvedValue({ items: [], total: 0 })
    const client = { listCommentThreads } as unknown as DocumentsClient
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentCommentThreadsQuery({
      client,
      workspaceId: 7,
      sessionRevision: 3,
    }, 11, true), { wrapper: wrapperWith(queryClient) })

    await waitFor(() => expect(hook.result.current.status).toBe('ready'))
    expect(hook.result.current).toMatchObject({ data: { items: [], total: 0 } })
    hook.unmount()
    queryClient.clear()
  })

  it('fails closed for comment threads or comments outside the active scope', async () => {
    const foreignThread = commentThread(8)
    const listCommentThreads = vi.fn().mockResolvedValue({ items: [foreignThread], total: 1 })
    const client = { listCommentThreads } as unknown as DocumentsClient
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentCommentThreadsQuery({
      client,
      workspaceId: 7,
      sessionRevision: 3,
    }, 11, true), { wrapper: wrapperWith(queryClient) })

    await waitFor(() => expect(hook.result.current.status).toBe('error'))
    expect(listCommentThreads).toHaveBeenCalledTimes(1)
    hook.unmount()
    queryClient.clear()
  })

  it('keeps comment requests disabled without server view capability', () => {
    const listCommentThreads = vi.fn()
    const client = { listCommentThreads } as unknown as DocumentsClient
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const hook = renderHook(() => useDocumentCommentThreadsQuery({
      client,
      workspaceId: 7,
      sessionRevision: 3,
    }, 11, false), { wrapper: wrapperWith(queryClient) })

    expect(hook.result.current.status).toBe('disabled')
    expect(listCommentThreads).not.toHaveBeenCalled()
    hook.unmount()
    queryClient.clear()
  })
})
