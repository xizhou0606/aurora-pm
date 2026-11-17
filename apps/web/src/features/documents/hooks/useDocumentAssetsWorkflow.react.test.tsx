import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { beforeEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type { DocumentsClient } from '../api'
import type { DocumentAsset, DocumentCapabilities } from '../model'
import { useDocumentAssetsWorkflow } from './useDocumentAssetsWorkflow'

const asset: DocumentAsset = {
  id: 23,
  workspaceId: 7,
  documentId: 11,
  fileName: 'decision.pdf',
  contentType: 'application/pdf',
  sizeBytes: 2048,
  contentHash: 'hash',
  createdBy: 5,
  createdAt: '2026-09-03T01:00:00Z',
  updatedAt: '2026-09-03T01:00:00Z',
}

const capabilities: DocumentCapabilities = {
  canView: true,
  canExport: false,
  canUpdate: true,
  canViewAssets: true,
  canUploadAssets: true,
  canDownloadAssets: true,
      canDeleteAssets: true,
      canViewComments: false,
      canCreateComments: false,
      canReplyComments: false,
}

function setup(input: {
  client?: Partial<DocumentsClient>
  capabilities?: DocumentCapabilities
} = {}) {
  const client = {
    listAssets: vi.fn<DocumentsClient['listAssets']>().mockResolvedValue([asset]),
    uploadAsset: vi.fn<DocumentsClient['uploadAsset']>().mockResolvedValue(asset),
    downloadAsset: vi.fn<DocumentsClient['downloadAsset']>().mockResolvedValue(new Blob(['asset'])),
    deleteAsset: vi.fn<DocumentsClient['deleteAsset']>().mockResolvedValue(undefined),
    ...input.client,
  } as unknown as DocumentsClient
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useDocumentAssetsWorkflow({
    environment: { client, workspaceId: 7, sessionRevision: 5 },
    documentId: 11,
    capabilities: input.capabilities ?? capabilities,
  }), { wrapper })
  return { ...hook, client, queryClient }
}

describe('document assets workflow', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 5 })
  })

  it('fails closed without the server-projected view capability', async () => {
    const hook = setup({
      capabilities: { ...capabilities, canViewAssets: false },
    })

    expect(hook.result.current.state).toEqual({ status: 'hidden' })
    expect(hook.client.listAssets).not.toHaveBeenCalled()
    await expect(hook.result.current.control.download(asset.id)).resolves.toBeNull()
    await expect(hook.result.current.control.loadThumbnail(asset.id)).resolves.toBeNull()
    expect(hook.client.downloadAsset).not.toHaveBeenCalled()
    hook.unmount()
    hook.queryClient.clear()
  })

  it('exposes download progress and restores idle after protected content resolves', async () => {
    let resolveDownload: ((blob: Blob) => void) | undefined
    const download = vi.fn<DocumentsClient['downloadAsset']>().mockImplementation(() => (
      new Promise<Blob>((resolve) => { resolveDownload = resolve })
    ))
    const hook = setup({ client: { downloadAsset: download } })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))

    let request: Promise<Blob | null> | undefined
    act(() => {
      request = hook.result.current.control.download(asset.id)
    })
    await waitFor(() => expect(hook.result.current.state).toMatchObject({
      status: 'ready', operation: 'downloading',
    }))
    await act(async () => {
      resolveDownload?.(new Blob(['asset']))
      await request
    })
    expect(download).toHaveBeenCalledWith(11, asset.id)
    expect(hook.result.current.state).toMatchObject({ status: 'ready', operation: 'idle' })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('loads protected thumbnails without disabling transfers and isolates thumbnail failures', async () => {
    const hook = setup()
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    await act(async () => {
      expect(await hook.result.current.control.loadThumbnail(asset.id)).toBeInstanceOf(Blob)
    })
    expect(hook.client.downloadAsset).toHaveBeenCalledWith(11, asset.id)
    ;((hook.client.downloadAsset) as unknown as Mock<typeof hook.client.downloadAsset>).mockRejectedValueOnce(new Error('preview unavailable'))
    await act(async () => {
      await expect(hook.result.current.control.loadThumbnail(asset.id)).resolves.toBeNull()
    })
    expect(hook.result.current.state).toMatchObject({ status: 'ready', operation: 'idle' })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('rejects invalid uploads before transport and exposes recoverable operation errors', async () => {
    const hook = setup()
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))

    await act(async () => {
      await hook.result.current.control.upload(new File([], 'empty.txt'))
    })
    expect(hook.client.uploadAsset).not.toHaveBeenCalled()
    expect(hook.result.current.state).toMatchObject({ status: 'ready', operation: 'error' })

    act(() => hook.result.current.control.clearError())
    expect(hook.result.current.state).toMatchObject({ status: 'ready', operation: 'idle' })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('refreshes the scoped asset list after upload and deletion', async () => {
    const hook = setup()
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    const file = new File(['attachment'], 'decision.txt', { type: 'text/plain' })

    await act(async () => {
      await expect(hook.result.current.control.upload(file)).resolves.toBe(true)
    })
    expect(hook.client.uploadAsset).toHaveBeenCalledWith(11, file)
    await waitFor(() => expect(hook.client.listAssets).toHaveBeenCalledTimes(2))

    await act(async () => {
      await expect(hook.result.current.control.remove(asset.id)).resolves.toBe(true)
    })
    expect(hook.client.deleteAsset).toHaveBeenCalledWith(11, asset.id)
    await waitFor(() => expect(hook.client.listAssets).toHaveBeenCalledTimes(3))
    hook.unmount()
    hook.queryClient.clear()
  })

  it('fails closed when the list response contains another workspace asset', async () => {
    const hook = setup({
      client: { listAssets: vi.fn().mockResolvedValue([{ ...asset, workspaceId: 8 }]) },
    })

    await waitFor(() => expect(hook.result.current.state.status).toBe('error'))
    hook.unmount()
    hook.queryClient.clear()
  })
})
