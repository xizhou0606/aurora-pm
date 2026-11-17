import type { MutableRefObject } from 'react'

import type { ProjectWorkTask } from '../../api'
import type { TaskDeepLinkState } from '../../model'
import {
  useTaskDeepLinkEffect,
  computeTaskDeepLinkState,
  createCloseTaskEditor,
} from './contentWorkflowDeepLink'
import type { ProjectContentSearchPatch } from './contentWorkflowHelpers'

export interface UseTaskDeepLinkInput {
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

export function useTaskDeepLink(input: UseTaskDeepLinkInput): {
  taskDeepLink: TaskDeepLinkState
  closeTaskEditor: () => void
} {
  useTaskDeepLinkEffect(input)

  const taskDeepLink = computeTaskDeepLinkState({
    taskDeepLinkId: input.taskDeepLinkId,
    tasksEnabled: input.tasksEnabled,
    projectId: input.projectId,
    planUnitId: input.planUnitId,
    requirementId: input.requirementId,
    taskDeepLinkPageTarget: input.taskDeepLinkPageTarget,
    taskDeepLinkQuery: input.taskDeepLinkQuery,
  })

  const closeTaskEditor = createCloseTaskEditor({
    taskDeepLinkId: input.taskDeepLinkId,
    navigateSearch: input.navigateSearch,
    dispatchTaskEditor: input.dispatchTaskEditor,
    closingTaskDeepLinkIdRef: input.closingTaskDeepLinkIdRef,
    openedTaskDeepLinkIdRef: input.openedTaskDeepLinkIdRef,
    taskRef: input.taskRef,
  })

  return { taskDeepLink, closeTaskEditor }
}