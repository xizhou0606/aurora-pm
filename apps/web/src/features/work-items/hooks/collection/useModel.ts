import { useCallback, useEffect, useMemo, useRef } from 'react'
import { GANTT_MAX_ITEMS } from '../../api/collection'
import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  ReadyRefresh,
  ScopeOptionsState,
  WorkItem,
  WorkItemDetailPatch,
  UserOptionsState,
} from '../../model/types'
import { applyWorkItemsOperationAccess } from '../../model/permissions'
import {
  buildBoardGroups,
  buildListGroups,
  hasWorkItemsBoardScope,
  isValidEstimatedMinutes,
  isValidWorkItemsBoardScope,
  isValidWorkItemsListScope,
  resolveCollectionState,
  workItemAssigneeDirectoryIds,
} from '../../model/utils'
import {
  useWorkItemBatchDeleteMutation,
  useWorkItemBatchUpdateMutation,
  useWorkItemDeleteMutation,
  useWorkItemUpdateMutation,
} from './useMutations'
import {
  usePMTasksQuery,
  usePMGanttTasksQuery,
  useScopeOptionsQuery,
  useWorkItemCollectionWorkflowQuery,
  useWorkItemUsersQuery,
  useWorkItemClientOptionsQuery,
  pmTasksQueryKey,
} from './useQueries'
import { useWorkItemDetailModel } from '../detail/useModel'
import { toMutationState } from '../mutationState'
import { useWorkItemSelection } from './useSelection'
import { useWorkItemsRouteWorkflow } from './useRouteWorkflow'
import { useWorkItemBoardTransition } from './useBoardTransition'

