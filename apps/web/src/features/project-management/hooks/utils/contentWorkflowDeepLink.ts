import { useEffect } from 'react'
import type { MutableRefObject } from 'react'

import { isSafeApiError } from '@/shared/api'
import type { ProjectWorkTask, ScrumProductRequirementNew } from '../../api'
import type {
  RequirementActions,
  TaskDeepLinkState,
  TaskEditorState,
} from '../../model'
import { taskDraftFromTask } from '../../model'
import type { ProjectContentSearchPatch } from './contentWorkflowHelpers'

export interface DeepLinkInput {
  taskDeepLinkId: number | undefined
  tasksEnabled: boolean
  projectId: number
  planUnitId: number
  requirementId: number | undefined
  taskDeepLinkPageTarget: ProjectWorkTask | undefined
  taskDeepLinkQuery: {
    data: ProjectWorkTask | undefined
    status: string
    error: unknown
    refetch: () => void
  }
  navigateSearch: (patch: ProjectContentSearchPatch, replace?: boolean) => void
  closeCompeting: () => void
  setFeedback: (fb: { status: 'idle' | 'success' | 'error'; message?: string }) => void
  dispatchTaskEditor: (action: { type: 'open'; draft: any } | { type: 'close' }) => void
  taskRef: MutableRefObject<ProjectWorkTask | null>
  openedTaskDeepLinkIdRef: MutableRefObject<number | null>
  closingTaskDeepLinkIdRef: MutableRefObject<number | null>
}

export function useTaskDeepLinkEffect(input: DeepLinkInput): void {
  const {
    taskDeepLinkId,
    tasksEnabled,
    projectId,
    planUnitId,
    requirementId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery,
    navigateSearch,
    closeCompeting,
    setFeedback,
    dispatchTaskEditor,
    taskRef,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  } = input

  useEffect(() => {
    if (!taskDeepLinkId) {
      closingTaskDeepLinkIdRef.current = null
      if (openedTaskDeepLinkIdRef.current !== null) {
        openedTaskDeepLinkIdRef.current = null
        taskRef.current = null
        dispatchTaskEditor({ type: 'close' })
      }
      return
    }
    if (closingTaskDeepLinkIdRef.current === taskDeepLinkId) return
    if (
      openedTaskDeepLinkIdRef.current !== null &&
      openedTaskDeepLinkIdRef.current !== taskDeepLinkId
    ) {
      openedTaskDeepLinkIdRef.current = null
      taskRef.current = null
      dispatchTaskEditor({ type: 'close' })
    }
    if (!tasksEnabled) {
      void navigateSearch({ task_id: undefined })
      return
    }

    const target = taskDeepLinkPageTarget ?? taskDeepLinkQuery.data ?? undefined
    const exactTarget = target &&
      target.id === taskDeepLinkId &&
      target.project_set_id === projectId &&
      target.project_id === planUnitId &&
      target.requirement_id === requirementId
        ? target
        : undefined
    if (!exactTarget) {
      if (taskDeepLinkQuery.status !== 'success') return
      openedTaskDeepLinkIdRef.current = null
      taskRef.current = null
      dispatchTaskEditor({ type: 'close' })
      return
    }
    if (openedTaskDeepLinkIdRef.current === taskDeepLinkId) return

    closeCompeting()
    taskRef.current = exactTarget
    openedTaskDeepLinkIdRef.current = taskDeepLinkId
    setFeedback({ status: 'idle' })
    dispatchTaskEditor({
      type: 'open',
      draft: taskDraftFromTask(exactTarget),
    })
  }, [
    closeCompeting,
    planUnitId,
    projectId,
    navigateSearch,
    requirementId,
    taskDeepLinkId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery.data,
    taskDeepLinkQuery.status,
    tasksEnabled,
  ])
}

export interface DeepLinkStateInput {
  taskDeepLinkId: number | undefined
  tasksEnabled: boolean
  projectId: number
  planUnitId: number
  requirementId: number | undefined
  taskDeepLinkPageTarget: ProjectWorkTask | undefined
  taskDeepLinkQuery: {
    data: ProjectWorkTask | undefined
    status: string
    error: unknown
    refetch: () => void
  }
}

export function computeTaskDeepLinkState(input: DeepLinkStateInput): TaskDeepLinkState {
  const {
    taskDeepLinkId,
    tasksEnabled,
    projectId,
    planUnitId,
    requirementId,
    taskDeepLinkPageTarget,
    taskDeepLinkQuery,
  } = input

  const deepLinkTarget = taskDeepLinkPageTarget ?? taskDeepLinkQuery.data ?? undefined
  const exactDeepLinkTarget = deepLinkTarget && taskDeepLinkId &&
    deepLinkTarget.id === taskDeepLinkId &&
    deepLinkTarget.project_set_id === projectId &&
    deepLinkTarget.project_id === planUnitId &&
    deepLinkTarget.requirement_id === requirementId
      ? deepLinkTarget
      : undefined
  const deepLinkNotFound = taskDeepLinkQuery.status === 'error' &&
    isSafeApiError(taskDeepLinkQuery.error) &&
    taskDeepLinkQuery.error.status === 404

  return !taskDeepLinkId || !tasksEnabled
    ? { status: 'idle' }
    : exactDeepLinkTarget
      ? { status: 'ready' }
      : deepLinkNotFound
        ? { status: 'not_found' }
        : taskDeepLinkQuery.status === 'error'
        ? { status: 'error', retry: () => void taskDeepLinkQuery.refetch() }
        : taskDeepLinkQuery.status === 'success'
          ? { status: 'not_found' }
          : { status: 'loading' }
}

export interface CloseTaskEditorInput {
  taskDeepLinkId: number | undefined
  navigateSearch: (patch: ProjectContentSearchPatch, replace?: boolean) => void
  dispatchTaskEditor: (action: { type: 'close' }) => void
  closingTaskDeepLinkIdRef: MutableRefObject<number | null>
  openedTaskDeepLinkIdRef: MutableRefObject<number | null>
  taskRef: MutableRefObject<ProjectWorkTask | null>
}

export function createCloseTaskEditor(input: CloseTaskEditorInput): () => void {
  return () => {
    if (input.taskDeepLinkId) {
      input.closingTaskDeepLinkIdRef.current = input.taskDeepLinkId
      input.openedTaskDeepLinkIdRef.current = null
      input.taskRef.current = null
      input.navigateSearch({ task_id: undefined })
    }
    input.dispatchTaskEditor({ type: 'close' })
  }
}