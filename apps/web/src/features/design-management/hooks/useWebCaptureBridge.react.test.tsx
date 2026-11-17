import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '../copy'
import { useWebCaptureBridge } from './useWebCaptureBridge'

describe('useWebCaptureBridge', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('fails a capture closed when a stale extension content script never reaches its background', () => {
    vi.useFakeTimers()
    const onError = vi.fn()
    const { result } = renderHook(() => useWebCaptureBridge('', vi.fn(), onError))

    act(() => result.current.beginCaptureRequest('capture-1'))
    expect(result.current.evidence.extension).toBe('capturing')

    act(() => vi.advanceTimersByTime(3_000))

    expect(result.current.evidence.extension).toBe('disconnected')
    expect(onError).toHaveBeenCalledWith(copy.designManagement.newReview.extensionUnavailable)
  })

  it('offers the explicit target-tab fallback after iframe capture is unavailable', () => {
    const onError = vi.fn()
    const { result } = renderHook(() => useWebCaptureBridge('', vi.fn(), onError))

    act(() => result.current.beginCaptureRequest('capture-2'))
    act(() => window.dispatchEvent(new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: {
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_ERROR',
        request_id: 'capture-2',
        error: '页面禁止 iframe',
        iframe_fallback_available: true,
      },
    })))

    expect(result.current.evidence.extension).toBe('connected')
    expect(result.current.evidence.iframeFallbackAvailable).toBe(true)
    expect(onError).toHaveBeenCalledWith('页面禁止 iframe')
  })

  it('closes a capture when the extension never returns a final result', () => {
    vi.useFakeTimers()
    const onError = vi.fn()
    const { result } = renderHook(() => useWebCaptureBridge('', vi.fn(), onError))

    act(() => result.current.beginCaptureRequest('capture-timeout'))
    act(() => window.dispatchEvent(new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: {
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_STARTED',
        request_id: 'capture-timeout',
      },
    })))
    act(() => vi.advanceTimersByTime(30_000))

    expect(result.current.evidence.extension).toBe('disconnected')
    expect(onError).toHaveBeenCalledWith(copy.designManagement.newReview.extensionUnavailable)
  })

  it('turns an invalidated extension context into a refreshable connection error', () => {
    const onError = vi.fn()
    const { result } = renderHook(() => useWebCaptureBridge('', vi.fn(), onError))

    act(() => result.current.beginCaptureRequest('capture-invalidated'))
    act(() => window.dispatchEvent(new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: {
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_ERROR',
        request_id: 'capture-invalidated',
        error: 'Extension context invalidated.',
      },
    })))

    expect(result.current.evidence.extension).toBe('disconnected')
    expect(onError).toHaveBeenCalledWith(copy.designManagement.newReview.extensionUnavailable)
  })

  it('keeps the viewport that produced the captured evidence', () => {
    const { result } = renderHook(() => useWebCaptureBridge('', vi.fn(), vi.fn()))

    act(() => result.current.beginCaptureRequest('capture-viewport'))
    act(() => window.dispatchEvent(new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: {
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_RESULT',
        request_id: 'capture-viewport',
        screenshot: 'data:image/png;base64,YQ==',
        elements: [],
        viewport: { width: 390, height: 1060, devicePixelRatio: 2, pageZoom: 1 },
      },
    })))

    expect(result.current.evidence.captureViewport).toEqual({ width: 390, height: 1060, devicePixelRatio: 2 })
  })
})
