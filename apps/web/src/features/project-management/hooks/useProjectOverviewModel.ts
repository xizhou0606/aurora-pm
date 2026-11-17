import { useCallback, useMemo } from 'react'

import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  projectOverviewAggregateModel,
  resolveAssignmentSummary,
  resolveProjectOverviewFactsState,
  useProjectOverviewQuery,
  type AssignmentSummaryState,
  type ProjectOverviewAggregateModel,
  type ProjectOverviewFactsState,
} from '../model'

type SettledState = Extract<ProjectOverviewFactsState, { status: 'empty' | 'ready' }>

export type ProjectOverviewModelState =
  | Exclude<ProjectOverviewFactsState, { status: 'empty' | 'ready' }>
  | (Omit<SettledState, 'facts'> & { model: ProjectOverviewAggregateModel })

export type ProjectAssignmentModelState =
  | Exclude<ProjectOverviewFactsState, { status: 'empty' | 'ready' }>
  | (Omit<SettledState, 'facts' | 'status'> & {
      status: AssignmentSummaryState['status']
      summary: AssignmentSummaryState
    })

function useFacts(
  projectId: number,
  teamId: number,
  projectWorkspaceId?: number,
  itemId?: number,
): ProjectOverviewFactsState {
  const query = useProjectOverviewQuery({
    projectId,
    teamId,
    projectWorkspaceId,
    itemId,
  })
  const retry = useCallback(() => {
    void query.refetch()
  }, [query.refetch])
  return useMemo(() => resolveProjectOverviewFactsState({
    status: query.status,
    facts: query.data,
    error: query.error,
    fetching: query.isFetching && !query.isPending,
    retry,
  }), [query.data, query.error, query.isFetching, query.isPending, query.status, retry])
}

export function useProjectOverviewModel(input: {
  project: ScrumProjectNew
  unit?: ScrumVersionTaskNew
  units: readonly ScrumVersionTaskNew[]
  now?: Date
}): ProjectOverviewModelState {
  const factsState = useFacts(
    input.project.id,
    input.project.team_id,
    input.project.workspace_id,
    input.unit?.id,
  )
  return useMemo(() => {
    if (factsState.status === 'loading' || factsState.status === 'error') return factsState
    return {
      status: factsState.status,
      model: projectOverviewAggregateModel({
        project: input.project,
        unit: input.unit,
        units: input.units,
        facts: factsState.facts,
        now: input.now,
      }),
      refreshing: factsState.refreshing,
      ...(factsState.staleError ? { staleError: factsState.staleError } : {}),
    }
  }, [factsState, input.now, input.project, input.unit, input.units])
}

export function useProjectAssignmentModel(input: {
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  users: readonly PlatformUser[]
}): ProjectAssignmentModelState {
  const factsState = useFacts(
    input.project.id,
    input.project.team_id,
    input.project.workspace_id,
    input.unit.id,
  )
  return useMemo(() => {
    if (factsState.status === 'loading' || factsState.status === 'error') return factsState
    const summary = resolveAssignmentSummary({
      project: input.project,
      unit: input.unit,
      requirements: factsState.facts.requirements,
      tasks: factsState.facts.tasks,
      users: input.users,
    })
    return {
      status: summary.status,
      summary,
      refreshing: factsState.refreshing,
      ...(factsState.staleError ? { staleError: factsState.staleError } : {}),
    }
  }, [factsState, input.project, input.unit, input.users])
}
