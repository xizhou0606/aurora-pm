import { useMemo, useRef } from 'react'

import type {
  ScrumProductRequirementNew,
  ScrumVersionTaskNew,
} from '../../api'
import {
  useProjectBoardQuery,
  useProjectListQuery,
  usePublishedRequirementWorkflowTemplatesQuery,
  useRequirementListQuery,
  useTaskDeepLinkQuery,
} from '../../model'
import { useRequirementListModel, useTaskListModel } from '../useContentListModels'
import type { ProjectOperationAccess } from '../useOperationAccess'
import type { PlanUnitContentSearch } from '../../model'
import { DEFAULT_SEARCH } from './contentWorkflowHelpers'

export interface ContentQueriesInput {
  projectId: number
  planUnitId: number
  planUnitTitle: string
  users: any[]
  search: PlanUnitContentSearch
  requirementsEnabled: boolean
  tasksEnabled: boolean
  operationAccess: Pick<ProjectOperationAccess, 'viewProjectSets' | 'viewBoard'>
  requirementEditor: { status: string; draft: { mode: string; projectId: number } }
  move: { status: string; draft: { targetProjectId: number } }
  defaultProjectId: number
}

export function useContentQueries(input: ContentQueriesInput) {
  const {
    projectId,
    planUnitId,
    planUnitTitle,
    users,
    search,
    requirementsEnabled,
    tasksEnabled,
    operationAccess,
    requirementEditor,
    move,
    defaultProjectId,
  } = input

  const requirements = useRequirementListModel({
    projectId,
    planUnitId,
    planUnitTitle,
    search: search.requirement,
    users,
    enabled: requirementsEnabled,
  })
  const taskRequirementOptionsQuery = useRequirementListQuery({
    projectNewId: projectId,
    versionTaskNewId: planUnitId,
    limit: 200,
    offset: 0,
  }, tasksEnabled)
  const taskRequirementRows = taskRequirementOptionsQuery.data?.rows ?? []
  const tasks = useTaskListModel({
    projectId,
    planUnitId,
    search: search.task,
    users,
    enabled: tasksEnabled,
  })
  const taskDeepLinkId = search.task.taskId
  const taskDeepLinkPageTarget = taskDeepLinkId
    ? tasks.rows.find((row) => (
        row.id === taskDeepLinkId &&
        row.project_set_id === projectId &&
        row.project_id === planUnitId &&
        row.requirement_id === search.task.requirementId
      ))
    : undefined
  const taskDeepLinkQuery = useTaskDeepLinkQuery({
    projectNewId: projectId,
    versionTaskNewId: planUnitId,
    requirementNewId: search.task.requirementId ?? 0,
    taskId: taskDeepLinkId ?? 0,
  }, Boolean(
    tasksEnabled &&
    taskDeepLinkId &&
    search.task.requirementId &&
    !taskDeepLinkPageTarget
  ))

  const targetsOpen = (
    requirementEditor.status === 'editing' && requirementEditor.draft.mode === 'create'
  ) || move.status === 'selecting'
  const targetProjectId = requirementEditor.status === 'editing' && requirementEditor.draft.mode === 'create'
    ? requirementEditor.draft.projectId
    : move.status === 'selecting'
      ? move.draft.targetProjectId
      : defaultProjectId
  const targetProjectsQuery = useProjectListQuery(
    { limit: 200, offset: 0 },
    targetsOpen && operationAccess.viewProjectSets,
  )
  const targetBoardQuery = useProjectBoardQuery(
    targetProjectId,
    targetsOpen && operationAccess.viewBoard,
  )
  const workflowTemplatesQuery = usePublishedRequirementWorkflowTemplatesQuery(
    requirementEditor.status === 'editing' && requirementEditor.draft.mode === 'create',
  )

  return {
    requirements,
    taskRequirementRows,
    tasks,
    taskDeepLinkId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery,
    targetsOpen,
    targetProjectId,
    targetProjectsQuery,
    targetBoardQuery,
    workflowTemplatesQuery,
  }
}