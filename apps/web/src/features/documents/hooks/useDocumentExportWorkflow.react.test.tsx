import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type { DocumentsClient } from '../api'
import type { DocumentRecord } from '../model'
import { saveDocumentDownload } from '../utils/download'
import { resolveDocumentCapabilities } from './useDocumentCapabilities'
import type { DocumentEditorViewState } from './useDocumentEditorWorkflow'
import { useDocumentExportWorkflow } from './useDocumentExportWorkflow'

vi.mock('../utils/download', () => ({ saveDocumentDownload: vi.fn() }))

const actions = [PM_ACTION.DocumentsContentView, PM_ACTION.DocumentsContentExport,
  PM_ACTION.DocumentsAssetsView, PM_ACTION.DocumentsAssetsDownload]
const content = JSON.stringify({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: '测试导出' }] }] })
const snapshot: DocumentRecord = {
  id: 11, workspaceId: 7, nodeId: 19, content, revision: 3, schemaVersion: 2,
  contentHash: 'hash', plainText: '测试导出', ownerUserId: 5, createdBy: 5, updatedBy: 5,
  createdAt: '2026-09-05T00:00:00Z', updatedAt: '2026-09-05T00:00:00Z', allowedActions: actions,
}
const ready: Extract<DocumentEditorViewState, { status: 'ready' }> = {
  status: 'ready', documentId: 11, content, revision: 3, schemaVersion: 2,
  updatedAt: snapshot.updatedAt, refresh: 'idle', saveStatus: 'saved', pasteStatus: 'idle',
  copyStatus: 'idle', capabilities: resolveDocumentCapabilities(actions),
}

function setup(client: Partial<DocumentsClient>, editor: DocumentEditorViewState = ready) {
  return renderHook(({ documentState, workspaceId }) => useDocumentExportWorkflow({
    environment: { client: client as DocumentsClient, workspaceId, sessionRevision: 3 },
    editor: documentState, title: '导出测试',
  }), { initialProps: { documentState: editor, workspaceId: 7 } })
}

