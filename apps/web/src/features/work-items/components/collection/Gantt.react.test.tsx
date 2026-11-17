import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItem, WorkItemsGroupBy } from '../../model/types'
import { Gantt } from './Gantt'

vi.mock('./GanttScheduler', () => ({
  GanttScheduler: ({ groups, window }: {
    groups: readonly { id: string; rows: readonly unknown[] }[]
    window: { startDate: string; days: number }
  }) => (
    <div
      data-testid="scheduler-stub"
      data-groups={groups.length}
      data-group-ids={groups.map((group) => group.id).join(',')}
      data-rows={groups.reduce((total, group) => total + group.rows.length, 0)}
      data-start={window.startDate}
      data-days={window.days}
    />
  ),
}))

let root: Root | undefined
let container: HTMLDivElement | undefined
;(globalThis as typeof globalThis & { React?: typeof React; IS_REACT_ACT_ENVIRONMENT?: boolean }).React = React
;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

afterEach(async () => {
  if (root) await act(async () => root?.unmount())
  container?.remove()
  root = undefined
  container = undefined
})

function item(overrides: Partial<WorkItem> = {}): WorkItem {
  return {
    id: 71,
    rowId: 'task:71',
    taskKey: 'GLOBAL-71',
    title: 'Verify native Gantt',
    assigneeId: 7,
    assigneeName: 'Teo',
    startsAt: '2026-08-02',
    dueAt: '2026-08-10',
    watched: false,
    capabilities: {
      canUpdate: true,
      canDelete: true,
      canComment: true,
      canAttach: true,
      canWatch: true,
      canClone: false,
      allowedTransitions: ['draft'],
    },
    ...overrides,
  }
}

async function render(
  rows: readonly WorkItem[],
  groupBy: WorkItemsGroupBy = 'assignee_id',
  pagination: {
    hasMore: boolean
    loadingMore: boolean
    loaded: number
    total: number
    loadMore: () => void
  } = { hasMore: false, loadingMore: false, loaded: rows.length, total: rows.length, loadMore: vi.fn() },
) {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => root?.render(
    <Gantt
      rows={rows}
      groupBy={groupBy}
      users={{ status: 'ready', data: [{ id: 7, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' }] }}
      pagination={pagination}
      onOpen={vi.fn()}
      onUpdate={vi.fn().mockResolvedValue(true)}
    />,
  ))
  await act(async () => Promise.resolve())
  return container
}

describe('Work-items Gantt collection', () => {
  it('renders the domain empty state before loading the third-party surface', async () => {
    const view = await render([])

    expect(view.textContent).toContain('当前任务没有可用的计划日期')
    expect(view.querySelector('[data-testid="scheduler-stub"]')).toBeNull()
  })

  it('passes normalized owner groups and dates through the interactive scheduler boundary', async () => {
    const view = await render([item(), item({ id: 72, rowId: 'task:72', dueAt: undefined, startsAt: undefined })])
    const scheduler = view.querySelector('[data-testid="scheduler-stub"]')

    expect(scheduler).not.toBeNull()
    expect(scheduler?.getAttribute('data-groups')).toBe('1')
    expect(scheduler?.getAttribute('data-rows')).toBe('1')
    expect(view.textContent).toContain('拖动任务条调整计划周期')
    expect(view.querySelector('[data-testid="work-items-daypilot-gantt"]')).not.toBeNull()
  })

  it('passes the Router-owned grouping mode into the Gantt presenter', async () => {
    const view = await render([
      item({ id: 71, rowId: 'task:71', priority: 'high' }),
      item({ id: 72, rowId: 'task:72', priority: 'normal' }),
    ], 'priority')
    const scheduler = view.querySelector('[data-testid="scheduler-stub"]')

    expect(scheduler?.getAttribute('data-group-ids')).toBe('priority:high,priority:normal')
  })

  it('loads the next task page when the timeline scroll reaches the lower threshold', async () => {
    const loadMore = vi.fn()
    const view = await render([item()], 'assignee_id', {
      hasMore: true,
      loadingMore: false,
      loaded: 50,
      total: 120,
      loadMore,
    })
    const timeline = view.querySelector<HTMLElement>('[role="region"]')
    Object.defineProperties(timeline, {
      scrollHeight: { configurable: true, value: 1000 },
      scrollTop: { configurable: true, value: 700 },
      clientHeight: { configurable: true, value: 200 },
    })

    await act(async () => timeline?.dispatchEvent(new Event('scroll')))

    expect(loadMore).toHaveBeenCalledTimes(1)
    expect(view.textContent).toContain('已加载 50 / 120')
  })

  it('keeps a manual load action when the current page has no scheduled task', async () => {
    const loadMore = vi.fn()
    const view = await render([], 'assignee_id', {
      hasMore: true,
      loadingMore: false,
      loaded: 50,
      total: 120,
      loadMore,
    })

    const button = [...view.querySelectorAll('button')]
      .find((entry) => entry.textContent === WORK_ITEMS_COPY.gantt.loadMore)
    await act(async () => button?.click())
    expect(loadMore).toHaveBeenCalledTimes(1)
  })
})
