import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

const latestProjectNewsQuery = (() => vi.fn())()

vi.mock('../hooks/useProjectNews', () => ({
  useLatestProjectNewsQuery: latestProjectNewsQuery,
}))

import { LatestProjectUpdate } from './LatestProjectUpdate'

afterEach(cleanup)

describe('latest project update', () => {
  const refetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders only factual update content without exposing internal IDs', () => {
    latestProjectNewsQuery.mockReturnValue({
      data: {
        id: 81,
        workspace_id: 7,
        project_set_id: 9,
        plan_unit_id: 0,
        user_id: 23,
        title: '发布候选版本完成',
        content: '回归测试已通过，准备发布。',
        created_at: '2026-08-10T08:00:00Z',
        updated_at: '2026-08-10T08:00:00Z',
      },
      isPending: false,
      isFetching: false,
      isError: false,
      refetch,
    })

    render(<LatestProjectUpdate projectId={9} />)

    expect(latestProjectNewsQuery).toHaveBeenCalledWith(9)
    expect(screen.getByRole('heading', {
      name: copy.projectManagement.overview.latestUpdate.heading,
    })).toBeTruthy()
    expect(screen.getByText('发布候选版本完成')).toBeTruthy()
    expect(screen.getByText('回归测试已通过，准备发布。')).toBeTruthy()
    expect(screen.getByText(/2026/u).getAttribute('datetime'))
      .toBe('2026-08-10T08:00:00Z')
    expect(screen.queryByText(/#?81|#?23/u)).toBeNull()
  })

  it('distinguishes loading, empty, and initial error states', () => {
    latestProjectNewsQuery.mockReturnValue({
      data: undefined,
      isPending: true,
      isFetching: true,
      isError: false,
      refetch,
    })
    const view = render(<LatestProjectUpdate projectId={9} />)
    expect(screen.getByRole('status').getAttribute('aria-label')).toBe(
      copy.projectManagement.overview.latestUpdate.loading,
    )

    latestProjectNewsQuery.mockReturnValue({
      data: null,
      isPending: false,
      isFetching: false,
      isError: false,
      refetch,
    })
    view.rerender(<LatestProjectUpdate projectId={9} />)
    expect(screen.getByText(copy.projectManagement.overview.latestUpdate.empty)).toBeTruthy()

    latestProjectNewsQuery.mockReturnValue({
      data: undefined,
      isPending: false,
      isFetching: false,
      isError: true,
      refetch,
    })
    view.rerender(<LatestProjectUpdate projectId={9} />)
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.common.retry,
    }))
    expect(screen.getByRole('alert').textContent).toContain('加载失败')
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('keeps the last factual update visible after a background refresh error', () => {
    latestProjectNewsQuery.mockReturnValue({
      data: {
        id: 1,
        title: '当前更新',
        content: '',
        created_at: '2026-08-10T08:00:00Z',
      },
      isPending: false,
      isFetching: false,
      isError: true,
      refetch,
    })

    render(<LatestProjectUpdate projectId={9} />)

    expect(screen.getByText('当前更新')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('上次加载结果')
  })
})
