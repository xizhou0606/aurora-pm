import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  DocumentRevisionConflictError,
  type DocumentCollaborationEvent,
  type DocumentCollaborationSessionFactory,
  type DocumentsClient,
} from '../api'
import type { DocumentRecord } from '../model'
import { useDocumentEditorWorkflow } from './useDocumentEditorWorkflow'

function documentRecord(overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id: 11,
    workspaceId: 7,
    nodeId: 19,
    content: '{"type":"doc","content":[{"type":"paragraph"}]}',
    schemaVersion: 1,
    revision: 3,
    contentHash: 'hash',
    plainText: '',
    createdBy: 3,
    updatedBy: 3,
    createdAt: '2026-09-03T01:00:00Z',
    updatedAt: '2026-09-03T01:00:00Z',
    allowedActions: [
      PM_ACTION.DocumentsContentView,
      PM_ACTION.DocumentsContentUpdate,
      PM_ACTION.DocumentsAssetsUpload,
      PM_ACTION.DocumentsAssetsDownload,
    ],
    ...overrides,
    ownerUserId: overrides.ownerUserId ?? 3,
  }
}

function setup(
  updateDocument: DocumentsClient['updateDocument'],
  overrides: Partial<DocumentsClient> = {},
  collaboration?: DocumentCollaborationSessionFactory,
) {
  const document = vi.fn<DocumentsClient['document']>().mockResolvedValue(documentRecord())
  const client = {
    document,
    updateDocument,
    ...overrides,
  } as unknown as DocumentsClient
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useDocumentEditorWorkflow({
    client,
    workspaceId: 7,
    sessionRevision: 5,
    collaboration,
  }, 11), { wrapper })
  return { ...hook, client, document, queryClient }
}

