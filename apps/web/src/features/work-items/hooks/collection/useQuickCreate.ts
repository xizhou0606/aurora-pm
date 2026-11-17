import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  QuickTaskChildDraft,
  QuickTaskChildField,
  QuickTaskCreateState,
  QuickTaskDraft,
  QuickTaskDraftErrors,
  QuickTaskField,
  ScopeOptionsState,
  UserOptionsState,
  WorkItemsRouteState,
} from '../../model/types'
import {
  applyQuickTaskFilledFields,
  createQuickTaskChildDraft,
  createQuickTaskDraft,
  quickTaskBatchPayload,
  quickTaskServerErrors,
  updateQuickTaskChild,
  updateQuickTaskParent,
  validateQuickTaskDrafts,
} from '../../model/quick-create'
import { useWorkItemBatchCreateMutation } from './useMutations'

function hasErrors(errors: QuickTaskDraftErrors) {
  return Object.keys(errors).length > 0
}

export function useQuickCreate(input: {
  environment: WorkItemsEnvironment
  route: WorkItemsRouteState
  scope: ScopeOptionsState
  users: UserOptionsState
}) {
  const { environment, route, scope, users } = input
  const mutation = useWorkItemBatchCreateMutation(environment)
  const [tasks, setTasks] = useState<QuickTaskDraft[] | null>(null)
  const [errors, setErrors] = useState<QuickTaskDraftErrors>({})
  const sequence = useRef(0)
  const ticket = `${environment.workspaceId}:${environment.sessionRevision}`
  const nextId = useCallback((kind: 'task' | 'child') => {
    sequence.current += 1
    return `quick-${kind}-${sequence.current}`
  }, [])
  const readyUsers = users.status === 'ready' ? users.data : []
  const currentUserId = useMemo(() => readyUsers.some(({ id }) => id === environment.userId)
    ? environment.userId
    : undefined, [environment.userId, readyUsers])
  const canOpen = environment.allowed && environment.createAllowed
    && environment.workspaceId > 0 && scope.status === 'ready'
    && users.status === 'ready' && Boolean(currentUserId)

  useEffect(() => {
    setTasks(null)
    setErrors({})
    mutation.reset()
  }, [ticket])

  const newTask = useCallback((selected = route.scope) => {
    if (scope.status !== 'ready') return undefined
    return createQuickTaskDraft({
      clientId: nextId('task'),
      selected,
      scope: scope.data,
      assigneeId: currentUserId,
    })
  }, [currentUserId, nextId, route.scope, scope])

  const open = useCallback(() => {
    if (!canOpen) return
    const task = newTask()
    if (!task) return
    mutation.reset()
    setErrors({})
    setTasks([task])
  }, [canOpen, mutation.reset, newTask])
  const close = useCallback(() => {
    if (mutation.isPending) return
    setTasks(null)
    setErrors({})
    mutation.reset()
  }, [mutation.isPending, mutation.reset])
  const addTask = useCallback(() => {
    const task = newTask()
    if (task) setTasks((current) => current ? [...current, task] : current)
  }, [newTask])
  const applyTaskToAll = useCallback((sourceClientId: string) => {
    if (scope.status !== 'ready') return
    setErrors({})
    setTasks((current) => current
      ? applyQuickTaskFilledFields(current, sourceClientId, scope.data)
      : current)
  }, [scope])
  const removeTask = useCallback((clientId: string) => {
    setErrors({})
    setTasks((current) => current && current.length > 1
      ? current.filter((task) => task.clientId !== clientId)
      : current)
  }, [])
  const updateTask = useCallback(<TKey extends QuickTaskField>(
    clientId: string,
    field: TKey,
    value: QuickTaskDraft[TKey],
  ) => {
    setErrors({})
    setTasks((current) => current?.map((task) => task.clientId === clientId
      ? updateQuickTaskParent(task, field, value)
      : task) ?? current)
  }, [])
  const addChild = useCallback((parentId: string) => {
    setTasks((current) => current?.map((task) => task.clientId === parentId
      ? { ...task, children: [...task.children, createQuickTaskChildDraft(nextId('child'), task)] }
      : task) ?? current)
  }, [nextId])
  const removeChild = useCallback((parentId: string, childId: string) => {
    setErrors({})
    setTasks((current) => current?.map((task) => task.clientId === parentId
      ? { ...task, children: task.children.filter((child) => child.clientId !== childId) }
      : task) ?? current)
  }, [])
  const updateChild = useCallback(<TKey extends QuickTaskChildField>(
    parentId: string,
    childId: string,
    field: TKey,
    value: QuickTaskChildDraft[TKey],
  ) => {
    setErrors({})
    setTasks((current) => current?.map((task) => task.clientId === parentId
      ? {
          ...task,
          children: task.children.map((child) => child.clientId === childId
            ? updateQuickTaskChild(child, field, value)
            : child),
        }
      : task) ?? current)
  }, [])
  const submit = useCallback(async () => {
    if (!tasks || scope.status !== 'ready' || users.status !== 'ready') return false
    const nextErrors = validateQuickTaskDrafts({ tasks, scope: scope.data, users: users.data })
    if (hasErrors(nextErrors)) {
      setErrors(nextErrors)
      return false
    }
    try {
      await mutation.mutateAsync(quickTaskBatchPayload(tasks))
      setTasks(null)
      setErrors({})
      return true
    } catch (error) {
      setErrors(quickTaskServerErrors(error))
      return false
    }
  }, [mutation.mutateAsync, scope, tasks, users])

  const state: QuickTaskCreateState = tasks
    ? {
        status: 'editing', tasks, errors,
        submit: mutation.isPending ? 'pending' : mutation.isError ? 'error' : 'idle',
      }
    : { status: 'closed' }
  return {
    state, canOpen, open, close, addTask, applyTaskToAll, removeTask, updateTask,
    addChild, removeChild, updateChild, submit,
  }
}
