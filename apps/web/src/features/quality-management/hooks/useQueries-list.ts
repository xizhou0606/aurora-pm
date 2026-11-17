import {
  keepPreviousData,
  queryOptions,
  useQuery,
} from '@tanstack/react-query'

import {
  isSafeApiError,
  normalizePlatformUsers,
  PM_API_ERROR_CODES,
} from '@/shared/api'
import {
  normalizeQualityDashboard,
  normalizeQualityList,
} from '../api'
import { QUALITY_UNCLOSED_STATUS_FILTER } from '../model'
import { qualityClient } from '../api/client'
import { qualityKeys } from '../api'
import type {
  QualityDashboardState,
  QualityListState,
  QualityRouteState,
  QualitySummaryState,
} from '../model/types'
import type { QualityEnvironment } from '../components/environment'

const STALE_TIME = 30_000

// Query option factories shared between the route loader (non-blocking
// prefetch) and the page hooks. Payload mapping stays in the hooks layer.
export function qualityScopeQueryOptions(workspaceId: number) {
  return queryOptions({
    queryKey: qualityKeys(workspaceId).scope,
    queryFn: ({ signal }) => qualityClient.scope(signal),
    staleTime: 5 * 60_000,
  })
}

export function qualityListQueryOptions(workspaceId: number, route: QualityRouteState) {
  return queryOptions({
    queryKey: qualityKeys(workspaceId).list(route),
    queryFn: async ({ signal }) => normalizeQualityList(
      await qualityClient.list(listQuery(route), signal),
      workspaceId,
    ),
    staleTime: STALE_TIME,
  })
}

function userIDsQuery(ids: readonly number[]): number | string | undefined {
  if (ids.length === 0) return undefined
  return ids.length === 1 ? ids[0] : ids.join(',')
}

function isWorkspaceAccessError(error: unknown): boolean {
  return isSafeApiError(error)
    && error.status === 403
    && error.code === PM_API_ERROR_CODES.WORKSPACE_FORBIDDEN
}

export function listQuery(route: QualityRouteState): Readonly<Record<string, unknown>> {
  const unclosed = route.filters.status === QUALITY_UNCLOSED_STATUS_FILTER
  return {
    project_set_ids: route.scope.projectSetIds.join(',') || undefined,
    project_ids: route.scope.projectIds.join(',') || undefined,
    requirement_ids: route.scope.requirementIds.join(',') || undefined,
    q: route.filters.q || undefined,
    ...(unclosed
      ? { exclude_statuses: 'done,canceled' }
      : { status: route.filters.status || undefined }),
    severity_key: route.filters.severities.join(',') || undefined,
    found_environment: route.filters.environments.join(',') || undefined,
    defect_domain: route.filters.defectDomain || undefined,
    assignee_user_id: userIDsQuery(route.filters.assigneeUserIds),
    current_owner_user_id: userIDsQuery(route.filters.ownerUserIds),
    reporter_user_id: userIDsQuery(route.filters.reporterUserIds),
    reported_by_me: route.filters.relation === 'reported' ? 1 : undefined,
    assigned_to_me: route.filters.relation === 'assigned' ? 1 : undefined,
    responsible_by_me: route.filters.relation === 'responsible' ? 1 : undefined,
    resolved_by_me: route.filters.relation === 'resolved' ? 1 : undefined,
    created_from: route.filters.createdFrom || undefined,
    created_to: route.filters.createdTo || undefined,
    sort_by: route.filters.sortBy,
    sort_order: route.filters.sortOrder,
    limit: route.pageSize,
    offset: (route.page - 1) * route.pageSize,
  }
}

export function dashboardQuery(route: QualityRouteState): Readonly<Record<string, unknown>> {
  const query = listQuery(route)
  const { limit: _limit, offset: _offset, sort_by: _sortBy, sort_order: _sortOrder, ...filters } = query
  return filters
}

