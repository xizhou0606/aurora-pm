export const QUALITY_UNCLOSED_STATUS_FILTER = 'unclosed'

const QUALITY_CLOSED_STATUSES = new Set([
  'done', 'closed', 'completed', 'canceled', 'cancelled',
])

export function isQualityTerminalStatus(status: string | undefined): boolean {
  return QUALITY_CLOSED_STATUSES.has(String(status || '').toLocaleLowerCase())
}

export function qualityReviewerOwnsStatus(status: string | undefined): boolean {
  return ['review', 'done', 'closed', 'completed', 'canceled', 'cancelled']
    .includes(String(status || '').toLocaleLowerCase())
}

export function matchesQualityRelationFilter({
  relation,
  userId,
  status,
  assigneeUserId,
  reporterUserId,
  currentOwnerUserId,
}: {
  relation: '' | 'reported' | 'assigned' | 'responsible' | 'resolved'
  userId: number
  status?: string
  assigneeUserId?: number
  reporterUserId?: number
  currentOwnerUserId?: number
}): boolean {
  if (!relation) return true
  if (userId <= 0) return false
  if (relation === 'reported') return reporterUserId === userId
  if (relation === 'assigned') return currentOwnerUserId === userId
  if (relation === 'responsible') return assigneeUserId === userId
  return assigneeUserId === userId && ['review', 'done', 'closed', 'completed'].includes(
    String(status || '').toLocaleLowerCase(),
  )
}

export function matchesQualityStatusFilter(status: string | undefined, filter: string): boolean {
  if (!filter) return true
  if (filter === QUALITY_UNCLOSED_STATUS_FILTER) {
    return !isQualityTerminalStatus(status)
  }
  return status === filter
}
