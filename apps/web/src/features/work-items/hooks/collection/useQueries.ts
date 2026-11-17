import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import type { WorkItemsEnvironment } from '../../model/environment'
import { workItemsKey } from '../../model/query-keys'
import {
  GANTT_MAX_ITEMS,
  GANTT_PAGE_SIZE,
  requestPMTasks,
  requestWorkItemsPage,
} from '../../api/collection'
import { requestScopeOptions } from '../../api/scope'
import { requestWorkItemDetail } from '../../api/resources'
import type { WorkItem, WorkItemsRouteState, WorkItemUserOption } from '../../model/types'
import {
  platformUsersToWorkItemUsers,
  searchPlatformUsersForWorkItems,
} from '../../api/users'
import { requestScope } from '../../model/utils'
import { requestWorkItemWorkflowCatalog } from '../../api/workflow'
import { requestClientOptions } from '../../api/client-options'

const STALE_TIME = 30_000

// The workspace user directory loads once (capped) and the selects filter it
// locally; only ids missing from the directory response (inactive users or
// beyond the cap) are hydrated through a separate ids query.
const USER_DIRECTORY_LIMIT = 500
const USER_HYDRATION_LIMIT = 100

const queryOptions = {
  staleTime: STALE_TIME,
  refetchOnWindowFocus: true,
  refetchInterval: false as const,
  retry: false as const,
}

export function pmTasksQueryKey(
  workspaceId: number,
  route: WorkItemsRouteState,
) {
  const tablePage = route.view === 'list' && route.layout === 'table'
  const filteredTree = tablePage
    && (route.groupBy === 'none' || route.groupBy === 'requirement_id')
  return workItemsKey(
    workspaceId,
    'collection',
    route.view,
    route.layout,
    {
      ...requestScope(route),
      ...(route.filterKinds?.length
        ? { filter_kinds: route.filterKinds.join(',') }
        : {}),
      limit: tablePage ? route.pageSize : 50,
      offset: tablePage ? (route.page - 1) * route.pageSize : 0,
      sort: route.sort,
      ...(filteredTree ? { hierarchy: 'filtered_tree' } : {}),
      ...(tablePage ? {} : { bounded_all_rows: 1_000 }),
    },
  )
}

export function pmGanttTasksQueryKey(
  workspaceId: number,
  route: WorkItemsRouteState,
) {
  return workItemsKey(
    workspaceId,
    'collection',
    'gantt',
    {
      ...requestScope(route),
      ...(route.filterKinds?.length
        ? { filter_kinds: route.filterKinds.join(',') }
        : {}),
      limit: GANTT_PAGE_SIZE,
      sort: route.sort,
    },
  )
}

export function useScopeOptionsQuery(
  environment: WorkItemsEnvironment,
  enabled: boolean,
) {
  return useQuery({
    queryKey: workItemsKey(environment.workspaceId, 'scope-options', 'pm-hierarchy'),
    queryFn: ({ signal }) => requestScopeOptions(environment.client, signal),
    enabled,
    ...queryOptions,
  })
}

export function usePMTasksQuery(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
  enabled: boolean,
) {
  return useQuery({
    queryKey: pmTasksQueryKey(environment.workspaceId, route),
    queryFn: ({ signal }) => requestPMTasks(environment.client, route, signal),
    enabled,
    ...queryOptions,
  })
}

export function usePMGanttTasksQuery(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
  enabled: boolean,
) {
  return useInfiniteQuery({
    queryKey: pmGanttTasksQueryKey(environment.workspaceId, route),
    queryFn: ({ pageParam, signal }) => requestWorkItemsPage(
      environment.client,
      route,
      Math.min(GANTT_PAGE_SIZE, GANTT_MAX_ITEMS - pageParam),
      pageParam,
      signal,
    ),
    initialPageParam: 0,
    getNextPageParam: (lastPage, pages) => {
      const loaded = pages.reduce((total, page) => total + page.items.length, 0)
      if (!lastPage.items.length || loaded >= lastPage.total || loaded >= GANTT_MAX_ITEMS) {
        return undefined
      }
      return loaded
    },
    enabled,
    ...queryOptions,
  })
}

export function useWorkItemDetailQuery(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  enabled: boolean,
) {
  return useQuery({
    queryKey: workItemsKey(environment.workspaceId, 'detail', id || 0),
    queryFn: ({ signal }) => requestWorkItemDetail(environment.client, id || 0, signal),
    enabled: enabled && Boolean(id),
    ...queryOptions,
  })
}

