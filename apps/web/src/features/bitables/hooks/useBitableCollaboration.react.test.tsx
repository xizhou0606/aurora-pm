import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import type {
  BitableCollaborationClientPort,
  BitableRealtimeSignal,
} from '../api/collaboration-port'
import type { BitableClient } from '../api/client'
import type { BitableCellKey, BitableCellLease } from '../model/collaboration'
import type { BitableEnvironment } from './environment'
import { useBitableCollaboration } from './useBitableCollaboration'

const environment: BitableEnvironment = {
  workspaceId: 7,
  sessionRevision: 3,
  client: {} as BitableClient,
}
const scope = { resourceId: 'bitable-uuid', tableId: 'table-1' }
const address = { recordId: 'record-1', fieldId: 'field-1' }

function lease(key: BitableCellKey, editorInstanceId: string): BitableCellLease {
  return {
    key,
    leaseId: `lease-${editorInstanceId}`,
    fencingToken: 4,
    connectionId: 'connection-1',
    editorInstanceId,
    actorUserId: 9,
    actorName: 'Current editor',
    policyRevision: 8,
    expiresAt: '2026-09-08T12:00:00Z',
  }
}

function port() {
  let signal: ((event: BitableRealtimeSignal) => void) | undefined
  const client: BitableCollaborationClientPort = {
    acquireCell: vi.fn(async (input) => ({
      status: 'acquired' as const, lease: lease(input.key, input.editorInstanceId),
    })),
    renewCell: vi.fn(async (proof) => lease(proof.key, proof.editorInstanceId)),
    releaseCell: vi.fn(async () => undefined),
    disconnect: vi.fn(async () => undefined),
    heartbeatPresence: vi.fn(async (input) => ({
      connectionId: 'public-connection', actorUserId: 9, actorName: 'Current editor',
      state: input.state, expiresAt: '2026-09-08T12:00:00Z',
    })),
    leavePresence: vi.fn(async () => undefined),
    saveCell: vi.fn(async (input) => ({ status: 'committed' as const, operationId: input.operationId })),
    subscribe: vi.fn((input) => {
      signal = input.onSignal
      return vi.fn()
    }),
  }
  return { client, emit: (event: BitableRealtimeSignal) => signal?.(event) }
}

