import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { Checkbox } from './checkbox'

describe('Checkbox disabled state', () => {
  it('exposes the Base UI disabled state on its nonnative control and blocks activation', () => {
    const onCheckedChange = vi.fn()
    render(<Checkbox aria-label="Select row" disabled onCheckedChange={onCheckedChange} />)
    const control = screen.getByRole('checkbox', { name: 'Select row' })
    expect(control.tagName).toBe('SPAN')
    expect(control.hasAttribute('data-disabled')).toBe(true)
    expect(control.className).toContain('data-disabled:cursor-not-allowed')
    fireEvent.click(control)
    expect(onCheckedChange).not.toHaveBeenCalled()
  })
})

it('exposes mixed selection and emits the checked intent on activation', () => {
  const onCheckedChange = vi.fn()
  const { rerender } = render(<Checkbox aria-label="Select group" indeterminate onCheckedChange={onCheckedChange} />)
  const control = screen.getByRole('checkbox', { name: 'Select group' })
  expect(control.getAttribute('aria-checked')).toBe('mixed')
  expect(control.hasAttribute('data-indeterminate')).toBe(true)
  fireEvent.click(control)
  expect(onCheckedChange).toHaveBeenCalledTimes(1)
  expect(onCheckedChange.mock.calls[0][0]).toBe(true)
  rerender(<Checkbox aria-label="Select group" checked onCheckedChange={onCheckedChange} />)
  expect(control.getAttribute('aria-checked')).toBe('true')
  expect(control.hasAttribute('data-indeterminate')).toBe(false)
})
