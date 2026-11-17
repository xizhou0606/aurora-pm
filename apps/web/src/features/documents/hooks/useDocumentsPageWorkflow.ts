import { useMemo } from 'react'
import { useDocumentExportWorkflow, type DocumentExportWorkflow } from './useDocumentExportWorkflow'

import {
  useDocumentCommentsWorkflow,
  type DocumentCommentsActor,
  type DocumentCommentsWorkflow,
} from './useDocumentCommentsWorkflow'
import {
  useDocumentAssetsWorkflow,
  type DocumentAssetsControl,
  type DocumentAssetsViewState,
} from './useDocumentAssetsWorkflow'
import {
  useDocumentEditorWorkflow,
  type DocumentEditorControl,
  type DocumentEditorViewState,
} from './useDocumentEditorWorkflow'
import {
  useDocumentOperationAccess,
  type DocumentOperationAccess,
} from './useDocumentOperationAccess'
import type { DocumentsEnvironment } from './useDocumentQueries'
import {
  useDocumentTreeWorkflow,
  type DocumentTreeControl,
  type DocumentTreeViewState,
} from './useDocumentTreeWorkflow'

export type DocumentsPageViewState =
  | { status: 'loading' }
  | { status: 'unavailable'; reason: 'workspace' | 'permission' | 'access_error'; retry?: () => void }
  | {
      status: 'ready'
      tree: DocumentTreeViewState
      editor: DocumentEditorViewState
      assets: DocumentAssetsViewState
      comments: Omit<DocumentCommentsWorkflow, 'control'>
      exporting?: DocumentExportWorkflow
    }

export type DocumentsPageControl = {
  tree: DocumentTreeControl
  editor: DocumentEditorControl
  assets: DocumentAssetsControl
  comments: DocumentCommentsWorkflow['control']
}

const deniedTreeCapabilities = {
  canCreate: false,
  canImportMarkdown: false,
  canImportMarkdownAssets: false,
  canCopy: false,
  canCreateShortcut: false,
  canUpdatePreferences: false,
  canTransferOwnership: false,
  canRename: false,
  canMove: false,
  canArchive: false,
} as const

function resolvePageState(
  workspaceId: number,
  access: DocumentOperationAccess,
  tree: DocumentTreeViewState,
  editor: DocumentEditorViewState,
  assets: DocumentAssetsViewState,
  comments: DocumentCommentsWorkflow,
  exporting: DocumentExportWorkflow,
): DocumentsPageViewState {
  if (workspaceId <= 0) return { status: 'unavailable', reason: 'workspace' }
  if (access.status === 'loading') return { status: 'loading' }
  if (access.status === 'error') {
    return { status: 'unavailable', reason: 'access_error', retry: access.retry }
  }
  if (!access.canViewTree) return { status: 'unavailable', reason: 'permission' }
  return {
    status: 'ready',
    tree,
    editor,
    assets,
    exporting,
    comments: {
      state: comments.state,
      mutation: comments.mutation,
      capabilities: comments.capabilities,
      composer: comments.composer,
    },
  }
}

export function useDocumentsPageWorkflow(input: {
  environment: DocumentsEnvironment
  documentId?: number
  onOpenDocument: (documentId?: number) => void
  actor?: DocumentCommentsActor
}): { state: DocumentsPageViewState; control: DocumentsPageControl } {
  const { environment, documentId, onOpenDocument, actor } = input
  const access = useDocumentOperationAccess()
  const enabled = environment.workspaceId > 0
    && access.status === 'ready'
    && access.canViewTree
  const queryEnvironment = useMemo<DocumentsEnvironment>(() => ({
    ...environment,
    workspaceId: enabled ? environment.workspaceId : 0,
  }), [enabled, environment])
  const tree = useDocumentTreeWorkflow({
    environment: queryEnvironment,
    selectedDocumentId: documentId,
    capabilities: enabled ? access.tree : deniedTreeCapabilities,
    onOpenDocument,
  })
  const editor = useDocumentEditorWorkflow(
    queryEnvironment,
    enabled ? documentId ?? 0 : 0,
  )
  const assets = useDocumentAssetsWorkflow({
    environment: queryEnvironment,
    documentId: enabled ? documentId ?? 0 : 0,
    capabilities: editor.state.status === 'ready' ? editor.state.capabilities : undefined,
  })
  const comments = useDocumentCommentsWorkflow({
    environment: queryEnvironment,
    documentId: enabled ? documentId ?? 0 : 0,
    documentCapabilities: editor.state.status === 'ready' ? editor.state.capabilities : undefined,
    actor,
  })
  const exporting = useDocumentExportWorkflow({
    environment: queryEnvironment,
    editor: editor.state,
    title: tree.state.status === 'ready' ? tree.state.selectedNode?.title ?? '' : '',
  })

  return {
    state: resolvePageState(
      environment.workspaceId,
      access,
      tree.state,
      editor.state,
      assets.state,
      comments,
      exporting,
    ),
    control: { tree: tree.control, editor: editor.control, assets: assets.control, comments: comments.control },
  }
}
