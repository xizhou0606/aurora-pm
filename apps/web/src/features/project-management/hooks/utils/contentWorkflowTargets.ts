import { useCallback, useMemo } from 'react'

import type {
  ContentTargetState,
  OwnerDirectoryState,
} from '../../model'
import { targetState } from './contentWorkflowHelpers'

export interface TargetProjectsInput {
  open: boolean
  viewProjectSets: boolean
  query: {
    status: string
    data: { rows: Array<{ id: number; name: string }> } | undefined
    refetch: () => void
  }
}

export function useTargetProjectsState(input: TargetProjectsInput): {
  projectsState: ContentTargetState
  retryProjects: () => void
} {
  const retryProjects = useCallback(() => { void input.query.refetch() }, [input.query.refetch])
  const projectsState = useMemo(() => targetState({
    status: input.query.status as any,
    items: (input.query.data?.rows ?? []).map((project) => ({ id: project.id, label: project.name })),
    retry: retryProjects,
  }), [retryProjects, input.query.data, input.query.status])

  return { projectsState, retryProjects }
}

export interface TargetPlanUnitsInput {
  open: boolean
  viewBoard: boolean
  move: { status: string; draft: { sourcePlanUnitId: number } }
  query: {
    status: string
    data: { version_tasks: Array<{ id: number; title: string }> } | undefined
    refetch: () => void
  }
}

export function useTargetPlanUnitsState(input: TargetPlanUnitsInput): {
  planUnitsState: ContentTargetState
  retryPlanUnits: () => void
} {
  const retryPlanUnits = useCallback(() => { void input.query.refetch() }, [input.query.refetch])
  const planUnitsState = useMemo(() => {
    const items = (input.query.data?.version_tasks ?? [])
      .filter((unit) => (
        !(input.move.status === 'selecting' && unit.id === input.move.draft.sourcePlanUnitId)
      ))
      .map((unit) => ({ id: unit.id, label: unit.title }))
    return targetState({ status: input.query.status as any, items, retry: retryPlanUnits })
  }, [input.move, retryPlanUnits, input.query.data, input.query.status])

  return { planUnitsState, retryPlanUnits }
}

export interface WorkflowTemplatesInput {
  open: boolean
  query: {
    status: string
    data: Array<{ id: number; name: string; published_version: number }> | undefined
    refetch: () => void
  }
}

export function useWorkflowTemplatesState(input: WorkflowTemplatesInput): ContentTargetState {
  return useMemo(() => targetState({
    status: input.query.status as any,
    items: (input.query.data ?? []).map((template) => ({
      id: template.id,
      label: `${template.name} · v${template.published_version}`,
    })),
    retry: () => { void input.query.refetch() },
  }), [input.query.data, input.query.refetch, input.query.status])
}

export interface UserDirectoryInput {
  users: Array<{ id: number; name: string }>
}

export function useUserDirectoryState(input: UserDirectoryInput, ownerDirectoryItems: (users: Array<{ id: number; name: string }>) => Array<{ id: number; label: string }>): OwnerDirectoryState {
  return useMemo<OwnerDirectoryState>(() => {
    const items = ownerDirectoryItems(input.users)
    return items.length > 0
      ? { status: 'ready', items }
      : { status: 'empty' }
  }, [input.users])
}
