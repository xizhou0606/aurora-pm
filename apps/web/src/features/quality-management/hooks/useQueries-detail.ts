import {
  useInfiniteQuery,
  useQuery,
} from '@tanstack/react-query'

import {
  canRetainStaleApiData,
  isSafeApiError,
  PM_API_ERROR_CODES,
} from '@/shared/api'
import {
  isNotFound,
  normalizeActivityPage,
  normalizeQualityDetail,
  normalizeQualityComments,
} from '../api'
import { QUALITY_ACTIVITY_PAGE_SIZE, qualityKeys } from '../api'
import { mergeQualityActivityPages } from '../model/selectors'
import type {
  QualityActivity,
  QualityComment,
  QualityDetailState,
  QualityRouteState,
} from '../model/types'
import type { QualityEnvironment } from '../components/environment'

const STALE_TIME = 30_000

function isWorkspaceAccessError(error: unknown): boolean {
  return isSafeApiError(error)
    && error.status === 403
    && error.code === PM_API_ERROR_CODES.WORKSPACE_FORBIDDEN
}

export function useQualityDetailQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled = true,
): QualityDetailState {
  const id = route.bugId || 0
  const query = useQuery({
    queryKey: environment.workspaceId > 0 && id > 0
      ? qualityKeys(environment.workspaceId).detail(id)
      : ['pm', 'quality', 'disabled', 'detail'],
    queryFn: async ({ signal }) => normalizeQualityDetail(
      await environment.client.detail(id, signal),
      environment.workspaceId,
    ),
    enabled: environment.allowed && enabled && route.kind === 'detail' && id > 0,
    staleTime: STALE_TIME,
  })
  const retry = async () => {
    await query.refetch()
  }
  if (query.isError && isSafeApiError(query.error) && query.error.status === 403 && query.error.code !== 'WORKSPACE_INITIALIZING') {
    return { status: 'access_required' }
  }
  if (query.isError && isNotFound(query.error)) {
    return { status: 'not_found' }
  }
  if (query.isError && !canRetainStaleApiData(query.error)) {
    return { status: 'error', error: query.error, retry }
  }
  if (query.data) {
    return {
      status: 'ready',
      bug: query.data,
      refresh: query.isError ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    }
  }
  if (query.isPending) return { status: 'loading' }
  if (query.isError) return { status: 'error', error: query.error, retry }
  return { status: 'loading' }
}

export type QualityActivitiesState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; error: unknown; retry: () => Promise<void> }
  | {
      status: 'ready'
      rows: QualityActivity[]
      refresh: 'idle' | 'refreshing' | 'stale_error'
      loadingMore: boolean
      hasMore: boolean
      loadMore: () => Promise<void>
      retry: () => Promise<void>
    }

export function useQualityActivitiesQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled: boolean,
): QualityActivitiesState {
  const id = route.bugId || 0
  const query = useInfiniteQuery({
    queryKey: environment.workspaceId > 0 && id > 0
      ? qualityKeys(environment.workspaceId).activities(id, QUALITY_ACTIVITY_PAGE_SIZE)
      : ['pm', 'quality', 'disabled', 'activities'],
    queryFn: async ({ pageParam, signal }) => normalizeActivityPage(
      await environment.client.activities(id, pageParam, QUALITY_ACTIVITY_PAGE_SIZE, signal),
      pageParam,
    ),
    initialPageParam: 0,
    getNextPageParam: (page) => page.nextOffset,
    enabled: environment.allowed && route.kind === 'detail' && id > 0 && enabled,
    staleTime: STALE_TIME,
  })
  const canUseCachedRows = !query.error || canRetainStaleApiData(query.error)
  const rows = canUseCachedRows ? mergeQualityActivityPages(query.data?.pages) : []
  const retry = async () => {
    await query.refetch()
  }
  if (!enabled || route.kind !== 'detail' || id <= 0) return { status: 'idle' }
  if (!query.data && query.isPending) return { status: 'loading' }
  if (!canUseCachedRows || (!query.data && query.isError)) {
    return { status: 'error', error: query.error, retry }
  }
  return {
    status: 'ready',
    rows,
    refresh: query.error
      ? 'stale_error'
      : query.isFetching && !query.isFetchingNextPage
        ? 'refreshing'
        : 'idle',
    loadingMore: query.isFetchingNextPage,
    hasMore: Boolean(query.hasNextPage),
    loadMore: async () => {
      await query.fetchNextPage()
    },
    retry,
  }
}

export type QualityCommentsState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; error: unknown; retry: () => Promise<void> }
  | {
      status: 'ready'
      rows: QualityComment[]
      refresh: 'idle' | 'refreshing' | 'stale_error'
      retry: () => Promise<void>
    }

export function useQualityCommentsQuery(
  environment: QualityEnvironment,
  route: QualityRouteState,
  enabled: boolean,
): QualityCommentsState {
  const id = route.bugId || 0
  const query = useQuery({
    queryKey: environment.workspaceId > 0 && id > 0
      ? qualityKeys(environment.workspaceId).comments(id)
      : ['pm', 'quality', 'disabled', 'comments'],
    queryFn: async ({ signal }) => normalizeQualityComments(
      await environment.client.comments(id, signal),
    ),
    enabled: environment.allowed && route.kind === 'detail' && id > 0 && enabled,
    staleTime: STALE_TIME,
  })
  const retry = async () => { await query.refetch() }
  if (!enabled || route.kind !== 'detail' || id <= 0) return { status: 'idle' }
  if (!query.data && query.isPending) return { status: 'loading' }
  if (!query.data && query.isError) return { status: 'error', error: query.error, retry }
  return {
    status: 'ready',
    rows: query.data || [],
    refresh: query.error
      ? 'stale_error'
      : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}