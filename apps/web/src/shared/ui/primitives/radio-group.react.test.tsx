import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { RadioGroup, RadioGroupItem } from './radio-group'

function Form() {
  const [value, setValue] = useState('first')
  return <form aria-label="Preferences">
    <RadioGroup name="mode" aria-label="Mode" value={value} onValueChange={setValue}>
      <label><RadioGroupItem value="first" />First</label>
      <label><RadioGroupItem value="unavailable" disabled />Unavailable</label>
      <label><RadioGroupItem value="last" />Last</label>
    </RadioGroup>
  </form>
}

describe('RadioGroup', () => {
  it('preserves native form values, label activation, and keyboard selection past disabled options', async () => {
    render(<Form />)
    const form = screen.getByRole('form', { name: 'Preferences' }) as HTMLFormElement
    const first = screen.getByRole('radio', { name: 'First' })
    const last = screen.getByRole('radio', { name: 'Last' })
    expect(new FormData(form).get('mode')).toBe('first')
    fireEvent.click(screen.getByText('Last'))
    expect(last.getAttribute('aria-checked')).toBe('true')
    expect(new FormData(form).get('mode')).toBe('last')
    await act(async () => { last.focus() })
    fireEvent.keyDown(last, { key: 'ArrowUp' })
    await waitFor(() => expect(first.getAttribute('aria-checked')).toBe('true'))
    expect(document.activeElement).toBe(first)
    expect(new FormData(form).get('mode')).toBe('first')
  })

  it('blocks changes for a disabled group', () => {
    const onValueChange = vi.fn()
    render(<RadioGroup aria-label="Mode" name="mode" value="first" onValueChange={onValueChange} disabled>
      <RadioGroupItem value="first" aria-label="First" />
      <RadioGroupItem value="last" aria-label="Last" />
    </RadioGroup>)
    expect(screen.getByRole('radiogroup', { name: 'Mode' })).not.toBeNull()
    const last = screen.getByRole('radio', { name: 'Last' })
    expect(last.hasAttribute('data-disabled')).toBe(true)
    fireEvent.click(last)
    expect(onValueChange).not.toHaveBeenCalled()
    expect(screen.getByRole('radio', { name: 'First' }).getAttribute('aria-checked')).toBe('true')
  })
})
