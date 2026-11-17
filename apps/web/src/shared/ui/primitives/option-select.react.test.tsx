import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { OptionMultiSelect, OptionSelect } from './option-select'

const options = [{ value: 'a', label: 'Alpha' }, { value: 'b', label: 'Beta', disabled: true }, { value: '__quality_select_empty__', label: 'Real value' }]

describe('OptionSelect', () => {
  it('uses null for an explicit empty option and reserves no string sentinel', async () => {
    const onValueChange = vi.fn()
    const props = { id: 'single', label: 'Status', options, emptyOption: { label: 'All' }, onValueChange }
    const { rerender } = render(<OptionSelect {...props} value={null} />)
    expect(screen.getByRole('combobox').textContent).toContain('All')
    rerender(<OptionSelect {...props} value="__quality_select_empty__" clearLabel="Clear" />)
    expect(screen.getByRole('combobox').textContent).toContain('Real value')
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(onValueChange).toHaveBeenLastCalledWith(null)
    onValueChange.mockClear()
    fireEvent.click(screen.getByRole('combobox'))
    const empty = await screen.findByRole('option', { name: 'All' })
    fireEvent.pointerDown(empty, { button: 0, pointerType: 'mouse' })
    fireEvent.click(empty)
    expect(onValueChange).toHaveBeenLastCalledWith(null)
  })
  it('keeps placeholder and accessibility, and disables unavailable options', async () => {
    const onValueChange = vi.fn()
    render(<OptionSelect id="empty" label="Status" value={null} options={options} emptyOption={{ label: 'All' }} placeholder="Choose" describedBy="help" invalid onValueChange={onValueChange} />)
    const trigger = screen.getByRole('combobox')
    expect(trigger.textContent).toContain('Choose')
    expect(trigger.getAttribute('aria-describedby')).toBe('help')
    expect(trigger.getAttribute('aria-invalid')).toBe('true')
    fireEvent.click(trigger)
    const option = await screen.findByRole('option', { name: 'Beta' })
    expect(option.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(option)
    expect(onValueChange).not.toHaveBeenCalled()

  })
  it('keeps controlled multiselect summaries and clear behavior across rerenders', () => {
    const onValueChange = vi.fn()
    const props = { id: 'multi', label: 'Statuses', options, placeholder: 'Choose', clearLabel: 'Clear', onValueChange }
    const { rerender } = render(<OptionMultiSelect {...props} value={['a']} />)
    expect(screen.getByRole('combobox').textContent).toContain('Alpha')
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(onValueChange).toHaveBeenCalledWith([])
    rerender(<OptionMultiSelect {...props} value={[]} />)
    expect(screen.getByRole('combobox').textContent).toContain('Choose')
    expect(screen.queryByRole('button', { name: 'Clear' })).toBeNull()
    rerender(<OptionMultiSelect {...props} value={['a']} disabled />)
    expect(screen.getByRole('combobox').hasAttribute('disabled')).toBe(true)
    expect(screen.queryByRole('button', { name: 'Clear' })).toBeNull()
  })
})