describe('document export workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 3 })
  })

  it('exports an authenticated saved snapshot and revalidates before downloading', async () => {
    const exportDocument = vi.fn().mockResolvedValue(snapshot)
    const hook = setup({ exportDocument })
    await act(() => hook.result.current.run('markdown'))
    expect(exportDocument).toHaveBeenCalledTimes(2)
    expect(exportDocument).toHaveBeenCalledWith(11, expect.any(AbortSignal))
    expect(saveDocumentDownload).toHaveBeenCalledWith(expect.any(Blob), '导出测试.md')
    expect(hook.result.current.state).toMatchObject({ status: 'success', warnings: [] })
  })

  it.each(['dirty', 'saving', 'conflict', 'save_error'] as const)('blocks %s content before any request', async (saveStatus) => {
    const exportDocument = vi.fn()
    const hook = setup({ exportDocument }, { ...ready, saveStatus })
    await act(() => hook.result.current.run('markdown'))
    expect(exportDocument).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'unsaved' })
  })

  it('blocks missing export capability before any request', async () => {
    const exportDocument = vi.fn()
    const hook = setup({ exportDocument }, { ...ready, capabilities: { ...ready.capabilities, canExport: false } })
    await act(() => hook.result.current.run('markdown'))
    expect(exportDocument).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'permission' })
  })

  it.each([
    { ...snapshot, workspaceId: 8 },
    { ...snapshot, id: 12 },
    { ...snapshot, allowedActions: [PM_ACTION.DocumentsContentView] },
  ])('rejects a wrong scope or missing server export grant', async (response) => {
    const hook = setup({ exportDocument: vi.fn().mockResolvedValue(response) })
    await act(() => hook.result.current.run('markdown'))
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'permission' })
  })

  it('does not silently export a newer unseen revision', async () => {
    const hook = setup({ exportDocument: vi.fn().mockResolvedValue({ ...snapshot, revision: 4 }) })
    await act(() => hook.result.current.run('markdown'))
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'changed' })
  })

  it('suppresses a download if access is revoked during preparation', async () => {
    const exportDocument = vi.fn().mockResolvedValueOnce(snapshot).mockRejectedValueOnce(new PMSafeApiError('FORBIDDEN', 403))
    const hook = setup({ exportDocument })
    await act(() => hook.result.current.run('markdown'))
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'permission' })
  })

  it('deduplicates clicks and discards results after workspace changes', async () => {
    let resolve!: (value: DocumentRecord) => void
    const exportDocument = vi.fn(() => new Promise<DocumentRecord>((done) => { resolve = done }))
    const hook = setup({ exportDocument })
    let pending!: Promise<void>
    act(() => { pending = hook.result.current.run('markdown') })
    await act(() => hook.result.current.run('zip'))
    expect(exportDocument).toHaveBeenCalledTimes(1)
    act(() => {
      runtimeWorkspaceStore.setState({ currentWorkspaceId: 8 })
      hook.rerender({ documentState: ready, workspaceId: 8 })
    })
    await act(async () => { resolve(snapshot); await pending })
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'idle' })
  })

  it('never downloads resources for plain Markdown containing private images', async () => {
    const privateImage = JSON.stringify({ type: 'doc', content: [{ type: 'image', attrs: {
      assetId: 9, src: '/api/v1/platform/documents/11/assets/9/content',
    } }] })
    const downloadAsset = vi.fn()
    const hook = setup({ exportDocument: vi.fn().mockResolvedValue({ ...snapshot, content: privateImage }), downloadAsset })
    await act(() => hook.result.current.run('markdown'))
    expect(downloadAsset).not.toHaveBeenCalled()
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'requires_bundle' })
  })

  it('uses only the current document asset endpoint and stops on a failed image', async () => {
    const privateImage = JSON.stringify({ type: 'doc', content: [{ type: 'image', attrs: {
      assetId: 9, src: '/api/v1/platform/documents/3/assets/9/content',
    } }] })
    const downloadAsset = vi.fn().mockRejectedValue(new PMSafeApiError('FORBIDDEN', 403))
    const hook = setup({ exportDocument: vi.fn().mockResolvedValue({ ...snapshot, content: privateImage }), downloadAsset })
    await act(() => hook.result.current.run('zip'))
    expect(downloadAsset).toHaveBeenCalledWith(11, 9, expect.any(AbortSignal))
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'asset_failed' })
  })
  it('rechecks asset permissions before releasing a completed ZIP', async () => {
    const privateImage = JSON.stringify({ type: 'doc', content: [{ type: 'image', attrs: {
      assetId: 9, src: '/api/v1/platform/documents/11/assets/9/content',
    } }] })
    const png = new Blob([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], { type: 'image/png' })
    const exportDocument = vi.fn().mockResolvedValueOnce({ ...snapshot, content: privateImage })
      .mockResolvedValueOnce({ ...snapshot, content: privateImage, allowedActions: [PM_ACTION.DocumentsContentView, PM_ACTION.DocumentsContentExport] })
    const hook = setup({ exportDocument, downloadAsset: vi.fn().mockResolvedValue(png) })
    await act(() => hook.result.current.run('zip'))
    expect(exportDocument).toHaveBeenCalledTimes(2)
    expect(saveDocumentDownload).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'permission' })
  })
  it('exports a plain-text ZIP without requiring asset permissions', async () => {
    const textSnapshot = { ...snapshot, allowedActions: [PM_ACTION.DocumentsContentView, PM_ACTION.DocumentsContentExport] }
    const hook = setup({ exportDocument: vi.fn().mockResolvedValue(textSnapshot) })
    await act(() => hook.result.current.run('zip'))
    expect(saveDocumentDownload).toHaveBeenCalledWith(expect.any(Blob), '导出测试.zip')
  })
  it('aborts snapshot requests and discards late completion after unmount', async () => {
    let resolve!: (value: DocumentRecord) => void
    const exportDocument = vi.fn(() => new Promise<DocumentRecord>((done) => { resolve = done }))
    const hook = setup({ exportDocument })
    let pending!: Promise<void>
    act(() => { pending = hook.result.current.run('markdown') })
    const signal = ((exportDocument) as unknown as Mock<typeof exportDocument>).mock.calls[0] as unknown as [number, AbortSignal]
    hook.unmount()
    expect(signal[1].aborted).toBe(true)
    await act(async () => { resolve(snapshot); await pending })
    expect(saveDocumentDownload).not.toHaveBeenCalled()
  })
})
