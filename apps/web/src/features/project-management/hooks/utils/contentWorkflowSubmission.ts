import { useCallback } from 'react'
import type { MutableRefObject } from 'react'

import type { NavigateOptions } from '@tanstack/react-router'
import type { ProjectWorkTask, ScrumProductRequirementNew, ScrumVersionTaskNew } from '../../api'
import type {
  RequirementActions,
  RequirementEditorState,
  RequirementMoveDraft,
  RequirementMoveState,
  TaskActions,
  TaskEditorState,
  ContentDeleteState,
} from '../../model'
import type { ProjectOperationAccess } from '../useOperationAccess'
import type { useUpdateRequirementMutation } from '../useContentMutations'
import type { ProjectContentSearchPatch } from './contentWorkflowHelpers'
import { submitRequirement, submitTask, submitMove, confirmDelete } from './contentWorkflowMutations'

export interface SubmissionHandlersInput {
  requirementEditor: RequirementEditorState
  taskEditor: TaskEditorState
  move: RequirementMoveState
  deleting: ContentDeleteState
  operationLock: MutableRefObject<symbol | null>
  currentRequirement: (id: number) => ScrumProductRequirementNew | undefined
  currentTask: (id: number) => ProjectWorkTask | undefined
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  taskActions: (row?: ProjectWorkTask) => TaskActions
  operationAccess: Pick<
    ProjectOperationAccess,
    'createPlanRequirement' | 'updatePlanRequirement' | 'deletePlanRequirement' | 'createTask' | 'updateTask' | 'deleteTask'
  >
  targetBoardData: { version_tasks: any[] } | undefined
  taskRequirementRows: ScrumProductRequirementNew[]
  createRequirement: { execute: (...args: any[]) => any }
  updateRequirement: ReturnType<typeof useUpdateRequirementMutation>
  moveRequirement: { execute: (...args: any[]) => any }
  deleteRequirement: { execute: (...args: any[]) => any }
  createTask: { execute: (...args: any[]) => any }
  updateTask: { execute: (...args: any[]) => any }
  deleteTask: { execute: (...args: any[]) => any }
  dispatchRequirementEditor: (action: any) => void
  dispatchTaskEditor: (action: any) => void
  dispatchMove: (action: any) => void
  dispatchDelete: (action: any) => void
  setFeedback: (fb: any) => void
  navigate: (opts: NavigateOptions) => void
  navigateSearch: (patch: ProjectContentSearchPatch, replace?: boolean) => void
  requirementRef: MutableRefObject<ScrumProductRequirementNew | null>
  taskRef: MutableRefObject<ProjectWorkTask | null>
  openedTaskDeepLinkIdRef: MutableRefObject<number | null>
  closingTaskDeepLinkIdRef: MutableRefObject<number | null>
}

export function useSubmissionHandlers(input: SubmissionHandlersInput) {
  const {
    requirementEditor,
    taskEditor,
    move,
    deleting,
    operationLock,
    currentRequirement,
    currentTask,
    requirementActions,
    taskActions,
    operationAccess,
    targetBoardData,
    taskRequirementRows,
    createRequirement,
    updateRequirement,
    moveRequirement,
    deleteRequirement,
    createTask,
    updateTask,
    deleteTask,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    setFeedback,
    navigate,
    navigateSearch,
    requirementRef,
    taskRef,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  } = input

  const handleSubmitRequirement = useCallback(() => submitRequirement({
    requirementEditor,
    operationLock,
    currentRequirement,
    requirementActions,
    operationAccess,
    targetBoardData,
    createRequirement,
    updateRequirement,
    dispatchRequirementEditor,
    setFeedback,
    navigate,
    requirementRef,
  }), [createRequirement, currentRequirement, navigate, operationAccess, requirementActions, requirementEditor, targetBoardData, updateRequirement])

  const handleSubmitTask = useCallback(() => submitTask({
    taskEditor,
    operationLock,
    currentTask,
    taskActions,
    operationAccess,
    requirementActions,
    taskRequirementRows,
    requirementRef,
    createTask,
    updateTask,
    dispatchTaskEditor,
    setFeedback,
    navigateSearch,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
    taskRef,
  }), [createTask, currentTask, navigateSearch, operationAccess, requirementActions, taskActions, taskEditor, taskRequirementRows, updateTask])

  const handleSubmitMove = useCallback(async (confirmSubjectSchedule = false) => {
    return submitMove({
      move, operationLock, currentRequirement,
      targetBoardData,
      moveRequirement, dispatchMove, setFeedback, requirementRef,
    }, confirmSubjectSchedule)
  }, [currentRequirement, move, moveRequirement, targetBoardData])

  const handleConfirmDelete = useCallback(() => confirmDelete({
    deleting, operationLock, currentRequirement, currentTask,
    requirementActions, taskActions,
    deleteRequirement, deleteTask,
    dispatchDelete, setFeedback,
  }), [currentRequirement, currentTask, deleteRequirement, deleteTask, deleting, requirementActions, taskActions])

  return {
    handleSubmitRequirement,
    handleSubmitTask,
    handleSubmitMove,
    handleConfirmDelete,
  }
}
