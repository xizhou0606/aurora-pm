import { useCallback, useMemo } from 'react'
import type { MutableRefObject } from 'react'
import { useNavigate } from '@tanstack/react-router'

import type { ProjectWorkTask, ScrumProductRequirementNew, ScrumVersionTaskNew } from '../../api'
import {
  requirementActionAvailability,
  requirementEditableFields,
  taskActionAvailability,
  type RequirementActions,
  type RequirementEditableFields,
  type TaskActions,
} from '../../model'
import type { ProjectOperationAccess } from '../useOperationAccess'
import {
  findRequirement,
  findTask,
  type ProjectContentSearchPatch,
} from './contentWorkflowHelpers'
import { createContentWorkflowActions } from './contentWorkflowActions'

export interface AccessorsInput {
  project: { id: number }
  unit: ScrumVersionTaskNew
  requirementsEnabled: boolean
  requirementsRows: ScrumProductRequirementNew[]
  taskRequirementRows: ScrumProductRequirementNew[]
  tasksRows: ProjectWorkTask[]
  requirementRef: MutableRefObject<ScrumProductRequirementNew | null>
  taskRef: MutableRefObject<ProjectWorkTask | null>
  operationAccess: Pick<
    ProjectOperationAccess,
    'createPlanRequirement' | 'createTask' | 'updatePlanRequirement' | 'deletePlanRequirement' | 'updateTask' | 'deleteTask'
  >
  search: { task: { requirementId?: number } }
  route: { projectId: number; planUnitId: number; tab: string }
  embedded: boolean
  closeCompeting: () => void
  setFeedback: (fb: { status: 'idle' | 'success' | 'error'; message?: string }) => void
  dispatchRequirementEditor: (action: any) => void
  dispatchTaskEditor: (action: any) => void
  dispatchMove: (action: any) => void
  dispatchDelete: (action: any) => void
  operationLock: MutableRefObject<symbol | null>
}

export function useContentAccessors(input: AccessorsInput) {
  const {
    project,
    unit,
    requirementsEnabled,
    requirementsRows,
    taskRequirementRows,
    tasksRows,
    requirementRef,
    taskRef,
    operationAccess,
    search,
    route,
    embedded,
    closeCompeting,
    setFeedback,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    operationLock,
  } = input

  const navigate = useNavigate()

  const currentRequirementRows = requirementsEnabled ? requirementsRows : taskRequirementRows
  const currentRequirement = useCallback((id: number) => findRequirement(
    id,
    currentRequirementRows,
    requirementRef.current,
  ), [currentRequirementRows])
  const currentTask = useCallback((id: number) => findTask(id, tasksRows, taskRef.current), [tasksRows])
  const requirementActions = useCallback((row?: ScrumProductRequirementNew): RequirementActions => {
    const availability = requirementActionAvailability(row)
    return {
      ...availability,
      createTask: operationAccess.createTask && availability.createTask,
      update: operationAccess.updatePlanRequirement && availability.update,
      delete: operationAccess.deletePlanRequirement && availability.delete,
      move: operationAccess.updatePlanRequirement && availability.move,
      editableActions: operationAccess.updatePlanRequirement
        ? availability.editableActions
        : new Set(),
    }
  }, [
    operationAccess.createTask,
    operationAccess.deletePlanRequirement,
    operationAccess.updatePlanRequirement,
  ])
  const taskActions = useCallback((row?: ProjectWorkTask): TaskActions => {
    const availability = taskActionAvailability(row)
    return {
      ...availability,
      update: operationAccess.updateTask && availability.update,
      delete: operationAccess.deleteTask && availability.delete,
    }
  }, [operationAccess.deleteTask, operationAccess.updateTask])
  const fields = useMemo<RequirementEditableFields>(() => {
    const available = requirementEditableFields()
    return operationAccess.updatePlanRequirement
      ? available
      : Object.fromEntries(Object.keys(available).map((key) => [key, false])) as RequirementEditableFields
  }, [operationAccess.updatePlanRequirement])

  const navigateSearch = useCallback((patch: ProjectContentSearchPatch, replace = true) => {
    void navigate({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: {
        projectId: String(route.projectId),
        projectItemId: String(route.planUnitId),
      },
      search: (previous: any) => ({
        ...previous,
        ...patch,
        tab: patch.tab ?? previous.tab ?? route.tab,
        from_item: undefined,
      }),
      replace,
    })
  }, [route.planUnitId, route.projectId, navigate])

  const selectedTaskRequirement = search.task.requirementId
    ? taskRequirementRows.find((row) => row.id === search.task.requirementId)
    : undefined

  const actions = useMemo(() => createContentWorkflowActions({
    project,
    unit,
    operationAccess,
    requirementActions,
    taskActions,
    closeCompeting,
    setFeedback,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    operationLock,
    requirementRef,
    taskRef,
    currentRequirement,
    navigateSearch,
    embedded,
    selectedTaskRequirement,
  }), [
    closeCompeting, currentRequirement, embedded, project, unit,
    navigateSearch, operationAccess, requirementActions, selectedTaskRequirement, taskActions,
  ])

  return {
    currentRequirement,
    currentTask,
    requirementActions,
    taskActions,
    fields,
    navigateSearch,
    selectedTaskRequirement,
    actions,
  }
}