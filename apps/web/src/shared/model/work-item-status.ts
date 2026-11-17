import { WORK_ITEM_ENUM_COPY } from '../copy/work-item-enums'

export type CanonicalWorkItemStatus = keyof typeof WORK_ITEM_ENUM_COPY.statuses

export type WorkItemStatusTone = 'neutral' | 'info' | 'warning' | 'success'

const LEGACY_STATUS_ALIASES: Readonly<Record<string, CanonicalWorkItemStatus>> = Object.freeze({
  todo: 'draft',
  open: 'draft',
  backlog: 'draft',
  new: 'draft',
  pending: 'draft',
  pending_review: 'draft',
  pending_schedule: 'draft',
  pending_dev: 'draft',
  blocked: 'draft',
  block: 'draft',
  hold: 'draft',
  doing: 'in_dev',
  in_progress: 'in_dev',
  processing: 'in_dev',
  development: 'in_dev',
  dev_done: 'pending_test',
  in_test: 'pending_test',
  test_passed: 'pending_test',
  pending_release: 'pending_test',
  releasing: 'pending_test',
  review: 'pending_test',
  acceptance: 'pending_test',
  released: 'completed',
  archived: 'completed',
  closed: 'completed',
  done: 'completed',
  resolved: 'completed',
  fixed: 'completed',
  canceled: 'cancelled',
  aborted: 'cancelled',
})

export function normalizeWorkItemStatus(value: unknown): string {
  const key = String(value || '').trim().toLowerCase()
  return LEGACY_STATUS_ALIASES[key] || key
}

export function isCanonicalWorkItemStatus(value: unknown): value is CanonicalWorkItemStatus {
  return Object.hasOwn(WORK_ITEM_ENUM_COPY.statuses, String(value || ''))
}

export function workItemStatusLabel(value: unknown, fallback: string): string {
  const normalized = normalizeWorkItemStatus(value)
  return isCanonicalWorkItemStatus(normalized)
    ? WORK_ITEM_ENUM_COPY.statuses[normalized]
    : normalized || fallback
}

export function workItemStatusTone(value: unknown): WorkItemStatusTone {
  switch (normalizeWorkItemStatus(value)) {
    case 'in_dev': return 'info'
    case 'pending_test': return 'warning'
    case 'completed': return 'success'
    case 'draft': return 'warning'
    case 'cancelled':
    default: return 'neutral'
  }
}
