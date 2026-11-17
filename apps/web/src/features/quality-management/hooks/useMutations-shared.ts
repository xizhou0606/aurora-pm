import { useQueryClient } from '@tanstack/react-query'
import type { UseMutationResult } from '@tanstack/react-query'

import type { QualityEnvironment } from '../components/environment'
import { qualityKeys } from '../api'
import { isQualityTerminalStatus, matchesQualityRelationFilter, matchesQualityStatusFilter, qualityReviewerOwnsStatus } from '../model/filters'
import type {
  QualityBug,
  QualityListResult,
  QualityMutationAction,
  QualityMutationState,
  QualityRouteState,
  QualityStatusFacets,
  QualitySummary,
} from '../model/types'

export function summaryContribution(bug: QualityBug): Omit<QualitySummary, 'total'> {
  const status = bug.status || ''
  return {
    open: isQualityTerminalStatus(status) ? 0 : 1,
    handling: ['in_progress', 'reopened'].includes(status) ? 1 : 0,
    review: status === 'review' ? 1 : 0,
    closed: isQualityTerminalStatus(status) ? 1 : 0,
    highSeverity: ['p0', 'p1'].includes(bug.severity || '') ? 1 : 0,
  }
}

export function patchSummary(
  summary: QualitySummary,
  before?: QualityBug,
  after?: QualityBug,
): QualitySummary {
  const previous = before ? summaryContribution(before) : null
  const next = after ? summaryContribution(after) : null
  return {
    total: Math.max(0, summary.total - (before ? 1 : 0) + (after ? 1 : 0)),
    open: Math.max(0, summary.open - (previous?.open || 0) + (next?.open || 0)),
    handling: Math.max(0, summary.handling - (previous?.handling || 0) + (next?.handling || 0)),
    review: Math.max(0, summary.review - (previous?.review || 0) + (next?.review || 0)),
    closed: Math.max(0, summary.closed - (previous?.closed || 0) + (next?.closed || 0)),
    highSeverity: Math.max(
      0,
      summary.highSeverity - (previous?.highSeverity || 0) + (next?.highSeverity || 0),
    ),
  }
}

export function facetKey(status: string | undefined): keyof QualityStatusFacets | null {
  switch (status) {
    case 'todo': return 'todo'
    case 'triage': return 'triage'
    case 'in_progress': return 'handling'
    case 'reopened': return 'reopened'
    case 'review': return 'review'
    case 'done':
    case 'closed': return 'done'
    case 'canceled': return 'canceled'
    default: return null
  }
}

export function patchFacets(
  facets: QualityStatusFacets,
  before?: QualityBug,
  after?: QualityBug,
): QualityStatusFacets {
  const next = { ...facets }
  const previousKey = facetKey(before?.status)
  const nextKey = facetKey(after?.status)
  if (previousKey) next[previousKey] = Math.max(0, next[previousKey] - 1)
  if (before && isQualityTerminalStatus(before.status)) next.closed = Math.max(0, next.closed - 1)
  if (nextKey) next[nextKey] += 1
  if (after && isQualityTerminalStatus(after.status)) next.closed += 1
  return next
}

export function currentOwnerId(bug: QualityBug): number | undefined {
  return qualityReviewerOwnsStatus(bug.status)
    ? bug.reviewerUserId || bug.reporterUserId
    : bug.assigneeUserId
}

export function matchesRoute(
  bug: QualityBug,
  route: QualityRouteState,
  userId: number,
  includeStatus = true,
): boolean {
  const { filters, scope } = route
  const query = filters.q.trim().toLocaleLowerCase()
  if (scope.projectSetIds.length && !scope.projectSetIds.includes(bug.projectSetId || 0)) return false
  if (scope.projectIds.length && !scope.projectIds.includes(bug.projectId || 0)) return false
  if (scope.requirementIds.length && !scope.requirementIds.includes(bug.requirementId || 0)) return false
  if (query && !`${bug.bugKey || ''} ${bug.title}`.toLocaleLowerCase().includes(query)) return false
  if (includeStatus && !matchesQualityStatusFilter(bug.status, filters.status)) return false
  if (filters.severities.length && !filters.severities.includes(bug.severity || '')) return false
  if (
    filters.environments.length &&
    !filters.environments.some((environment) => (
      String(bug.foundEnvironment || '').toLocaleLowerCase() === environment.toLocaleLowerCase()
    ))
  ) return false
  if (filters.assigneeUserIds.length && !filters.assigneeUserIds.includes(bug.assigneeUserId || 0)) return false
  if (filters.ownerUserIds.length && !filters.ownerUserIds.includes(currentOwnerId(bug) || 0)) return false
  if (filters.reporterUserIds.length && !filters.reporterUserIds.includes(bug.reporterUserId || 0)) return false
  if (filters.defectDomain && (bug.defectDomain === 'ui' ? 'ui' : 'functional') !== filters.defectDomain) return false
  if (!matchesQualityRelationFilter({
    relation: filters.relation,
    userId,
    status: bug.status,
    assigneeUserId: bug.assigneeUserId,
    reporterUserId: bug.reporterUserId,
    currentOwnerUserId: currentOwnerId(bug),
  })) return false
  const createdAt = bug.createdAt ? new Date(bug.createdAt) : null
  if (filters.createdFrom && (!createdAt || createdAt < new Date(`${filters.createdFrom}T00:00:00`))) return false
  if (filters.createdTo && (!createdAt || createdAt > new Date(`${filters.createdTo}T23:59:59.999`))) return false
  return true
}

export function replaceQualityListBug(
  current: QualityListResult | undefined,
  before: QualityBug,
  after: QualityBug,
  route: QualityRouteState,
  userId: number,
): QualityListResult | undefined {
  if (!current || current.status !== 'available') return current
  const rowExists = current.rows.some((row) => row.id === before.id)
  if (!rowExists) return current
  const afterMatches = matchesRoute(after, route, userId)
  const beforeMatchesFacet = matchesRoute(before, route, userId, false)
  const afterMatchesFacet = matchesRoute(after, route, userId, false)
  return {
    ...current,
    rows: afterMatches
      ? current.rows.map((row) => row.id === before.id ? after : row)
      : current.rows.filter((row) => row.id !== before.id),
    total: Math.max(0, current.total - (afterMatches ? 0 : 1)),
    summary: patchSummary(current.summary, before, afterMatches ? after : undefined),
    statusFacets: patchFacets(
      current.statusFacets,
      beforeMatchesFacet ? before : undefined,
      afterMatchesFacet ? after : undefined,
    ),
  }
}

export function state<TData, TError, TVariables>(
  action: QualityMutationAction,
  mutation: UseMutationResult<TData, TError, TVariables>,
): QualityMutationState {
  if (mutation.status === 'pending') return { status: 'pending', action }
  if (mutation.status === 'error') {
    return { status: 'error', action, error: mutation.error, reset: mutation.reset }
  }
  if (mutation.status === 'success') return { status: 'success', action, reset: mutation.reset }
  return { status: 'idle', action }
}

export function useQualityInvalidation(environment: QualityEnvironment) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  return {
    all: async () => {
      if (keys) await client.invalidateQueries({ queryKey: keys.root })
    },
    detail: async (id: number) => {
      if (!keys) return
      await Promise.all([
        client.invalidateQueries({ queryKey: keys.detail(id) }),
        client.invalidateQueries({ queryKey: keys.activities(id) }),
        client.invalidateQueries({ queryKey: keys.comments(id) }),
        client.invalidateQueries({ queryKey: keys.root }),
      ])
    },
  }
}
