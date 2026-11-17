import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  DocumentArchiveImpactChangedError,
  type DocumentsClient,
} from '../api'
import type {
  DocumentDeletionImpact,
  DocumentNode,
  DocumentRecord,
  DocumentSpace,
} from '../model'
import { useDocumentTreeWorkflow } from './useDocumentTreeWorkflow'

const timestamps = {
  createdAt: '2026-09-03T01:00:00Z',
  updatedAt: '2026-09-03T01:00:00Z',
}

const space: DocumentSpace = {
  id: 2,
  workspaceId: 7,
  name: '文档',
  isDefault: true,
  ...timestamps,
}

const folder: DocumentNode = {
  id: 19,
  workspaceId: 7,
  spaceId: 2,
  type: 'folder',
  title: '架构决策',
  sortOrder: 1024,
  systemManaged: false,
  favorited: false,
  createdBy: 3,
  updatedBy: 3,
  ...timestamps,
}

const documentNode: DocumentNode = {
  ...folder,
  id: 20,
  parentId: 19,
  type: 'document',
  title: '平台 ADR',
  documentId: 11,
}

const documentRecord: DocumentRecord = {
  id: 12,
  workspaceId: 7,
  nodeId: 21,
  content: '{"type":"doc"}',
  schemaVersion: 1,
  revision: 1,
  contentHash: 'hash',
  plainText: '',
  ownerUserId: 3,
  createdBy: 3,
  updatedBy: 3,
  allowedActions: [],
  ...timestamps,
}

function setup(input: {
  initialImpact: DocumentDeletionImpact
  archiveNode: DocumentsClient['archiveNode']
  nodes?: readonly DocumentNode[]
  client?: Partial<DocumentsClient>
}) {
  const client = {
    defaultSpace: vi.fn<DocumentsClient['defaultSpace']>().mockResolvedValue(space),
    tree: vi.fn<DocumentsClient['tree']>().mockResolvedValue({ space, nodes: input.nodes ?? [folder] }),
    deletionImpact: vi.fn<DocumentsClient['deletionImpact']>().mockResolvedValue(input.initialImpact),
    archiveNode: input.archiveNode,
    ...input.client,
  } as unknown as DocumentsClient
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const onOpenDocument = vi.fn()
  const hook = renderHook(() => useDocumentTreeWorkflow({
    environment: { client, workspaceId: 7, sessionRevision: 5 },
    capabilities: {
      canCreate: true,
      canImportMarkdown: true,
      canImportMarkdownAssets: true,
      canCopy: true,
      canCreateShortcut: true,
      canUpdatePreferences: true,
      canTransferOwnership: true,
      canRename: true,
      canMove: true,
      canArchive: true,
    },
    onOpenDocument,
  }), { wrapper })
  return { ...hook, client, queryClient }
}