function wrapper(queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  return {
    queryClient,
    Wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  }
}

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('bitable realtime collaboration workflow', () => {
  it('releases the cell and presence in background, then resumes presence on return', async () => {
    const visibility = { current: 'visible' as 'visible' | 'hidden' }
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility.current })
    const collaboration = port()
    const { Wrapper } = wrapper()
    const { result, unmount } = renderHook(() => useBitableCollaboration({ environment, scope, resourceId: 'unified-resource', client: collaboration.client, connectionId: 'connection-1', editorInstanceId: 'editor-1', canRead: true, canEdit: true }), { wrapper: Wrapper })
    await act(async () => { await result.current.cell.begin(address, 2, 'draft') })
    await act(async () => { visibility.current = 'hidden'; document.dispatchEvent(new Event('visibilitychange')) })
    expect(collaboration.client.releaseCell).toHaveBeenCalled()
    expect(collaboration.client.leavePresence).toHaveBeenCalled()
    expect(result.current.cell.state.phase).toBe('frozen')
    const calls = ((collaboration.client.heartbeatPresence) as unknown as Mock<typeof collaboration.client.heartbeatPresence>).mock.calls.length
    await act(async () => { visibility.current = 'visible'; document.dispatchEvent(new Event('visibilitychange')) })
    expect(((collaboration.client.heartbeatPresence) as unknown as Mock<typeof collaboration.client.heartbeatPresence>).mock.calls.length).toBeGreaterThan(calls)
    unmount()
  })
  it('removes presence again if an in-flight heartbeat completes after leaving', async () => {
    const collaboration = port()
    let resolveHeartbeat!: (value: Awaited<ReturnType<BitableCollaborationClientPort['heartbeatPresence']>>) => void
    ;((collaboration.client.heartbeatPresence) as unknown as Mock<typeof collaboration.client.heartbeatPresence>).mockImplementationOnce(() => new Promise(resolve => { resolveHeartbeat = resolve }))
    const { Wrapper } = wrapper()
    const { unmount } = renderHook(() => useBitableCollaboration({ environment, scope, resourceId: 'unified-resource', client: collaboration.client, connectionId: 'connection-1', editorInstanceId: 'editor-1', canRead: true, canEdit: true }), { wrapper: Wrapper })
    unmount()
    expect(collaboration.client.leavePresence).toHaveBeenCalledTimes(1)
    await act(async () => { resolveHeartbeat({ connectionId: 'public', actorUserId: 9, actorName: 'Editor', state: {}, expiresAt: 'later' }); await Promise.resolve() })
    expect(collaboration.client.leavePresence).toHaveBeenCalledTimes(2)
  })
  it('acquires the newly selected cell after another cell was held, without retrying the old address', async () => {
    const collaboration = port()
    ;((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mockResolvedValueOnce({ status: 'held', holder: { actorUserId: 12, actorName: 'Other', expiresAt: 'later' } })
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useBitableCollaboration({ environment, scope, resourceId: 'unified-resource', client: collaboration.client, connectionId: 'connection-1', editorInstanceId: 'editor-1', canRead: true, canEdit: true }), { wrapper: Wrapper })
    await act(async () => { expect(await result.current.cell.begin(address, 2, 'before')).toBe(false) })
    const other = { recordId: 'record-3', fieldId: address.fieldId }
    await act(async () => { expect(await result.current.cell.begin(other, 1, 'third')).toBe(true) })
    expect(collaboration.client.acquireCell).toHaveBeenLastCalledWith(expect.objectContaining({ key: expect.objectContaining(other) }))
    expect(result.current.cell.state).toMatchObject({ phase: 'editing', draft: { key: other, original: 'third' } })
  })
  it('coalesces event bursts without cancelling reads and schedules one trailing refresh', async () => {
    vi.useFakeTimers()
    const collaboration = port()
    const { Wrapper, queryClient } = wrapper()
    let finish: () => void = () => undefined
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve })).mockResolvedValue()
    const { unmount } = renderHook(() => useBitableCollaboration({ environment, scope, resourceId: 'unified-resource', client: collaboration.client, connectionId: 'connection-1', editorInstanceId: 'editor-1', canRead: true, canEdit: true }), { wrapper: Wrapper })
    const emit = (sequence: number) => collaboration.emit({ kind: 'committed', event: {
      room: { workspaceId: 7, resourceId: 'unified-resource' }, sequence, operationId: `remote-${sequence}`, actorUserId: 12,
      type: 'bitable.cell.updated', payload: {}, policyRevision: 8, committedAt: 'now',
    } })
    act(() => { emit(1); emit(2); emit(3) })
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(invalidate).toHaveBeenCalledTimes(1)
    expect(invalidate).toHaveBeenLastCalledWith(expect.anything(), { cancelRefetch: false })
    act(() => { emit(4); emit(5) })
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(invalidate).toHaveBeenCalledTimes(1)
    await act(async () => { finish(); await Promise.resolve(); vi.advanceTimersByTime(100) })
    expect(invalidate).toHaveBeenCalledTimes(2)
    act(() => { emit(3); emit(5) })
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(invalidate).toHaveBeenCalledTimes(2)
    act(() => emit(6))
    unmount()
    await act(async () => { vi.advanceTimersByTime(100) })
    expect(invalidate).toHaveBeenCalledTimes(2)
  })
  it('fails closed without an explicit unified resource id and never derives it from the bitable id', async () => {
    const collaboration = port()
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: undefined,
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
    }), { wrapper: Wrapper })
    expect(result.current.availability).toBe('unavailable')
    expect(result.current.lockKey(address)).toBeUndefined()
    await act(async () => {
      expect(await result.current.cell.begin(address, 2, 'before')).toBe(false)
    })
    expect(collaboration.client.acquireCell).not.toHaveBeenCalled()
    expect(collaboration.client.subscribe).not.toHaveBeenCalled()
  })

  it('lets a read-only viewer observe presence without sending an acquire request', async () => {
    const collaboration = port()
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'viewer-instance',
      canRead: true,
      canEdit: false,
    }), { wrapper: Wrapper })
    act(() => collaboration.emit({
      kind: 'presence',
      room: { workspaceId: 7, resourceId: 'unified-resource' },
      members: [{
        connectionId: 'other', actorUserId: 12, actorName: 'Other editor',
        state: {}, expiresAt: 'later',
      }],
    }))
    expect(result.current.realtime.members[0]?.actorName).toBe('Other editor')
    expect(result.current.cell.view.availability).toBe('forbidden')
    await act(async () => {
      expect(await result.current.cell.begin(address, 2, 'before')).toBe(false)
    })
    expect(collaboration.client.acquireCell).not.toHaveBeenCalled()
    expect(collaboration.client.heartbeatPresence).not.toHaveBeenCalled()
    expect(collaboration.client.leavePresence).not.toHaveBeenCalled()
  })

  it('publishes an editor selection periodically and leaves once when destroyed', async () => {
    vi.useFakeTimers()
    const collaboration = port()
    const { Wrapper } = wrapper()
    const selection = { anchor: address, focus: address }
    let activeCell: typeof address | undefined = address
    const { unmount } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
      heartbeatEveryMs: 1_000,
      getPresenceState: () => ({ tableId: scope.tableId, viewId: 'view-1', ...(activeCell ? { activeCell } : {}), selection }),
    }), { wrapper: Wrapper })
    await act(async () => { await Promise.resolve() })
    expect(collaboration.client.heartbeatPresence).toHaveBeenCalledWith({
      room: { workspaceId: 7, resourceId: 'unified-resource' },
      connectionId: 'connection-1',
      state: { tableId: 'table-1', viewId: 'view-1', activeCell: address, selection },
    })
    await act(async () => { vi.advanceTimersByTime(1_000) })
    expect(collaboration.client.heartbeatPresence).toHaveBeenCalledTimes(2)
    activeCell = undefined
    await act(async () => { vi.advanceTimersByTime(250) })
    expect(collaboration.client.heartbeatPresence).toHaveBeenCalledTimes(3)
    expect(collaboration.client.heartbeatPresence).toHaveBeenLastCalledWith(expect.objectContaining({
      state: { tableId: 'table-1', viewId: 'view-1', selection },
    }))
    act(() => window.dispatchEvent(new Event('pagehide')))
    expect(collaboration.client.leavePresence).toHaveBeenCalledTimes(1)
    unmount()
    expect(collaboration.client.leavePresence).toHaveBeenCalledTimes(1)
    vi.useRealTimers()
  })

  it('invalidates server facts on a remote commit without changing the local draft', async () => {
    const collaboration = port()
    const { Wrapper, queryClient } = wrapper()
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue()
    const { result } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
    }), { wrapper: Wrapper })
    await act(async () => {
      expect(await result.current.cell.begin(address, 2, 'before')).toBe(true)
    })
    act(() => result.current.cell.change('local draft'))
    act(() => collaboration.emit({
      kind: 'committed',
      event: {
        room: { workspaceId: 7, resourceId: 'unified-resource' },
        sequence: 11, operationId: 'remote-operation', actorUserId: 12,
        type: 'bitable.cell.updated', payload: {}, policyRevision: 8, committedAt: 'now',
      },
    }))
    await waitFor(() => expect(invalidate).toHaveBeenCalled())
    expect(result.current.cell.state).toMatchObject({
      phase: 'editing', draft: { value: 'local draft' },
    })
  })

  it('freezes an active draft as soon as the server revokes room access', async () => {
    const collaboration = port()
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
    }), { wrapper: Wrapper })
    await act(async () => { await result.current.cell.begin(address, 2, 'before') })
    act(() => result.current.cell.change('kept after revocation'))
    act(() => collaboration.emit({
      kind: 'access_revoked',
      room: { workspaceId: 7, resourceId: 'unified-resource' },
    }))
    await waitFor(() => expect(result.current.cell.state).toMatchObject({
      phase: 'frozen', reason: 'lease_lost', draft: { value: 'kept after revocation' },
    }))
    expect(result.current.availability).toBe('forbidden')
    expect(result.current.realtime.message).not.toBe('')
  })

  it('freezes a draft and ignores a late save after edit capability is revoked', async () => {
    const collaboration = port()
    let resolveSave!: (value: { status: 'committed'; operationId: string }) => void
    ;((collaboration.client.saveCell) as unknown as Mock<typeof collaboration.client.saveCell>).mockImplementation((input) => new Promise((resolve) => {
      resolveSave = () => resolve({ status: 'committed', operationId: input.operationId })
    }))
    const { Wrapper } = wrapper()
    const { result, rerender } = renderHook(({ canEdit }) => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit,
    }), { wrapper: Wrapper, initialProps: { canEdit: true } })
    await act(async () => { await result.current.cell.begin(address, 2, 'before') })
    act(() => result.current.cell.change('kept draft'))
    let saving!: Promise<boolean>
    act(() => { saving = result.current.cell.save() })
    rerender({ canEdit: false })
    await waitFor(() => expect(result.current.cell.state).toMatchObject({
      phase: 'frozen', reason: 'lease_lost', draft: { value: 'kept draft' },
    }))
    await act(async () => { resolveSave({ status: 'committed', operationId: 'ignored' }); await saving })
    expect(result.current.cell.state).toMatchObject({
      phase: 'frozen', reason: 'lease_lost', draft: { value: 'kept draft' },
    })
  })

  it('freezes and retains the draft when lease renewal fails', async () => {
    vi.useFakeTimers()
    const collaboration = port()
    ;((collaboration.client.renewCell) as unknown as Mock<typeof collaboration.client.renewCell>).mockRejectedValue(new Error('expired'))
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useBitableCollaboration({
      environment,
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
      renewAfterMs: 1,
    }), { wrapper: Wrapper })
    await act(async () => { await result.current.cell.begin(address, 2, 'before') })
    act(() => result.current.cell.change('kept after expiry'))
    await act(async () => { vi.advanceTimersByTime(1) })
    expect(result.current.cell.state).toMatchObject({
      phase: 'frozen', reason: 'lease_lost', draft: { value: 'kept after expiry' },
    })
  })

  it('drops a late acquire from an earlier session and releases its lease', async () => {
    const collaboration = port()
    let resolveAcquire!: (value: ReturnType<typeof lease>) => void
    ;((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mockImplementation((input) => new Promise((resolve) => {
      resolveAcquire = (value) => resolve({ status: 'acquired', lease: value })
    }))
    const { Wrapper } = wrapper()
    const { result, rerender } = renderHook(({ sessionRevision }) => useBitableCollaboration({
      environment: { ...environment, sessionRevision },
      scope,
      resourceId: 'unified-resource',
      client: collaboration.client,
      connectionId: 'connection-1',
      editorInstanceId: 'editor-1',
      canRead: true,
      canEdit: true,
    }), { wrapper: Wrapper, initialProps: { sessionRevision: 3 } })
    let acquiring!: Promise<boolean>
    act(() => { acquiring = result.current.cell.begin(address, 2, 'before') })
    rerender({ sessionRevision: 4 })
    await waitFor(() => expect(result.current.cell.state.phase).toBe('view'))
    await act(async () => {
      resolveAcquire(lease({
        workspaceId: 7, resourceId: 'unified-resource', tableId: 'table-1', ...address,
      }, 'editor-1'))
      expect(await acquiring).toBe(false)
    })
    expect(result.current.cell.state.phase).toBe('view')
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
  })

  it('shares the server lock key across surfaces while keeping their drafts local', async () => {
    const collaboration = port()
    const one = wrapper(), two = wrapper()
    const first = renderHook(() => useBitableCollaboration({
      environment, scope, resourceId: 'unified-resource', client: collaboration.client,
      connectionId: 'connection-1', editorInstanceId: 'standalone', canRead: true, canEdit: true,
    }), { wrapper: one.Wrapper })
    const second = renderHook(() => useBitableCollaboration({
      environment, scope, resourceId: 'unified-resource', client: collaboration.client,
      connectionId: 'connection-1', editorInstanceId: 'reference', canRead: true, canEdit: true,
    }), { wrapper: two.Wrapper })
    await act(async () => {
      await first.result.current.cell.begin(address, 2, 'before')
      await second.result.current.cell.begin(address, 2, 'before')
    })
    act(() => {
      first.result.current.cell.change('standalone draft')
      second.result.current.cell.change('reference draft')
    })
    const calls = ((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mock.calls
    expect(calls[0][0].key).toEqual(calls[1][0].key)
    expect(calls[0][0].editorInstanceId).not.toBe(calls[1][0].editorInstanceId)
    expect(first.result.current.cell.view.draft?.value).toBe('standalone draft')
    expect(second.result.current.cell.view.draft?.value).toBe('reference draft')
  })
})
