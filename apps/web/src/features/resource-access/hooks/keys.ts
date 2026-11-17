import { pmKeys } from '@/shared/query'

export function resourceAccessKeys(input: {
  workspaceId: number
  sessionRevision: number
  resourceId: string
}) {
  // Hooks are mounted before session/workspace hydration, even for closed
  // sharing dialogs. Disabled reads must not construct an active tenant key.
  const root = Number.isInteger(input.workspaceId) && input.workspaceId > 0
    ? pmKeys.resource(
      input.workspaceId,
      'documents',
      'resource-access',
      'session',
      input.sessionRevision,
      input.resourceId,
    )
    : ['pm', 'documents', 'resource-access', 'disabled', input.sessionRevision, input.resourceId] as const
  return {
    root,
    policy: [...root, 'policy'] as const,
    links: [...root, 'share-links'] as const,
    subjects: (kind: string, query: string, limit: number) => [
      ...root,
      'subjects',
      kind,
      { query, limit },
    ] as const,
  }
}
