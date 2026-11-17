import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'
import { useBitableFieldChoices } from './useBitableFieldChoices'
describe('field choice draft identity', () => {
  it('keeps IDs and color while reordering and removes deleted default references', () => {
    const { result } = renderHook(() => useBitableFieldChoices(false))
    const [a, b] = result.current.choices
    act(() => { result.current.update(a.id, { name: 'A', color: 'pink' }); result.current.update(b.id, { name: 'B' }); result.current.select(a.id) })
    expect(result.current.valid).toBe(true)
    act(() => result.current.move(a.id, 1))
    expect(result.current.normalized.map(choice => choice.id)).toEqual([b.id, a.id])
    expect(result.current.options.defaultValue).toBe(a.id)
    expect(result.current.normalized[1].color).toBe('pink')
    act(() => result.current.remove(a.id))
    expect(result.current.options.defaultValue).toBeUndefined()
  })
  it('supports multiple defaults, blank and duplicate validation, and type transitions', () => {
    const { result, rerender } = renderHook(({ multiple }) => useBitableFieldChoices(multiple), { initialProps: { multiple: true } })
    const [a, b] = result.current.choices
    expect(result.current.valid).toBe(false)
    act(() => { result.current.update(a.id, { name: 'Same' }); result.current.update(b.id, { name: 'Same' }) })
    expect(result.current.valid).toBe(false)
    act(() => { result.current.update(b.id, { name: 'Other' }); result.current.select(a.id); result.current.select(b.id) })
    expect(result.current.options.defaultValue).toEqual([a.id, b.id])
    rerender({ multiple: false })
    expect(result.current.options.defaultValue).toBe(a.id)
    act(() => result.current.clear())
    expect(result.current.selected).toEqual([])
  })
})
