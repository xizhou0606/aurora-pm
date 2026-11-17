import type { QueryClient, QueryKey } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { isSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'

export type PMMutationTicket = {
  workspaceId: number
  sessionRevision: number
}

export type PMMutationResult<T> =
  | { status: 'applied'; data: T; ticket: PMMutationTicket }
  | { status: 'stale'; data: T; ticket: PMMutationTicket }

export function capturePMMutationTicket(): PMMutationTicket {
  return {
    workspaceId: runtimeWorkspaceStore.getState().currentWorkspaceId,
    sessionRevision: runtimeSessionStore.getState().revision,
  }
}

export function isPMMutationTicketCurrent(ticket: PMMutationTicket): boolean {
  return (
    ticket.workspaceId > 0 &&
    runtimeWorkspaceStore.getState().currentWorkspaceId === ticket.workspaceId &&
    runtimeSessionStore.getState().revision === ticket.sessionRevision
  )
}

export function mutationResultFor<T>(
  ticket: PMMutationTicket,
  data: T,
): PMMutationResult<T> {
  return isPMMutationTicketCurrent(ticket)
    ? { status: 'applied', data, ticket }
    : { status: 'stale', data, ticket }
}

export function projectListsKey(ticket: PMMutationTicket): QueryKey {
  return pmKeys.resource(ticket.workspaceId, 'projectSets', 'list')
}

export function projectBoardKey(
  ticket: PMMutationTicket,
  projectId: number,
): QueryKey {
  return pmKeys.resource(
    ticket.workspaceId,
    'planUnits',
    'board',
    projectId,
    'summary-version-tasks',
  )
}

export function projectAuditKey(
  ticket: PMMutationTicket,
  projectId: number,
): QueryKey {
  return pmKeys.resource(ticket.workspaceId, 'audit', 'project', projectId)
}

export function requirementRootKey(
  ticket: PMMutationTicket,
  projectId?: number,
): QueryKey {
  return projectId
    ? pmKeys.resource(ticket.workspaceId, 'requirements', 'list', 'project', projectId)
    : pmKeys.resource(ticket.workspaceId, 'requirements', 'list')
}

export function taskRootKey(
  ticket: PMMutationTicket,
  projectId?: number,
): QueryKey {
  return projectId
    ? pmKeys.resource(ticket.workspaceId, 'tasks', 'list', 'project', projectId)
    : pmKeys.resource(ticket.workspaceId, 'tasks', 'list')
}

export function deliveryLabelsKey(
  ticket: PMMutationTicket,
  planUnitId: number,
): QueryKey {
  return pmKeys.resource(ticket.workspaceId, 'delivery', 'labels', 'plan-unit', planUnitId)
}

export async function cancelPMReads(
  client: QueryClient,
  ticket: PMMutationTicket,
  input: {
    projectIds?: number[]
    requirements?: boolean
    tasks?: boolean
    deliveryPlanUnitIds?: number[]
  } = {},
): Promise<void> {
  const projectIds = [...new Set(input.projectIds?.filter((id) => id > 0) ?? [])]
  const deliveryPlanUnitIds = [...new Set(
    input.deliveryPlanUnitIds?.filter((id) => id > 0) ?? [],
  )]
  const requests: Promise<unknown>[] = [
    client.cancelQueries({ queryKey: projectListsKey(ticket) }),
  ]
  for (const projectId of projectIds) {
    requests.push(
      client.cancelQueries({ queryKey: projectBoardKey(ticket, projectId) }),
      client.cancelQueries({ queryKey: projectAuditKey(ticket, projectId) }),
    )
    if (input.requirements) {
      requests.push(client.cancelQueries({ queryKey: requirementRootKey(ticket, projectId) }))
    }
    if (input.tasks) {
      requests.push(client.cancelQueries({ queryKey: taskRootKey(ticket, projectId) }))
    }
  }
  for (const planUnitId of deliveryPlanUnitIds) {
    requests.push(client.cancelQueries({ queryKey: deliveryLabelsKey(ticket, planUnitId) }))
  }
  await Promise.all(requests)
}

export async function invalidatePMReads(
  client: QueryClient,
  ticket: PMMutationTicket,
  input: {
    projectIds?: number[]
    lists?: boolean
    requirements?: boolean
    tasks?: boolean
    deliveryPlanUnitIds?: number[]
    refetchType?: 'active' | 'none'
  } = {},
): Promise<void> {
  if (!isPMMutationTicketCurrent(ticket)) return
  const projectIds = [...new Set(input.projectIds?.filter((id) => id > 0) ?? [])]
  const deliveryPlanUnitIds = [...new Set(
    input.deliveryPlanUnitIds?.filter((id) => id > 0) ?? [],
  )]
  const requests: Promise<unknown>[] = []
  if (input.lists !== false) {
    requests.push(client.invalidateQueries({
      queryKey: projectListsKey(ticket),
      refetchType: input.refetchType,
    }))
  }
  for (const projectId of projectIds) {
    requests.push(
      client.invalidateQueries({
        queryKey: projectBoardKey(ticket, projectId),
        refetchType: input.refetchType,
      }),
      client.invalidateQueries({
        queryKey: projectAuditKey(ticket, projectId),
        refetchType: input.refetchType,
      }),
    )
    if (input.requirements) {
      requests.push(client.invalidateQueries({
        queryKey: requirementRootKey(ticket, projectId),
        refetchType: input.refetchType,
      }))
    }
    if (input.tasks) {
      requests.push(client.invalidateQueries({
        queryKey: taskRootKey(ticket, projectId),
        refetchType: input.refetchType,
      }))
    }
  }
  for (const planUnitId of deliveryPlanUnitIds) {
    requests.push(client.invalidateQueries({
      queryKey: deliveryLabelsKey(ticket, planUnitId),
      refetchType: input.refetchType,
    }))
  }
  await Promise.all(requests)
}

export async function refreshPMAfterDenied(
  client: QueryClient,
  ticket: PMMutationTicket,
  error: unknown,
  input: {
    projectIds?: number[]
    requirements?: boolean
    tasks?: boolean
    deliveryPlanUnitIds?: number[]
  } = {},
): Promise<void> {
  if (
    !isPMMutationTicketCurrent(ticket) ||
    !isSafeApiError(error) ||
    (error.status !== 403 && error.status !== 404)
  ) return
  await invalidatePMReads(client, ticket, input)
}
