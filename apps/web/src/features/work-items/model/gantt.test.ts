import { describe, expect, it } from 'bun:test'

import type { WorkItem } from './types'
import { ganttDatePatch, ganttWindow, groupScheduledWorkItems, shiftGanttAnchor } from './gantt'

function item(overrides: Partial<WorkItem>): WorkItem {
  return {
    id: 1,
    rowId: 'task:1',
    title: 'Task 1',
    watched: false,
    capabilities: {
      canUpdate: false,
      canDelete: false,
      canComment: false,
      canAttach: false,
      canWatch: false,
      canClone: false,
      allowedTransitions: [],
    },
    ...overrides,
  }
}

describe('work-items Gantt model', () => {
  it('converts the scheduler exclusive end into an inclusive PM task due date', () => {
    expect(ganttDatePatch('2026-08-05', '2026-08-14')).toEqual({
      startsAt: '2026-08-05',
      dueAt: '2026-08-13',
    })
    expect(ganttDatePatch('2026-08-05', '2026-08-05')).toBeUndefined()
  })

  it('normalizes reversed dates and exposes an inclusive duration to the read-only scheduler', () => {
    const [group] = groupScheduledWorkItems([
      item({ startsAt: '2026-08-10T08:00:00Z', dueAt: '2026-08-02T18:00:00Z' }),
    ], 'none')

    expect(group.rows[0]).toMatchObject({
      start: '2026-08-02',
      endExclusive: '2026-08-11',
      durationDays: 9,
    })
  })

  it('uses the available date as a one-day plan and ignores tasks without plan dates', () => {
    const [group] = groupScheduledWorkItems([
      item({ id: 1, rowId: 'task:1', dueAt: '2026-08-03' }),
      item({ id: 2, rowId: 'task:2', startsAt: 'invalid', dueAt: undefined }),
    ], 'none')

    expect(group.rows).toHaveLength(1)
    expect(group.rows[0]).toMatchObject({
      start: '2026-08-03',
      endExclusive: '2026-08-04',
      durationDays: 1,
    })
  })

  it('groups by assignee without merging separate owner ids or losing unassigned rows', () => {
    const groups = groupScheduledWorkItems([
      item({ id: 1, rowId: 'task:1', assigneeId: 7, assigneeName: 'Teo', dueAt: '2026-08-01' }),
      item({ id: 2, rowId: 'task:2', assigneeId: 8, assigneeName: 'Teo', dueAt: '2026-08-02' }),
      item({ id: 3, rowId: 'task:3', dueAt: '2026-08-03' }),
    ], 'assignee_id', [{ id: 7, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' }])

    expect(groups.map(({ id, label, rows }) => [id, label, rows.length])).toEqual([
      ['assignee_id:7', 'Teo', 1],
      ['assignee_id:8', 'Teo', 1],
      ['assignee_id:none', '未分配', 1],
    ])
    expect(groups[0]).toMatchObject({ secondary: 'teo', avatarUrl: '/teo.png' })
  })

  it('uses the shared requirement and priority grouping semantics', () => {
    const rows = [
      item({ id: 1, rowId: 'task:1', requirementId: 9, requirementTitle: '需求 A', priority: 'high', dueAt: '2026-08-01' }),
      item({ id: 2, rowId: 'task:2', requirementId: 10, requirementTitle: '需求 B', priority: 'normal', dueAt: '2026-08-02' }),
      item({ id: 3, rowId: 'task:3', requirementId: 9, requirementTitle: '需求 A', priority: 'high', dueAt: '2026-08-03' }),
    ]

    expect(groupScheduledWorkItems(rows, 'none').map(({ id, label, rows: plans }) => (
      [id, label, plans.length]
    ))).toEqual([['all', '全部任务', 3]])
    expect(groupScheduledWorkItems(rows, 'requirement_id').map(({ id, label, rows: plans }) => (
      [id, label, plans.length]
    ))).toEqual([
      ['requirement_id:9', '需求 A', 2],
      ['requirement_id:10', '需求 B', 1],
    ])
    expect(groupScheduledWorkItems(rows, 'priority').map(({ id, label, rows: plans }) => (
      [id, label, plans.length]
    ))).toEqual([
      ['priority:high', '高', 2],
      ['priority:normal', '普通', 1],
    ])
  })

  it('builds explicit week, month, quarter and leap-year windows', () => {
    const anchor = new Date('2028-02-16T12:00:00Z')

    expect(ganttWindow(anchor, 'week', false)).toMatchObject({
      startDate: '2028-02-14', days: 7, cellWidth: 92, headers: ['Month', 'Day'],
    })
    expect(ganttWindow(anchor, 'month', true)).toMatchObject({
      startDate: '2028-02-01', days: 29, cellWidth: 48,
    })
    expect(ganttWindow(anchor, 'quarter', false)).toMatchObject({
      startDate: '2028-01-01', days: 91, cellWidth: 30,
    })
    expect(ganttWindow(anchor, 'year', false)).toMatchObject({
      startDate: '2028-01-01', days: 366, cellScale: 'Week', headers: ['Month', 'Week'],
    })
  })

  it('shifts each scale by its complete time window', () => {
    const anchor = new Date('2026-07-15T00:00:00Z')

    expect(shiftGanttAnchor(anchor, 'week', 1).toISOString().slice(0, 10)).toBe('2026-07-22')
    expect(shiftGanttAnchor(anchor, 'month', -1).toISOString().slice(0, 10)).toBe('2026-06-01')
    expect(shiftGanttAnchor(anchor, 'quarter', 1).toISOString().slice(0, 10)).toBe('2026-10-01')
    expect(shiftGanttAnchor(anchor, 'year', -1).toISOString().slice(0, 10)).toBe('2025-07-01')
  })
})
