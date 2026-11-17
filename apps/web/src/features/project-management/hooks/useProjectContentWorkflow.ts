import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  ownerDirectoryItems,
} from '../model'
import {
  useCreateRequirementMutation,
  useCreateTaskMutation,
  useDeleteRequirementMutation,
  useDeleteTaskMutation,
  useMoveRequirementMutation,
  useUpdateRequirementMutation,
  useUpdateTaskMutation,
} from './useContentMutations'
import type { ProjectOperationAccess } from './useOperationAccess'
import { DEFAULT_SEARCH, DEFAULT_OPERATION_ACCESS } from './utils/contentWorkflowHelpers'
import { useRequirementFilters, useTaskFilters } from './utils/contentWorkflowFilters'
import { useReloadLatestMove } from './utils/contentWorkflowMoveReload'
import { useSubmissionHandlers } from './utils/contentWorkflowSubmission'
import {
  useTargetProjectsState,
  useTargetPlanUnitsState,
  useWorkflowTemplatesState,
  useUserDirectoryState,
} from './utils/contentWorkflowTargets'
import { useContentQueries } from './utils/contentWorkflowQueries'
import { useContentAccessors } from './utils/contentWorkflowAccessors'
import { useContentWorkflowState } from './utils/contentWorkflowState'
import { useTaskDeepLink } from './utils/contentWorkflowDeepLinkHook'
import { buildContentWorkflowReturn } from './utils/contentWorkflowReturnBuilder'

import type { ProjectWorkspaceRoute } from '../model'

type PlanUnitRoute = Extract<ProjectWorkspaceRoute, { kind: 'plan-unit' }>

