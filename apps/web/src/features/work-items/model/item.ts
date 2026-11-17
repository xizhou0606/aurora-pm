import type { WorkItemDetailPatch } from './mutation'

export type WorkItemCapabilities = {
  canUpdate: boolean
  canDelete: boolean
  canComment: boolean
  canAttach: boolean
  canWatch: boolean
  canClone: boolean
  /** Time logging is a separate owner capability; missing must remain false. */
  canLogTime?: boolean
  allowedTransitions: readonly string[]
}

export type WorkItem = {
  id: number
  rowId: string
  taskKey?: string
  title: string
  description?: string
  kind?: string
  status?: string
  priority?: string
  assigneeId?: number
  assigneeName?: string
  reporterId?: number
  reporterName?: string
  createdByUserId?: number
  teamId?: number
  projectSetId?: number
  projectId?: number
  requirementId?: number
  taskClientId?: number
  requirementTitle?: string
  parentId?: number
  startsAt?: string
  dueAt?: string
  estimatedMinutes?: number
  createdAt?: string
  updatedAt?: string
  externalSource?: string
  contextOnly?: boolean
  watched: boolean
  capabilities: WorkItemCapabilities
}

export function applyWorkItemDetailPatch(
  row: WorkItem,
  patch: WorkItemDetailPatch,
): WorkItem {
  return {
    ...row,
    ...(patch.title !== undefined ? { title: patch.title } : {}),
    ...(patch.description !== undefined ? { description: patch.description } : {}),
    ...(patch.status !== undefined ? { status: patch.status } : {}),
    ...(patch.priority !== undefined ? { priority: patch.priority } : {}),
    ...(patch.taskClientId !== undefined ? { taskClientId: patch.taskClientId ?? undefined } : {}),
    ...(patch.assigneeId !== undefined ? { assigneeId: patch.assigneeId ?? undefined } : {}),
    ...(patch.startsAt !== undefined ? { startsAt: patch.startsAt ?? undefined } : {}),
    ...(patch.dueAt !== undefined ? { dueAt: patch.dueAt ?? undefined } : {}),
    ...(patch.estimatedMinutes !== undefined
      ? { estimatedMinutes: patch.estimatedMinutes ?? undefined }
      : {}),
  }
}
