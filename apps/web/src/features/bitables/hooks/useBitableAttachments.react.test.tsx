import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock, type Mock } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { downloadBlob } from '@/shared/lib/download-blob'
import type { BitableClient } from '../api/client'
import type { BitableCollaborationClientPort } from '../api/collaboration-port'
import type { CommandResult, Row } from '../model/types'
import type { BitableEnvironment } from './environment'
import { useBitableAttachmentPreview } from './useBitableAttachmentPreview'
import { useBitableAttachmentUpload } from './useBitableAttachmentUpload'

const runtime = (() => ({ revision: 1 }))()
mock.module('./environment', () => ({ currentEnvironment: (env: BitableEnvironment) => env.sessionRevision === runtime.revision }))
vi.mock('@/shared/lib/download-blob', () => ({ downloadBlob: vi.fn() }))
const scope = { resourceId: 'resource', tableId: 'table' }, address = { recordId: 'row', fieldId: 'files' }
const lease = { key: { workspaceId: 1, resourceId: 'resource', tableId: 'table', ...address }, leaseId: 'lease', fencingToken: 4, connectionId: 'connection', editorInstanceId: 'editor', actorUserId: 1, actorName: 'User', expiresAt: '2026-09-09T12:00:00Z', policyRevision: 1 }
const collaboration = { room: { workspaceId: 1, resourceId: 'resource' }, connectionId: 'connection', editorInstanceId: 'editor', client: { acquireCell: vi.fn(), renewCell: vi.fn(), releaseCell: vi.fn() } as unknown as BitableCollaborationClientPort }
const rows: Row[] = [{ id: 'row', revision: 3, fieldRevisions: { files: 3 }, values: { files: [{ id: 'asset', name: '截图.png', size: 3, mimeType: 'image/png', downloadPath: '/protected/no-public-url' }] } }]
function setup() { const client = { asset: vi.fn(), upload: vi.fn(), operation: vi.fn() } as unknown as BitableClient; const env: BitableEnvironment = { workspaceId: 1, sessionRevision: 1, client }; return { client, env } }
beforeEach(() => {
  ((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mockReset().mockResolvedValue({ status: 'acquired', lease })
  ;((collaboration.client.renewCell) as unknown as Mock<typeof collaboration.client.renewCell>).mockReset().mockResolvedValue(lease)
  ;((collaboration.client.releaseCell) as unknown as Mock<typeof collaboration.client.releaseCell>).mockReset().mockResolvedValue(undefined)
  runtime.revision = 1; ((downloadBlob) as unknown as Mock<typeof downloadBlob>).mockClear()
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn().mockReturnValue('blob:protected-preview') })
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
})
describe('protected bitable attachment lifecycle', () => {
  it('switching table scope clears pending state and ignores the previous upload receipt', async () => {
    const { env, client } = setup(), invalidate = vi.fn()
    let finish!: (value: CommandResult) => void
    ;((client.upload) as unknown as Mock<typeof client.upload>).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const h = renderHook(({ tableId }) => useBitableAttachmentUpload(env, { ...scope, tableId }, 7, ['files'], true, invalidate, collaboration), { initialProps: { tableId: scope.tableId } })
    let pending!: Promise<boolean>
    await act(async () => { pending = h.result.current.upload(address, 3, new File(['png'], 'a.png')); await Promise.resolve() })
    h.rerender({ tableId: 'new-table' })
    expect(h.result.current.pending).toBe(false)
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
    await act(async () => { finish({ operationId: ((client.upload) as unknown as Mock<typeof client.upload>).mock.calls[0][4], state: 'committed', records: rows, deletedRecordIds: [] }); expect(await pending).toBe(false) })
    expect(invalidate).not.toHaveBeenCalled()
    expect(h.result.current.error).toBe('')
  })
  it('unmount releases the in-flight upload lease immediately and ignores its late receipt', async () => {
    const { env, client } = setup(), invalidate = vi.fn()
    let finish!: (value: CommandResult) => void
    ;((client.upload) as unknown as Mock<typeof client.upload>).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, invalidate, collaboration))
    let pending!: Promise<boolean>
    await act(async () => { pending = h.result.current.upload(address, 3, new File(['png'], 'a.png')); await Promise.resolve() })
    h.unmount()
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
    await act(async () => { finish({ operationId: ((client.upload) as unknown as Mock<typeof client.upload>).mock.calls[0][4], state: 'committed', records: rows, deletedRecordIds: [] }); expect(await pending).toBe(false) })
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
    expect(invalidate).not.toHaveBeenCalled()
  })
  it('renews a slow upload lease and releases it after the receipt returns', async () => {
    vi.useFakeTimers()
    try {
      const { env, client } = setup()
      let finish!: (value: CommandResult) => void
      ;((client.upload) as unknown as Mock<typeof client.upload>).mockImplementation(() => new Promise(resolve => { finish = resolve }))
      const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, vi.fn(), collaboration))
      let pending!: Promise<boolean>
      await act(async () => { pending = h.result.current.upload(address, 3, new File(['png'], 'a.png')); await Promise.resolve() })
      await act(async () => { vi.advanceTimersByTime(10_000) })
      expect(collaboration.client.renewCell).toHaveBeenCalledTimes(1)
      await act(async () => { finish({ operationId: ((client.upload) as unknown as Mock<typeof client.upload>).mock.calls[0][4], state: 'committed', records: rows, deletedRecordIds: [] }); expect(await pending).toBe(true) })
      expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
      await act(async () => { vi.advanceTimersByTime(20_000) })
      expect(collaboration.client.renewCell).toHaveBeenCalledTimes(1)
    } finally { vi.useRealTimers() }
  })
  it('releases a late acquired lease without uploading after access disappears', async () => {
    const { env, client } = setup()
    let finish!: (value: Awaited<ReturnType<BitableCollaborationClientPort['acquireCell']>>) => void
    ;((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const h = renderHook(({ allowed }) => useBitableAttachmentUpload(env, scope, 7, ['files'], allowed, vi.fn(), collaboration), { initialProps: { allowed: true } })
    let pending!: Promise<boolean>
    act(() => { pending = h.result.current.upload(address, 3, new File(['png'], 'a.png')) })
    h.rerender({ allowed: false })
    await act(async () => { finish({ status: 'acquired', lease }); expect(await pending).toBe(false) })
    expect(client.upload).not.toHaveBeenCalled()
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
    expect(h.result.current.pending).toBe(false)
  })
  it('requires a lease and field revision, releases it after upload, and never uploads an occupied cell', async () => {
    const { env, client } = setup()
    ;((client.upload) as unknown as Mock<typeof client.upload>).mockImplementation(async (_scope, _address, _contract, _schema, operationId) => ({ operationId, state: 'committed', records: rows, deletedRecordIds: [] }))
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, vi.fn(), collaboration))
    await act(async () => { expect(await h.result.current.upload(address, 3, new File(['png'], 'a.png', { type: 'image/png' }))).toBe(true) })
    expect(client.upload).toHaveBeenCalledWith(scope, address, { ...address, baseRevision: 3, leaseId: 'lease', fencingToken: 4, connectionId: 'connection', editorInstanceId: 'editor' }, 7, expect.any(String), expect.any(File))
    expect(collaboration.client.releaseCell).toHaveBeenCalledTimes(1)
    ;((collaboration.client.acquireCell) as unknown as Mock<typeof collaboration.client.acquireCell>).mockResolvedValue({ status: 'held', holder: lease })
    await act(async () => { expect(await h.result.current.upload(address, 3, new File(['png'], 'b.png'))).toBe(false) })
    expect(client.upload).toHaveBeenCalledTimes(1)
    expect(h.result.current.unknown).toBe(false)
  })
  it('does not upload without collaboration proof even when attachment permission is allowed', async () => {
    const { env, client } = setup()
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, vi.fn()))
    await act(async () => { expect(await h.result.current.upload(address, 3, new File(['png'], 'a.png'))).toBe(false) })
    expect(client.upload).not.toHaveBeenCalled()
  })
  it('previews only protected bytes, revokes on close, and downloads the real filename', async () => {
    const { env, client } = setup(), blob = new Blob(['png'], { type: 'image/png' })
    ;((client.asset) as unknown as Mock<typeof client.asset>).mockResolvedValue(blob)
    const h = renderHook(() => useBitableAttachmentPreview(env, scope, rows, true))
    await act(async () => { await h.result.current.open(address, 'asset') })
    expect(client.asset).toHaveBeenCalledWith(scope, address, 'asset', 'preview')
    expect(h.result.current.preview).toEqual({ url: 'blob:protected-preview', title: '截图.png' })
    act(() => { h.result.current.close() })
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:protected-preview')
    await act(async () => { await h.result.current.download(address, 'asset') })
    expect(downloadBlob).toHaveBeenCalledWith(blob, '截图.png')
  })
  it('revokes when permission changes and never opens stale-session bytes', async () => {
    const { env, client } = setup(), blob = new Blob(['png'], { type: 'image/png' })
    ;((client.asset) as unknown as Mock<typeof client.asset>).mockResolvedValueOnce(blob)
    const h = renderHook(({ allowed }) => useBitableAttachmentPreview(env, scope, rows, allowed), { initialProps: { allowed: true } })
    await act(async () => { await h.result.current.open(address, 'asset') })
    h.rerender({ allowed: false })
    expect(h.result.current.preview).toBeNull(); expect(URL.revokeObjectURL).toHaveBeenCalled()
    h.rerender({ allowed: true })
    let resolve!: (value: Blob) => void
    ;((client.asset) as unknown as Mock<typeof client.asset>).mockImplementationOnce(() => new Promise<Blob>(yes => { resolve = yes }))
    let opening!: Promise<void>
    act(() => { opening = h.result.current.open(address, 'asset') })
    runtime.revision = 2
    await act(async () => { resolve(blob); await opening })
    expect(h.result.current.preview).toBeNull()
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
  })
  it('unmount releases URLs and unknown attachment IDs never call transport', async () => {
    const { env, client } = setup()
    ;((client.asset) as unknown as Mock<typeof client.asset>).mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
    const h = renderHook(() => useBitableAttachmentPreview(env, scope, rows, true))
    await act(async () => { await h.result.current.open(address, 'missing'); await h.result.current.download(address, 'missing') })
    expect(client.asset).not.toHaveBeenCalled()
    await act(async () => { await h.result.current.open(address, 'asset') })
    h.unmount(); expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:protected-preview')
  })
  it('unknown upload is resolved with the original operation ID, never re-uploaded', async () => {
    const { env, client } = setup(), invalidate = vi.fn().mockResolvedValue(undefined), file = new File(['png'], '截图.png', { type: 'image/png' })
    ;((client.upload) as unknown as Mock<typeof client.upload>).mockRejectedValue(new TypeError('lost response'))
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, invalidate, collaboration))
    await act(async () => { expect(await h.result.current.upload(address, 3, file)).toBe(false) })
    expect(h.result.current.unknown).toBe(true)
    const operationId = ((client.upload) as unknown as Mock<typeof client.upload>).mock.calls[0][4]
    ;((client.operation) as unknown as Mock<typeof client.operation>).mockResolvedValue({ operationId, state: 'committed', records: rows, deletedRecordIds: [] })
    await act(async () => { expect(await h.result.current.upload(address, 3, new File(['other'], 'other.png'))).toBe(false) })
    expect(client.operation).not.toHaveBeenCalled()
    await act(async () => { expect(await h.result.current.upload(address, 3, file)).toBe(true) })
    expect(client.upload).toHaveBeenCalledTimes(1); expect(client.operation).toHaveBeenCalledWith(scope, operationId)
    expect(invalidate).toHaveBeenCalledTimes(1); expect(h.result.current.unknown).toBe(false)
  })
  it('upload double-click and stale permission handlers cannot issue extra mutations', async () => {
    const { env, client } = setup(), file = new File(['a'], 'a.png')
    let resolve!: (value: CommandResult) => void
    ;((client.upload) as unknown as Mock<typeof client.upload>).mockImplementationOnce(() => new Promise<CommandResult>(yes => { resolve = yes }))
    const h = renderHook(({ allowed }) => useBitableAttachmentUpload(env, scope, 7, ['files'], allowed, vi.fn(), collaboration), { initialProps: { allowed: true } })
    const staleUpload = h.result.current.upload
    let first!: Promise<boolean>
    await act(async () => { first = staleUpload(address, 3, file); void staleUpload(address, 3, file); await Promise.resolve() })
    expect(client.upload).toHaveBeenCalledTimes(1)
    h.rerender({ allowed: false })
    await act(async () => { expect(await staleUpload(address, 3, file)).toBe(false); resolve({ operationId: ((client.upload) as unknown as Mock<typeof client.upload>).mock.calls[0][4], state: 'committed', records: rows, deletedRecordIds: [] }); expect(await first).toBe(false) })
    expect(h.result.current.pending).toBe(false)
  })
  it('explicit upload denial is not classified as an unknown outcome', async () => {
    const { env, client } = setup(); ((client.upload) as unknown as Mock<typeof client.upload>).mockRejectedValue(new PMSafeApiError('BITABLE_FORBIDDEN', 403))
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], true, vi.fn(), collaboration))
    await act(async () => { await h.result.current.upload(address, 3, new File(['a'], 'a.png')) })
    expect(h.result.current.unknown).toBe(false); expect(h.result.current.pending).toBe(false); expect(h.result.current.error).toBeTruthy()
  })
  it('does not call the legacy upload transport when collaboration lease proof is unavailable', async () => {
    const { env, client } = setup()
    const h = renderHook(() => useBitableAttachmentUpload(env, scope, 7, ['files'], false, vi.fn()))
    await act(async () => {
      expect(await h.result.current.upload(address, 3, new File(['a'], 'a.png'))).toBe(false)
    })
    expect(client.upload).not.toHaveBeenCalled()
    expect(client.operation).not.toHaveBeenCalled()
  })
})
