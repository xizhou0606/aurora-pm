import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  DetailResourceRow,
  WorkItemActivity,
  WorkItemGitLink,
  WorkItemStatusTransition,
  WorkItemUserOption,
} from './types'
import { formatActivityDateTime, formatDateTime, statusLabel } from './presenters'

function activityCoversGit(activity: WorkItemActivity, sha: string): boolean {
  const normalized = sha.trim().toLowerCase()
  if (!normalized) return false
  const short = normalized.slice(0, 8)
  const haystack = `${activity.action || ''} ${activity.newValue || ''} ${activity.summary}`.toLowerCase()
  return haystack.includes(normalized) || (short.length >= 7 && haystack.includes(short))
}

function normalizedIdentity(value: string | undefined): string {
  return value?.trim().toLocaleLowerCase() || ''
}

/**
 * Task collaboration rows expose PM-internal actor ids while the shared user
 * directory can expose an external fact id. Prefer the server-provided name;
 * only unnamed legacy rows may fall back to their numeric id.
 */
export function workItemCollaborationUser(
  row: { userId?: number; userName?: string },
  users: readonly WorkItemUserOption[],
): WorkItemUserOption | undefined {
  const name = normalizedIdentity(row.userName)
  if (name) {
    return users.find((user) => (
      normalizedIdentity(user.label) === name
      || normalizedIdentity(user.username) === name
    ))
  }
  return row.userId ? users.find((user) => user.id === row.userId) : undefined
}

function activityUserLabel(
  raw: string | undefined,
  users: readonly WorkItemUserOption[],
): string {
  const value = String(raw || '').trim()
  if (!value || value === '0') return WORK_ITEMS_COPY.list.unassigned
  const userId = Number(value)
  return Number.isInteger(userId) && userId > 0
    ? users.find((user) => user.id === userId)?.label || value
    : value
}

export function presentActivitySummary(
  activity: WorkItemActivity,
  users: readonly WorkItemUserOption[] = [],
): string {
  if (activity.action === 'comment') {
    return activity.summary?.trim()
      ? `${WORK_ITEMS_COPY.detail.activityCommented}\n${activity.summary}`
      : WORK_ITEMS_COPY.detail.activityCommented
  }
  const change = (from: string, to: string) => (
    WORK_ITEMS_COPY.detail.transitionSummary(from, to)
  )
  if (activity.fieldKey === 'status' && (
    activity.oldValue !== undefined || activity.newValue !== undefined
  )) {
    return WORK_ITEMS_COPY.format.fieldLabel(
      WORK_ITEMS_COPY.detail.status,
      change(statusLabel(activity.oldValue), statusLabel(activity.newValue)),
    )
  }
  if (activity.fieldKey === 'assignee_id' && (
    activity.oldValue !== undefined || activity.newValue !== undefined
  )) {
    return WORK_ITEMS_COPY.format.fieldLabel(
      WORK_ITEMS_COPY.detail.assignee,
      change(
        activityUserLabel(activity.oldValue, users),
        activityUserLabel(activity.newValue, users),
      ),
    )
  }
  if (activity.fieldKey === 'reporter_user_id' && (
    activity.oldValue !== undefined || activity.newValue !== undefined
  )) {
    return WORK_ITEMS_COPY.format.fieldLabel(
      WORK_ITEMS_COPY.detail.reporter,
      change(
        activityUserLabel(activity.oldValue, users),
        activityUserLabel(activity.newValue, users),
      ),
    )
  }
  const dateFieldLabel = activity.fieldKey === 'starts_at'
    ? WORK_ITEMS_COPY.detail.startsAt
    : activity.fieldKey === 'due_at'
      ? WORK_ITEMS_COPY.detail.dueAt
      : undefined
  if (dateFieldLabel && (
    activity.oldValue !== undefined || activity.newValue !== undefined
  )) {
    return WORK_ITEMS_COPY.format.fieldLabel(
      dateFieldLabel,
      change(
        formatActivityDateTime(activity.oldValue),
        formatActivityDateTime(activity.newValue),
      ),
    )
  }
  return activity.summary || WORK_ITEMS_COPY.detail.activityFallback
}

export function presentStatusTransitions(
  activities: readonly WorkItemActivity[],
): WorkItemStatusTransition[] {
  return activities
    .filter((activity) => (
      activity.fieldKey === 'status'
      || activity.action === 'status_change'
      || activity.action === 'auto_complete'
    ))
    .flatMap((activity) => {
      const fromKey = String(activity.oldValue || '').trim()
      const toKey = String(activity.newValue || '').trim()
      return fromKey || toKey ? [{
        id: activity.id,
        fromKey,
        toKey,
        actorId: activity.actorId,
        actorName: activity.actorName,
        createdAt: activity.createdAt,
        automatic: activity.action === 'auto_complete',
      }] : []
    })
    .sort((left, right) => (
      (Date.parse(left.createdAt || '') || 0) - (Date.parse(right.createdAt || '') || 0)
    ))
}

export function mergeActivityTimeline(
  activities: readonly WorkItemActivity[],
  gitLinks: readonly WorkItemGitLink[],
  users: readonly WorkItemUserOption[] = [],
): DetailResourceRow[] {
  const rows: Array<DetailResourceRow & { createdAt?: string }> = activities.map((activity) => ({
    id: `activity:${activity.id}`,
    activity,
    primary: presentActivitySummary(activity, users),
    userId: activity.actorId,
    userName: activity.actorName,
    timestamp: formatDateTime(activity.createdAt),
    dateTime: activity.createdAt,
    createdAt: activity.createdAt,
  }))
  for (const git of gitLinks) {
    if (activities.some((activity) => activityCoversGit(activity, git.commitSHA))) continue
    const shortSHA = git.commitSHA.slice(0, 8)
    rows.push({
      id: `git:${git.id}`,
      primary: git.commitMessage || WORK_ITEMS_COPY.detail.gitCommit(shortSHA),
      secondary: [git.branch || git.ref, shortSHA, formatDateTime(git.createdAt)].filter(Boolean).join(' · ') || undefined,
      createdAt: git.createdAt,
    })
  }
  return rows
    .sort((left, right) => Date.parse(right.createdAt || '') - Date.parse(left.createdAt || ''))
    .map(({ createdAt: _createdAt, ...row }) => row)
}
