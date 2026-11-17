import { useCallback, useMemo } from 'react'

import type { PlatformUser } from '../api'
import {
  resolveRequirementListState,
  resolveTaskListState,
  useRequirementListQuery,
  useTaskListQuery,
  type PlanUnitContentSearch,
  type RequirementListState,
  type TaskListState,
} from '../model'

function mergeUsers(primary: PlatformUser[], supplemental: PlatformUser[]): PlatformUser[] {
  const users = new Map(primary.map((user) => [user.id, user]))
  for (const user of supplemental) users.set(user.id, user)
  return [...users.values()]
}

export function useRequirementListModel(input: {
  projectId: number
  planUnitId: number
  planUnitTitle: string
  search: PlanUnitContentSearch['requirement']
  users: PlatformUser[]
  enabled: boolean
}): {
  state: RequirementListState
  rows: NonNullable<ReturnType<typeof useRequirementListQuery>['data']>['rows']
  users: PlatformUser[]
  reload: () => Promise<NonNullable<ReturnType<typeof useRequirementListQuery>['data']>['rows']>
} {
  const query = useRequirementListQuery({
    projectNewId: input.projectId,
    versionTaskNewId: input.planUnitId,
    q: input.search.q || undefined,
    status: input.search.status,
    priority: input.search.priority,
    limit: input.search.pageSize,
    offset: (input.search.page - 1) * input.search.pageSize,
  }, input.enabled)
  const retry = useCallback(() => { void query.refetch() }, [query.refetch])
  const reload = useCallback(async () => (
    await query.refetch({ throwOnError: true })
  ).data?.rows ?? [], [query.refetch])
  const rows = query.data?.rows ?? []
  const users = useMemo(
    () => mergeUsers(input.users, query.data?.users ?? []),
    [input.users, query.data?.users],
  )
  return {
    state: resolveRequirementListState({
      status: query.status,
      data: query.data,
      error: query.error,
      page: input.search.page,
      pageSize: input.search.pageSize,
      fetching: query.isFetching,
      filtered: Boolean(input.search.q || input.search.status || input.search.priority),
      users,
      planUnitTitle: input.planUnitTitle,
      retry,
    }),
    rows,
    users,
    reload,
  }
}

export function useTaskListModel(input: {
  projectId: number
  planUnitId: number
  search: PlanUnitContentSearch['task']
  users: PlatformUser[]
  enabled: boolean
}): {
  state: TaskListState
  rows: NonNullable<ReturnType<typeof useTaskListQuery>['data']>['rows']
} {
  const query = useTaskListQuery({
    projectNewId: input.projectId,
    versionTaskNewId: input.planUnitId,
    requirementNewId: input.search.requirementId,
    limit: input.search.pageSize,
    offset: (input.search.page - 1) * input.search.pageSize,
  }, input.enabled)
  const retry = useCallback(() => { void query.refetch() }, [query.refetch])
  const rows = query.data?.rows ?? []
  const users = useMemo(
    () => mergeUsers(input.users, query.data?.users ?? []),
    [input.users, query.data?.users],
  )
  return {
    state: resolveTaskListState({
      status: query.status,
      data: query.data,
      error: query.error,
      page: input.search.page,
      pageSize: input.search.pageSize,
      fetching: query.isFetching,
      users,
      retry,
    }),
    rows,
  }
}
