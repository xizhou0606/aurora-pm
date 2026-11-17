import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { ViewTabs } from './view-tabs'

describe('ViewTabs', () => {
  it('keeps the default presentation on one line', () => {
    render(
      <ViewTabs
        value="list"
        label="展示方式"
        tabs={[
          { id: 'list', label: '列表' },
          { id: 'board', label: '看板' },
        ]}
        onValueChange={() => undefined}
      />,
    )

    expect(screen.getByRole('tab', { name: '列表' }).textContent).toBe('列表')
    expect(screen.queryByText('12 项')).toBeNull()
    const indicator = document.querySelector('[data-slot="view-tabs-indicator"]')
    expect(indicator).not.toBeNull()
    expect(indicator?.className).toContain('dark:bg-card')
    expect(screen.getByRole('tab', { name: '列表' }).className).toContain('dark:data-active:text-white')
  })

  it('renders optional supporting information in the two-line presentation', () => {
    const onValueChange = vi.fn()
    render(
      <ViewTabs
        value="todo"
        label="状态视图"
        lines={2}
        tabs={[
          { id: 'todo', label: '待处理', secondaryLabel: '12 项' },
          { id: 'done', label: '已关闭', secondaryLabel: '8 项' },
        ]}
        onValueChange={onValueChange}
      />,
    )

    const done = screen.getByRole('tab', { name: '已关闭' })
    expect(screen.getByText('12 项')).toBeTruthy()
    expect(screen.getByText('8 项')).toBeTruthy()
    fireEvent.click(done)
    expect(onValueChange.mock.calls[0]?.[0]).toBe('done')
  })

  it('keeps two-line tabs aligned when supporting information is absent', () => {
    render(
      <ViewTabs
        value="list"
        label="展示方式"
        lines={2}
        tabs={[
          { id: 'list', label: '列表' },
          { id: 'board', label: '看板' },
        ]}
        onValueChange={() => undefined}
      />,
    )

    expect(document.querySelectorAll('[role="tab"] [aria-hidden="true"]')).toHaveLength(2)
  })
})