export function useWorkItemsModel(environment: WorkItemsEnvironment) {
  const { workspaceId } = environment
  const operationAccess = environment.operationAccess
  const routeWorkflow = useWorkItemsRouteWorkflow(environment)
  const { route } = routeWorkflow
  const permissionReady = operationAccess.status === 'ready'
  const enabled = environment.allowed && permissionReady
    && operationAccess.viewTaskTable && !route.invalidLegacyScope
  const referenceDataEnabled = enabled && operationAccess.viewReferenceData
  const scopeQuery = useScopeOptionsQuery(environment, enabled)
  const hasSelectedScope = Boolean(
    route.scope.projectSetIds.length
    || route.scope.projectIds.length
    || route.scope.requirementIds.length,
  )
  const boardScopeRequired = route.view === 'board' && !hasWorkItemsBoardScope(route.scope)
  const validSelectedScope = route.view === 'board'
    ? boardScopeRequired || Boolean(
      scopeQuery.data && isValidWorkItemsBoardScope(route.scope, scopeQuery.data),
    )
    : !hasSelectedScope || Boolean(
      scopeQuery.data && isValidWorkItemsListScope(route.scope, scopeQuery.data),
    )
  const collectionEnabled = enabled
    && !boardScopeRequired
    && (!hasSelectedScope || Boolean(scopeQuery.data && validSelectedScope))
  const ganttMode = route.view === 'list' && route.layout === 'gantt'
  const collectionQuery = usePMTasksQuery(environment, route, collectionEnabled && !ganttMode)
  const ganttQuery = usePMGanttTasksQuery(environment, route, collectionEnabled && ganttMode)
  const workflowQuery = useWorkItemCollectionWorkflowQuery(environment, collectionEnabled && referenceDataEnabled)
  const clientOptionsQuery = useWorkItemClientOptionsQuery(
    environment,
    enabled && Boolean(operationAccess.viewClientOptions),
  )
  const updateMutation = useWorkItemUpdateMutation(environment, route)
  const deleteMutation = useWorkItemDeleteMutation(environment, route)
  const batchUpdateMutation = useWorkItemBatchUpdateMutation(environment, route)
  const batchDeleteMutation = useWorkItemBatchDeleteMutation(environment, route)

  const pageData = useMemo(() => {
    if (!ganttMode) return collectionQuery.data ? {
      ...collectionQuery.data,
      items: collectionQuery.data.items.map((item) => (
        applyWorkItemsOperationAccess(item, operationAccess, environment.userId)
      )),
    } : undefined
    const pages = ganttQuery.data?.pages
    if (!pages?.length) return undefined
    const items = pages.flatMap((page) => page.items)
      .map((item) => applyWorkItemsOperationAccess(item, operationAccess, environment.userId))
    const total = pages[0]?.total || 0
    return {
      items,
      total,
      truncated: items.length >= GANTT_MAX_ITEMS && items.length < total,
    }
  }, [collectionQuery.data, environment.userId, ganttMode, ganttQuery.data?.pages, operationAccess])
  const assigneeDirectoryIds = useMemo(() => workItemAssigneeDirectoryIds(
    pageData?.items ?? [],
    route.assigneeIds,
  ), [pageData?.items, route.assigneeIds])
  const usersQuery = useWorkItemUsersQuery(environment, enabled, assigneeDirectoryIds)
  const workflow = workflowQuery.data
  const collection = resolveCollectionState({
    data: pageData,
    pending: ganttMode ? ganttQuery.isPending : collectionQuery.isPending,
    fetching: ganttMode
      ? ganttQuery.isFetching && !ganttQuery.isFetchingNextPage
      : collectionQuery.isFetching,
    failed: ganttMode ? ganttQuery.isError && !ganttQuery.data : collectionQuery.isError,
    retry: () => void (ganttMode ? ganttQuery.refetch() : collectionQuery.refetch()),
  })
  const listGroups = useMemo(() => collection.status === 'ready'
    ? buildListGroups(collection.rows, route.groupBy, usersQuery.data)
    : [], [collection, route.groupBy, usersQuery.data])
  const boardGroups = useMemo(() => (
    collection.status === 'ready' || collection.status === 'empty'
      ? buildBoardGroups(
        collection.status === 'ready' ? collection.rows : [],
        route.groupBy,
        workflow?.statuses,
        usersQuery.data,
      )
      : []
  ), [collection, route.groupBy, usersQuery.data, workflow?.statuses])
  const selection = useWorkItemSelection(
    collection.status === 'ready' ? collection.rows : undefined,
    listGroups,
    route.view === 'list' && route.layout === 'table',
  )

  useEffect(() => {
    if (!pageData || route.view !== 'list' || route.layout !== 'table') return
    const maxPage = Math.max(1, Math.ceil(pageData.total / route.pageSize))
    if (route.page > maxPage) {
      routeWorkflow.navigateRoute({ ...route, page: maxPage }, true)
    }
  }, [pageData, route, routeWorkflow.navigateRoute])
  const scope: ScopeOptionsState = scopeQuery.data
    ? { status: 'ready', data: scopeQuery.data }
    : scopeQuery.isPending
      ? { status: 'loading' }
      : { status: 'error', retry: () => void scopeQuery.refetch() }
  const users: UserOptionsState = !enabled
    ? { status: 'hidden' }
    : usersQuery.isPending
      ? { status: 'loading' }
      : usersQuery.isError
        ? { status: 'error', retry: () => void usersQuery.refetch() }
        : { status: 'ready', data: usersQuery.data }
  const updateState = toMutationState({ status: updateMutation.status, action: 'update' })
  const deleteState = toMutationState({ status: deleteMutation.status, action: 'delete' })
  const batchState = batchUpdateMutation.status !== 'idle'
    ? toMutationState({
      status: batchUpdateMutation.status,
      action: 'batch-patch',
      failures: batchUpdateMutation.data?.failures,
      succeeded: batchUpdateMutation.data?.succeeded,
    })
    : toMutationState({
      status: batchDeleteMutation.status,
      action: 'batch-delete',
      failures: batchDeleteMutation.data?.failures,
      succeeded: batchDeleteMutation.data?.succeeded,
    })
  const detailModel = useWorkItemDetailModel({
    environment,
    id: route.detailId,
    enabled: enabled && operationAccess.viewTaskDetail,
    collectionKey: pmTasksQueryKey(environment.workspaceId, route),
  })

  const updateItem = useCallback(async (row: WorkItem, patch: WorkItemDetailPatch) => {
    if (!row.capabilities.canUpdate) return false
    if (patch.status && !row.capabilities.allowedTransitions.includes(patch.status)) return false
    if (patch.priority !== undefined
      && !['low', 'normal', 'high', 'urgent'].includes(patch.priority)) return false
    if (patch.assigneeId !== undefined && patch.assigneeId !== null) {
      if (users.status !== 'ready'
        || !users.data.some((user) => user.id === patch.assigneeId)) return false
    }
    const title = patch.title?.trim().slice(0, 200)
    if (patch.title !== undefined && !title) return false
    const validDate = (value: string | null | undefined) => (
      value === null || value === undefined || /^\d{4}-\d{2}-\d{2}$/.test(value)
    )
    if (!validDate(patch.startsAt) || !validDate(patch.dueAt)) return false
    const startsAt = patch.startsAt === undefined ? row.startsAt : patch.startsAt || undefined
    const dueAt = patch.dueAt === undefined ? row.dueAt : patch.dueAt || undefined
    if (startsAt && dueAt && startsAt > dueAt) return false
    if (!isValidEstimatedMinutes(patch.estimatedMinutes)) return false
    await updateMutation.mutateAsync({ row, patch: { ...patch, ...(title ? { title } : {}) } })
    return true
  }, [updateMutation, users])
  const boardTransition = useWorkItemBoardTransition(workflow, updateItem)
  const deleteItem = useCallback(async (row: WorkItem, options?: { cascade?: boolean }) => {
    if (!row.capabilities.canDelete) return false
    await deleteMutation.mutateAsync({ row, cascade: options?.cascade === true })
    if (route.detailId === row.id) routeWorkflow.closeDetail()
    return true
  }, [deleteMutation, route.detailId, routeWorkflow.closeDetail])
  const batchUpdateStatus = useCallback(async (status: string) => {
    if (!selection.selectedRows.length || !selection.selectedRows.every((row) => (
      row.capabilities.canUpdate
      && row.capabilities.allowedTransitions.includes(status)
    ))) return false
    batchDeleteMutation.reset()
    await batchUpdateMutation.mutateAsync({ rows: selection.selectedRows, patch: { status } })
    return true
  }, [batchDeleteMutation, batchUpdateMutation, selection.selectedRows])
  const batchDelete = useCallback(async () => {
    if (!selection.canBatchDelete) return false
    batchUpdateMutation.reset()
    await batchDeleteMutation.mutateAsync(selection.selectedRows)
    return true
  }, [batchDeleteMutation, batchUpdateMutation, selection.canBatchDelete, selection.selectedRows])
  const refresh: ReadyRefresh = collection.status === 'ready' ? collection.refresh : 'idle'
  const ganttLoadRequested = useRef(false)
  const loadMoreGantt = useCallback(() => {
    if (!ganttQuery.hasNextPage || ganttQuery.isFetchingNextPage || ganttLoadRequested.current) return
    ganttLoadRequested.current = true
    void ganttQuery.fetchNextPage().finally(() => {
      ganttLoadRequested.current = false
    })
  }, [ganttQuery.fetchNextPage, ganttQuery.hasNextPage, ganttQuery.isFetchingNextPage])

  return {
    allowed: enabled,
    canViewDetail: permissionReady && operationAccess.viewTaskDetail,
    permissionStatus: operationAccess.status,
    retryPermissions: operationAccess.retry,
    ...routeWorkflow,
    invalidScope: route.invalidLegacyScope || (hasSelectedScope && Boolean(
      scopeQuery.data && !validSelectedScope && !boardScopeRequired,
    )),
    boardScopeRequired,
    ...selection,
    updateItem,
    deleteItem,
    batchUpdateStatus,
    batchDelete,
    updateState,
    deleteState,
    batchState,
    scope,
    users,
    clientOptions: clientOptionsQuery.data || [],
    collection,
    listGroups,
    boardGroups,
    boardTransition,
    statusOptions: workflow?.statuses.map(({ key, label }) => ({ value: key, label })) || [],
    workflow,
    ganttPagination: {
      hasMore: Boolean(ganttQuery.hasNextPage),
      loadingMore: ganttQuery.isFetchingNextPage,
      loaded: pageData?.items.length || 0,
      total: pageData?.total || 0,
      loadMore: loadMoreGantt,
    },
    detailPanel: { ...detailModel, state: detailModel.detail },
    refresh,
  }
}
