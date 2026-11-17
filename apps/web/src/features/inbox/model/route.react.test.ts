import { describe, expect, it, vi } from 'bun:test'

import {
  inboxPath,
  parseInboxRoute,
  requestScope,
} from './route'
import {
  aggregateRefresh,
  resolveCollectionState,
  resolveSummaryState,
} from './state'

describe('Inbox Router ownership', () => {
  it('normalizes canonical scope, search and independent feed pages', () => {
    const route = parseInboxRoute({
      project_set_ids: '8,8',
      project_ids: '13',
      requirement_ids: '21,34',
      q: '  登录问题  ',
      workflow_page: 5,
      task_page: 2,
      bug_page: '3',
      mention_page: '4',
    })

    expect(route).toEqual({
      projectSetIds: [8],
      projectIds: [13],
      requirementIds: [21, 34],
      search: '登录问题',
      workflowPage: 5,
      taskPage: 2,
      bugPage: 3,
      mentionPage: 4,
      invalidScope: false,
      invalidLegacyScope: false,
    })
    expect(requestScope(route)).toEqual({
      project_set_ids: '8',
      project_ids: '13',
      requirement_ids: '21,34',
      q: '登录问题',
    })
    expect(inboxPath(route)).toBe(
      '/workspace/inbox?project_set_ids=8&project_ids=13&requirement_ids=21%2C34&q=%E7%99%BB%E5%BD%95%E9%97%AE%E9%A2%98&workflow_page=5&task_page=2&bug_page=3&mention_page=4',
    )
  })

  it('fails closed for unresolved legacy or malformed canonical scope', () => {
    expect(parseInboxRoute({ team_id: 7 })).toMatchObject({
      invalidScope: true,
      invalidLegacyScope: true,
    })
    expect(parseInboxRoute({ project_id: 9 })).toMatchObject({
      invalidScope: true,
      invalidLegacyScope: true,
    })
    expect(parseInboxRoute({ project_set_ids: '8,unsafe' })).toMatchObject({
      invalidScope: true,
      invalidLegacyScope: false,
    })
    expect(parseInboxRoute({ project_id: 9, project_ids: '11' })).toMatchObject({
      invalidScope: true,
      invalidLegacyScope: true,
    })
  })
})

describe('Inbox collection state machines', () => {
  it('covers loading, error, empty and refreshing ready', () => {
    const retry = vi.fn()
    expect(resolveCollectionState({
      pending: true,
      fetching: true,
      failed: false,
      retry,
    })).toEqual({ status: 'loading' })
    expect(resolveCollectionState({
      pending: false,
      fetching: false,
      failed: true,
      retry,
    })).toEqual({ status: 'error', retry })
    expect(resolveCollectionState({
      data: { items: [], total: 0 },
      pending: false,
      fetching: false,
      failed: false,
      retry,
    })).toEqual({ status: 'empty', total: 0, refresh: 'idle' })
    expect(resolveCollectionState({
      data: { items: [{ id: 1 }], total: 1 },
      pending: false,
      fetching: true,
      failed: false,
      retry,
    })).toMatchObject({ status: 'ready', refresh: 'refreshing' })
  })

  it('retains successful feed and summary data after a stale refresh error', () => {
    const retry = vi.fn()
    expect(resolveCollectionState({
      data: { items: [{ id: 1 }], total: 1 },
      pending: false,
      fetching: false,
      failed: true,
      retry,
    })).toMatchObject({ status: 'ready', refresh: 'stale_error' })
    expect(resolveSummaryState({
      data: {
        total: 1,
        tasks: 1,
        bugs: 0,
        workflows: 0,
        overdue: 0,
        dueToday: 0,
        blockedTasks: 0,
        reviewBugs: 0,
        workflowAIFailed: 0,
        mentions: 0,
      },
      pending: false,
      fetching: false,
      failed: true,
      retry,
    })).toMatchObject({ status: 'ready', refresh: 'stale_error' })
  })

  it('aggregates background refresh from both populated and empty collections', () => {
    expect(aggregateRefresh([
      { status: 'empty', total: 0, refresh: 'refreshing' },
      { status: 'ready', rows: [{ id: 1 }], total: 1, refresh: 'idle' },
    ])).toBe('refreshing')
    expect(aggregateRefresh([
      { status: 'empty', total: 0, refresh: 'stale_error' },
      { status: 'ready', rows: [{ id: 1 }], total: 1, refresh: 'refreshing' },
    ])).toBe('stale_error')
  })
})
