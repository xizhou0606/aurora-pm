import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'bun:test'
import { WorkItemsMultiSelect, WorkItemsSelect } from './SelectField'
import { WORK_ITEMS_COPY } from '@/shared/copy'

it('adapts task empty strings to placeholders and clears back to the task model', () => {
  const onValueChange = vi.fn()
  const props = { id: 'task-status', label: 'Status', options: [{ value: 'ready', label: 'Ready' }], clearable: true, onValueChange }
  const { rerender } = render(<WorkItemsSelect {...props} value="" placeholder="Choose" />)
  expect(screen.getByRole('combobox').textContent).toContain('Choose')
  rerender(<WorkItemsSelect {...props} value="ready" />)
  fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.clearSelection }))
  expect(onValueChange).toHaveBeenCalledWith('')
})

it('adapts the task multiselect clear copy without owning the selected state', () => {
  const onValueChange = vi.fn()
  render(<WorkItemsMultiSelect id="task-multi" label="Statuses" options={[{ value: 'ready', label: 'Ready' }]} value={['ready']} clearable onValueChange={onValueChange} />)
  fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.filter.clearSelection }))
  expect(onValueChange).toHaveBeenCalledWith([])
  expect(screen.getByRole('combobox').textContent).toContain('Ready')
})
