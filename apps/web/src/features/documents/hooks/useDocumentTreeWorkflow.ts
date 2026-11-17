import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { notifyOperationError, operationFeedback } from '@/shared/ui'

import { DocumentArchiveImpactChangedError } from '../api'
import {
  allFolderIds,
  documentNodeFor,
  moveTargetFolders,
  visibleDocumentTreeRows,
  type DocumentTreeRow,
} from '../model/tree'
import type {
  DocumentDeletionImpact,
  DocumentNode,
  CreatableDocumentNodeType,
  DocumentSpace,
  DocumentTreeCapabilities,
} from '../model/types'
import {
  useArchiveDocumentNodeMutation,
  useCreateDocumentNodeMutation,
  useDefaultDocumentSpaceQuery,
  useDocumentDeletionImpactMutation,
  useDocumentTreeQuery,
  useMoveDocumentNodeMutation,
  useRenameDocumentNodeMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'
import {
  useDocumentTreeContextWorkflow,
  type DocumentTreeContextControl,
  type DocumentTreeContextView,
} from './useDocumentTreeContextWorkflow'
import {
  useDocumentMarkdownImportWorkflow,
  type DocumentMarkdownImportControl,
  type DocumentMarkdownImportState,
} from './useDocumentMarkdownImportWorkflow'

type SubmitStatus = 'idle' | 'pending' | 'error'

export type DocumentTreeDialog =
  | { kind: 'closed' }
  | {
      kind: 'create'
      nodeType: CreatableDocumentNodeType
      parentId?: number
      title: string
      submit: SubmitStatus
      error?: 'required' | 'request'
    }
  | {
      kind: 'rename'
      node: DocumentNode
      title: string
      submit: SubmitStatus
      error?: 'required' | 'request'
    }
  | {
      kind: 'move'
      node: DocumentNode
      parentId?: number
      folders: readonly DocumentNode[]
      submit: SubmitStatus
    }
  | {
      kind: 'archive'
      node: DocumentNode
      impact:
        | { status: 'loading' }
        | { status: 'error' }
        | { status: 'ready'; data: DocumentDeletionImpact; changed?: boolean }
      submit: SubmitStatus
    }

export type DocumentTreeViewState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => Promise<void> }
  | {
      status: 'ready'
      space: DocumentSpace
      nodes: readonly DocumentNode[]
      rows: readonly DocumentTreeRow[]
      selectedNode?: DocumentNode
      refresh: 'idle' | 'refreshing' | 'stale_error'
      dialog: DocumentTreeDialog
      reorderError: boolean
      capabilities: DocumentTreeCapabilities
      context: DocumentTreeContextView
      markdownImport: DocumentMarkdownImportState
    }

export type DocumentTreeControl = {
  context: DocumentTreeContextControl
  markdownImport: DocumentMarkdownImportControl
  toggleFolder: (nodeId: number) => void
  openDocument: (documentId: number) => void
  beginCreate: (type: CreatableDocumentNodeType, parentId?: number) => void
  beginRename: (node: DocumentNode) => void
  renameNode: (node: DocumentNode, title: string) => Promise<boolean>
  beginMove: (node: DocumentNode) => void
  moveNode: (nodeId: number, parentId?: number, placement?: { beforeNodeId?: number; afterNodeId?: number }) => Promise<void>
  beginArchive: (node: DocumentNode) => Promise<void>
  updateDialogTitle: (title: string) => void
  updateMoveTarget: (parentId?: number) => void
  submitDialog: () => Promise<void>
  closeDialog: () => void
}

