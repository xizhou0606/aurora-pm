import { useCallback, useEffect, useRef } from 'react'

import type { WorkItemsEnvironment } from '../../model/environment'
import type { DetailNavigation } from '../../model/types'
import {
  parseWorkItemsRoute,
  workItemRequirementPath,
  workItemsPath,
} from '../../model/utils'

export function useWorkItemDetailNavigation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  active: boolean,
): DetailNavigation {
  const taskStack = useRef<number[]>([])
  useEffect(() => {
    if (!active || !id) {
      taskStack.current = []
      return
    }
    const existing = taskStack.current.lastIndexOf(id)
    if (existing >= 0) {
      taskStack.current = taskStack.current.slice(0, existing + 1)
    } else if (taskStack.current.at(-1) !== id) {
      taskStack.current = [id]
    }
  }, [active, id])

  const openTask = useCallback((taskId: number, mode: 'view' | 'edit' = 'view') => {
    if (!Number.isInteger(taskId) || taskId <= 0 || taskId === id) return
    if (taskStack.current.at(-1) !== taskId) taskStack.current.push(taskId)
    const route = parseWorkItemsRoute(environment.pathname, environment.search)
    void environment.navigate(workItemsPath({ ...route, mode }, { detailId: taskId }))
  }, [environment, id])
  const openRequirement = useCallback<DetailNavigation['openRequirement']>((item) => {
    const path = workItemRequirementPath(item)
    if (path) void environment.navigate(path)
  }, [environment])
  const goBack = useCallback(() => {
    if (taskStack.current.length <= 1) return
    taskStack.current.pop()
    const previous = taskStack.current.at(-1)
    if (!previous) return
    const route = parseWorkItemsRoute(environment.pathname, environment.search)
    void environment.navigate(workItemsPath({ ...route, mode: 'view' }, { detailId: previous }))
  }, [environment])

  return {
    canGoBack: taskStack.current.length > 1,
    openTask,
    openRequirement,
    goBack,
  }
}
