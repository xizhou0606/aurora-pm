import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS } from '../utils/requirement-pool-filter-fields'
import { RequirementPoolFilterSettings } from './RequirementPoolFilterSettings'

afterEach(cleanup)

it('opens requirement preferences and delegates reset to the existing control', async () => {
  const filterControls = {
    preference: { order: REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.map(({ id }) => id), hidden: [] },
    setVisible: vi.fn(),
    move: vi.fn(),
    moveTo: vi.fn(),
    reset: vi.fn(),
  }
  const text = copy.aiPM.requirementPool.filterSettings
  render(<RequirementPoolFilterSettings filterControls={filterControls} />)
  fireEvent.click(screen.getByRole('button', { name: text.trigger }))
  expect(await screen.findByRole('region', { name: text.controlsCard })).toBeTruthy()
  expect(screen.getAllByRole('checkbox').length).toBe(REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.length)
  fireEvent.click(screen.getByRole('button', { name: text.reset }))
  expect(filterControls.reset).toHaveBeenCalledTimes(1)
})
