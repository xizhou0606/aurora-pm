import { describe, expect, it, vi } from 'bun:test'
import { WORK_ITEMS_COPY } from '@/shared/copy'

import type { WorkItem } from './types'
import {
  buildBoardLanes,
  buildBoardGroups,
  buildListGroups,
  hasWorkItemsBoardScope,
  isWorkItemOverdue,
  isValidEstimatedMinutes,
  isValidWorkItemsBoardScope,
  isValidWorkItemsListScope,
  normalizeWorkItemsBoardScope,
  parseWorkItemsRoute,
  readyRefresh,
  requestScope,
  resolveCollectionState,
  workItemAssigneeDirectoryIds,
  workItemsPath,
} from './utils'
import { visibleTreeRows } from './presenters'

function item(id: number, parentId?: number): WorkItem {
  return {
    id,
    rowId: `task:${id}`,
    title: `Task ${id}`,
    kind: parentId ? 'subtask' : 'task',
    parentId,
    status: 'draft',
    watched: false,
    capabilities: {
      canUpdate: true,
      canDelete: true,
      canComment: true,
      canAttach: true,
      canWatch: true,
      canClone: false,
      canLogTime: false,
      allowedTransitions: ['draft', 'in_dev'],
    },
  }
}

describe('work-items route and state model', () => {
  it('parses the canonical PM route and emits complete server filters', () => {
    const route = parseWorkItemsRoute('/workspace/work-items/31', {
      project_set_ids: '8', project_ids: '13', requirement_ids: '21',
      assignee_ids: '7,8', statuses: 'draft,in_dev', priorities: 'normal,high',
      q: 'needle', overdue: '1', group_by: 'requirement_id',
      sort: 'updated_at_desc', page: '2', page_size: '50',
    })

    expect(route).toMatchObject({
      assigneeIds: [7, 8], statuses: ['draft', 'in_dev'], priorities: ['normal', 'high'],
      detailId: 31, page: 2, pageSize: 50, sort: 'updated_at_desc',
      invalidLegacyScope: false, needsRouteNormalization: true,
    })
    expect(workItemsPath(route)).toContain('task_id=31')
    expect(workItemsPath(route)).toMatch(/^\/workspace\/work-items\?/)
    expect(requestScope(route)).toEqual({
      project_set_ids: '8', project_ids: '13', requirement_ids: '21',
      q: 'needle', overdue: 1, assignee_ids: '7,8', statuses: 'draft,in_dev',
      priorities: 'normal,high', group_by: 'requirement_id', sort: 'updated_at_desc',
    })
  })

  it.each([
    [{ source: 'one2all', one2all_assignee_ids: '7,8' }],
    [{ source: 'pm', pm_assignee_ids: '7,8' }],
  ])('normalizes legacy source and assignee parameters into assignee_ids', (search) => {
    const route = parseWorkItemsRoute('/workspace/work-items', search)
    expect(route.assigneeIds).toEqual([7, 8])
    expect(route.needsRouteNormalization).toBe(true)
    expect(workItemsPath(route)).toBe('/workspace/work-items?assignee_ids=7%2C8')
  })

  it('fails conflicting legacy assignee scopes closed', () => {
    const route = parseWorkItemsRoute('/workspace/work-items', {
      pm_assignee_ids: '7', one2all_assignee_ids: '8',
    })
    expect(route.invalidLegacyScope).toBe(true)
  })

  it('normalizes legacy task enums into the canonical PM route contract', () => {
    const route = parseWorkItemsRoute('/workspace/work-items', {
      statuses: 'todo,IN_PROGRESS,done,unknown',
      priorities: 'medium,normal,high,unknown',
    })

    expect(route.statuses).toEqual(['draft', 'in_dev', 'completed'])
    expect(route.priorities).toEqual(['normal', 'high'])
    expect(route.needsRouteNormalization).toBe(true)
    expect(workItemsPath(route)).toBe(
      '/workspace/work-items?statuses=draft%2Cin_dev%2Ccompleted&priorities=normal%2Chigh',
    )
    expect(requestScope(route)).toMatchObject({
      statuses: 'draft,in_dev,completed', priorities: 'normal,high',
    })
  })

  it('exposes only main-backed status and priority enum keys', () => {
    expect(Object.keys(WORK_ITEMS_COPY.enums.statuses)).toEqual([
      'draft', 'in_dev', 'pending_test', 'completed', 'cancelled',
    ])
    expect(Object.keys(WORK_ITEMS_COPY.enums.priorities)).toEqual([
      'low', 'normal', 'high', 'urgent',
    ])
    expect(Object.values(WORK_ITEMS_COPY.enums.priorities)).toEqual([
      '低', '普通', '高', '紧急',
    ])
  })

  it('keeps the explicit product default at 50 rows', () => {
    expect(parseWorkItemsRoute('/workspace/work-items', {}).view).toBe('list')
    expect(parseWorkItemsRoute('/workspace/work-items', {}).pageSize).toBe(50)
    expect(workItemsPath(parseWorkItemsRoute('/workspace/work-items', { page_size: '20' })))
      .toBe('/workspace/work-items?page_size=20')
  })

  it('normalizes a legacy detail pathname to a query-owned overlay', () => {
    const route = parseWorkItemsRoute('/workspace/work-items/31', { mode: 'edit' })
    expect(route).toMatchObject({ detailId: 31, mode: 'edit', needsRouteNormalization: true })
    expect(workItemsPath(route)).toBe('/workspace/work-items?task_id=31&mode=edit')
  })

  it('starts board scope at project/version and validates requirement ownership', () => {
    const options = {
      projectSets: [{ id: 8, label: 'Project set' }],
      projects: [{ id: 13, parentId: 8, label: 'Project' }],
      requirements: [
        { id: 21, parentId: 13, label: 'Requirement' },
        { id: 22, parentId: 14, label: 'Foreign requirement' },
      ],
    }
    const normalized = normalizeWorkItemsBoardScope({
      projectSetIds: [8], projectIds: [13], requirementIds: [21],
    })

    expect(normalized).toEqual({ projectSetIds: [], projectIds: [13], requirementIds: [21] })
    expect(hasWorkItemsBoardScope(normalized)).toBe(true)
    expect(isValidWorkItemsBoardScope(normalized, options)).toBe(true)
    expect(isValidWorkItemsBoardScope({
      projectSetIds: [], projectIds: [13], requirementIds: [22],
    }, options)).toBe(false)
    expect(hasWorkItemsBoardScope({
      projectSetIds: [8], projectIds: [], requirementIds: [],
    })).toBe(false)
    expect(isValidWorkItemsListScope(normalized, options)).toBe(true)
    expect(isValidWorkItemsListScope({
      projectSetIds: [99], projectIds: [13], requirementIds: [21],
    }, options)).toBe(false)
    expect(isValidWorkItemsListScope({
      projectSetIds: [], projectIds: [13], requirementIds: [22],
    }, options)).toBe(false)
  })

  it('round-trips a board-owned scope through a list deep link', () => {
    const route = parseWorkItemsRoute('/workspace/work-items', {
      view: 'list', project_ids: '13', requirement_ids: '21',
    })
    expect(route.scope).toEqual({
      projectSetIds: [], projectIds: [13], requirementIds: [21],
    })
    expect(workItemsPath(route)).toBe(
      '/workspace/work-items?project_ids=13&requirement_ids=21',
    )
  })

  it('keeps board grouping canonical and ordered by the workflow catalog', () => {
    const draft = item(1)
    const inProgress = { ...item(2), status: 'in_progress' }
    const lanes = buildBoardLanes([draft, inProgress], [
      { key: 'todo', label: '待处理', sortOrder: 10 },
      { key: 'draft', label: '重复待处理', sortOrder: 15 },
      { key: 'in_dev', label: '进行中', sortOrder: 20 },
      { key: 'completed', label: '已完成', sortOrder: 30 },
    ])

    expect(lanes.map(({ id, label }) => [id, label])).toEqual([
      ['draft', '待处理'],
      ['in_dev', '进行中'],
      ['completed', '已完成'],
    ])
    expect(lanes.map(({ id, rows }) => [id, rows.map((row) => row.id)])).toEqual([
      ['draft', [1]],
      ['in_dev', [2]],
      ['completed', []],
    ])
  })

  it('derives one complete workflow lane set per board group', () => {
    const statuses = [
      { key: 'draft', label: '待处理', sortOrder: 10 },
      { key: 'in_dev', label: '进行中', sortOrder: 20 },
      { key: 'pending_test', label: '待测试', sortOrder: 30 },
      { key: 'completed', label: '已完成', sortOrder: 40 },
      { key: 'cancelled', label: '已取消', sortOrder: 50 },
    ]
    const groups = buildBoardGroups([
      { ...item(1), requirementId: 11, requirementTitle: '需求 A' },
      { ...item(2), requirementId: 12, requirementTitle: '需求 B' },
    ], 'requirement_id', statuses)

    expect(groups.map(({ label }) => label)).toEqual(['需求 A', '需求 B'])
    expect(groups.every(({ lanes }) => lanes.length === 5)).toBe(true)
    expect(groups[0].lanes.map(({ rows }) => rows.length)).toEqual([1, 0, 0, 0, 0])
    expect(groups[1].lanes.map(({ rows }) => rows.length)).toEqual([1, 0, 0, 0, 0])
  })

  it('preserves loading, empty, ready and stale_error state semantics', () => {
    const retry = vi.fn()
    expect(resolveCollectionState({ pending: true, fetching: false, failed: false, retry }))
      .toEqual({ status: 'loading' })
    expect(resolveCollectionState({ pending: false, fetching: false, failed: true, retry }))
      .toEqual({ status: 'error', retry })
    expect(resolveCollectionState({
      data: { items: [], total: 0 }, pending: false, fetching: false, failed: false, retry,
    })).toEqual({ status: 'empty', refresh: 'idle', retry })
    expect(resolveCollectionState({
      data: { items: [item(1)], total: 1 }, pending: false, fetching: false, failed: true, retry,
    })).toMatchObject({ status: 'ready', refresh: 'stale_error', total: 1 })
    expect(readyRefresh(true, true)).toBe('refreshing')
  })

  it('builds the PM task tree without cross-authority row IDs', () => {
    const groups = buildListGroups([item(1), item(2, 1)], 'none')
    expect(groups[0].rows.map((row) => [row.rowId, row.depth])).toEqual([
      ['task:1', 0], ['task:2', 1],
    ])
    expect(visibleTreeRows(groups[0].rows, new Set(['task:1'])).map((row) => row.id))
      .toEqual([1, 2])
    expect(visibleTreeRows(groups[0].rows, new Set()).map((row) => row.id)).toEqual([1])
  })

  it('shows paged subtasks when their parents are not included in the current page', () => {
    const groups = buildListGroups([item(2, 1), item(3, 1)], 'none')

    expect(groups[0].rows.map((row) => [row.id, row.depth])).toEqual([
      [2, 0], [3, 0],
    ])
    expect(visibleTreeRows(groups[0].rows, new Set()).map((row) => row.id)).toEqual([2, 3])
  })

  it('keeps assignee and priority groups flat when parent and child attributes diverge', () => {
    const parent = { ...item(1), assigneeId: 7, priority: 'high' }
    const child = { ...item(2, 1), assigneeId: 8, priority: 'normal' }

    expect(buildListGroups([parent, child], 'assignee_id', [
      { id: 7, label: 'Parent owner' },
      { id: 8, label: 'Child owner' },
    ]).flatMap((group) => group.rows.map((row) => [row.id, row.depth, row.hasChildren])))
      .toEqual([[1, 0, false], [2, 0, false]])
  })

  it('hydrates assignee avatars for users referenced by the current task page', () => {
    expect(workItemAssigneeDirectoryIds([
      { ...item(1), assigneeId: 47 },
      { ...item(2), assigneeId: 110 },
      { ...item(3), assigneeId: 47 },
      item(4),
    ], [2, 110, 0])).toEqual([2, 47, 110])
  })

  it('marks unfinished tasks overdue only after their local calendar due date', () => {
    const now = new Date(2026, 7, 5, 23, 59, 59).getTime()
    expect(isWorkItemOverdue({ dueAt: '2026-08-04', status: 'in_dev' }, now)).toBe(true)
    expect(isWorkItemOverdue({ dueAt: '2026-08-05', status: 'in_dev' }, now)).toBe(false)
    expect(isWorkItemOverdue({ dueAt: '2026-08-04', status: 'completed' }, now)).toBe(false)
    expect(isWorkItemOverdue({ dueAt: '2026-08-04', status: 'cancelled' }, now)).toBe(false)
    expect(isWorkItemOverdue({ dueAt: '2026-08-06', status: 'draft' }, now)).toBe(false)
  })

  it('accepts only backend-safe estimated-minute values', () => {
    expect(isValidEstimatedMinutes(undefined)).toBe(true)
    expect(isValidEstimatedMinutes(0)).toBe(true)
    expect(isValidEstimatedMinutes(999_960)).toBe(true)
    expect(isValidEstimatedMinutes(-1)).toBe(false)
    expect(isValidEstimatedMinutes(1.5)).toBe(false)
    expect(isValidEstimatedMinutes(999_961)).toBe(false)
  })
})
