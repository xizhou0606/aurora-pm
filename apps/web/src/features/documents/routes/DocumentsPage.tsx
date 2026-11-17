import { useNavigate, useParams, useRouterState } from '@tanstack/react-router'
import { useCallback, useMemo, type ReactNode } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { useShareLinkAccessEntry } from '@/features/resource-access'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy'
import { AlertBanner, EmptyState, LoadingSkeleton, OperationalPage } from '@/shared/ui'
import { productionDocumentCollaborationFactory } from '../api'
import { DocumentsWorkspace } from '../_components'
import { createDocumentsEnvironment, useDocumentsPageWorkflow } from '../hooks'
import { documentPath, parseDocumentCommentHash } from '../model'
import type { DocumentResourceEntries } from '../model/resourceSlot'

function positiveDocumentId(value: unknown): number | undefined {
  const id = Number(value)
  return Number.isSafeInteger(id) && id > 0 ? id : undefined
}

export type DocumentsPageSlots = {
  resourceEntries?: DocumentResourceEntries
  renderCreate?: (createDocument: () => void) => ReactNode
  alternativeContent?: ReactNode
}
export function DocumentsPage(slots: DocumentsPageSlots = {}) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const sessionRevision = useSessionStore((state) => state.revision)
  const user = useSessionStore((state) => state.user)
  const params = useParams({ strict: false }) as { documentId?: string }
  const documentId = positiveDocumentId(params.documentId)
  const shareAccess = useShareLinkAccessEntry(documentId && workspaceId > 0 ? {
    workspaceId,
    sessionRevision,
    kind: 'document',
    key: String(documentId),
  } : undefined)
  const commentThreadId = useRouterState({
    select: (state) => parseDocumentCommentHash(state.location.hash),
  })
  const navigate = useNavigate()
  const environment = useMemo(
    () => createDocumentsEnvironment({
      workspaceId: shareAccess.status === 'ready' ? workspaceId : 0,
      sessionRevision,
    }, undefined, productionDocumentCollaborationFactory),
    [sessionRevision, shareAccess.status, workspaceId],
  )
  const onOpenDocument = useCallback((nextDocumentId?: number) => {
    void navigate({ to: documentPath(nextDocumentId) })
  }, [navigate])
  const actor = user ? {
    id: user.id,
    name: user.display_name?.trim() || user.username,
    username: user.username,
    avatarUrl: user.avatar_url,
  } : undefined
  const workflow = useDocumentsPageWorkflow({
    environment,
    documentId,
    onOpenDocument,
    actor,
  })
  if (shareAccess.status === 'checking') return <OperationalPage title={RESOURCE_ACCESS_COPY.links.checking}><LoadingSkeleton rows={6} label={RESOURCE_ACCESS_COPY.links.checking} /></OperationalPage>
  if (shareAccess.status === 'error') return <OperationalPage title={RESOURCE_ACCESS_COPY.links.invalidTitle}><div className="grid gap-3 p-5"><EmptyState title={RESOURCE_ACCESS_COPY.links.invalidTitle} /><AlertBanner tone="error">{RESOURCE_ACCESS_COPY.links.invalidDescription}</AlertBanner></div></OperationalPage>
  return (
    <DocumentsWorkspace
      state={workflow.state}
      control={workflow.control}
      actor={actor}
      commentThreadId={commentThreadId}
      {...slots}
    />
  )
}
