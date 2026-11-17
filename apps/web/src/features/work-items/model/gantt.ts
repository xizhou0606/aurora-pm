import { WORK_ITEMS_COPY } from '@/shared/copy'
import { groupWorkItems } from './presenters'
import type { WorkItem, WorkItemUserOption, WorkItemsGroupBy } from './types'

export type GanttScale = 'week' | 'month' | 'quarter' | 'year'

export type ScheduledWorkItem = {
  item: WorkItem
  start: string
  endExclusive: string
  durationDays: number
}

export type GanttOwnerGroup = {
  id: string
  label: string
  secondary?: string
  avatarUrl?: string
  rows: readonly ScheduledWorkItem[]
}

export type GanttWindow = {
  startDate: string
  days: number
  cellWidth: number
  cellScale: 'Day' | 'Week'
  headers: readonly ('Month' | 'Day' | 'Week')[]
}

export type GanttDatePatch = {
  startsAt: string
  dueAt: string
}

const DAY = 86_400_000

function dateOnly(value: string | undefined): Date | undefined {
  if (!value) return undefined
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  if (!match) return undefined
  const parsed = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
  return Number.isFinite(parsed.getTime()) ? parsed : undefined
}

function isoDate(value: Date): string {
  return value.toISOString().slice(0, 10)
}

function addDays(value: Date, amount: number): Date {
  return new Date(value.getTime() + amount * DAY)
}

export function ganttDatePatch(
  startValue: string,
  endExclusiveValue: string,
): GanttDatePatch | undefined {
  const start = dateOnly(startValue)
  const endExclusive = dateOnly(endExclusiveValue)
  if (!start || !endExclusive || endExclusive.getTime() <= start.getTime()) return undefined
  return {
    startsAt: isoDate(start),
    dueAt: isoDate(addDays(endExclusive, -1)),
  }
}

function scheduled(item: WorkItem): ScheduledWorkItem | undefined {
  const rawStart = dateOnly(item.startsAt) ?? dateOnly(item.dueAt)
  const rawEnd = dateOnly(item.dueAt) ?? dateOnly(item.startsAt)
  if (!rawStart || !rawEnd) return undefined
  const start = rawStart.getTime() <= rawEnd.getTime() ? rawStart : rawEnd
  const end = rawStart.getTime() <= rawEnd.getTime() ? rawEnd : rawStart
  return {
    item,
    start: isoDate(start),
    endExclusive: isoDate(addDays(end, 1)),
    durationDays: Math.max(1, Math.round((end.getTime() - start.getTime()) / DAY) + 1),
  }
}

export function groupScheduledWorkItems(
  rows: readonly WorkItem[],
  groupBy: WorkItemsGroupBy,
  users: readonly WorkItemUserOption[] = [],
): readonly GanttOwnerGroup[] {
  return groupWorkItems(rows, groupBy, users).flatMap((group) => {
    const plans = group.rows.flatMap((item) => {
      const plan = scheduled(item)
      return plan ? [plan] : []
    })
    if (!plans.length) return []
    const user = groupBy === 'assignee_id'
      ? users.find((candidate) => candidate.id === group.rows[0]?.assigneeId)
      : undefined
    return [{
      id: group.id,
      label: group.label || WORK_ITEMS_COPY.gantt.allTasks,
      ...(user?.username ? { secondary: user.username } : {}),
      ...(user?.avatarUrl ? { avatarUrl: user.avatarUrl } : {}),
      rows: plans,
    }]
  })
}

function monthStart(anchor: Date, monthOffset = 0): Date {
  return new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth() + monthOffset, 1))
}

function weekStart(anchor: Date): Date {
  const current = new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth(), anchor.getUTCDate()))
  const offset = (current.getUTCDay() + 6) % 7
  return addDays(current, -offset)
}

export function ganttWindow(anchor: Date, scale: GanttScale, narrow: boolean): GanttWindow {
  if (scale === 'week') {
    return { startDate: isoDate(weekStart(anchor)), days: 7, cellWidth: narrow ? 56 : 92, cellScale: 'Day', headers: ['Month', 'Day'] }
  }
  if (scale === 'quarter') {
    const quarterStart = monthStart(anchor, -(anchor.getUTCMonth() % 3))
    const end = monthStart(quarterStart, 3)
    return {
      startDate: isoDate(quarterStart),
      days: Math.round((end.getTime() - quarterStart.getTime()) / DAY),
      cellWidth: narrow ? 24 : 30,
      cellScale: 'Day',
      headers: ['Month', 'Day'],
    }
  }
  if (scale === 'year') {
    const start = new Date(Date.UTC(anchor.getUTCFullYear(), 0, 1))
    const end = new Date(Date.UTC(anchor.getUTCFullYear() + 1, 0, 1))
    return {
      startDate: isoDate(start),
      days: Math.round((end.getTime() - start.getTime()) / DAY),
      cellWidth: narrow ? 24 : 30,
      cellScale: 'Week',
      headers: ['Month', 'Week'],
    }
  }
  const start = monthStart(anchor)
  const end = monthStart(anchor, 1)
  return {
    startDate: isoDate(start),
    days: Math.round((end.getTime() - start.getTime()) / DAY),
    cellWidth: narrow ? 48 : 64,
    cellScale: 'Day',
    headers: ['Month', 'Day'],
  }
}

export function shiftGanttAnchor(anchor: Date, scale: GanttScale, direction: -1 | 1): Date {
  if (scale === 'week') return addDays(anchor, direction * 7)
  const months = scale === 'month' ? direction : scale === 'quarter' ? direction * 3 : direction * 12
  return new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth() + months, 1))
}
