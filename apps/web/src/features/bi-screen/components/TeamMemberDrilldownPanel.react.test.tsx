import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { Environment } from './environment'
import type { RouteState } from '../types'
import { TeamMemberDrilldownPanel } from './TeamMemberDrilldownPanel'

const route: RouteState = {
  scope: { projectSetIds: [], projectIds: [], requirementIds: [] },
  period: 'month',
  tab: 'team',
  teamDimension: 'month',
  teamDimensionKey: '2026-08',
  teamView: 'data',
  invalidScope: false,
}

describe('TeamMemberDrilldownPanel', () => {
  it('keeps the panel and list viewport stable while detail data is loading', async () => {
    const environment = {
      workspaceId: 2,
      sessionRevision: 1,
      client: {
        teamMemberItems: vi.fn(() => new Promise(() => {})),
      },
    } as unknown as Environment
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: Infinity } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <TeamMemberDrilldownPanel
          environment={environment}
          route={route}
          target={{ userId: 47, userName: '铁奥', role: 'development' }}
          itemType="task"
          onItemTypeChange={vi.fn()}
          onOpenChange={vi.fn()}
        />
      </QueryClientProvider>,
    )

    expect((await screen.findByRole('dialog', { name: /研发 · 铁奥/ })).className).toContain('h-[90dvh]')
    const content = document.querySelector('[data-slot="responsive-panel-content"]')
    expect(content?.className).toContain('grid-rows-[auto_minmax(0,1fr)]')
    expect(content?.className).toContain('overflow-hidden')

    const table = document.querySelector('[data-slot="data-table"]')
    expect(table?.className).toContain('h-full')
    expect(table?.className).toContain('[&_[data-slot=table-scroll-shell]]:h-full')
    expect(document.querySelector('[data-slot="table-container"]')?.className).toContain('h-full')
    expect(screen.getByRole('status', { name: '正在加载成员明细' })).not.toBeNull()
  })

  it('shows the effective deadline and delivery node for development tasks', async () => {
    const environment = {
      workspaceId: 2,
      sessionRevision: 1,
      client: {
        teamMemberItems: vi.fn().mockResolvedValue({
          role: 'development',
          item_type: 'task',
          user_id: 47,
          dimension: 'month',
          dimension_key: '2026-08',
          total: 1,
          items: [{
            id: 701,
            key: 'TASK-701',
            title: '继承父任务截止时间的子任务',
            status: 'pending_test',
            priority: 'high',
            created_at: '2026-08-20T08:00:00Z',
            effective_due_at: '2026-08-22T15:59:59Z',
            due_at_source: 'parent',
            due_at_source_task_id: 700,
            delivery_at: '2026-08-21T09:30:00Z',
            overdue: false,
          }],
        }),
      },
    } as unknown as Environment
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: Infinity } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <TeamMemberDrilldownPanel
          environment={environment}
          route={route}
          target={{ userId: 47, userName: '铁奥', role: 'development' }}
          itemType="task"
          onItemTypeChange={vi.fn()}
          onOpenChange={vi.fn()}
        />
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('columnheader', { name: '结束时间' })).not.toBeNull()
    expect(screen.getByRole('columnheader', { name: '结转时间' })).not.toBeNull()
    expect(document.querySelector('time[datetime="2026-08-22T15:59:59Z"]')).not.toBeNull()
    expect(document.querySelector('time[datetime="2026-08-21T09:30:00Z"]')).not.toBeNull()
		expect(screen.getByText('未设置，继承自父任务')).not.toBeNull()
  })
})