export function useDocumentTreeWorkflow(input: {
  environment: DocumentsEnvironment
  selectedDocumentId?: number
  capabilities: DocumentTreeCapabilities
  onOpenDocument: (documentId?: number) => void
}): { state: DocumentTreeViewState; control: DocumentTreeControl } {
  const { environment, selectedDocumentId, capabilities, onOpenDocument } = input
  const spaceQuery = useDefaultDocumentSpaceQuery(environment)
  const spaceId = spaceQuery.status === 'ready' ? spaceQuery.data.id : 0
  const canCreateCollaboratively = spaceQuery.status === 'ready' && spaceQuery.data.collaborationEnabled === true
  const treeQuery = useDocumentTreeQuery(environment, spaceId)
  const createMutation = useCreateDocumentNodeMutation(environment)
  const renameMutation = useRenameDocumentNodeMutation(environment)
  const moveMutation = useMoveDocumentNodeMutation(environment)
  const impactMutation = useDocumentDeletionImpactMutation(environment)
  const archiveMutation = useArchiveDocumentNodeMutation(environment)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const initializedSpaceRef = useRef(0)
  const archiveRequestRef = useRef(0)
  const creatingRef = useRef(false)
  const createGeneration = useRef(0)
  const [dialog, setDialog] = useState<DocumentTreeDialog>({ kind: 'closed' })
  const [reorderError, setReorderError] = useState(false)

  const nodes = treeQuery.status === 'ready' ? treeQuery.data.nodes : []
  const context = useDocumentTreeContextWorkflow({
    environment,
    nodes,
    capabilities,
    onOpenDocument,
  })
  const markdownImport = useDocumentMarkdownImportWorkflow({
    environment,
    canImport: capabilities.canImportMarkdown,
    canImportAssets: capabilities.canImportMarkdownAssets,
    onOpenDocument,
  })

  useEffect(() => {
    if (!spaceId || treeQuery.status !== 'ready') return
    if (initializedSpaceRef.current === spaceId) return
    initializedSpaceRef.current = spaceId
    setExpanded(new Set(allFolderIds(treeQuery.data.nodes)))
  }, [spaceId, treeQuery])

  useEffect(() => {
    createGeneration.current += 1
    creatingRef.current = false
    archiveRequestRef.current += 1
    initializedSpaceRef.current = 0
    setExpanded(new Set())
    setDialog({ kind: 'closed' })
    setReorderError(false)
    return () => { createGeneration.current += 1 }
  }, [environment.sessionRevision, environment.workspaceId])

  const toggleFolder = useCallback((nodeId: number) => {
    setExpanded((current) => {
      const next = new Set(current)
      if (next.has(nodeId)) next.delete(nodeId)
      else next.add(nodeId)
      return next
    })
  }, [])

  const beginCreate = useCallback((nodeType: CreatableDocumentNodeType, parentId?: number) => {
    if (!capabilities.canCreate) return
    if (parentId && nodes.find((node) => node.id === parentId)?.systemManaged) return
    if (nodeType === 'document') {
      if (creatingRef.current) return
      creatingRef.current = true
      const generation = createGeneration.current
      void createMutation.create({ parentId, type: nodeType, title: `${DOCUMENTS_COPY.editor.untitled}_${Date.now()}`,
        ...(canCreateCollaboratively ? { collaborative: true } : {}),
      }).then(created => {
        if (generation !== createGeneration.current) return
        if (created.node.parentId) setExpanded(value => new Set(value).add(created.node.parentId!))
        if (created.document) onOpenDocument(created.document.id)
      }).catch(error => {
        if (generation === createGeneration.current) notifyOperationError(operationFeedback('create'), error)
      }).finally(() => { if (generation === createGeneration.current) creatingRef.current = false })
      return
    }
    setDialog({ kind: 'create', nodeType, parentId, title: '', submit: 'idle' })
  }, [capabilities.canCreate, nodes, createMutation, canCreateCollaboratively, onOpenDocument])

  const beginRename = useCallback((node: DocumentNode) => {
    if (!capabilities.canRename || node.systemManaged || node.type === 'shortcut') return
    setDialog({ kind: 'rename', node, title: node.title, submit: 'idle' })
  }, [capabilities.canRename])

  const renameNode = useCallback(async (node: DocumentNode, rawTitle: string) => {
    if (!capabilities.canRename || node.systemManaged || node.type === 'shortcut') return false
    const title = rawTitle.trim()
    if (!title) return false
    if (title === node.title) return true
    try {
      await renameMutation.rename({ nodeId: node.id, title })
      return true
    } catch {
      return false
    }
  }, [capabilities.canRename, renameMutation])

  const beginMove = useCallback((node: DocumentNode) => {
    if (!capabilities.canMove || node.systemManaged) return
    setDialog({
      kind: 'move',
      node,
      parentId: node.parentId,
      folders: moveTargetFolders(nodes, node.id),
      submit: 'idle',
    })
  }, [capabilities.canMove, nodes])

  const beginArchive = useCallback(async (node: DocumentNode) => {
    if (!capabilities.canArchive || node.systemManaged) return
    const requestId = archiveRequestRef.current + 1
    archiveRequestRef.current = requestId
    setDialog({ kind: 'archive', node, impact: { status: 'loading' }, submit: 'idle' })
    try {
      const impact = await impactMutation.loadImpact(node.id)
      if (archiveRequestRef.current !== requestId) return
      setDialog({ kind: 'archive', node, impact: { status: 'ready', data: impact }, submit: 'idle' })
    } catch {
      if (archiveRequestRef.current !== requestId) return
      setDialog({ kind: 'archive', node, impact: { status: 'error' }, submit: 'idle' })
    }
  }, [capabilities.canArchive, impactMutation])

  const updateDialogTitle = useCallback((title: string) => {
    setDialog((current) => (
      current.kind === 'create' || current.kind === 'rename'
        ? { ...current, title, submit: 'idle', error: undefined }
        : current
    ))
  }, [])

  const updateMoveTarget = useCallback((parentId?: number) => {
    setDialog((current) => current.kind === 'move'
      ? { ...current, parentId, submit: 'idle' }
      : current)
  }, [])

  const closeDialog = useCallback(() => {
    setDialog((current) => {
      if (current.kind !== 'closed' && current.submit === 'pending') return current
      archiveRequestRef.current += 1
      return { kind: 'closed' }
    })
  }, [])

  const submitDialog = useCallback(async () => {
    const current = dialog
    if (current.kind === 'closed' || current.submit === 'pending') return
    if (current.kind === 'create') {
      if (!capabilities.canCreate) return
      const title = current.title.trim()
      if (!title) {
        setDialog({ ...current, error: 'required' })
        return
      }
      setDialog({ ...current, title, submit: 'pending', error: undefined })
      try {
        const created = await createMutation.create({
          parentId: current.parentId,
          type: current.nodeType,
          title,
          ...(current.nodeType === 'document' && canCreateCollaboratively ? { collaborative: true } : {}),
        })
        setDialog({ kind: 'closed' })
        if (created.node.parentId) {
          setExpanded((value) => new Set(value).add(created.node.parentId as number))
        }
        if (created.document) onOpenDocument(created.document.id)
      } catch {
        setDialog({ ...current, title, submit: 'error', error: 'request' })
      }
      return
    }
    if (current.kind === 'rename') {
      if (!capabilities.canRename) return
      const title = current.title.trim()
      if (!title) {
        setDialog({ ...current, error: 'required' })
        return
      }
      setDialog({ ...current, title, submit: 'pending', error: undefined })
      try {
        await renameMutation.rename({ nodeId: current.node.id, title })
        setDialog({ kind: 'closed' })
      } catch {
        setDialog({ ...current, title, submit: 'error', error: 'request' })
      }
      return
    }
    if (current.kind === 'move') {
      if (!capabilities.canMove) return
      setDialog({ ...current, submit: 'pending' })
      try {
        await moveMutation.move({
          nodeId: current.node.id,
          input: { parentId: current.parentId, expectedDirectoryRevision: current.node.directoryRevision },
        })
        if (current.parentId) setExpanded((value) => new Set(value).add(current.parentId as number))
        setDialog({ kind: 'closed' })
      } catch (error) {
        setDialog({ ...current, submit: 'error' })
      }
      return
    }
    if (current.kind === 'archive' && current.impact.status === 'ready') {
      if (!capabilities.canArchive) return
      const requestId = archiveRequestRef.current
      setDialog({ ...current, submit: 'pending' })
      try {
        await archiveMutation.archive({
          nodeId: current.node.id,
          impact: current.impact.data,
        })
        if (archiveRequestRef.current !== requestId) return
        if (current.node.documentId === selectedDocumentId) onOpenDocument(undefined)
        setDialog({ kind: 'closed' })
      } catch (error) {
        if (archiveRequestRef.current !== requestId) return
        if (error instanceof DocumentArchiveImpactChangedError) {
          const latest = error.latestImpact?.nodeId === current.node.id
            ? error.latestImpact
            : await impactMutation.loadImpact(current.node.id).catch(() => undefined)
          if (archiveRequestRef.current !== requestId) return
          setDialog(latest
            ? {
                ...current,
                impact: { status: 'ready', data: latest, changed: true },
                submit: 'idle',
              }
            : { ...current, impact: { status: 'error' }, submit: 'idle' })
          return
        }
        setDialog({ ...current, submit: 'error' })
      }
    }
  }, [
    archiveMutation,
    capabilities,
    createMutation,
    canCreateCollaboratively,
    dialog,
    impactMutation,
    moveMutation,
    onOpenDocument,
    renameMutation,
    selectedDocumentId,
  ])

  const movingRef = useRef(false)
  const moveNode = useCallback(async (nodeId: number, parentId?: number, placement?: { beforeNodeId?: number; afterNodeId?: number }) => {
    const node = nodes.find(candidate => candidate.id === nodeId)
    if (!capabilities.canMove || movingRef.current || !node || node.canMove === false || node.systemManaged || (node.parentId === parentId && placement === undefined)) return
    if (parentId !== undefined && !moveTargetFolders(nodes, nodeId).some(folder => folder.id === parentId)) return
    movingRef.current = true
    const generation = createGeneration.current
    setReorderError(false)
    try {
      await moveMutation.move({ nodeId, input: { parentId, ...placement, expectedDirectoryRevision: node.directoryRevision } })
      if (generation === createGeneration.current && parentId !== undefined) {
        setExpanded(value => new Set(value).add(parentId))
      }
    } catch (error) {
      if (generation === createGeneration.current) setReorderError(true)
    } finally {
      movingRef.current = false
    }
  }, [capabilities.canMove, moveMutation, nodes])


  const state = useMemo<DocumentTreeViewState>(() => {
    if (spaceQuery.status === 'error') {
      return { status: 'error', retry: async () => { await spaceQuery.retry() } }
    }
    if (spaceQuery.status !== 'ready' || treeQuery.status === 'loading') return { status: 'loading' }
    if (treeQuery.status === 'error') {
      return { status: 'error', retry: async () => { await treeQuery.retry() } }
    }
    if (
      treeQuery.data.space.id !== spaceQuery.data.id
      || treeQuery.data.space.workspaceId !== environment.workspaceId
    ) {
      return { status: 'error', retry: async () => { await treeQuery.retry() } }
    }
    return {
      status: 'ready',
      space: treeQuery.data.space,
      nodes: treeQuery.data.nodes,
      rows: visibleDocumentTreeRows(treeQuery.data.nodes, expanded, selectedDocumentId),
      selectedNode: documentNodeFor(treeQuery.data.nodes, selectedDocumentId),
      refresh: treeQuery.refresh === 'stale_error' || spaceQuery.refresh === 'stale_error'
        ? 'stale_error'
        : treeQuery.refresh === 'refreshing' || spaceQuery.refresh === 'refreshing'
          ? 'refreshing'
          : 'idle',
      dialog,
      reorderError,
      capabilities,
      context: context.state,
      markdownImport: markdownImport.state,
    }
  }, [capabilities, context.state, dialog, environment.workspaceId, expanded, markdownImport.state, reorderError, selectedDocumentId, spaceQuery, treeQuery])

  return {
    state,
    control: {
      context: context.control,
      markdownImport: markdownImport.control,
      toggleFolder,
      openDocument: (id) => onOpenDocument(id),
      beginCreate,
      beginRename,
      renameNode,
      beginMove,
      moveNode,
      beginArchive,
      updateDialogTitle,
      updateMoveTarget,
      submitDialog,
      closeDialog,
    },
  }
}