export function workItemWorkflowQueryKey(
  workspaceId: number,
  teamId: number,
  projectId: number,
) {
  return workItemsKey(workspaceId, 'workflow', 'task', teamId, projectId)
}

export function useWorkItemCollectionWorkflowQuery(
  environment: WorkItemsEnvironment,
  enabled: boolean,
) {
  return useQuery({
    queryKey: workItemWorkflowQueryKey(environment.workspaceId, 0, 0),
    queryFn: ({ signal }) => requestWorkItemWorkflowCatalog(environment.client, {}, signal),
    enabled,
    ...queryOptions,
  })
}

export function useWorkItemWorkflowQuery(
  environment: WorkItemsEnvironment,
  item: WorkItem | undefined,
  enabled: boolean,
) {
  const teamId = item?.teamId || 0
  const projectId = item?.projectId || 0
  return useQuery({
    queryKey: workItemWorkflowQueryKey(environment.workspaceId, teamId, projectId),
    queryFn: ({ signal }) => requestWorkItemWorkflowCatalog(
      environment.client,
      { teamId, projectId },
      signal,
    ),
    enabled: enabled && teamId > 0 && projectId > 0,
    ...queryOptions,
  })
}

export function useWorkItemUsersQuery(
  environment: WorkItemsEnvironment,
  enabled: boolean,
  selectedIds: readonly number[] = [],
) {
  const workspaceId = environment.workspaceId > 0 ? environment.workspaceId : 1
  const normalizedSelectedIds = useMemo(() => [...new Set(selectedIds)]
    .filter((id) => Number.isInteger(id) && id > 0)
    .sort((left, right) => left - right)
    .slice(0, USER_HYDRATION_LIMIT), [selectedIds])
  const directoryQuery = useQuery({
    queryKey: workItemsKey(workspaceId, 'users', 'directory'),
    queryFn: ({ signal }) => searchPlatformUsersForWorkItems(
      environment.client,
      { limit: USER_DIRECTORY_LIMIT },
      signal,
    ),
    enabled,
    staleTime: 120_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
  const directoryItems = useMemo(
    () => (directoryQuery.data ? platformUsersToWorkItemUsers(directoryQuery.data.items) : undefined),
    [directoryQuery.data],
  )
  const missingIds = useMemo(() => {
    if (!directoryItems) return []
    const known = new Set(directoryItems.map((item) => item.id))
    return normalizedSelectedIds.filter((id) => !known.has(id))
  }, [directoryItems, normalizedSelectedIds])
  const hydrationQuery = useQuery({
    queryKey: workItemsKey(workspaceId, 'users', 'selected', missingIds),
    queryFn: ({ signal }) => searchPlatformUsersForWorkItems(
      environment.client,
      { ids: missingIds, limit: USER_HYDRATION_LIMIT },
      signal,
    ),
    enabled: enabled && missingIds.length > 0,
    staleTime: 120_000,
  })
  const hydrationItems = useMemo(
    () => (hydrationQuery.data ? platformUsersToWorkItemUsers(hydrationQuery.data.items) : []),
    [hydrationQuery.data],
  )
  const data = useMemo(() => {
    const byId = new Map<number, WorkItemUserOption>()
    for (const item of hydrationItems) byId.set(item.id, item)
    for (const item of directoryItems ?? []) byId.set(item.id, item)
    return [...byId.values()]
  }, [hydrationItems, directoryItems])
  return {
    data,
    isPending: directoryQuery.isPending || (missingIds.length > 0 && hydrationQuery.isPending),
    isError: directoryQuery.isError || hydrationQuery.isError,
    error: directoryQuery.error || hydrationQuery.error,
    refetch: async () => {
      await Promise.all([
        directoryQuery.refetch(),
        ...(missingIds.length > 0 ? [hydrationQuery.refetch()] : []),
      ])
    },
  }
}

export function useWorkItemClientOptionsQuery(environment: WorkItemsEnvironment, enabled: boolean) {
  return useQuery({
    queryKey: workItemsKey(environment.workspaceId, 'client-options'),
    queryFn: ({ signal }) => requestClientOptions(environment.client, signal),
    enabled,
    ...queryOptions,
  })
}