describe('document editor workflow', () => {
  it('fails closed for a cutover document when the collaboration session is unavailable', async () => {
    const updateDocument = vi.fn<DocumentsClient['updateDocument']>()
    const collaborationTicket = vi.fn<DocumentsClient['collaborationTicket']>()
    const document = vi.fn<DocumentsClient['document']>().mockResolvedValue(documentRecord({
      resourceId: 'resource-doc-11',
      collaborationCutoverAt: '2026-09-08T08:00:00Z',
    }))
    const hook = setup(updateDocument, { document, collaborationTicket })
    await waitFor(() => expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      saveStatus: 'save_error',
      collaboration: { status: 'unavailable', editable: false },
    }))
    act(() => hook.result.current.control.changeContent('{"type":"doc","content":[]}'))
    await act(async () => { await hook.result.current.control.flush() })
    expect(updateDocument).not.toHaveBeenCalled()
    expect(collaborationTicket).not.toHaveBeenCalled()
    hook.unmount(); hook.queryClient.clear()
  })

  it.each([1, 3])('uses collaboration exclusively for new/cutover revision %i and saves only after durable_commit', async (revision) => {
    let onEvent: ((event: DocumentCollaborationEvent) => void) | undefined
    const destroy = vi.fn()
    const factory: DocumentCollaborationSessionFactory = {
      open: vi.fn((input) => {
        onEvent = input.onEvent
        return { adapter: { key: 'room:11', extensions: [] }, destroy }
      }),
    }
    const updateDocument = vi.fn<DocumentsClient['updateDocument']>()
    const collaborationTicket = vi.fn<DocumentsClient['collaborationTicket']>().mockResolvedValue({
      ticket: 'memory-ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      expiresAt: '2099-09-08T08:00:00Z',
    })
    const document = vi.fn<DocumentsClient['document']>().mockResolvedValue(documentRecord({
      resourceId: 'resource-doc-11',
      collaborationCutoverAt: '2026-09-08T08:00:00Z',
      revision,
    }))
    const hook = setup(updateDocument, { document, collaborationTicket }, factory)
    await waitFor(() => expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      saveStatus: 'saving',
      collaboration: { status: 'connecting', editable: false, adapter: { key: 'room:11' } },
    }))
    expect(collaborationTicket).toHaveBeenCalledWith(
      'resource-doc-11',
      expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/),
      expect.any(AbortSignal),
    )

    act(() => onEvent?.({ kind: 'connected' }))
    expect(hook.result.current.state).toMatchObject({
      saveStatus: 'saved', collaboration: { status: 'connected', editable: true },
    })

    act(() => onEvent?.({ kind: 'local_update' }))
    expect(hook.result.current.state).toMatchObject({ saveStatus: 'saving' })
    act(() => onEvent?.({ kind: 'reconnecting' }))
    expect(hook.result.current.state).toMatchObject({ saveStatus: 'saving', collaboration: { status: 'reconnecting' } })
    act(() => onEvent?.({ kind: 'connected' }))
    expect(hook.result.current.state).toMatchObject({ saveStatus: 'saving', collaboration: { status: 'saving' } })
    act(() => onEvent?.({
      kind: 'durable_commit', durableRevision: 8, storedAt: '2026-09-08T09:00:00Z',
    }))
    expect(hook.result.current.state).toMatchObject({
      saveStatus: 'saved', revision: 8, updatedAt: '2026-09-08T09:00:00Z',
    })
    act(() => onEvent?.({
      kind: 'awareness',
      members: [{ clientId: 5, actorId: 'actor-5', actorName: '服务端成员', hasCursor: true }],
    }))
    expect(hook.result.current.state).toMatchObject({
      collaboration: { members: [{ actorId: 'actor-5', actorName: '服务端成员', hasCursor: true }] },
    })
    expect(factory.open).toHaveBeenCalledTimes(1)
    act(() => hook.result.current.control.changeContent('{"type":"doc","content":[]}'))
    await act(async () => { await hook.result.current.control.flush() })
    expect(updateDocument).not.toHaveBeenCalled()
    act(() => onEvent?.({ kind: 'edit_conflict' }))
    expect(hook.result.current.state).toMatchObject({
      saveStatus: 'save_error', collaboration: { status: 'conflict', editable: false, adapter: { key: 'room:11' } },
    })
    expect(destroy).not.toHaveBeenCalled()
    act(() => onEvent?.({ kind: 'connected' }))
    expect(hook.result.current.state).toMatchObject({ collaboration: { status: 'conflict', editable: false } })
    act(() => onEvent?.({ kind: 'access_revoked' }))
    expect(hook.result.current.state).toMatchObject({
      saveStatus: 'save_error', collaboration: { status: 'forbidden', editable: false },
    })
    hook.unmount()
    expect(destroy).toHaveBeenCalledTimes(1)
    hook.queryClient.clear()
  })

  it('does not report a failed unacknowledged collaboration update as saved', async () => {
    let onEvent: ((event: DocumentCollaborationEvent) => void) | undefined
    const factory: DocumentCollaborationSessionFactory = {
      open: vi.fn((input) => {
        onEvent = input.onEvent
        return { adapter: { key: 'room:11', extensions: [] }, destroy: vi.fn() }
      }),
    }
    const collaborationTicket = vi.fn<DocumentsClient['collaborationTicket']>().mockResolvedValue({
      ticket: 'memory-ticket', websocketUrl: 'wss://collaboration.example.test/socket', expiresAt: '2099-09-08T08:00:00Z',
    })
    const document = vi.fn<DocumentsClient['document']>().mockResolvedValue(documentRecord({
      resourceId: 'resource-doc-11', collaborationCutoverAt: '2026-09-08T08:00:00Z',
    }))
    const hook = setup(vi.fn(), { document, collaborationTicket }, factory)
    await waitFor(() => expect(hook.result.current.state).toMatchObject({ status: 'ready', collaboration: { status: 'connecting' } }))
    act(() => onEvent?.({ kind: 'local_update' }))
    act(() => onEvent?.({ kind: 'error' }))
    expect(hook.result.current.state).toMatchObject({
      saveStatus: 'save_error', collaboration: { status: 'error', editable: false },
    })
    hook.unmount(); hook.queryClient.clear()
  })
  it('upgrades schema only after the user inserts a valid reference, not while opening a document', async () => {
    const nextContent = JSON.stringify({ type: 'doc', content: [{ type: 'bitable', attrs: { schemaVersion: 1, resourceId: 'resource', tableId: 'table', viewId: 'view', height: 420 } }] })
    const updateDocument = vi.fn<DocumentsClient['updateDocument']>().mockResolvedValue(documentRecord({ content: nextContent, schemaVersion: 3, revision: 4 }))
    const hook = setup(updateDocument)
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    expect(hook.result.current.state).toMatchObject({ schemaVersion: 1 })
    expect(updateDocument).not.toHaveBeenCalled()
    vi.useFakeTimers()
    act(() => hook.result.current.control.changeContent(nextContent))
    expect(hook.result.current.state).toMatchObject({ schemaVersion: 3, saveStatus: 'dirty' })
    await act(async () => { vi.advanceTimersByTime(800) })
    expect(updateDocument).toHaveBeenCalledWith(11, expect.objectContaining({ schemaVersion: 3, content: nextContent, expectedRevision: 3 }))
    hook.unmount(); hook.queryClient.clear()
  })
  beforeEach(() => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 5 })
  })

  afterEach(async () => {
    try {
      cleanup()
      // React Query notifications may schedule a React callback after the autosave tick.
      // Drain it before restoring timers so later hooks can render their query result.
      if (vi.isFakeTimers()) await act(async () => { vi.runOnlyPendingTimers() })
    } finally {
      vi.useRealTimers()
    }
  })

  it('autosaves a JSON draft with the current revision without locking later edits', async () => {
    const nextContent = '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"ADR"}]}]}'
    const updateDocument = vi.fn<DocumentsClient['updateDocument']>().mockResolvedValue(
      documentRecord({ content: nextContent, revision: 4, plainText: 'ADR' }),
    )
    const hook = setup(updateDocument)
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    vi.useFakeTimers()

    act(() => hook.result.current.control.changeContent(nextContent))
    expect(hook.result.current.state).toMatchObject({ status: 'ready', saveStatus: 'dirty' })
    await act(async () => {
      vi.advanceTimersByTime(800); for (let i = 0; i < 10; i += 1) await Promise.resolve()
    })

    expect(updateDocument).toHaveBeenCalledWith(11, {
      content: nextContent,
      expectedRevision: 3,
      schemaVersion: 1,
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      revision: 4,
      saveStatus: 'saved',
    })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('keeps the local draft on a revision conflict and replaces it only on explicit reload', async () => {
    const localDraft = '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"local"}]}]}'
    const latest = documentRecord({
      content: '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"latest"}]}]}',
      revision: 4,
      plainText: 'latest',
    })
    const updateDocument = vi.fn<DocumentsClient['updateDocument']>()
      .mockRejectedValue(new DocumentRevisionConflictError(4))
    const hook = setup(updateDocument)
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    vi.useFakeTimers()

    act(() => hook.result.current.control.changeContent(localDraft))
    await act(async () => {
      vi.advanceTimersByTime(800); for (let i = 0; i < 10; i += 1) await Promise.resolve()
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      content: localDraft,
      saveStatus: 'conflict',
      latestRevision: 4,
    })

    hook.document.mockResolvedValueOnce(latest)
    await act(async () => {
      await hook.result.current.control.loadLatest()
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      content: latest.content,
      revision: 4,
      saveStatus: 'saved',
    })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('uploads clipboard images as protected document assets with stable asset ids', async () => {
    const uploadAsset = vi.fn<DocumentsClient['uploadAsset']>().mockResolvedValue({
      id: 31,
      workspaceId: 7,
      documentId: 11,
      fileName: 'diagram.png',
      contentType: 'image/png',
      sizeBytes: 5,
      contentHash: 'hash',
      createdBy: 3,
      createdAt: '2026-09-03T01:00:00Z',
      updatedAt: '2026-09-03T01:00:00Z',
    })
    const hook = setup(vi.fn(), { uploadAsset })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    const image = new File(['image'], 'diagram.png', { type: 'image/png' })
    const attachment = new File(['rows'], 'tasks.csv', { type: 'text/csv' })

    let pasted: Awaited<ReturnType<typeof hook.result.current.control.pasteFiles>> = []
    await act(async () => {
      pasted = await hook.result.current.control.pasteFiles([image, attachment])
    })

    expect(uploadAsset).toHaveBeenCalledWith(11, image)
    expect(uploadAsset).toHaveBeenCalledTimes(1)
    expect(uploadAsset.mock.calls[0][1]).toBe(image)
    expect(pasted).toEqual([{
      src: '/api/v1/platform/documents/11/assets/31/content',
      alt: 'diagram.png',
      assetId: 31,
    }])
    expect(hook.result.current.state).toMatchObject({ pasteStatus: 'idle' })
    hook.unmount()
    hook.queryClient.clear()
  })
})
