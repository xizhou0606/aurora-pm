import { useCallback, useEffect, useMemo } from 'react'

import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  WorkItem,
  WorkItemsRouteState,
  WorkItemsScope,
} from '../../model/types'
import {
  emptyScope,
  normalizeWorkItemsBoardScope,
  parseWorkItemsRoute,
  resetPage,
  workItemsPath,
} from '../../model/utils'

export function useWorkItemsRouteWorkflow(environment: WorkItemsEnvironment) {
  const route = useMemo(
    () => parseWorkItemsRoute(environment.pathname, environment.search),
    [environment.pathname, environment.search],
  )
  const navigateRoute = useCallback((next: WorkItemsRouteState, replace = false) => {
    void environment.navigate(workItemsPath(next), replace)
  }, [environment])

  useEffect(() => {
    if (route.invalidLegacyScope) return
    const normalizeBoardScope = route.view === 'board' && route.scope.projectSetIds.length > 0
    if (!route.needsRouteNormalization && !normalizeBoardScope) return
    const normalized = {
      ...route,
      scope: normalizeBoardScope ? normalizeWorkItemsBoardScope(route.scope) : route.scope,
      needsRouteNormalization: false,
    }
    // Canonicalizing a legacy detail path must retain its notification destination.
    const hash = environment.hash?.replace(/^#/, '') ?? ''
    const anchor = route.detailId && /^(activity|comment-[1-9]\d*)$/.test(hash) ? `#${hash}` : ''
    void environment.navigate(`${workItemsPath(normalized)}${anchor}`, true)
  }, [environment, route])
  const setSearch = useCallback((value: string) => {
    const normalized = value.trim().slice(0, 120)
    if (normalized === route.search) return
    navigateRoute(resetPage({ ...route, search: normalized }), true)
  }, [navigateRoute, route])

  const patchRoute = useCallback((patch: Partial<WorkItemsRouteState>, replace = false) => {
    navigateRoute(resetPage({ ...route, ...patch }), replace)
  }, [navigateRoute, route])
  const setScope = useCallback((scope: WorkItemsScope) => {
    patchRoute({
      scope: route.view === 'board' ? normalizeWorkItemsBoardScope(scope) : scope,
      assigneeIds: [],
      invalidLegacyScope: false,
    })
  }, [patchRoute, route.view])
  const setView = useCallback((view: WorkItemsRouteState['view']) => {
    if (view === 'board') {
      navigateRoute(resetPage({
        ...route,
        view,
        layout: 'table',
        detailId: undefined,
        mode: 'view',
        scope: emptyScope(),
        assigneeIds: [],
      }))
      return
    }
    navigateRoute(resetPage({ ...route, view, detailId: undefined, mode: 'view' }))
  }, [navigateRoute, route])
  const clearLegacyScope = useCallback(() => {
    navigateRoute({ ...route, scope: emptyScope(), invalidLegacyScope: false })
  }, [navigateRoute, route])
  const setPage = useCallback((page: number) => {
    navigateRoute({ ...route, page: Number.isInteger(page) && page > 0 ? page : 1 })
  }, [navigateRoute, route])
  const setPageSize = useCallback((pageSize: 10 | 20 | 50 | 100) => {
    navigateRoute({ ...route, page: 1, pageSize })
  }, [navigateRoute, route])
  const clearFilters = useCallback(() => {
    navigateRoute(resetPage({
      ...route,
      search: '',
      overdue: false,
      assigneeIds: [],
      statuses: [],
      priorities: [],
      taskClientIds: [],
      filterKinds: [],
      groupBy: 'none',
    }))
  }, [navigateRoute, route])
  const openItem = useCallback((row: WorkItem, mode: WorkItemsRouteState['mode'] = 'view') => {
    void environment.navigate(workItemsPath({ ...route, mode }, { detailId: row.id }))
  }, [environment, route])
  const closeDetail = useCallback(() => {
    void environment.navigate(
      route.from === 'inbox'
        ? '/workspace/inbox'
        : workItemsPath(route, { omitDetail: true }),
    )
  }, [environment, route])
  const openTaskDetail = useCallback((taskId: number) => {
    if (!Number.isInteger(taskId) || taskId <= 0) return
    void environment.navigate(workItemsPath({ ...route, mode: 'view' }, { detailId: taskId }))
  }, [environment, route])
  const setDetailMode = useCallback((mode: WorkItemsRouteState['mode']) => {
    if (!route.detailId) return
    void environment.navigate(workItemsPath({ ...route, mode }), true)
  }, [environment, route])

  return {
    route,
    setSearch,
    setScope,
    setView,
    clearLegacyScope,
    setFilter: patchRoute,
    clearFilters,
    setPage,
    setPageSize,
    openItem,
    openTaskDetail,
    closeDetail,
    setDetailMode,
    navigateRoute,
  }
}
