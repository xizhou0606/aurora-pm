import { createRef } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { Label } from './label'
import { Textarea } from './textarea'

describe('Textarea', () => {
  it('preserves native labeling, ref, invalid, and change behavior', () => {
    const ref = createRef<HTMLTextAreaElement>()
    const onChange = vi.fn()

    render(
      <div>
        <Label htmlFor="notes">Notes</Label>
        <Textarea
          ref={ref}
          id="notes"
          aria-invalid="true"
          defaultValue="Initial notes"
          onChange={onChange}
        />
      </div>,
    )

    const textarea = screen.getByRole('textbox', { name: 'Notes' }) as HTMLTextAreaElement
    expect(textarea.getAttribute('data-slot')).toBe('textarea')
    expect(textarea.getAttribute('aria-invalid')).toBe('true')
    expect(textarea.value).toBe('Initial notes')
    expect(ref.current).toBe(textarea)

    fireEvent.change(textarea, { target: { value: 'Updated notes' } })
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(textarea.value).toBe('Updated notes')
  })
})
