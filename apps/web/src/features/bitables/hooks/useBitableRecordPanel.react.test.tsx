import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi, mock, type Mock } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import type { BitableClient } from '../api/client'
import type { Capabilities, RecordComment, Row } from '../model/types'
import type { BitableEnvironment } from './environment'
import { useBitableRecordPanel } from './useBitableRecordPanel'

const runtime = (() => ({ workspaceId: 1, revision: 1 }))()
mock.module('./environment', () => ({ currentEnvironment: (env: BitableEnvironment) => env.workspaceId === runtime.workspaceId && env.sessionRevision === runtime.revision }))
const scope = { resourceId: 'resource', tableId: 'table' }
const row: Row = { id: 'record', revision: 1, fieldRevisions: { title: 1 }, values: { title: 'Example' } }
const caps: Capabilities = { canRead: true, canComment: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title'] }
const result: RecordComment = { id: 'comment', recordId: row.id, body: 'first\nsecond', author: { id: 1, name: 'Tester' }, createdAt: '2026-09-07T01:00:00Z' }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(yes => { resolve = yes }); return { promise, resolve } }
function setup(initialRecordId?: string) {
  const client = { query: vi.fn().mockResolvedValue({ records: [row], total: 201, offset: 200 }), comments: vi.fn().mockResolvedValue({ items: [], total: 0, offset: 0 }), comment: vi.fn().mockResolvedValue(result) } as unknown as BitableClient
  const env: BitableEnvironment = { workspaceId: 1, sessionRevision: 1, client }
  const query = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={query}>{children}</QueryClientProvider>
  const hook = renderHook(({ environment, capabilities }) => useBitableRecordPanel(environment, scope, 'view', [{ id: 'title', name: 'Title', type: 'text', readOnly: false }], capabilities, initialRecordId), { wrapper, initialProps: { environment: env, capabilities: caps } })
  return { ...hook, env, client }
}
beforeEach(() => { runtime.workspaceId = 1; runtime.revision = 1 })
describe('record panel scoped workflow', () => {
  it('opens a deep link with locate query and exposes its actual page offset', async () => {
    const h = setup(row.id)
    await waitFor(() => expect(h.result.current.location).toEqual({ recordId: row.id, offset: 200 }))
    expect(h.client.query).toHaveBeenCalledWith(scope, { viewId: 'view', offset: 0, limit: 200, locateRecordId: row.id }, expect.any(AbortSignal))
  })
  it('preserves multiline body and replays an unknown submission with the exact idempotency key', async () => {
    const h = setup(); act(() => h.result.current.open(row, 'comments'))
    await waitFor(() => expect(h.result.current.panelProps.state).toBe('ready'))
    act(() => h.result.current.panelProps.onDraftChange(result.body))
    ;((h.client.comment) as unknown as Mock<typeof h.client.comment>).mockRejectedValueOnce(new TypeError('offline'))
    await act(async () => { await h.result.current.panelProps.onSend() })
    expect(h.result.current.panelProps.phase).toBe('unknown')
    const args = ((h.client.comment) as unknown as Mock<typeof h.client.comment>).mock.calls[0]
    await act(async () => { await h.result.current.panelProps.onSend() })
    expect(h.client.comment).toHaveBeenCalledTimes(1)
    await act(async () => { await h.result.current.panelProps.onRetrySend() })
    expect(((h.client.comment) as unknown as Mock<typeof h.client.comment>).mock.calls[1]).toEqual(args)
    expect(args[2]).toBe('first\nsecond')
    expect(h.result.current.panelProps.draft).toBe('')
  })
  it('rejects duplicate clicks synchronously and discards late results after scope changes', async () => {
    const h = setup(); act(() => h.result.current.open(row, 'comments'))
    act(() => h.result.current.panelProps.onDraftChange(result.body))
    const pending = deferred<RecordComment>(); ((h.client.comment) as unknown as Mock<typeof h.client.comment>).mockReturnValue(pending.promise)
    let first!: Promise<boolean>
    act(() => { first = h.result.current.panelProps.onSend(); void h.result.current.panelProps.onSend() })
    expect(h.client.comment).toHaveBeenCalledTimes(1)
    runtime.workspaceId = 2
    h.rerender({ environment: { ...h.env, workspaceId: 2 }, capabilities: caps })
    await act(async () => { pending.resolve(result); expect(await first).toBe(false) })
    expect(h.result.current.panelProps.open).toBe(false)
    expect(h.result.current.panelProps.comments).toEqual([])
  })
  it('clears record data and refuses stale callbacks after read capability is revoked', async () => {
    const h = setup(); act(() => h.result.current.open(row, 'comments'))
    await waitFor(() => expect(h.result.current.panelProps.state).toBe('ready'))
    act(() => h.result.current.panelProps.onDraftChange(result.body))
    const send = h.result.current.panelProps.onSend
    h.rerender({ environment: h.env, capabilities: { ...caps, canRead: false, canComment: false } })
    await act(async () => { expect(await send()).toBe(false) })
    expect(h.client.comment).not.toHaveBeenCalled()
    expect(h.result.current.panelProps.record).toBeUndefined()
  })
  it('cannot submit without the dedicated comment capability', async () => {
    const h = setup(); h.rerender({ environment: h.env, capabilities: { ...caps, canComment: false } })
    act(() => h.result.current.open(row, 'comments'))
    act(() => h.result.current.panelProps.onDraftChange(result.body))
    await act(async () => { expect(await h.result.current.panelProps.onSend()).toBe(false) })
    expect(h.client.comment).not.toHaveBeenCalled()
  })
  it('jumps to the last chronological comments page after acknowledged submission', async () => {
    const h = setup()
    ;((h.client.comments) as unknown as Mock<typeof h.client.comments>).mockImplementation(async (_, __, offset) => ({ items: offset === 50 ? [result] : [], total: 51, offset }))
    act(() => h.result.current.open(row, 'comments'))
    await waitFor(() => expect(h.result.current.panelProps.total).toBe(51))
    act(() => h.result.current.panelProps.onDraftChange(result.body))
    await act(async () => { await h.result.current.panelProps.onSend() })
    await waitFor(() => expect(h.result.current.panelProps.offset).toBe(50))
    await waitFor(() => expect(h.result.current.panelProps.comments[0]?.body).toBe(result.body))
  })
  it('fails closed on forbidden read responses', async () => {
    const h = setup(); ((h.client.query) as unknown as Mock<typeof h.client.query>).mockRejectedValue(new PMSafeApiError('FORBIDDEN', 403))
    act(() => h.result.current.open(row, 'details'))
    await waitFor(() => expect(h.result.current.panelProps.open).toBe(false))
    expect(h.result.current.panelProps.record).toBeUndefined()
  })
})
