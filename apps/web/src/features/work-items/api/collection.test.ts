import { describe, expect, it, vi } from 'bun:test'

import type { PMTransportRequest, WorkItemsClient } from './client'
import { requestPMTasks, requestWorkItemsPage } from './collection'
import { pmTasksQueryKey } from '../hooks/collection/useQueries'
import { parseWorkItemsRoute } from '../model/utils'

function task(id: number) {
  return {
    id,
    workspace_id: 7,
    task_key: `PM-${id}`,
    title: `Task ${id}`,
    kind: 'task',
    status: 'draft',
    priority: 'normal',
    can_update: true,
    can_delete: true,
    can_comment: true,
    allowed_status_transitions: ['draft', 'in_dev'],
  }
}

function client(
  requestPage: (request: PMTransportRequest) => Promise<{ items: unknown[]; total: number }>,
): WorkItemsClient {
  return {
    request: vi.fn(),
    requestPage: requestPage as WorkItemsClient['requestPage'],
    requestPMScopeOptions: vi.fn(),
    requestPMUsers: vi.fn(),
  }
}

describe('PM work-items collection contract', () => {
  it('uses the sole PM /tasks source with complete server filters and pagination', async () => {
    const requestPage = vi.fn(async <T,>() => ({
      items: [{ ...task(31), filter_match: true }] as T[],
      total: 40,
    }))
    const route = parseWorkItemsRoute('/workspace/work-items', {
      view: 'list', q: 'needle', project_set_ids: '8', project_ids: '13',
      requirement_ids: '21', assignee_ids: '7,8', statuses: 'draft,in_dev',
      priorities: 'normal,high', filter_kinds: 'task,subtask', page: '2',
      page_size: '50', sort: 'updated_at_desc',
    })

    const page = await requestPMTasks(client(requestPage), route)

    expect(page.items[0]).toMatchObject({ id: 31, rowId: 'task:31', title: 'Task 31' })
    expect(page.total).toBe(40)
    expect(requestPage).toHaveBeenCalledWith({
      path: '/tasks',
      query: {
        project_set_ids: '8', project_ids: '13', requirement_ids: '21',
        q: 'needle', assignee_ids: '7,8', statuses: 'draft,in_dev',
        priorities: 'normal,high', filter_kinds: 'task,subtask',
        limit: 50, offset: 50, sort: 'updated_at_desc', hierarchy: 'filtered_tree',
      },
      signal: undefined,
    })
  })

  it('loads board rows from PM in bounded 50-row pages', async () => {
    const requests: PMTransportRequest[] = []
    const requestPage = vi.fn(async <T,>(request: PMTransportRequest) => {
      requests.push(request)
      const offset = Number(request.query?.offset || 0)
      const rows = offset === 0
        ? Array.from({ length: 50 }, (_, index) => task(index + 1))
        : Array.from({ length: 5 }, (_, index) => task(offset + index + 1))
      return { items: rows as T[], total: 55 }
    })

    const page = await requestPMTasks(
      client(requestPage),
      parseWorkItemsRoute('/workspace/work-items', { view: 'board' }),
    )

    expect(page.items).toHaveLength(55)
    expect(requests.map((request) => request.query)).toEqual([
      { limit: 50, offset: 0, sort: 'created_at_desc' },
      { limit: 50, offset: 50, sort: 'created_at_desc' },
    ])
  })

  it('loads only the first Gantt page before the initial render', async () => {
    const requestPage = vi.fn(async <T,>() => ({
      items: Array.from({ length: 50 }, (_, index) => task(index + 1)) as T[],
      total: 120,
    }))

    const page = await requestPMTasks(
      client(requestPage),
      parseWorkItemsRoute('/workspace/work-items', { view: 'list', layout: 'gantt' }),
    )

    expect(page.items).toHaveLength(50)
    expect(page.total).toBe(120)
    expect(requestPage).toHaveBeenCalledTimes(1)
    expect(requestPage).toHaveBeenCalledWith(expect.objectContaining({
      path: '/tasks',
      query: { limit: 50, offset: 0, sort: 'created_at_desc' },
    }))
  })

  it('rejects malformed PM task rows fail closed', async () => {
    const requestPage = vi.fn(async <T,>() => ({ items: [{ id: 1 }] as T[], total: 1 }))
    await expect(requestWorkItemsPage(
      client(requestPage),
      parseWorkItemsRoute('/workspace/work-items', { view: 'list' }),
      50,
      0,
    )).rejects.toThrow('invalid_work_item_page')
  })

  it('normalizes legacy DTO enum aliases without exposing duplicate options', async () => {
    const requestPage = vi.fn(async <T,>() => ({
      items: [{
        ...task(31), status: 'in_progress', priority: 'medium',
        allowed_status_transitions: ['doing', 'DONE', 'unknown'],
      }] as T[],
      total: 1,
    }))

    const page = await requestWorkItemsPage(
      client(requestPage),
      parseWorkItemsRoute('/workspace/work-items', { view: 'list' }),
      50,
      0,
    )

    expect(page.items[0]).toMatchObject({
      status: 'in_dev', priority: 'normal',
      capabilities: { allowedTransitions: ['in_dev', 'completed'] },
    })
  })

  it('does not widen malformed owner transitions through a legacy fallback field', async () => {
    const requestPage = vi.fn(async <T,>() => ({
      items: [{
        ...task(31),
        allowed_status_transitions: ['unknown'],
        allowed_transitions: ['completed'],
      }] as T[],
      total: 1,
    }))

    const page = await requestWorkItemsPage(
      client(requestPage),
      parseWorkItemsRoute('/workspace/work-items', { view: 'list' }),
      50,
      0,
    )

    expect(page.items[0].capabilities.allowedTransitions).toEqual([])
  })

  it('owns workspace, complete server filters, pagination and sort in the collection key', () => {
    const route = parseWorkItemsRoute('/workspace/work-items', {
      view: 'list', layout: 'table', q: 'needle', project_set_ids: '8',
      project_ids: '13', requirement_ids: '21', assignee_ids: '7,8',
      statuses: 'draft,in_dev', priorities: 'normal,high',
      filter_kinds: 'task,subtask', overdue: '1', page: '2',
      page_size: '50', sort: 'updated_at_desc',
    })

    expect(pmTasksQueryKey(7, route)).toEqual(expect.arrayContaining([
      7,
      expect.objectContaining({
        q: 'needle',
        project_set_ids: '8',
        project_ids: '13',
        requirement_ids: '21',
        assignee_ids: '7,8',
        statuses: 'draft,in_dev',
        priorities: 'normal,high',
        filter_kinds: 'task,subtask',
        overdue: 1,
        limit: 50,
        offset: 50,
        sort: 'updated_at_desc',
        hierarchy: 'filtered_tree',
      }),
    ]))
  })
})
