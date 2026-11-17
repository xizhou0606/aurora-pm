import { isSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import type { MemberTask, PutMembersPayload, RemovalImpact } from '../api'

export type MemberRemovalPreview = {
  payload: PutMembersPayload
  impacts: ReadonlyMap<number, RemovalImpact>
  tasks: MemberTask[]
  blockedReason: string | null
}

export function memberSaveError(error: unknown, handover = false): string {
  const text = copy.projectManagement.members
  if (isSafeApiError(error)) {
    if (error.status === 403) return text.accessDenied
    if (error.code === 'PM_USER_REFERENCE_INACTIVE') return text.ownerReferenceInactive
    if (error.code === 'PROJECT_MEMBER_TASKS_CHANGED' || error.code === 'PROJECT_MEMBERS_CHANGED') return text.tasksChanged
    if (error.code === 'PROJECT_MEMBER_RESPONSIBILITIES') return text.removalBlocked
  }
  return handover ? text.handoverFailed : text.saveFailed
}

export function removalPreview(payload: PutMembersPayload, impacts: ReadonlyMap<number, RemovalImpact>): MemberRemovalPreview | null {
  const entries = [...impacts.values()]
  if (!entries.some((value) => value.open_task_count || value.open_bug_count || value.active_workflow_count || value.plan_unit_owner_count)) return null
  const text = copy.projectManagement.members
  const tasks = entries.flatMap((value) => value.tasks ?? [])
  const timestamps = new Set(entries.map((value) => value.members_updated_at))
  let blockedReason: string | null = null
  if (entries.some((value) => value.open_bug_count || value.active_workflow_count || value.plan_unit_owner_count)) blockedReason = text.otherResponsibilities
  else if (entries.some((value) => value.open_task_count !== value.tasks?.length) || timestamps.size !== 1 || !entries[0]?.members_updated_at) blockedReason = text.taskPreviewIncomplete
  else if (entries.some((value) => value.open_task_count > 0 && value.can_resolve_tasks !== true)) blockedReason = text.taskPermissionRequired
  return { payload: { ...payload, expected_updated_at: entries[0]?.members_updated_at }, impacts, tasks, blockedReason }
}
