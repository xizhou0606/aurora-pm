import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'

const fakes = (() => ({
  auditHook: vi.fn(),
  overviewHook: vi.fn(),
  usersHook: vi.fn(),
  auditRefetch: vi.fn(async () => undefined),
  overviewRefetch: vi.fn(async () => undefined),
}))()

mock.module('../model', () => ({
    useProjectAuditQuery: fakes.auditHook,
    useProjectOverviewQuery: fakes.overviewHook,
    usePlatformUsersQuery: fakes.usersHook,
  }))

import { copy } from '@/shared/copy'
import { useActivityModel } from './useActivityModel'

describe('project activity workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fakes.auditHook.mockReturnValue({
      status: 'error',
      data: {
        rows: [{
          id: 1,
          user_id: 11,
          resource: 'requirement',
          resource_id: 51,
          action: 'update',
        }],
      },
      error: new PMSafeApiError('AUDIT_UNAVAILABLE', 503),
      isFetching: false,
      isPending: false,
      isPlaceholderData: false,
      refetch: fakes.auditRefetch,
    })
    fakes.overviewHook.mockReturnValue({
      status: 'success',
      data: {
        requirements: [{
          id: 51,
          team_id: 2,
          project_new_id: 9,
          version_task_new_id: 31,
          title: 'Search',
          status: 'development',
        }],
        tasks: [],
      },
      error: null,
      isFetching: false,
      isPending: false,
      refetch: fakes.overviewRefetch,
    })
    fakes.usersHook.mockReturnValue({
      data: [{ id: 11, username: 'other-workspace', display_name: 'Other Workspace' }],
      isError: true,
    })
  })

  it('reuses the project aggregate query, isolates team scope and retries both facts', async () => {
    const { result } = renderHook(() => useActivityModel({
      kind: 'plan-unit',
      projectId: 9,
      planUnitId: 31,
      teamId: 2,
    }))

    expect(fakes.auditHook).toHaveBeenCalledWith(9, true, { limit: 30, offset: 0 })
    expect(fakes.overviewHook).toHaveBeenCalledWith({
      projectId: 9,
      teamId: 2,
    }, true)
    expect(result.current).toMatchObject({
      status: 'ready',
      rows: [{ id: 1, actor: { userId: 11, name: copy.projectManagement.activity.user(11) } }],
      refresh: { status: 'stale_error' },
    })

    if (result.current.status !== 'ready' || result.current.refresh.status !== 'stale_error') {
      throw new Error('expected cached stale activity')
    }
    act(() => result.current.status === 'ready' && result.current.refresh.status === 'stale_error'
      ? result.current.refresh.retry()
      : undefined)
    await waitFor(() => {
      expect(fakes.auditRefetch).toHaveBeenCalledTimes(1)
      expect(fakes.overviewRefetch).toHaveBeenCalledTimes(1)
    })
  })

  it('fails closed as loading while audit rows are cross-key placeholder data', () => {
    fakes.auditHook.mockReturnValue({
      status: 'success',
      data: {
        rows: [{
          id: 99,
          resource: 'version',
          resource_id: 31,
          action: 'update',
        }],
      },
      error: null,
      isFetching: true,
      isPending: false,
      isPlaceholderData: true,
      refetch: fakes.auditRefetch,
    })

    const { result } = renderHook(() => useActivityModel({
      kind: 'project',
      projectId: 9,
      teamId: 2,
    }))

    expect(result.current).toEqual({ status: 'loading', scope: 'project' })
    expect(fakes.overviewHook).toHaveBeenCalledWith({ projectId: 9, teamId: 2 }, false)
  })
})
