import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { useProjectListModel } from './useProjectListModel'

const queries = (() => ({
  list: {
    data: {
      rows: [{ id: 9, team_id: 1, name: 'Existing project set', status: 'active' }],
      total: 41,
    },
    status: 'success' as const,
    isFetching: false,
    isPending: false,
    refetch: vi.fn(async () => undefined),
  },
  users: {
    data: [],
  },
}))()

mock.module('../model', () => ({
    useProjectListQuery: () => queries.list,
    usePlatformUsersQuery: () => queries.users,
  }))

describe('project-set master selection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    queries.list.data = {
      rows: [{ id: 9, team_id: 1, name: 'Existing project set', status: 'active' }],
      total: 41,
    }
  })

  it('keeps the collection route explicitly unselected', () => {
    const { result } = renderHook(() => useProjectListModel())

    expect(result.current.state.status).toBe('ready')
    if (result.current.state.status !== 'ready') return
    expect(result.current.state.items[0]?.selected).toBe(false)
  })
})
