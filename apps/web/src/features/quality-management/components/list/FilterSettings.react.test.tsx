import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy, QUALITY_COPY } from '@/shared/copy'
import type { TableColumnPreferenceControl } from '@/shared/hooks'
import {
  QUALITY_FILTER_CONTROL_OPTIONS,
  QUALITY_STATUS_TAB_OPTIONS,
  type QualityFilterControlId,
  type QualityStatusTabId,
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

describe('Quality filter settings', () => {
  it('opens two independently configurable cards backed by the shared preference list', async () => {
    const statusTabs = control<QualityStatusTabId>(
      QUALITY_STATUS_TAB_OPTIONS.map(({ id }) => id),
    )
    const filterControls = control<QualityFilterControlId>(
      QUALITY_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
    )
    render(<FilterSettings statusTabs={statusTabs} filterControls={filterControls} />)

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.filterSettings.trigger }))

    expect(await screen.findByRole('heading', { name: QUALITY_COPY.filterSettings.statusCard })).toBeTruthy()
    expect(screen.getByRole('heading', { name: QUALITY_COPY.filterSettings.controlsCard })).toBeTruthy()
    const requiredTabToggle = screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(QUALITY_COPY.tabs.all),
    })
    expect(requiredTabToggle.hasAttribute('disabled') || requiredTabToggle.getAttribute('aria-disabled') === 'true').toBe(true)
    fireEvent.click(screen.getByRole('checkbox', { name: copy.ui.tableColumns.toggle(QUALITY_COPY.filters.search) }))
    expect(filterControls.setVisible).toHaveBeenCalledWith('search', false)

    screen.getAllByRole('button', { name: QUALITY_COPY.filterSettings.reset })[0]?.click()
    expect(statusTabs.reset).toHaveBeenCalledTimes(1)
    expect(filterControls.reset).not.toHaveBeenCalled()
  })
})
