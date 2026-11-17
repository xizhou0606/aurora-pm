import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { WorkItemsEnvironment } from '../../model/environment'
import { useWorkItemsRouteWorkflow } from './useRouteWorkflow'

function environment(
  search: Readonly<Record<string, unknown>>,
  pathname = '/workspace/work-items',
): WorkItemsEnvironment {
  return {
    workspaceId: 7,
    userId: 101,
    sessionRevision: 1,
    pathname,
    search,
    allowed: true,
    createAllowed: true,
    operationAccess: {
      status: 'ready', retry: vi.fn(), viewReferenceData: true,
      viewTaskTable: true, viewTaskDetail: true, createTask: true,
      updateTask: true, transitionTask: true, deleteTask: true, viewAttachments: true,
      uploadAttachment: true, downloadAttachment: true, deleteAttachment: true,
      viewComments: true, createComment: true, deleteComment: true,
      updateWatching: true,
    },
    navigate: vi.fn().mockResolvedValue(undefined),
    client: {
      request: vi.fn(),
      requestPage: vi.fn(),
      requestPMScopeOptions: vi.fn(),
      requestPMUsers: vi.fn(),
    },
  }
}

describe('useWorkItemsRouteWorkflow board scope', () => {
  it('commits a normalized search value without retaining input draft state', () => {
    const current = environment({})
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    act(() => result.current.setSearch('  登录  '))
    expect(current.navigate).toHaveBeenCalledTimes(1)
    expect(current.navigate).toHaveBeenCalledWith('/workspace/work-items?q=%E7%99%BB%E5%BD%95', true)
  })

  it('clears the previous list scope when entering the board', () => {
    const current = environment({
      view: 'list',
      project_set_ids: '8',
      project_ids: '13',
      requirement_ids: '21',
    })
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    act(() => result.current.setView('board'))
    expect(current.navigate).toHaveBeenCalledWith('/workspace/work-items?view=board', false)
  })

  it('preserves a board project/requirement scope when switching to the list', () => {
    const current = environment({
      view: 'board',
      project_ids: '13',
      requirement_ids: '21',
    })
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    act(() => result.current.setView('list'))
    expect(current.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?project_ids=13&requirement_ids=21',
      false,
    )
  })

  it('removes project-set-only board input from the canonical URL', async () => {
    const current = environment({
      view: 'board',
      project_set_ids: '8',
      project_ids: '13',
    })
    renderHook(() => useWorkItemsRouteWorkflow(current))

    await waitFor(() => expect(current.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?view=board&project_ids=13',
      true,
    ))
  })

  it('preserves grouping in a board deep link', async () => {
    const current = environment({ view: 'board', project_ids: '13', group_by: 'priority' })
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    await waitFor(() => expect(result.current.route.groupBy).toBe('priority'))
    expect(current.navigate).not.toHaveBeenCalled()
  })

  it('returns to the inbox when closing a task opened from my todos', () => {
    const current = environment(
      { from: 'inbox' },
      '/workspace/work-items',
    )
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    act(() => result.current.closeDetail())

    expect(current.navigate).toHaveBeenCalledWith('/workspace/inbox')
  })

  it('returns to the work-items collection when closing a regular task detail', () => {
    const current = environment(
      { q: 'login', page: 2 },
      '/workspace/work-items',
    )
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))

    act(() => result.current.closeDetail())

    expect(current.navigate).toHaveBeenCalledWith('/workspace/work-items?q=login&page=2')
  })

  it('opens a task in edit mode only when explicitly requested', () => {
    const current = environment({ q: 'login' })
    const { result } = renderHook(() => useWorkItemsRouteWorkflow(current))
    const row = { id: 31 } as Parameters<typeof result.current.openItem>[0]

    act(() => result.current.openItem(row))
    expect(current.navigate).toHaveBeenLastCalledWith('/workspace/work-items?q=login&task_id=31')

    act(() => result.current.openItem(row, 'edit'))
    expect(current.navigate).toHaveBeenLastCalledWith('/workspace/work-items?q=login&task_id=31&mode=edit')
  })
})
