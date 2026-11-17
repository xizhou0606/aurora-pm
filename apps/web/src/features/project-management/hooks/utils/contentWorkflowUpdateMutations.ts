import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
} from '../../api'
import type {
  RequirementActions,
  RequirementMoveState,
  ContentDeleteState,
} from '../../model'
import {
  requirementMoveAvailable,
} from '../../model'
import { isPMMutationTicketCurrent } from '../../model/mutation-cache'
import { safeErrorMessage } from './contentWorkflowHelpers'
import type {
  useDeleteRequirementMutation,
  useDeleteTaskMutation,
  useMoveRequirementMutation,
} from '../useContentMutations'

const contentCopy = copy.projectManagement.content

interface SubmitMoveParams {
  move: RequirementMoveState
  operationLock: React.MutableRefObject<symbol | null>
  currentRequirement: (id: number) => ScrumProductRequirementNew | undefined
  targetBoardData: { version_tasks: Array<{ id: number; title: string; project_new_id: number }> } | undefined
  moveRequirement: ReturnType<typeof useMoveRequirementMutation>
  dispatchMove: (action: any) => void
  setFeedback: (fb: any) => void
  requirementRef: React.MutableRefObject<ScrumProductRequirementNew | null>
}

export async function submitMove(
  params: SubmitMoveParams,
  confirmSubjectSchedule = false,
) {
  const {
    move,
    operationLock,
    currentRequirement,
    targetBoardData,
    moveRequirement,
    dispatchMove,
    setFeedback,
    requirementRef,
  } = params

  if (move.status !== 'selecting' || operationLock.current || !move.draft.targetPlanUnitId) return
  const source = currentRequirement(move.draft.requirementId)
  const target = targetBoardData?.version_tasks.find((unit) => unit.id === move.draft.targetPlanUnitId)
  if (!source || !requirementMoveAvailable(source, target)) {
    dispatchMove({ type: 'submit' })
    dispatchMove({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
    setFeedback({ status: 'error', message: contentCopy.common.contextChanged })
    return
  }
  const operation = Symbol('requirement-move')
  operationLock.current = operation
  dispatchMove({ type: 'submit' })
  try {
    const result = await moveRequirement.execute({
      sourceProjectId: move.draft.sourceProjectId,
      targetProjectId: move.draft.targetProjectId,
      requirementId: move.draft.requirementId,
      targetPlanUnitId: move.draft.targetPlanUnitId,
      expectedVersion: move.draft.expectedVersion,
      ...(confirmSubjectSchedule ? { confirmSubjectSchedule: true as const } : {}),
    })
    if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
      dispatchMove({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
      return
    }
    requirementRef.current = result.data
    dispatchMove({ type: 'submit-succeeded' })
    setFeedback({ status: 'success', message: contentCopy.requirement.moved })
  } catch (error) {
    const conflict = isSafeApiError(error) && error.status === 409
    const stale = conflict && error.code === PM_API_ERROR_CODES.REQUIREMENT_STALE
    const scheduleConflict = conflict && (
      error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED ||
      error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT
    )
    dispatchMove({
      type: 'submit-failed',
      errorCode: scheduleConflict ? error.code : stale ? error.code : conflict ? 'REQUIREMENT_VERSION_CONFLICT' : 'REQUIREMENT_MOVE_FAILED',
      schedule: scheduleConflict ? error.schedule : undefined,
    })
    setFeedback({
      status: 'error',
      message: scheduleConflict
        ? copy.aiPM.workflowSubject.scheduleConflictDescription
        : stale || conflict
        ? contentCopy.requirement.moveConflict
        : safeErrorMessage(error, contentCopy.common.moveFailed),
    })
  } finally {
    if (operationLock.current === operation) operationLock.current = null
  }
}

interface ConfirmDeleteParams {
  deleting: ContentDeleteState
  operationLock: React.MutableRefObject<symbol | null>
  currentRequirement: (id: number) => ScrumProductRequirementNew | undefined
  currentTask: (id: number) => ProjectWorkTask | undefined
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  taskActions: (row?: ProjectWorkTask) => any
  deleteRequirement: ReturnType<typeof useDeleteRequirementMutation>
  deleteTask: ReturnType<typeof useDeleteTaskMutation>
  dispatchDelete: (action: any) => void
  setFeedback: (fb: any) => void
}

export async function confirmDelete(params: ConfirmDeleteParams) {
  const {
    deleting,
    operationLock,
    currentRequirement,
    currentTask,
    requirementActions,
    taskActions,
    deleteRequirement,
    deleteTask,
    dispatchDelete,
    setFeedback,
  } = params

  if (deleting.status !== 'confirming' || operationLock.current) return
  const { target } = deleting
  const allowed = target.kind === 'requirement'
    ? requirementActions(currentRequirement(target.requirementId)).delete
    : taskActions(currentTask(target.taskId)).delete
  if (!allowed) {
    dispatchDelete({ type: 'submit' })
    dispatchDelete({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
    setFeedback({ status: 'error', message: contentCopy.common.contextChanged })
    return
  }
  const operation = Symbol('content-delete')
  operationLock.current = operation
  dispatchDelete({ type: 'submit' })
  try {
    const result = target.kind === 'requirement'
      ? await deleteRequirement.execute(target.projectId, target.requirementId)
      : await deleteTask.execute(target.projectId, target.taskId)
    if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
      dispatchDelete({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
      return
    }
    dispatchDelete({ type: 'submit-succeeded' })
    setFeedback({
      status: 'success',
      message: target.kind === 'requirement'
        ? contentCopy.requirement.deleted
        : contentCopy.task.deleted,
    })
  } catch (error) {
    dispatchDelete({ type: 'submit-failed', errorCode: 'CONTENT_DELETE_FAILED' })
    setFeedback({ status: 'error', message: safeErrorMessage(error, contentCopy.common.deleteFailed) })
  } finally {
    if (operationLock.current === operation) operationLock.current = null
  }
}