export function useQualityDashboardQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled = true,
): QualityDashboardState {
  const query = useQuery({
    queryKey: environment.workspaceId > 0
      ? qualityKeys(environment.workspaceId).dashboard(route)
      : ['pm', 'quality', 'disabled', 'dashboard'],
    queryFn: async ({ signal }) => normalizeQualityDashboard(
      await environment.client.dashboard(dashboardQuery(route), signal),
    ),
    enabled: environment.allowed && enabled && route.kind === 'list' && route.view === 'dashboard',
    staleTime: STALE_TIME,
    refetchOnWindowFocus: true,
  })
  const retry = async () => { await query.refetch() }
  if (!query.data && query.isPending) return { status: 'loading' }
  if (!query.data && query.isError && isWorkspaceAccessError(query.error)) {
    return { status: 'unavailable', reason: 'workspace_access' }
  }
  if (!query.data && query.isError) return { status: 'error', error: query.error, retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.isError ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
  }
}

export function useQualityListQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled = true,
): { state: QualityListState; refresh: () => Promise<void> } {
  const query = useQuery({
    queryKey: environment.workspaceId > 0
      ? qualityKeys(environment.workspaceId).list(route)
      : ['pm', 'quality', 'disabled', 'list'],
    queryFn: async ({ signal }) => normalizeQualityList(
      await environment.client.list(listQuery(route), signal),
      environment.workspaceId,
    ),
    enabled: environment.allowed && route.kind === 'list' && route.view === 'list' && enabled,
    staleTime: STALE_TIME,
    refetchOnWindowFocus: true,
    placeholderData: keepPreviousData,
  })

  const refresh = async () => {
    await query.refetch()
  }

  if (!query.data && query.isPending) return { state: { status: 'loading' }, refresh }
  if (!query.data && query.isError && isWorkspaceAccessError(query.error)) {
    return { state: { status: 'unavailable', reason: 'workspace_access' }, refresh }
  }
  if (!query.data && query.isError) {
    return { state: { status: 'error', error: query.error, retry: refresh }, refresh }
  }
  if (!query.data) return { state: { status: 'loading' }, refresh }
  if (query.data.status === 'unavailable') {
    return { state: { status: 'unavailable', reason: query.data.reason }, refresh }
  }
  if (query.isPlaceholderData && query.isFetching && !query.data.rows.length) {
    return { state: { status: 'loading' }, refresh }
  }
  if (!query.data.rows.length) return {
    state: {
      status: 'empty',
      summary: query.data.summary,
      statusFacets: query.data.statusFacets,
    },
    refresh,
  }
  return {
    state: {
      status: 'ready',
      rows: query.data.rows,
      total: query.data.total,
      summary: query.data.summary,
      statusFacets: query.data.statusFacets,
      refresh: query.isError
        ? 'stale_error'
        : query.isFetching
          ? 'refreshing'
          : 'idle',
    },
    refresh,
  }
}

export function useQualitySummaryQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled = true,
): QualitySummaryState {
  const query = useQuery({
    queryKey: environment.workspaceId > 0
      ? qualityKeys(environment.workspaceId).summary(route)
      : ['pm', 'quality', 'disabled', 'summary'],
    queryFn: async ({ signal }) => normalizeQualityList(
      await environment.client.statistics(dashboardQuery(route), signal),
      environment.workspaceId,
    ),
    enabled: environment.allowed && enabled && route.kind === 'list' && route.view === 'list',
    staleTime: STALE_TIME,
    refetchOnWindowFocus: true,
    placeholderData: keepPreviousData,
  })

  const retry = async () => {
    await query.refetch()
  }
  if (!query.data && query.isPending) return { status: 'loading' }
  if (!query.data && query.isError && isWorkspaceAccessError(query.error)) {
    return { status: 'unavailable', reason: 'workspace_access' }
  }
  if (!query.data && query.isError) return { status: 'error', error: query.error, retry }
  if (!query.data) return { status: 'loading' }
  if (query.data.status === 'unavailable') {
    return { status: 'unavailable', reason: query.data.reason }
  }
  return {
    status: 'ready',
    summary: query.data.summary,
    statusFacets: query.data.statusFacets,
  }
}
