import { pmKeys } from '@/app/query'
import type { DocumentContext } from './types'

export function documentKeys(context: DocumentContext) {
  const root = pmKeys.resource(
    context.workspaceId,
    'documents',
    'session',
    context.sessionRevision,
  )
  return {
    root,
    defaultSpace: [...root, 'spaces', 'default'] as const,
    tree: (spaceId: number) => [...root, 'spaces', spaceId, 'tree'] as const,
    document: (documentId: number) => [...root, 'documents', documentId] as const,
    assets: (documentId: number) => [...root, 'documents', documentId, 'assets'] as const,
    commentThreads: (documentId: number) => [...root, 'documents', documentId, 'comment-threads'] as const,
    deletionImpact: (nodeId: number) => [...root, 'nodes', nodeId, 'deletion-impact'] as const,
  }
}

export const disabledDocumentKey = (resource: string) => (
  ['pm', 'documents', 'disabled', resource] as const
)
