import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { GridColumnHeader } from './grid-column-header'

function setup() {
  const onWidthChange = vi.fn()
  render(<table><thead><tr><GridColumnHeader field={{ id: 'title', name: 'Title', type: 'text', readOnly: false }} width={180} disabled={false} onWidthChange={onWidthChange} /></tr></thead></table>)
  const handle = screen.getByRole('separator')
  // jsdom has pointer events but does not implement native pointer capture.
  handle.setPointerCapture = vi.fn()
  handle.hasPointerCapture = vi.fn().mockReturnValue(true)
  handle.releasePointerCapture = vi.fn()
  return { handle, onWidthChange }
}

describe('integer column resize contract', () => {
  it.each([[391.09375, 471], [391.75, 472], [-500.5, 80], [1000.5, 640]])('normalizes fractional pointer coordinates %s to %s pixels', (clientX, expected) => {
    const { handle, onWidthChange } = setup()
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 100 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX })
    expect(handle.getAttribute('aria-valuenow')).toBe(String(expected))
    expect(onWidthChange).not.toHaveBeenCalled()
    fireEvent.pointerUp(handle, { pointerId: 1, clientX })
    expect(onWidthChange).toHaveBeenCalledTimes(1)
    expect(onWidthChange).toHaveBeenCalledWith('title', expected)
  })
  it('does not submit a cancelled drag or a subpixel movement that rounds to the original width', () => {
    const { handle, onWidthChange } = setup()
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 100 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 100.1 })
    fireEvent.pointerUp(handle, { pointerId: 1 })
    fireEvent.pointerDown(handle, { pointerId: 2, clientX: 100 })
    fireEvent.pointerMove(handle, { pointerId: 2, clientX: 391.09375 })
    fireEvent.pointerCancel(handle, { pointerId: 2 })
    expect(onWidthChange).not.toHaveBeenCalled()
  })
  it('preserves keyboard resize increments', () => {
    const { handle, onWidthChange } = setup()
    fireEvent.keyDown(handle, { key: 'ArrowRight' })
    expect(onWidthChange).toHaveBeenLastCalledWith('title', 196)
    fireEvent.keyDown(handle, { key: 'ArrowLeft' })
    expect(onWidthChange).toHaveBeenLastCalledWith('title', 164)
  })
})