export function useProjectContentWorkflow(input: {
  route: PlanUnitRoute
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  users: PlatformUser[]
  operationAccess?: Pick<
    ProjectOperationAccess,
    | 'viewProjectSets'
    | 'viewBoard'
    | 'viewTaskExecution'
    | 'viewPlanRequirements'
    | 'createPlanRequirement'
    | 'updatePlanRequirement'
    | 'deletePlanRequirement'
    | 'createTask'
    | 'updateTask'
    | 'deleteTask'
  >
}) {
  const operationAccess = input.operationAccess ?? DEFAULT_OPERATION_ACCESS
  const navigate = useNavigate()
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const search = input.route.search ?? DEFAULT_SEARCH
  const scopeKey = `${workspaceId}:${session.revision}:${input.route.projectId}:${input.route.planUnitId}`
  const embedded = session.mode === 'embedded'
  const requirementsEnabled = input.route.tab === 'requirements'
    && operationAccess.viewPlanRequirements
  const tasksEnabled = input.route.tab === 'tasks'
    && operationAccess.viewTaskExecution
    && !embedded

  const {
    requirementEditor,
    dispatchRequirementEditor,
    taskEditor,
    dispatchTaskEditor,
    move,
    dispatchMove,
    deleting,
    dispatchDelete,
    feedback,
    setFeedback,
    workflowScopeRef,
    operationLock,
    moveReloadOperation,
    moveStateRef,
    requirementRef,
    taskRef,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  } = useContentWorkflowState({ scopeKey })

  const {
    requirements,
    taskRequirementRows,
    tasks,
    taskDeepLinkId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery,
    targetsOpen,
    targetProjectsQuery,
    targetBoardQuery,
    workflowTemplatesQuery,
  } = useContentQueries({
    projectId: input.route.projectId,
    planUnitId: input.route.planUnitId,
    planUnitTitle: input.unit.title,
    users: input.users as any,
    search,
    requirementsEnabled,
    tasksEnabled,
    operationAccess: operationAccess as any,
    requirementEditor: requirementEditor as any,
    move: move as any,
    defaultProjectId: input.project.id,
  })

  const { projectsState } = useTargetProjectsState({
    open: targetsOpen,
    viewProjectSets: operationAccess.viewProjectSets,
    query: targetProjectsQuery,
  })
  const { planUnitsState } = useTargetPlanUnitsState({
    open: targetsOpen,
    viewBoard: operationAccess.viewBoard,
    move: move as any,
    query: targetBoardQuery,
  })
  const workflowTemplatesState = useWorkflowTemplatesState({
    open: requirementEditor.status === 'editing' && requirementEditor.draft.mode === 'create',
    query: workflowTemplatesQuery,
  })
  const userState = useUserDirectoryState(
    { users: input.users as any },
    ownerDirectoryItems as any,
  )

  const closeCompeting = useCallback(() => {
    moveReloadOperation.current = null
    dispatchRequirementEditor({ type: 'close' })
    dispatchTaskEditor({ type: 'close' })
    dispatchMove({ type: 'close' })
    dispatchDelete({ type: 'close' })
  }, [])

  const {
    currentRequirement,
    currentTask,
    requirementActions,
    taskActions,
    fields,
    navigateSearch,
    selectedTaskRequirement,
    actions,
  } = useContentAccessors({
    project: input.project,
    unit: input.unit,
    requirementsEnabled,
    requirementsRows: requirements.rows,
    taskRequirementRows,
    tasksRows: tasks.rows,
    requirementRef: requirementRef as any,
    taskRef: taskRef as any,
    operationAccess: operationAccess as any,
    search,
    route: input.route as any,
    embedded,
    closeCompeting,
    setFeedback: setFeedback as any,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    operationLock,
  })

  const requirementFilters = useRequirementFilters({ search: search.requirement, navigateSearch })
  const taskFilters = useTaskFilters({ search: search.task, taskRequirementRows, navigateSearch })

  const { taskDeepLink, closeTaskEditor } = useTaskDeepLink({
    taskDeepLinkId,
    tasksEnabled,
    projectId: input.route.projectId,
    planUnitId: input.route.planUnitId,
    requirementId: search.task.requirementId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery: taskDeepLinkQuery as any,
    navigateSearch,
    closeCompeting,
    setFeedback: setFeedback as any,
    dispatchTaskEditor,
    taskRef: taskRef as any,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  })

  // Mutations
  const createRequirement = useCreateRequirementMutation()
  const updateRequirement = useUpdateRequirementMutation()
  const moveRequirement = useMoveRequirementMutation()
  const deleteRequirement = useDeleteRequirementMutation()
  const createTask = useCreateTaskMutation()
  const updateTask = useUpdateTaskMutation()
  const deleteTask = useDeleteTaskMutation()

  const {
    handleSubmitRequirement,
    handleSubmitTask,
    handleSubmitMove,
    handleConfirmDelete,
  } = useSubmissionHandlers({
    requirementEditor,
    taskEditor,
    move,
    deleting: deleting as any,
    operationLock,
    currentRequirement,
    currentTask,
    requirementActions,
    taskActions,
    operationAccess: operationAccess as any,
    targetBoardData: targetBoardQuery.data as any,
    taskRequirementRows,
    createRequirement: createRequirement as any,
    updateRequirement: updateRequirement as any,
    moveRequirement: moveRequirement as any,
    deleteRequirement: deleteRequirement as any,
    createTask: createTask as any,
    updateTask: updateTask as any,
    deleteTask: deleteTask as any,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    setFeedback: setFeedback as any,
    navigate,
    navigateSearch,
    requirementRef: requirementRef as any,
    taskRef: taskRef as any,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  })

  const { reloadLatestMove, closeMove } = useReloadLatestMove({
    projectId: input.route.projectId,
    planUnitId: input.route.planUnitId,
    scopeKey,
    requirements: requirements as any,
    targetBoardData: targetBoardQuery.data as any,
    targetProjectsData: targetProjectsQuery.data as any,
    dispatchMove,
    setFeedback: setFeedback as any,
    requirementRef: requirementRef as any,
    moveReloadOperation,
    moveStateRef,
    workflowScopeRef,
  })

  return buildContentWorkflowReturn({
    embedded,
    feedback,
    requirements: {
      state: requirements.state,
      rows: requirements.rows,
      users: requirements.users,
      filters: requirementFilters,
    },
    tasks: { state: tasks.state, rows: tasks.rows, filters: taskFilters },
    selectedTaskRequirement,
    requirementActions,
    taskActions,
    operationAccess,
    unit: input.unit,
    actions,
    requirementEditor,
    taskEditor,
    taskDeepLink,
    move,
    deleting,
    projectsState,
    planUnitsState,
    workflowTemplatesState,
    userState,
    fields,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    handleSubmitRequirement,
    handleSubmitTask,
    handleSubmitMove,
    handleConfirmDelete,
    closeTaskEditor,
    reloadLatestMove,
    closeMove,
  })
}
