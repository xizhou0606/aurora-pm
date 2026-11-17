import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'
import { useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemsRouteState } from '../../model/types'
import { Toolbar } from './Toolbar'

afterEach(() => {
  vi.useRealTimers()
  cleanup()
})

it('round-trips controlled mine and overdue filters exactly once for checkbox and label clicks', () => {
  const onChange = vi.fn()
  function ControlledToolbar() {
    const [currentRoute, setCurrentRoute] = useState(route)
    return <Toolbar route={currentRoute} currentUserId={9} statusOptions={[]} searchDraft=""
      users={{ status: 'ready', data: [{ id: 9, label: 'Teo' }] }}
      onSearchChange={vi.fn()} onViewChange={vi.fn()} onReset={vi.fn()}
      onRouteChange={(patch) => {
        onChange(patch)
        setCurrentRoute((current) => ({ ...current, ...patch }))
      }} />
  }
  render(<ControlledToolbar />)
  for (const [name, onPatch, offPatch] of [
    [WORK_ITEMS_COPY.filter.mine, { assigneeIds: [9] }, { assigneeIds: [] }],
    [WORK_ITEMS_COPY.filter.overdue, { overdue: true }, { overdue: false }],
  ] as const) {
    const checkbox = screen.getByRole('checkbox', { name })
    expect(checkbox.getAttribute('aria-checked')).toBe('false')
    for (const target of [checkbox, screen.getByText(name)]) {
      onChange.mockClear()
      fireEvent.click(target)
      expect(checkbox.getAttribute('aria-checked')).toBe('true')
      expect(onChange).toHaveBeenCalledTimes(1)
      expect(onChange).toHaveBeenCalledWith(onPatch)
      onChange.mockClear()
      fireEvent.click(target)
      expect(checkbox.getAttribute('aria-checked')).toBe('false')
      expect(onChange).toHaveBeenCalledTimes(1)
      expect(onChange).toHaveBeenCalledWith(offPatch)
    }
  }
})

const route: WorkItemsRouteState = {
  scope: { projectSetIds: [], projectIds: [], requirementIds: [] },
  view: 'list',
  layout: 'table',
  search: '',
  overdue: false,
  assigneeIds: [],
  statuses: [],
  priorities: [],
  taskClientIds: [],
  groupBy: 'none',
  sort: 'created_at_desc',
  page: 1,
  pageSize: 50,
  mode: 'view',
  invalidLegacyScope: false,
  needsRouteNormalization: false,
}

