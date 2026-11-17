import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { zipSync } from 'fflate'
import type { PropsWithChildren } from 'react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type { DocumentsClient } from '../api'
import type { CreatedDocumentNode, DocumentAsset, DocumentRecord } from '../model'
import { useDocumentMarkdownImportWorkflow } from './useDocumentMarkdownImportWorkflow'

const encoder = new TextEncoder()
const png = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])

function documentRecord(overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id: 11,
    workspaceId: 7,
    nodeId: 19,
    content: '{"type":"doc","content":[{"type":"paragraph"}]}',
    schemaVersion: 1,
    revision: 1,
    contentHash: 'hash',
    plainText: '',
    ownerUserId: 5,
    createdBy: 5,
    updatedBy: 5,
    createdAt: '2026-09-04T00:00:00Z',
    updatedAt: '2026-09-04T00:00:00Z',
    allowedActions: [],
    ...overrides,
  }
}

function createdResult(): CreatedDocumentNode {
  return {
    node: {
      id: 19,
      workspaceId: 7,
      spaceId: 2,
      type: 'document',
      title: '导入文档',
      sortOrder: 0,
      documentId: 11,
      systemManaged: false,
      favorited: false,
      createdBy: 5,
      updatedBy: 5,
      createdAt: '2026-09-04T00:00:00Z',
      updatedAt: '2026-09-04T00:00:00Z',
    },
    document: documentRecord(),
  }
}

function setup(client: Partial<DocumentsClient>, permissions = { canImport: true, canImportAssets: true }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const onOpenDocument = vi.fn()
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useDocumentMarkdownImportWorkflow({
    environment: { client: client as DocumentsClient, workspaceId: 7, sessionRevision: 3 },
    ...permissions,
    onOpenDocument,
  }), { wrapper })
  return { ...hook, onOpenDocument }
}

describe('document Markdown import workflow', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 3 })
  })

  it('creates a schema-v2 document and opens it after saving', async () => {
    const createNode = vi.fn().mockResolvedValue(createdResult())
    const updateDocument = vi.fn().mockImplementation(async (_id, input) => documentRecord({
      content: input.content,
      schemaVersion: input.schemaVersion,
      revision: 2,
    }))
    const hook = setup({ createNode, updateDocument })

    await act(async () => {
      await hook.result.current.control.importFile(new File([
        encoder.encode('# 导入文档\n\n| A | B |\n| - | - |\n| 1 | 2 |'),
      ], 'document.md', { type: 'text/markdown' }))
    })

    await waitFor(() => expect(hook.result.current.state.status).toBe('success'))
    expect(createNode).toHaveBeenCalledWith({ type: 'document', title: '导入文档' })
    expect(updateDocument).toHaveBeenCalledWith(11, expect.objectContaining({
      expectedRevision: 1,
      schemaVersion: 2,
    }))
    expect(hook.onOpenDocument).toHaveBeenCalledWith(11)
  })

  it('uploads bundled images once and rewrites every matching image source', async () => {
    const archive = zipSync({
      'doc.md': encoder.encode('# Doc\n\n![one](assets/image.png)\n\n![two](assets/image.png)'),
      'assets/image.png': png,
    })
    const uploadAsset = vi.fn().mockResolvedValue({
      id: 9,
      workspaceId: 7,
      documentId: 11,
      fileName: 'image.png',
      contentType: 'image/png',
      sizeBytes: 8,
      contentHash: 'asset-hash',
      createdBy: 5,
      createdAt: '2026-09-04T00:00:00Z',
      updatedAt: '2026-09-04T00:00:00Z',
    } satisfies DocumentAsset)
    const updateDocument = vi.fn().mockResolvedValue(documentRecord({ schemaVersion: 2, revision: 2 }))
    const hook = setup({
      createNode: vi.fn().mockResolvedValue(createdResult()),
      uploadAsset,
      updateDocument,
    })

    await act(async () => {
      await hook.result.current.control.importFile(
        new File([archive], 'bundle.zip', { type: 'application/zip' }),
      )
    })

    expect(uploadAsset).toHaveBeenCalledTimes(1)
    const content = JSON.parse(updateDocument.mock.calls[0][1].content)
    const images = content.content.filter((node: { type: string }) => node.type === 'image')
    expect(images).toHaveLength(2)
    expect(images.every((node: { attrs: { assetId: number } }) => node.attrs.assetId === 9)).toBe(true)
  })

  it('fails before creating a document when image import permission is missing', async () => {
    const archive = zipSync({
      'doc.md': encoder.encode('![one](assets/image.png)'),
      'assets/image.png': png,
    })
    const createNode = vi.fn()
    const hook = setup({ createNode }, { canImport: true, canImportAssets: false })

    await act(async () => {
      await hook.result.current.control.importFile(
        new File([archive], 'bundle.zip', { type: 'application/zip' }),
      )
    })

    expect(hook.result.current.state).toEqual({ status: 'error', reason: 'permission' })
    expect(createNode).not.toHaveBeenCalled()
  })
})
