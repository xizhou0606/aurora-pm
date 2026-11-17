import { describe, expect, it, vi } from 'bun:test'

import type { PMTransportRequest, WorkItemsClient } from './client'
import { workflowStatusOptions } from '../model/presenters'
import type { WorkItem } from '../model/types'
import { requestWorkItemWorkflowCatalog } from './workflow'

function client(request: (entry: PMTransportRequest) => Promise<unknown>): WorkItemsClient {
  return {
    request: async <T,>(entry: PMTransportRequest) => request(entry) as Promise<T>,
    requestPage: async <T,>() => ({ items: [] as T[], total: 0 }),
    requestPMScopeOptions: async <T,>() => undefined as T,
    requestPMUsers: async <T,>() => undefined as T,
  }
}

const item = {
  id: 31,
  rowId: 'task:31',
  title: 'Task',
  status: 'IN_DEV',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: false,
    canComment: false,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: ['COMPLETED', 'cancelled', 'unknown'],
  },
} satisfies WorkItem

describe('work-items workflow contract', () => {
  it('loads, normalizes and sorts the scoped task workflow through the owner client', async () => {
    const request = vi.fn((entry: PMTransportRequest) => {
      if (entry.path === '/issue-statuses') return Promise.resolve({ items: [
        { status_key: 'completed', label: '已完成', sort_order: 30 },
        { status_key: 'in_dev', label: '进行中', sort_order: 10 },
      ] })
      return Promise.resolve({ data: {
        enforce: true,
        transitions: [{ from_status: 'IN_DEV', to_status: 'COMPLETED' }],
      } })
    })
    const signal = new AbortController().signal

    const result = await requestWorkItemWorkflowCatalog(
      client(request),
      { teamId: 4, projectId: 9 },
      signal,
    )

    expect(result).toEqual({
      statuses: [
        { key: 'in_dev', label: '进行中', sortOrder: 10 },
        { key: 'completed', label: '已完成', sortOrder: 30 },
      ],
      transitions: [{ from: 'in_dev', to: 'completed' }],
      strict: true,
    })
    expect(request).toHaveBeenNthCalledWith(1, {
      path: '/issue-statuses',
      query: { team_id: 4, project_id: 9, kind: 'task' },
      signal,
    })
    expect(request).toHaveBeenNthCalledWith(2, {
      path: '/issue-workflow-transitions',
      query: { team_id: 4, project_id: 9, kind: 'task' },
      signal,
    })
  })

  it('intersects row capabilities with catalog statuses and strict workflow edges', () => {
    expect(workflowStatusOptions(item, {
      statuses: [
        { key: 'in_dev', label: '进行中', sortOrder: 10 },
        { key: 'cancelled', label: '已取消', sortOrder: 20 },
        { key: 'completed', label: '已完成', sortOrder: 30 },
      ],
      transitions: [{ from: 'in_dev', to: 'completed' }],
      strict: true,
    })).toEqual([
      { value: 'in_dev', label: '进行中' },
      { value: 'completed', label: '已完成' },
    ])
  })

  it.each([
    [{ data: [{ status_key: 'done' }, { status_key: 'DONE' }] }, { transitions: [] }],
    [{ data: [{ status_key: 'done' }] }, { transitions: [{ from_status: '', to_status: 'done' }] }],
  ])('rejects malformed workflow facts instead of widening status options', async (statuses, workflow) => {
    const request = vi.fn((entry: PMTransportRequest) => Promise.resolve(
      entry.path === '/issue-statuses' ? statuses : workflow,
    ))

    await expect(requestWorkItemWorkflowCatalog(
      client(request),
      { teamId: 4, projectId: 9 },
    )).rejects.toThrow()
  })
})
