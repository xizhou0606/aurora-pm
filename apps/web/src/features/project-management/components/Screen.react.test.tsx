import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import type { ProjectScreenState } from '../model'

vi.mock('./Ready', () => ({
  ProjectReady: () => <div>project-ready-content</div>,
  PlanUnitReady: () => <div>plan-unit-ready-content</div>,
}))

import { Screen } from './Screen'

function projectReadyState(
  refresh: Extract<ProjectScreenState, { status: 'ready' }>['refresh'],
): ProjectScreenState {
  return {
    status: 'ready',
    refresh,
    resource: {
      kind: 'project',
      projectId: 9,
      project: {
        id: 9,
        team_id: 2,
        name: 'Project Nine',
        status: 'active',
      },
      planUnits: [],
      header: {
        title: 'Project Nine',
        description: 'Cached project board',
        meta: [],
      },
      content: { kind: 'overview', units: [], users: [] },
    },
  }
}

describe('project screen cached refresh feedback', () => {
  it('renders a destination-shaped skeleton while the project board is loading', () => {
    const { container } = render(
      <Screen
        state={{ status: 'loading' }}
        actions={{ kind: 'none' }}
        variant="master"
      />,
    )

    const skeleton = container.querySelector('[data-slot="project-board-skeleton"]')
    expect(skeleton).toBeTruthy()
    expect(skeleton?.getAttribute('aria-busy')).toBe('true')
    expect(skeleton?.getAttribute('aria-label'))
      .toBe(copy.projectManagement.overview.loading)
    expect(container.querySelectorAll('[data-slot="skeleton"]').length)
      .toBeGreaterThan(10)
  })

  it('keeps native board content visible and exposes stale_error retry', () => {
    const retry = vi.fn()
    render(
      <Screen
        state={projectReadyState({ status: 'stale_error', retry })}
        actions={{ kind: 'none' }}
        variant="scoped"
      />,
    )

    expect(screen.getByText('project-ready-content')).toBeTruthy()
    expect(screen.getByRole('alert').textContent)
      .toContain(copy.projectManagement.common.staleError)
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.common.retry,
    }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('announces a background board refresh without replacing content', () => {
    render(
      <Screen
        state={projectReadyState({ status: 'refreshing' })}
        actions={{ kind: 'none' }}
        variant="scoped"
      />,
    )

    expect(screen.getByText('project-ready-content')).toBeTruthy()
    expect(screen.getByRole('status').textContent)
      .toContain(copy.projectManagement.common.refreshing)
  })
})