describe('document tree archive workflow', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    runtimeSessionStore.setState({ revision: 5 })
  })

  it.each([true, false, undefined])('requests collaboration only when server capability is %s', async (collaborationEnabled) => {
    const createNode = vi.fn<DocumentsClient['createNode']>().mockResolvedValue({ node: documentNode, document: documentRecord })
    const hook = setup({
      initialImpact: { nodeId: 20, descendantCount: 0, documentCount: 1, requiresConfirmation: false },
      archiveNode: vi.fn(), client: { createNode, defaultSpace: vi.fn().mockResolvedValue({ ...space, collaborationEnabled }) },
    })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
    act(() => hook.result.current.control.beginCreate('document'))
    await waitFor(() => expect(createNode).toHaveBeenCalled())
    expect(createNode).toHaveBeenLastCalledWith({ parentId: undefined, type: 'document', title: expect.stringMatching(/^未命名文档_\d+$/), ...(collaborationEnabled ? { collaborative: true } : {}) })
    act(() => hook.result.current.control.beginCreate('folder'))
    act(() => hook.result.current.control.updateDialogTitle('文件夹'))
    await act(async () => { await hook.result.current.control.submitDialog() })
    expect(createNode).toHaveBeenLastCalledWith({ parentId: undefined, type: 'folder', title: '文件夹' })
    hook.unmount(); hook.queryClient.clear()
  })

  it('uses canonical context mutations while keeping destination choice in the workflow', async () => {
    const copyNode = vi.fn<DocumentsClient['copyNode']>().mockResolvedValue({
      node: { ...documentNode, id: 21, documentId: 12 },
      document: documentRecord,
    })
    const updateNodePreference = vi.fn<DocumentsClient['updateNodePreference']>().mockResolvedValue({
      nodeId: 20,
      favorited: true,
      updatedAt: timestamps.updatedAt,
    })
    const hook = setup({
      initialImpact: {
        nodeId: 20,
        descendantCount: 0,
        documentCount: 1,
        requiresConfirmation: false,
      },
      archiveNode: vi.fn().mockResolvedValue(undefined),
      nodes: [folder, documentNode],
      client: { copyNode, updateNodePreference },
    })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))

    act(() => hook.result.current.control.context.beginCopy(documentNode))
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      context: { dialog: { kind: 'destination', action: 'copy', parentId: 19 } },
    })
    await act(async () => { await hook.result.current.control.context.submit() })
    expect(copyNode).toHaveBeenCalledWith(20, { parentId: 19 })

    await act(async () => { await hook.result.current.control.context.toggleFavorite(documentNode) })
    expect(updateNodePreference).toHaveBeenCalledWith(20, { favorited: true })
    hook.unmount()
    hook.queryClient.clear()
  })

  it('shows the latest impact after a TOCTOU conflict and requires confirmation again', async () => {
    const initialImpact: DocumentDeletionImpact = {
      nodeId: 19,
      descendantCount: 1,
      documentCount: 1,
      requiresConfirmation: true,
    }
    const latestImpact: DocumentDeletionImpact = {
      nodeId: 19,
      descendantCount: 3,
      documentCount: 2,
      requiresConfirmation: true,
    }
    const archiveNode = vi.fn<DocumentsClient['archiveNode']>()
      .mockRejectedValueOnce(new DocumentArchiveImpactChangedError(latestImpact))
      .mockResolvedValueOnce(undefined)
    const hook = setup({ initialImpact, archiveNode })
    await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))

    await act(async () => {
      await hook.result.current.control.beginArchive(folder)
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      dialog: { kind: 'archive', impact: { status: 'ready', data: initialImpact } },
    })

    await act(async () => {
      await hook.result.current.control.submitDialog()
    })
    expect(archiveNode).toHaveBeenNthCalledWith(1, 19, {
      confirmNonEmpty: true,
      expectedDescendantCount: 1,
      expectedDocumentCount: 1,
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      dialog: {
        kind: 'archive',
        impact: { status: 'ready', data: latestImpact, changed: true },
        submit: 'idle',
      },
    })

    await act(async () => {
      await hook.result.current.control.submitDialog()
    })
    expect(archiveNode).toHaveBeenNthCalledWith(2, 19, {
      confirmNonEmpty: true,
      expectedDescendantCount: 3,
      expectedDocumentCount: 2,
    })
    expect(hook.result.current.state).toMatchObject({
      status: 'ready',
      dialog: { kind: 'closed' },
    })
    hook.unmount()
    hook.queryClient.clear()
  })
})

it('moves a document through the existing mutation and reports a rejected folder move', async () => {
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
  runtimeSessionStore.setState({ revision: 5 })
  const moveNode = vi.fn<DocumentsClient['moveNode']>().mockResolvedValue(documentNode)
  const destination = { ...folder, id: 21 }
  const hook = setup({
    initialImpact: { nodeId: 20, descendantCount: 0, documentCount: 1, requiresConfirmation: false },
    archiveNode: vi.fn(), nodes: [folder, documentNode, destination], client: { moveNode },
  })
  await waitFor(() => expect(hook.result.current.state.status).toBe('ready'))
  await act(async () => { await hook.result.current.control.moveNode(20, 21) })
  expect(moveNode).toHaveBeenCalledWith(20, { parentId: 21, expectedDirectoryRevision: undefined })
  await act(async () => { await hook.result.current.control.moveNode(19, 19) })
  expect(moveNode).toHaveBeenCalledTimes(1)
  moveNode.mockRejectedValueOnce(new Error('forbidden'))
  await act(async () => { await hook.result.current.control.moveNode(19, 21) })
  expect(hook.result.current.state).toMatchObject({ status: 'ready', reorderError: true })
})
