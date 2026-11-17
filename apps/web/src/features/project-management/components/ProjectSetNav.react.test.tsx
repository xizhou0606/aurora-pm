import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import type { ProjectListState } from '../model'
import { ProjectSetNav } from './ProjectSetNav'

const projectState: ProjectListState = {
  status: 'ready',
  summary: 'summary',
  items: [{
    id: 9,
    name: 'Project Nine',
    description: 'Project description',
    owner: { userId: 11, name: 'Teo', avatarUrl: '/teo.png' },
    status: '进行中',
    statusKey: 'active',
    health: '正常',
    planUnitCount: 2,
    requirementCount: 3,
    progress: 50,
    selected: true,
  }],
  total: 1,
  refreshing: false,
}

function renderNavigation() {
  const select = vi.fn()
  const root = createRootRoute({ component: Outlet })
  const list = createRoute({
    getParentRoute: () => root,
    path: '/workspace/projects',
    component: () => (
      <ProjectSetNav
        state={projectState}
        search={{ value: '', onChange: vi.fn() }}
        pagination={{ page: 1, pageSize: 20, onChange: vi.fn() }}
        actions={{ select, items: new Map() }}
      />
    ),
  })
  const detail = createRoute({
    getParentRoute: () => root,
    path: '/workspace/projects/$projectId',
    component: () => <div data-testid="project-detail" />,
  })
  const router = createRouter({
    routeTree: root.addChildren([list, detail]),
    history: createMemoryHistory({ initialEntries: ['/workspace/projects'] }),
  })
  render(<RouterProvider router={router} />)
  return { router, select }
}

describe('ProjectSetNav', () => {
  it('selects from the project card body without a detail toggle entry', async () => {
    const { router, select } = renderNavigation()
    const card = await screen.findByRole('button', { name: /Project Nine/ })

    expect(card.getAttribute('aria-pressed')).toBe('true')
    expect(document.querySelector('[data-test="project-set-detail-toggle"]')).toBeNull()
    expect(document.querySelector('a[href*="/workspace/projects/9"]')).toBeNull()
    expect(document.querySelector('[data-test="pm-project-sidebar"]')?.className)
      .toContain('overflow-hidden')
    expect(document.querySelector('[data-test="project-set-list"]')?.className)
      .toContain('overflow-y-auto')
    expect(document.querySelector('[data-test="project-set-stats"]')?.textContent)
      .toContain(`2${copy.projectManagement.projectList.planUnitLabel}`)
    expect(screen.getByText('进行中').getAttribute('data-tone')).toBe('success')

    fireEvent.click(card)
    expect(select).toHaveBeenCalledWith(9)
    expect(router.state.location.pathname).toBe('/workspace/projects')
  })
})
