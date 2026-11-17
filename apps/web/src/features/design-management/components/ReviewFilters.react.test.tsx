import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { copy } from '../copy'
import { ReviewFilters } from './ReviewFilters'

const text = copy.designManagement
const requirements = [{ id: 23, title: '设计需求', project_set_id: 17, project_set_name: 'PM', plan_unit_id: 19, plan_unit_title: 'V1' }]

describe('design review filter labels', () => {
  it('shows labels before opening any dropdown and refreshes asynchronous scope labels', async () => {
    const props = { value: { project_set_id: 17, plan_unit_id: 19, requirement_id: 23, status: 'completed' as const }, requirements: [], onChange: vi.fn(), onReset: vi.fn() }
    const { rerender } = render(<ReviewFilters {...props} />)
    expect(screen.getByRole('combobox', { name: text.filters.status }).textContent).toContain(text.status.completed)
    expect(screen.getByRole('combobox', { name: text.filters.conclusion }).textContent).toContain(text.common.all)
    rerender(<ReviewFilters {...props} requirements={requirements} />)
    await waitFor(() => expect(screen.getByRole('combobox', { name: text.filters.projectSet }).textContent).toContain('PM'))
    expect(screen.getByRole('combobox', { name: text.filters.planUnit }).textContent).toContain('V1')
    expect(screen.getByRole('combobox', { name: text.filters.requirement }).textContent).toContain('设计需求')
  })
  it('keeps numeric IDs in filter changes while displaying names', async () => {
    const onChange = vi.fn()
    render(<ReviewFilters value={{}} requirements={requirements} onChange={onChange} onReset={vi.fn()} />)
    const trigger = screen.getByRole('combobox', { name: text.filters.projectSet })
    expect(trigger.textContent).toContain(text.common.all)
    fireEvent.click(trigger)
    const option = await screen.findByRole('option', { name: 'PM' })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    expect(onChange).toHaveBeenCalledWith('project_set_id', 17)
  })
})
