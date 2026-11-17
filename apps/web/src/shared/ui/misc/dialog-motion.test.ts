import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { usePanelMotion } from './dialog-motion'

describe('usePanelMotion', () => {
  it('keeps parent state open until close motion completes', () => {
    const onOpenChange = vi.fn()
    const { result } = renderHook(() => usePanelMotion(true, onOpenChange))

    act(() => result.current.handleOpenChange(false))
    expect(result.current.visible).toBe(false)
    expect(onOpenChange).not.toHaveBeenCalled()

    act(() => result.current.handleOpenChangeComplete(false))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('lets onCloseRequest veto before exit motion starts', () => {
    const onOpenChange = vi.fn()
    const onCloseRequest = vi.fn(() => false)
    const { result } = renderHook(() => usePanelMotion(true, onOpenChange, false, onCloseRequest))

    act(() => result.current.handleOpenChange(false))
    expect(onCloseRequest).toHaveBeenCalledTimes(1)
    expect(result.current.exiting).toBe(false)
    expect(onOpenChange).not.toHaveBeenCalled()

    act(() => result.current.handleOpenChangeComplete(false))
    expect(onOpenChange).not.toHaveBeenCalled()
  })
})
