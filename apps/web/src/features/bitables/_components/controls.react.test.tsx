import { createRef } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BitableInput, BitableTextarea } from './controls'

describe('compact bitable text controls', () => {
  it('removes the outer focus ring while retaining input semantics and refs', () => {
    const ref = createRef<HTMLInputElement>(), onChange = vi.fn()
    render(<><BitableInput ref={ref} aria-label="search" onChange={onChange} /><BitableTextarea aria-label="comment" /></>)
    for (const input of screen.getAllByRole('textbox')) {
      expect(input.className).toContain('focus-visible:ring-0')
      expect(input.className).toContain('focus-visible:ring-offset-0')
      expect(input.className).not.toContain('focus-visible:ring-3')
    }
    fireEvent.change(screen.getByRole('textbox', { name: 'search' }), { target: { value: 'test' } })
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(ref.current?.value).toBe('test')
  })
})
