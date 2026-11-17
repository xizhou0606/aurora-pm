import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { ActivityTable } from './ActivityTable'

afterEach(cleanup)

describe('project activity table', () => {
  it('keeps cached rows visible and exposes a stale retry intent', () => {
    const retry = vi.fn()
    render(<ActivityTable state={{
      status: 'ready',
      scope: 'plan-unit',
      rows: [{
        id: 1,
        created_at: '2026-08-03T08:00:00Z',
        actor: { name: 'Teo', avatarUrl: '/teo.png' },
        actionLabel: '更新',
        resourceLabel: '需求',
        resultLabel: '成功',
      }],
      refresh: { status: 'stale_error', retry },
    }} />)

    expect(screen.getByRole('alert').textContent).toContain(
      copy.projectManagement.activity.staleError,
    )
    expect(screen.getByText('Teo')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.common.retry,
    }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('announces background refresh without replacing the scoped empty state', () => {
    render(<ActivityTable state={{
      status: 'empty',
      scope: 'project',
      refresh: { status: 'refreshing' },
    }} />)

    expect(screen.getByText(copy.projectManagement.activity.refreshing)).toBeTruthy()
    expect(screen.getByText(
      copy.projectManagement.activity.project.emptyTitle,
    )).toBeTruthy()
  })
})
