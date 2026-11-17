import { useCallback, useEffect, useMemo, useState } from 'react'

import type { QualityEnvironment } from '../components/environment'
import {
  qualityCreatePath,
  qualityDetailPath,
  qualityListPath,
} from '../model/route'
import { qualityBugDirectoryUserIds } from '../model/presenters'
import type {
  QualityBug,
  QualityAssignPayload,
  QualityFilters,
  QualityResolvePayload,
  QualityReturnPayload,
  QualityRouteState,
  QualityScope,
} from '../model/types'
import {
  useBatchDeleteMutation,
  useDeleteMutation,
  useInlineEditMutation,
  useListAssignMutation,
  useUploadMutation,
} from './useMutations'
import {
  useQualityListQuery,
  useQualityDashboardQuery,
  useQualitySummaryQuery,
  useQualityUsersQuery,
  useQualitySceneOptionCatalogs,
} from './useQueries'
import { useQualitySelection } from './useSelection'
import { useQualityOperationAccess } from './useOperationAccess'

export function useQualityListWorkflow(
  environment: QualityEnvironment,
  route: QualityRouteState,
) {
  const access = useQualityOperationAccess()
  const list = useQualityListQuery(environment, route, access.canViewBugs)
  const dashboard = useQualityDashboardQuery(environment, route, access.canViewDashboard)
  const summary = useQualitySummaryQuery(environment, route, access.canViewBugs)
  const directoryUserIds = useMemo(() => qualityBugDirectoryUserIds(
    list.state.status === 'ready' ? list.state.rows : [],
    [
      ...route.filters.assigneeUserIds,
      ...route.filters.ownerUserIds,
      ...route.filters.reporterUserIds,
    ],
  ), [
    list.state,
    route.filters.assigneeUserIds,
    route.filters.ownerUserIds,
    route.filters.reporterUserIds,
  ])
  const users = useQualityUsersQuery(environment, directoryUserIds)
  const sceneOptions = useQualitySceneOptionCatalogs(environment, access.canViewSceneOptions)
  const remove = useDeleteMutation(environment, access.canDeleteBug)
  const batchRemove = useBatchDeleteMutation(environment, access.canDeleteBug)
  const assignment = useListAssignMutation(environment, access.canAssignBug)
  const inlineEdit = useInlineEditMutation(
    environment,
    route,
    access.canUpdateBug,
    access.canTransitionBug,
  )
  const [searchDraft, setSearchDraftState] = useState(route.filters.q)
  const [deleteTarget, setDeleteTarget] = useState<QualityBug | null>(null)
  const [resolutionTarget, setResolutionTarget] = useState<{
    bug: QualityBug
    status: QualityResolvePayload['status']
  } | null>(null)
  const [returnTarget, setReturnTarget] = useState<QualityBug | null>(null)
  const [assignmentTargets, setAssignmentTargets] = useState<readonly QualityBug[]>([])
  const canUploadResolution = access.canUploadMaterial(Boolean(
    resolutionTarget && access.canUpdateBug(resolutionTarget.bug),
  ))
  const canUploadReturn = access.canUploadMaterial(Boolean(
    returnTarget && access.canUpdateBug(returnTarget),
  ))
  const canUploadAssignment = access.canUploadMaterial(
    assignmentTargets.length === 1 && access.canAssignBug(assignmentTargets[0]),
  )
  const upload = useUploadMutation(
    environment,
    canUploadResolution || canUploadReturn || canUploadAssignment,
  )
  const canSelectBug = useCallback(
    (bug: QualityBug) => access.canAssignBug(bug) || access.canDeleteBug(bug),
    [access.canAssignBug, access.canDeleteBug],
  )
  const selection = useQualitySelection(
    list.state.status === 'ready' ? list.state.rows : undefined,
    canSelectBug,
  )

  useEffect(() => setSearchDraftState(route.filters.q), [route.filters.q])
  const setSearchDraft = useCallback((value: string) => {
    const normalized = value.trim().slice(0, 120)
    setSearchDraftState(normalized)
    if (normalized === route.filters.q) return
    void environment.navigate(qualityListPath({
      ...route,
      kind: 'list',
      page: 1,
      filters: {
        ...route.filters,
        q: normalized,
      },
    }), true)
  }, [environment, route])
  useEffect(() => {
    setDeleteTarget(null)
    setResolutionTarget(null)
    setReturnTarget(null)
    setAssignmentTargets([])
    remove.reset()
    batchRemove.reset()
    assignment.reset()
    inlineEdit.reset()
    upload.reset()
  }, [assignment.reset, batchRemove.reset, environment.workspaceId, inlineEdit.reset, remove.reset, upload.reset])

  const replaceList = useCallback((next: QualityRouteState) => (
    environment.navigate(qualityListPath(next), true)
  ), [environment])
  const setScope = useCallback((scope: QualityScope) => replaceList({
    ...route,
    scope,
    page: 1,
  }), [replaceList, route])
  const setFilters = useCallback((filters: Partial<QualityFilters>) => replaceList({
    ...route,
    filters: { ...route.filters, ...filters },
    page: 1,
  }), [replaceList, route])
  const setPage = useCallback((page: number) => replaceList({
    ...route,
    page: Math.max(1, page),
  }), [replaceList, route])
  const setPageSize = useCallback((pageSize: QualityRouteState['pageSize']) => replaceList({
    ...route,
    page: 1,
    pageSize,
  }), [replaceList, route])
  const setView = useCallback((view: QualityRouteState['view']) => replaceList({
    ...route,
    view,
    page: 1,
  }), [replaceList, route])
  const clearFilters = useCallback(() => replaceList({
    ...route,
    filters: {
      q: '', status: '', severities: [], environments: [], assigneeUserIds: [], ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '',
      createdFrom: '', createdTo: '', sortBy: 'created_at', sortOrder: 'desc',
    },
    page: 1,
  }), [replaceList, route])
  const openCreate = useCallback(
    () => access.canCreateBug
      ? environment.navigate(qualityCreatePath(route))
      : Promise.resolve(),
    [access.canCreateBug, environment, route],
  )
  const openDetail = useCallback(
    (row: QualityBug, mode: 'view' | 'edit' = 'view') => (
      access.canViewBugs && (mode === 'view' || access.canUpdateBug(row))
        ? environment.navigate(qualityDetailPath(route, row.id, mode))
        : Promise.resolve()
    ),
    [access, environment, route],
  )
  const changeDeleteTarget = useCallback((target: QualityBug | null) => {
    if (!target) remove.reset()
    setDeleteTarget(target)
  }, [remove.reset])
  const saveInline = useCallback(
    (bug: QualityBug, payload: Record<string, unknown>) => inlineEdit.run({ bug, payload }),
    [inlineEdit.run],
  )
  const changeStatus = useCallback(async (
    bug: QualityBug,
    status: string,
    resolutionReason?: string,
  ) => {
    if (status === 'reopened') {
      inlineEdit.reset()
      upload.reset()
      setReturnTarget(bug)
      return
    }
    if (status === 'review') {
      inlineEdit.reset()
      upload.reset()
      setResolutionTarget({ bug, status })
      return
    }
    if (status === 'done') {
      if (!resolutionReason) {
        inlineEdit.reset()
        upload.reset()
        setResolutionTarget({ bug, status })
        return
      }
      await saveInline(bug, {
        status,
        resolution_reason: resolutionReason,
      })
      return
    }
    await saveInline(bug, { status })
  }, [inlineEdit.reset, saveInline, upload.reset])
  const changeResolutionOpen = useCallback((open: boolean) => {
    if (open) return
    setResolutionTarget(null)
    inlineEdit.reset()
    upload.reset()
  }, [inlineEdit.reset, upload.reset])
  const submitResolution = useCallback(async (payload: QualityResolvePayload) => {
    if (!resolutionTarget) return
    await saveInline(resolutionTarget.bug, payload)
    setResolutionTarget(null)
  }, [resolutionTarget, saveInline])
  const changeReturnOpen = useCallback((open: boolean) => {
    if (open) return
    setReturnTarget(null)
    inlineEdit.reset()
    upload.reset()
  }, [inlineEdit.reset, upload.reset])
  const submitReturn = useCallback(async (payload: QualityReturnPayload) => {
    if (!returnTarget) return
    await saveInline(returnTarget, payload)
    setReturnTarget(null)
  }, [returnTarget, saveInline])
  const openAssignment = useCallback((bug: QualityBug) => {
    if (!access.canAssignBug(bug)) return
    assignment.reset()
    upload.reset()
    setAssignmentTargets([bug])
  }, [access.canAssignBug, assignment.reset, upload.reset])
  const openBatchAssignment = useCallback(() => {
    if (!selection.selectedRows.length || !selection.selectedRows.every(access.canAssignBug)) return
    assignment.reset()
    upload.reset()
    setAssignmentTargets(selection.selectedRows)
  }, [access.canAssignBug, assignment.reset, selection.selectedRows, upload.reset])
  const changeAssignmentOpen = useCallback((open: boolean) => {
    if (open) return
    setAssignmentTargets([])
    assignment.reset()
    upload.reset()
  }, [assignment.reset, upload.reset])
  const submitAssignment = useCallback(async (payload: QualityAssignPayload) => {
    if (!assignmentTargets.length) return
    const wasBatch = assignmentTargets.length > 1
    await assignment.run({ bugs: assignmentTargets, payload })
    setAssignmentTargets([])
    if (wasBatch) selection.clearSelection()
  }, [assignment.run, assignmentTargets, selection.clearSelection])
  const confirmDelete = useCallback(async () => {
    if (!deleteTarget || !access.canDeleteBug(deleteTarget)) return
    await remove.run(deleteTarget)
    setDeleteTarget(null)
    if (
      list.state.status === 'ready' &&
      list.state.rows.length === 1 &&
      route.page > 1
    ) await setPage(route.page - 1)
  }, [access, deleteTarget, list.state, remove.run, route.page, setPage])
  const confirmBatchDelete = useCallback(async () => {
    if (!selection.selectedRows.length || !selection.selectedRows.every(access.canDeleteBug)) return
    const deletesWholePage = list.state.status === 'ready'
      && selection.selectedRows.length === list.state.rows.length
    await batchRemove.run(selection.selectedRows)
    selection.clearSelection()
    if (deletesWholePage && route.page > 1) await setPage(route.page - 1)
  }, [access.canDeleteBug, batchRemove.run, list.state, route.page, selection, setPage])

  return {
    list: list.state,
    dashboard,
    summary,
    users,
    sceneOptions,
    canViewBugs: access.canViewBugs,
    canViewDashboard: access.canViewDashboard,
    canCreate: access.canCreateBug,
    canUpdateBug: access.canUpdateBug,
    canTransitionBug: access.canTransitionBug,
    canAssignBug: access.canAssignBug,
    canSelectBug,
    canDeleteBug: access.canDeleteBug,
    canUploadResolution,
    canUploadReturn,
    canUploadAssignment,
    searchDraft,
    deleteTarget,
    resolutionTarget,
    returnTarget,
    assignmentTargets,
    remove,
    batchRemove,
    assignment,
    inlineEdit,
    upload,
    setSearchDraft,
    setScope,
    setFilters,
    setPage,
    setPageSize,
    setView,
    clearFilters,
    openCreate,
    openDetail,
    saveInline,
    changeStatus,
    changeResolutionOpen,
    submitResolution,
    changeReturnOpen,
    submitReturn,
    openAssignment,
    openBatchAssignment,
    changeAssignmentOpen,
    submitAssignment,
    setDeleteTarget: changeDeleteTarget,
    confirmDelete,
    confirmBatchDelete,
    selection,
  }
}
