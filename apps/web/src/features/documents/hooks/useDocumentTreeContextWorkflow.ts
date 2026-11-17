import { useCallback, useEffect, useMemo, useState } from 'react'

import { usePlatformUsersQuery } from '@/shared/query'
import {
  createResourceAccessEnvironment,
  useResourceAccessWorkflow,
  type ResourceAccessWorkflow,
} from '@/features/resource-access'
import {
  copyInternalPath,
  resolveInternalShareHref,
  type UserDirectoryState,
} from '@/shared/ui'
import { documentPath, moveTargetFolders, type DocumentNode } from '../model'
import type { DocumentTreeCapabilities } from '../model/types'
import {
  useCopyDocumentNodeMutation,
  useCreateDocumentShortcutMutation,
  useDocumentQuery,
  useTransferDocumentOwnerMutation,
  useUpdateDocumentNodePreferenceMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'

type SubmitStatus = 'idle' | 'pending' | 'error'

export type DocumentTreeContextDialog =
  | { kind: 'closed' }
  | {
      kind: 'destination'
      action: 'copy' | 'shortcut'
      node: DocumentNode
      parentId?: number
      folders: readonly DocumentNode[]
      submit: SubmitStatus
    }
  | {
      kind: 'share'
      purpose?: 'share' | 'permissions'
      node: DocumentNode
    }
  | {
      kind: 'owner'
      node: DocumentNode
      ownerUserId?: number
      submit: SubmitStatus
      required: boolean
    }

export type DocumentTreeContextControl = {
  openInNewTab: (node: DocumentNode) => void
  beginShare: (node: DocumentNode, purpose?: 'share' | 'permissions') => void
  copyLink: (node: DocumentNode) => Promise<void>
  beginCopy: (node: DocumentNode) => void
  beginShortcut: (node: DocumentNode) => void
  togglePin: (node: DocumentNode) => Promise<void>
  toggleFavorite: (node: DocumentNode) => Promise<void>
  beginTransferOwner: (node: DocumentNode) => void
  updateDestination: (parentId?: number) => void
  updateOwner: (ownerUserId?: number) => void
  submit: () => Promise<void>
  close: () => void
}

export type DocumentTreeContextView = {
  dialog: DocumentTreeContextDialog
  operationError: boolean
  linkCopied: boolean
  owners: UserDirectoryState
  access: ResourceAccessWorkflow
}

export function useDocumentTreeContextWorkflow(input: {
  environment: DocumentsEnvironment
  nodes: readonly DocumentNode[]
  capabilities: DocumentTreeCapabilities
  onOpenDocument: (documentId?: number) => void
}): { state: DocumentTreeContextView; control: DocumentTreeContextControl } {
  const { environment, nodes, capabilities, onOpenDocument } = input
  const copyMutation = useCopyDocumentNodeMutation(environment)
  const shortcutMutation = useCreateDocumentShortcutMutation(environment)
  const preferenceMutation = useUpdateDocumentNodePreferenceMutation(environment)
  const ownerMutation = useTransferDocumentOwnerMutation(environment)
  const [dialog, setDialog] = useState<DocumentTreeContextDialog>({ kind: 'closed' })
  const [operationError, setOperationError] = useState(false)
  const [linkCopied, setLinkCopied] = useState(false)
  const ownerDocumentId = dialog.kind === 'owner' ? dialog.node.documentId ?? 0 : 0
  const documentQuery = useDocumentQuery(environment, ownerDocumentId)
  const usersQuery = usePlatformUsersQuery(environment.workspaceId, dialog.kind === 'owner')
  const shareNode = dialog.kind === 'share' ? dialog.node : undefined
  const accessEnvironment = useMemo(() => createResourceAccessEnvironment({
    workspaceId: environment.workspaceId,
    sessionRevision: environment.sessionRevision,
    resourceId: shareNode?.resourceId ?? '',
  }), [environment.sessionRevision, environment.workspaceId, shareNode?.resourceId])
  const access = useResourceAccessWorkflow({
    environment: accessEnvironment,
    enabled: dialog.kind === 'share',
    sharePath: shareNode?.documentId ? documentPath(shareNode.documentId) : '',
  })

  useEffect(() => {
    setDialog({ kind: 'closed' })
    setOperationError(false)
    setLinkCopied(false)
  }, [environment.sessionRevision, environment.workspaceId])

  useEffect(() => {
    if (dialog.kind !== 'owner' || documentQuery.status !== 'ready' || dialog.ownerUserId) return
    setDialog((current) => current.kind === 'owner'
      ? { ...current, ownerUserId: documentQuery.data.ownerUserId }
      : current)
  }, [dialog, documentQuery])

  const owners = useMemo<UserDirectoryState>(() => {
    if (usersQuery.isPending) return { status: 'loading' }
    if (usersQuery.isError) return { status: 'error', retry: () => { void usersQuery.refetch() } }
    if (!usersQuery.data?.length) return { status: 'empty' }
    return {
      status: 'ready',
      items: usersQuery.data.map((user) => ({
        id: user.id,
        name: user.display_name,
        username: user.username,
        avatarUrl: user.avatar_url,
      })),
    }
  }, [usersQuery.data, usersQuery.isError, usersQuery.isPending, usersQuery.refetch])

  const openInNewTab = useCallback((node: DocumentNode) => {
    if (!node.documentId) return
    setOperationError(false)
    setLinkCopied(false)
    try {
      const href = resolveInternalShareHref(documentPath(node.documentId))
      window.open(href, '_blank', 'noopener,noreferrer')
    } catch {
      setOperationError(true)
    }
  }, [])

  const copyLink = useCallback(async (node: DocumentNode) => {
    if (!node.documentId) return
    setOperationError(false)
    setLinkCopied(false)
    try {
      await copyInternalPath(documentPath(node.documentId))
      setLinkCopied(true)
    } catch {
      setOperationError(true)
    }
  }, [])

  const beginDestination = useCallback((action: 'copy' | 'shortcut', node: DocumentNode) => {
    const allowed = action === 'copy' ? capabilities.canCopy : capabilities.canCreateShortcut
    if (!allowed || node.type !== 'document' || node.systemManaged) return
    setOperationError(false)
    setLinkCopied(false)
    setDialog({
      kind: 'destination',
      action,
      node,
      parentId: node.parentId,
      folders: moveTargetFolders(nodes, node.id),
      submit: 'idle',
    })
  }, [capabilities.canCopy, capabilities.canCreateShortcut, nodes])

  const togglePreference = useCallback(async (
    node: DocumentNode,
    update: { pinned: boolean } | { favorited: boolean },
  ) => {
    if (!capabilities.canUpdatePreferences || node.type !== 'document' || node.systemManaged) return
    setOperationError(false)
    setLinkCopied(false)
    try {
      await preferenceMutation.update({ nodeId: node.id, input: update })
    } catch {
      setOperationError(true)
    }
  }, [capabilities.canUpdatePreferences, preferenceMutation])

  const close = useCallback(() => {
    setDialog((current) => (
      current.kind !== 'closed' && 'submit' in current && current.submit === 'pending'
        ? current
        : { kind: 'closed' }
    ))
  }, [])

  const submit = useCallback(async () => {
    const current = dialog
    if (current.kind === 'destination' && current.submit !== 'pending') {
      setDialog({ ...current, submit: 'pending' })
      try {
        if (current.action === 'copy') {
          const created = await copyMutation.copy({
            nodeId: current.node.id,
            input: { parentId: current.parentId },
          })
          setDialog({ kind: 'closed' })
          if (created.document) onOpenDocument(created.document.id)
        } else {
          await shortcutMutation.create({
            nodeId: current.node.id,
            input: { parentId: current.parentId },
          })
          setDialog({ kind: 'closed' })
        }
      } catch {
        setDialog({ ...current, submit: 'error' })
      }
      return
    }
    if (current.kind !== 'owner' || current.submit === 'pending') return
    if (!current.ownerUserId) {
      setDialog({ ...current, required: true })
      return
    }
    if (!current.node.documentId) return
    setDialog({ ...current, submit: 'pending', required: false })
    try {
      await ownerMutation.transfer({
        documentId: current.node.documentId,
        ownerUserId: current.ownerUserId,
      })
      setDialog({ kind: 'closed' })
    } catch {
      setDialog({ ...current, submit: 'error', required: false })
    }
  }, [copyMutation, dialog, onOpenDocument, ownerMutation, shortcutMutation])

  return {
    state: { dialog, operationError, linkCopied, owners, access },
    control: {
      openInNewTab,
      beginShare: (node, purpose = 'share') => {
        if (!node.documentId) return
        try {
          setOperationError(false)
          setLinkCopied(false)
          setDialog({
            kind: 'share',
            purpose,
            node,
          })
        } catch {
          setOperationError(true)
        }
      },
      copyLink,
      beginCopy: (node) => beginDestination('copy', node),
      beginShortcut: (node) => beginDestination('shortcut', node),
      togglePin: (node) => togglePreference(node, { pinned: !node.pinnedAt }),
      toggleFavorite: (node) => togglePreference(node, { favorited: !node.favorited }),
      beginTransferOwner: (node) => {
        if (!capabilities.canTransferOwnership || node.type !== 'document' || node.systemManaged) return
        setOperationError(false)
        setLinkCopied(false)
        setDialog({ kind: 'owner', node, submit: 'idle', required: false })
      },
      updateDestination: (parentId) => setDialog((current) => current.kind === 'destination'
        ? { ...current, parentId, submit: 'idle' }
        : current),
      updateOwner: (ownerUserId) => setDialog((current) => current.kind === 'owner'
        ? { ...current, ownerUserId, submit: 'idle', required: false }
        : current),
      submit,
      close,
    },
  }
}
