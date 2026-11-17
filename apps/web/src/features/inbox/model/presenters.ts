import { INBOX_COPY } from '@/shared/copy'
import { workItemStatusLabel } from '@/shared/model'
import type { InboxBug, InboxScope, InboxWorkflowTodo } from './types'

export function scopeSummary(scope: InboxScope): string {
  const count = scope.requirementIds.length
    || scope.projectIds.length
    || scope.projectSetIds.length
  return count
    ? INBOX_COPY.scope.summarySelected(count)
    : INBOX_COPY.scope.summaryAll
}

export function formatDate(
  value: string | undefined,
  fallback = INBOX_COPY.feed.noDueDate,
): string {
  if (!value) return fallback
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return fallback
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

export function formatDateTime(
  value: string | undefined,
  fallback = INBOX_COPY.feed.justNow,
): string {
  if (!value) return fallback
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return fallback
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function enumLabel(
  value: string | undefined,
  labels: Readonly<Record<string, string>>,
): string {
  const normalized = String(value || '').trim().toLowerCase()
  return labels[normalized] || value || INBOX_COPY.feed.enumFallback
}

export const taskStatusLabel = (value: string | undefined) => (
  workItemStatusLabel(value, INBOX_COPY.feed.enumFallback)
)

export const bugStatusLabel = (value: string | undefined) => (
  enumLabel(value, INBOX_COPY.feed.bugStatuses)
)

export const priorityLabel = (value: string | undefined) => (
  enumLabel(value, INBOX_COPY.feed.priorities)
)

export const severityLabel = (value: string | undefined) => (
  enumLabel(value, INBOX_COPY.feed.severities)
)

export function bugScope(row: InboxBug): string {
  return [row.projectSetName, row.projectName, row.requirementTitle]
    .filter(Boolean)
    .join(' / ') || INBOX_COPY.feed.noRequirement
}

export function workflowScope(row: InboxWorkflowTodo): string {
  return [row.projectSetName, row.projectName, row.requirementTitle]
    .filter(Boolean)
    .join(' / ') || row.subjectTitle
}

export function workflowActionLabel(value: string): string {
  return INBOX_COPY.feed.workflowActions[value]
    || value
    || INBOX_COPY.feed.enumFallback
}
