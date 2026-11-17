import { act, renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi, mock, type Mock } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import type { BitableClient } from '../api/client'
import type { Bootstrap, CommandBatch, CommandResult, Row } from '../model/types'
import { bitableKey, type BitableEnvironment } from './environment'
import { useBitableCommands } from './useBitableCommands'

const runtime = (() => ({ workspaceId: 1, revision: 1 }))()
mock.module('./environment', () => ({ currentEnvironment: (env: BitableEnvironment) => runtime.workspaceId === env.workspaceId && runtime.revision === env.sessionRevision }))
const scope = { resourceId: 'resource', tableId: 'table' }
const original: Row = { id: 'row', revision: 4, fieldRevisions: { title: 4, other: 4 }, values: { title: 'Before', other: 'Keep' } }
const mutation = { kind: 'delete' as const, recordId: original.id, expectedRevision: original.revision }
const bootstrap: Bootstrap = { resource: { id: scope.resourceId, workspaceId: 1, name: 'Table', tables: [{ id: scope.tableId, name: 'Table' }], allowedActions: [] }, tableId: scope.tableId, schemaRevision: 3, fields: [{ id: 'title', name: 'Title', type: 'text', readOnly: false }], views: [], capabilities: { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title'] } }
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }
function receipt(batch: CommandBatch, revision = 5, title = 'After'): CommandResult { return { operationId: batch.operationId, state: 'committed', records: [{ ...original, revision, values: { ...original.values, title } }], deletedRecordIds: [] } }
function setup() {
  const client = { execute: vi.fn(), operation: vi.fn() } as unknown as BitableClient
  const env: BitableEnvironment = { workspaceId: 1, sessionRevision: 1, client }
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue()
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  const hook = renderHook(({ environment, data }: { environment: BitableEnvironment; data: Bootstrap | undefined }) => useBitableCommands(environment, scope, data), { wrapper, initialProps: { environment: env, data: bootstrap } })
  return { ...hook, env, client, queryClient, invalidate }
}
beforeEach(() => { runtime.workspaceId = 1; runtime.revision = 1 })

describe('bitable command lifecycle and receipt ownership', () => {
  it('locks synchronously so two clicks dispatch exactly one atomic CAS request', async () => {
    const h = setup(), pending = deferred<CommandResult>()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockReturnValue(pending.promise)
    let first!: Promise<CommandResult | null>, second!: Promise<CommandResult | null>
    act(() => { first = h.result.current.execute([mutation], [original]); second = h.result.current.execute([mutation], [original]) })
    expect(h.client.execute).toHaveBeenCalledTimes(1)
    expect(await second).toBeNull()
    expect(h.result.current.pending).toBe(true)
    const batch = ((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls[0][1]
    expect(batch).toMatchObject({ expectedSchemaRevision: 3, atomic: true, commands: [mutation] })
    await act(async () => { pending.resolve(receipt(batch)); await first })
    expect(h.result.current.pending).toBe(false)
    expect(h.result.current.canUndo).toBe(true)
    expect(h.invalidate).toHaveBeenCalledWith({ queryKey: bitableKey(h.env, scope) })
  })
  it('holds unknown outcomes and resolves the original receipt without re-executing', async () => {
    const h = setup()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockRejectedValue(new TypeError('network disconnected'))
    await act(async () => { await h.result.current.execute([mutation], [original], true) })
    const batch = ((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls[0][1]
    expect(h.result.current.unknown).toBe(true)
    expect(h.result.current.canUndo).toBe(false)
    await act(async () => { expect(await h.result.current.execute([mutation], [original])).toBeNull() })
    ;((h.client.operation) as unknown as Mock<typeof h.client.operation>).mockResolvedValue(receipt(batch))
    await act(async () => { await h.result.current.resolve() })
    expect(h.client.execute).toHaveBeenCalledTimes(1)
    expect(h.client.operation).toHaveBeenCalledWith(scope, batch.operationId)
    expect(h.result.current.unknown).toBe(false)
    expect(h.result.current.canUndo).toBe(true)
  })
  it('undo and redo carry the latest committed structural revision', async () => {
    const h = setup()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockImplementationOnce(async (_, batch) => receipt(batch, 5, 'After')).mockImplementationOnce(async (_, batch) => receipt(batch, 6, 'Before')).mockImplementationOnce(async (_, batch) => receipt(batch, 7, 'After'))
    await act(async () => { await h.result.current.execute([mutation], [original]) })
    await act(async () => { expect(await h.result.current.undo()).toBe(true) })
    expect(((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls[1][1].commands).toEqual([{ kind: 'restore', recordId: 'row', expectedRevision: 5 }])
    expect(h.result.current.canRedo).toBe(true)
    await act(async () => { expect(await h.result.current.redo()).toBe(true) })
    expect(((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls[2][1].commands).toEqual([{ kind: 'delete', recordId: 'row', expectedRevision: 6 }])
    expect(new Set(((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls.map(call => call[1].operationId)).size).toBe(3)
  })
  it('a rejected conflict does not advance or erase undo history', async () => {
    const h = setup()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockImplementationOnce(async (_, batch) => receipt(batch)).mockRejectedValueOnce(new PMSafeApiError('BITABLE_REVISION_CONFLICT', 409))
    await act(async () => { await h.result.current.execute([mutation], [original]) })
    await act(async () => { expect(await h.result.current.undo()).toBe(false) })
    expect(h.result.current.canUndo).toBe(true)
    expect(h.result.current.canRedo).toBe(false)
    expect(h.result.current.unknown).toBe(false)
  })
  it('rejects stale handlers and ignores old completions after identity changes', async () => {
    const h = setup(), pending = deferred<CommandResult>()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockReturnValue(pending.promise)
    const staleExecute = h.result.current.execute
    let first!: Promise<CommandResult | null>
    act(() => { first = staleExecute([mutation], [original]) })
    const batch = ((h.client.execute) as unknown as Mock<typeof h.client.execute>).mock.calls[0][1]
    runtime.revision = 2
    h.rerender({ environment: { ...h.env, sessionRevision: 2 }, data: bootstrap })
    await act(async () => { expect(await staleExecute([mutation], [original])).toBeNull(); pending.resolve(receipt(batch)); expect(await first).toBeNull() })
    expect(h.result.current.canUndo).toBe(false)
    expect(h.result.current.pending).toBe(false)
    expect(h.invalidate).not.toHaveBeenCalled()
  })
  it('does not turn a wrong-operation receipt into successful history', async () => {
    const h = setup()
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockImplementation(async (_, batch) => ({ ...receipt(batch), operationId: 'other-operation' }))
    await act(async () => { expect(await h.result.current.execute([mutation], [original])).toBeNull() })
    expect(h.result.current.canUndo).toBe(false)
    expect(h.result.current.unknown).toBe(true)
  })
  it('permission denial clears cached resource data and retained mutation history', async () => {
    const h = setup(), key = bitableKey(h.env, scope, 'records')
    h.queryClient.setQueryData(key, { records: [original] })
    ;((h.client.execute) as unknown as Mock<typeof h.client.execute>).mockImplementationOnce(async (_, batch) => receipt(batch)).mockRejectedValueOnce(new PMSafeApiError('BITABLE_FORBIDDEN', 403))
    await act(async () => { await h.result.current.execute([mutation], [original]) })
    await act(async () => { await h.result.current.undo() })
    expect(h.queryClient.getQueryData(key)).toBeUndefined()
    expect(h.result.current.canUndo).toBe(false)
    expect(h.result.current.canRedo).toBe(false)
  })
  it('revoking read access rejects an already captured command handler', async () => {
    const h = setup(), staleExecute = h.result.current.execute
    h.rerender({ environment: h.env, data: { ...bootstrap, capabilities: { ...bootstrap.capabilities, canRead: false, canUpdateRecords: false, writableFieldIds: [] } } })
    await act(async () => { expect(await staleExecute([mutation], [original])).toBeNull() })
    expect(h.client.execute).not.toHaveBeenCalled()
  })
})
