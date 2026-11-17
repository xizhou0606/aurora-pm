import { useCallback } from 'react'

import {
  resolveActivityState,
  usePlatformUsersQuery,
  useProjectAuditQuery,
  useProjectOverviewQuery,
  type ActivityState,
} from '../model'

const AUDIT_PAGE_SIZE = 30

export type ActivityRequest =
  | { kind: 'project'; projectId: number; teamId: number }
  | { kind: 'plan-unit'; projectId: number; planUnitId: number; teamId: number }

export function useActivityModel(input: ActivityRequest): ActivityState {
  const auditQuery = useProjectAuditQuery(input.projectId, true, {
    limit: AUDIT_PAGE_SIZE,
    offset: 0,
  })
  const overviewQuery = useProjectOverviewQuery({
    projectId: input.projectId,
    teamId: input.teamId,
  }, input.kind === 'plan-unit')
  const usersQuery = usePlatformUsersQuery()
  const retry = useCallback(() => {
    void Promise.all([
      auditQuery.refetch(),
      ...(input.kind === 'plan-unit' ? [overviewQuery.refetch()] : []),
    ])
  }, [auditQuery.refetch, input.kind, overviewQuery.refetch])

  const common = {
    audit: {
      status: auditQuery.isPlaceholderData ? 'pending' as const : auditQuery.status,
      data: auditQuery.isPlaceholderData ? undefined : auditQuery.data?.rows,
      error: auditQuery.error,
      fetching: auditQuery.isFetching && !auditQuery.isPending,
    },
    users: usersQuery.isError ? [] : (usersQuery.data ?? []),
    retry,
  }
  return input.kind === 'plan-unit'
    ? resolveActivityState({
        ...common,
        scope: 'plan-unit',
        planUnitId: input.planUnitId,
        facts: {
          status: overviewQuery.status,
          data: overviewQuery.data,
          error: overviewQuery.error,
          fetching: overviewQuery.isFetching && !overviewQuery.isPending,
        },
      })
    : resolveActivityState({ ...common, scope: 'project' })
}
