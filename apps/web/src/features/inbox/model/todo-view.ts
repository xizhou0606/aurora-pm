import { workItemStatusTone } from '@/shared/model'

import { INBOX_COPY } from '@/shared/copy'
import {
  bugScope,
  bugStatusLabel,
  priorityLabel,
  severityLabel,
  taskStatusLabel,
  workflowActionLabel,
  workflowScope,
} from './presenters'
import type { InboxBug, InboxTask, InboxWorkflowTodo } from './types'

export type InboxTodoKind = 'workflow' | 'task' | 'bug'
export type InboxTodoTone = 'neutral' | 'info' | 'warning' | 'danger' | 'success' | 'accent'

type InboxTodoSource =
  | { kind: 'workflow'; row: InboxWorkflowTodo }
  | { kind: 'task'; row: InboxTask }
  | { kind: 'bug'; row: InboxBug }

export type InboxTodoItem = {
  id: string
  kind: InboxTodoKind
  itemKey: string
  title: string
  context: string
  status: string
  statusTone: InboxTodoTone
  detail?: string
  dueAt?: string
  createdAt?: string
  source: InboxTodoSource
}

export type InboxCalendarDay = {
  date: Date
  key: string
  day: number
  inMonth: boolean
  today: boolean
}

export function workflowTodoItem(row: InboxWorkflowTodo): InboxTodoItem {
  const action = workflowActionLabel(row.actionKind)
  return {
    id: row.todoId,
    kind: 'workflow',
    itemKey: row.templateName || INBOX_COPY.feed.workflowFallback(row.workflowInstanceId),
    title: row.title,
    context: workflowScope(row),
    status: action,
    statusTone: row.actionKind === 'ai_retry'
      ? 'danger'
      : row.actionKind === 'ai_confirm'
        ? 'accent'
        : row.actionKind === 'approval'
          ? 'warning'
          : 'info',
    detail: row.subjectTitle,
    dueAt: row.dueAt,
    createdAt: row.createdAt,
    source: { kind: 'workflow', row },
  }
}

export function taskTodoItem(row: InboxTask): InboxTodoItem {
  return {
    id: `task-${row.id}`,
    kind: 'task',
    itemKey: row.taskKey || INBOX_COPY.feed.fallbackId(row.id),
    title: row.title || INBOX_COPY.feed.unnamedTask,
    context: row.requirementTitle || INBOX_COPY.feed.noRequirement,
    status: taskStatusLabel(row.status),
    statusTone: workItemStatusTone(row.status),
    detail: priorityLabel(row.priority),
    dueAt: row.dueAt,
    source: { kind: 'task', row },
  }
}

export function bugTodoItem(row: InboxBug): InboxTodoItem {
  const status = bugStatusLabel(row.status)
  return {
    id: `bug-${row.id}`,
    kind: 'bug',
    itemKey: row.bugKey || INBOX_COPY.feed.fallbackId(row.id),
    title: row.title || INBOX_COPY.feed.unnamedBug,
    context: bugScope(row),
    status,
    statusTone: ['review', 'triage'].includes(String(row.status || '').toLowerCase())
      ? 'danger'
      : 'warning',
    detail: severityLabel(row.severity),
    dueAt: row.dueDate,
    source: { kind: 'bug', row },
  }
}

export function inboxDateKey(value: string | Date | undefined): string | undefined {
  if (!value) return undefined
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value)) return value
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return [
    String(date.getFullYear()).padStart(4, '0'),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
}

export function inboxCalendarDateKey(item: Pick<InboxTodoItem, 'dueAt' | 'createdAt'>) {
  return inboxDateKey(item.dueAt) || inboxDateKey(item.createdAt)
}

export function inboxCalendarMonth(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), 1)
}

export function moveInboxCalendarMonth(month: Date, delta: number): Date {
  return new Date(month.getFullYear(), month.getMonth() + delta, 1)
}

export function inboxCalendarDays(month: Date, today: Date): InboxCalendarDay[] {
  const normalizedMonth = inboxCalendarMonth(month)
  const mondayOffset = (normalizedMonth.getDay() + 6) % 7
  const start = new Date(
    normalizedMonth.getFullYear(),
    normalizedMonth.getMonth(),
    1 - mondayOffset,
  )
  const todayKey = inboxDateKey(today)
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start.getFullYear(), start.getMonth(), start.getDate() + index)
    const key = inboxDateKey(date) || ''
    return {
      date,
      key,
      day: date.getDate(),
      inMonth: date.getMonth() === normalizedMonth.getMonth(),
      today: key === todayKey,
    }
  })
}

export function inboxMonthLabel(month: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'long',
  }).format(month)
}

export function inboxCalendarDayLabel(day: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  }).format(day)
}

export function inboxBoardGroup(item: InboxTodoItem): 'confirm' | 'waiting' | 'active' {
  if (item.kind === 'task' && item.statusTone === 'info') return 'active'
  if (
    item.kind === 'bug'
    || (item.kind === 'workflow' && ['accent', 'danger', 'warning'].includes(item.statusTone))
  ) return 'confirm'
  return 'waiting'
}
