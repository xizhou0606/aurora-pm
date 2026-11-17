import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy, WORK_ITEMS_COPY } from '@/shared/copy'
import type { TableColumnPreferenceControl } from '@/shared/hooks'
import {
  WORK_ITEMS_FILTER_CONTROL_OPTIONS,
  WORK_ITEMS_VIEW_CONTROL_OPTIONS,
  type WorkItemsFilterControlId,
  type WorkItemsViewControlId,
} from '../../model/control-preferences'
import { FilterSettings } from './FilterSettings'

afterEach(cleanup)

function control<TId extends string>(order: TId[]): TableColumnPreferenceControl<TId> {
  return {
    preference: { order, hidden: [] },
    setVisible: vi.fn(),
    move: vi.fn(),
    moveTo: vi.fn(),
    reset: vi.fn(),
  }
}

describe('Work-items filter settings', () => {
  it('opens two independently configurable cards backed by the shared preference list', async () => {
    const viewControls = control<WorkItemsViewControlId>(
      WORK_ITEMS_VIEW_CONTROL_OPTIONS.map(({ id }) => id),
    )
    const filterControls = control<WorkItemsFilterControlId>(
      WORK_ITEMS_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
    )
    render(<FilterSettings viewControls={viewControls} filterControls={filterControls} />)

    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.filterSettings.trigger }))

    expect(await screen.findByRole('heading', { name: WORK_ITEMS_COPY.filterSettings.viewCard })).toBeTruthy()
    expect(screen.getByRole('heading', { name: WORK_ITEMS_COPY.filterSettings.controlsCard })).toBeTruthy()
    expect(screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(WORK_ITEMS_COPY.filterSettings.layoutControl),
    })).toBeTruthy()
    expect(screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(WORK_ITEMS_COPY.filter.overdue),
    })).toBeTruthy()
    const fixedSearchToggle = screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(WORK_ITEMS_COPY.filter.searchLabel),
    })
    expect(
      fixedSearchToggle.hasAttribute('disabled')
      || fixedSearchToggle.hasAttribute('data-disabled')
      || fixedSearchToggle.getAttribute('aria-disabled') === 'true',
    ).toBe(true)
    expect((screen.getByRole('button', {
      name: copy.ui.tableColumns.drag(WORK_ITEMS_COPY.filter.searchLabel),
    }) as HTMLButtonElement).disabled).toBe(true)

    screen.getAllByRole('button', { name: WORK_ITEMS_COPY.filterSettings.reset })[0]?.click()
    expect(viewControls.reset).toHaveBeenCalledTimes(1)
    expect(filterControls.reset).not.toHaveBeenCalled()
  })
})