describe('Work-items filter placeholders', () => {
  it('keeps keystrokes local and commits only the latest value after an input pause', () => {
    vi.useFakeTimers()
    const onSearchChange = vi.fn()
    render(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[]}
        searchDraft=""
        users={{ status: 'ready', data: [] }}
        onSearchChange={onSearchChange}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )
    const input = screen.getByRole('searchbox', {
      name: WORK_ITEMS_COPY.filter.searchLabel,
    }) as HTMLInputElement

    fireEvent.change(input, { target: { value: '登' } })
    expect(input.value).toBe('登')
    act(() => vi.advanceTimersByTime(300))
    fireEvent.change(input, { target: { value: '登录' } })
    act(() => vi.advanceTimersByTime(499))
    expect(onSearchChange).not.toHaveBeenCalled()

    act(() => vi.advanceTimersByTime(1))
    expect(onSearchChange).toHaveBeenCalledTimes(1)
    expect(onSearchChange).toHaveBeenCalledWith('登录')
  })

  it('does not submit an unfinished input-method composition', () => {
    vi.useFakeTimers()
    const onSearchChange = vi.fn()
    render(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[]}
        searchDraft=""
        users={{ status: 'ready', data: [] }}
        onSearchChange={onSearchChange}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )
    const input = screen.getByRole('searchbox', {
      name: WORK_ITEMS_COPY.filter.searchLabel,
    })

    fireEvent.compositionStart(input)
    fireEvent.change(input, { target: { value: 'ren' } })
    act(() => vi.advanceTimersByTime(1_000))
    expect(onSearchChange).not.toHaveBeenCalled()

    fireEvent.compositionEnd(input, { currentTarget: { value: '人' } })
    fireEvent.change(input, { target: { value: '人' } })
    act(() => vi.advanceTimersByTime(500))
    expect(onSearchChange).toHaveBeenCalledWith('人')
  })

  it('keeps empty status and priority filters visibly named', () => {
    render(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )

    expect(screen.getByRole('combobox', { name: WORK_ITEMS_COPY.filter.status }).textContent)
      .toContain(WORK_ITEMS_COPY.filter.statusPlaceholder)
    expect(screen.getByRole('combobox', { name: WORK_ITEMS_COPY.filter.priority }).textContent)
      .toContain(WORK_ITEMS_COPY.filter.priorityPlaceholder)
    const assignee = screen.getByRole('combobox', { name: WORK_ITEMS_COPY.filter.assignee })
    expect((assignee as HTMLInputElement).placeholder).toBe(WORK_ITEMS_COPY.filter.noAssignees)
    expect((assignee as HTMLInputElement).disabled).toBe(true)
    for (const id of [
      'work-items-search',
      'work-items-status',
      'work-items-priority',
      'work-items-assignee',
      'work-items-group',
    ]) {
      expect(document.querySelector(`label[for="${id}"]`)?.classList.contains('sr-only')).toBe(true)
    }
    expect(screen.getByRole('searchbox', { name: WORK_ITEMS_COPY.filter.searchLabel })).toBeTruthy()
    expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.filter.reset })).toBeNull()
  })

  it('shows reset only while a filter is active', () => {
    const onReset = vi.fn()
    render(
      <Toolbar
        route={{ ...route, overdue: true }}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={onReset}
      />,
    )

    const reset = screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.reset })
    const overdue = screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.overdue })
    expect(reset.classList.contains('h-7')).toBe(true)
    expect(reset.closest('[data-slot="filter-bar"]')?.classList.contains('sm:items-stretch')).toBe(true)
    expect(reset.closest('[data-slot="adaptive-filter-bar"]'))
      .toBe(overdue.closest('[data-slot="adaptive-filter-bar"]'))
    reset.click()
    expect(onReset).toHaveBeenCalledTimes(1)
  })

  it('filters by the current user through the canonical assignee filter', () => {
    const onRouteChange = vi.fn()
    const props = {
      currentUserId: 9,
      statusOptions: [{ value: 'draft', label: '草稿' }],
      searchDraft: '',
      users: { status: 'ready' as const, data: [{ id: 9, label: 'Teo' }] },
      onSearchChange: vi.fn(),
      onViewChange: vi.fn(),
      onRouteChange,
      onReset: vi.fn(),
    }
    const { rerender } = render(<Toolbar {...props} route={route} />)

    const mine = screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.mine })
    expect(mine.getAttribute('aria-checked')).toBe('false')
    mine.click()
    expect(onRouteChange).toHaveBeenLastCalledWith({ assigneeIds: [9] })

    rerender(<Toolbar {...props} route={{ ...route, assigneeIds: [9] }} />)
    expect(mine.getAttribute('aria-checked')).toBe('true')
    mine.click()
    expect(onRouteChange).toHaveBeenLastCalledWith({ assigneeIds: [] })
    expect(WORK_ITEMS_COPY.filter.reset).toBe('重置')
    expect(screen.getByRole('searchbox', { name: WORK_ITEMS_COPY.filter.searchLabel })
      .closest('[data-slot="adaptive-filter-bar"]')).toBeTruthy()
  })

  it('exposes board grouping while status lanes remain inside every group', () => {
    render(
      <Toolbar
        route={{ ...route, view: 'board', groupBy: 'requirement_id' }}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )

    const groupBy = screen.getByRole('combobox', { name: WORK_ITEMS_COPY.filter.groupBy })
    expect(groupBy.textContent).toContain(WORK_ITEMS_COPY.filter.requirement)
    expect(groupBy.className).not.toContain('pr-14')
    expect(groupBy.querySelector('[data-slot="select-value"]')?.className).toContain('pr-7')
    expect(screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.reset })).toBeTruthy()
  })

  it('keeps loading and failed assignee states inside a stable disabled control', () => {
    const retry = vi.fn()
    const { rerender } = render(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'loading' }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )

    const loadingAssignee = screen.getByRole('combobox', {
      name: WORK_ITEMS_COPY.filter.assignee,
    }) as HTMLInputElement
    expect(loadingAssignee.placeholder).toBe(WORK_ITEMS_COPY.filter.loadingAssignees)
    expect(loadingAssignee.disabled).toBe(true)

    rerender(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [{ id: 9, label: 'Teo' }] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )

    const readyAssignee = screen.getByRole('combobox', {
      name: WORK_ITEMS_COPY.filter.assignee,
    }) as HTMLInputElement
    expect(readyAssignee).toBe(loadingAssignee)
    expect(readyAssignee.disabled).toBe(false)

    rerender(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'error', retry }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )

    const failedAssignee = screen.getByRole('combobox', {
      name: WORK_ITEMS_COPY.filter.assignee,
    }) as HTMLInputElement
    expect(failedAssignee).toBe(loadingAssignee)
    expect(failedAssignee.placeholder).toBe(WORK_ITEMS_COPY.filter.noAssignees)
    screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.retryAssignees }).click()
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('applies locally configured visibility and order to view and filter controls', () => {
    render(
      <Toolbar
        route={route}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [{ id: 9, label: 'Teo' }] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
        viewControlPreference={{ order: ['layout', 'view'], hidden: ['layout'] }}
        filterControlPreference={{
          order: ['priority', 'search', 'status', 'assignee', 'group', 'overdue', 'mine'],
          hidden: ['assignee', 'mine'],
        }}
      />,
    )

    expect(screen.queryByRole('tablist', { name: WORK_ITEMS_COPY.views.layoutLabel })).toBeNull()
    expect(screen.getByRole('tablist', { name: WORK_ITEMS_COPY.views.label })).toBeTruthy()
    expect(screen.queryByRole('combobox', { name: WORK_ITEMS_COPY.filter.assignee })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: WORK_ITEMS_COPY.filter.mine })).toBeNull()
    expect(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.overdue })).toBeTruthy()

    const priority = screen.getByRole('combobox', { name: WORK_ITEMS_COPY.filter.priority })
    const search = screen.getByRole('searchbox', { name: WORK_ITEMS_COPY.filter.searchLabel })
    expect(priority.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('keeps configured controls and settings available in the expanded layout', () => {
    render(
      <Toolbar
        route={{ ...route, overdue: true }}
        currentUserId={9}
        statusOptions={[{ value: 'draft', label: '草稿' }]}
        searchDraft=""
        users={{ status: 'ready', data: [{ id: 9, label: 'Teo' }] }}
        onSearchChange={vi.fn()}
        onViewChange={vi.fn()}
        onRouteChange={vi.fn()}
        onReset={vi.fn()}
        filterSettings={<button type="button">{WORK_ITEMS_COPY.filterSettings.trigger}</button>}
        columnSettings={<button type="button">配置表字段</button>}
      />,
    )

    const search = screen.getByRole('searchbox', { name: WORK_ITEMS_COPY.filter.searchLabel })
    const controls = search.closest('[data-slot="filter-field-grid"]')
    expect(controls?.querySelectorAll(':scope > div')).toHaveLength(6)
    expect(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.mine }).closest('[data-slot="filter-bar-actions"]')).toBeTruthy()
    expect(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.overdue }).closest('[data-slot="filter-bar-actions"]')).toBeTruthy()
    expect(screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.reset })
      .closest('[data-slot="filter-bar-actions"]')).toBeTruthy()

    const viewTabs = screen.getByRole('tablist', { name: WORK_ITEMS_COPY.views.label })
    const layoutTabs = screen.getByRole('tablist', { name: WORK_ITEMS_COPY.views.layoutLabel })
    const settings = screen.getByRole('button', { name: WORK_ITEMS_COPY.filterSettings.trigger })
    expect(viewTabs.parentElement).not.toBe(layoutTabs.parentElement)
    expect(settings.compareDocumentPosition(layoutTabs) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})


it('keeps mine disabled without an authoritative user and emits overdue boolean changes', () => {
  const onRouteChange = vi.fn()
  const props = {
    currentUserId: 0, statusOptions: [], searchDraft: '',
    users: { status: 'ready' as const, data: [] },
    onSearchChange: vi.fn(), onViewChange: vi.fn(), onRouteChange, onReset: vi.fn(),
  }
  const { rerender } = render(<Toolbar {...props} route={route} />)
  const mine = screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.mine })
  expect(mine.hasAttribute('disabled') || mine.getAttribute('aria-disabled') === 'true').toBe(true)
  fireEvent.click(mine)
  expect(onRouteChange).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.overdue }))
  expect(onRouteChange).toHaveBeenLastCalledWith({ overdue: true })
  rerender(<Toolbar {...props} route={{ ...route, overdue: true }} />)
  fireEvent.click(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.filter.overdue }))
  expect(onRouteChange).toHaveBeenLastCalledWith({ overdue: false })
})
