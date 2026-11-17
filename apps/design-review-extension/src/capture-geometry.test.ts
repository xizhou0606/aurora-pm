import { describe, expect, it } from 'bun:test'
import { captureRectIsVisible, captureRectMoved, captureWithStableRect, renderedCaptureScale } from './capture-geometry'

describe('iframe capture geometry', () => {
  it('accepts a stable uniformly scaled iframe inside the visible tab', () => {
    expect(renderedCaptureScale({ x: 20, y: 30, width: 720, height: 450 }, { width: 1440, height: 900 })).toBe(0.5)
    expect(captureRectIsVisible({ x: 20, y: 30, width: 720, height: 450 }, { width: 1200, height: 800 })).toBe(true)
    expect(captureRectIsVisible({ x: -0.5, y: 0, width: 1200.5, height: 800 }, { width: 1200, height: 800 })).toBe(true)
  })

  it('rejects non-uniform scaling and clipped iframe bounds', () => {
    expect(() => renderedCaptureScale({ x: 0, y: 0, width: 720, height: 600 }, { width: 1440, height: 900 })).toThrow('不是等比缩放')
    expect(captureRectIsVisible({ x: 20, y: 500, width: 720, height: 450 }, { width: 1200, height: 800 })).toBe(false)
    expect(captureRectIsVisible({ x: -1.5, y: 0, width: 1200, height: 800 }, { width: 1200, height: 800 })).toBe(false)
  })

  it('retries when layout moves during capture', async () => {
    const rects = [
      { x: 10, y: 10, width: 100, height: 100 },
      { x: 20, y: 10, width: 100, height: 100 },
      { x: 20, y: 10, width: 100, height: 100 },
      { x: 20, y: 10, width: 100, height: 100 },
    ]
    let captures = 0
    const result = await captureWithStableRect({
      settle: async () => {},
      measure: () => rects.shift()!,
      capture: async () => { captures += 1; return captures },
    })
    expect(captureRectMoved({ x: 10, y: 10, width: 1, height: 1 }, { x: 20, y: 10, width: 1, height: 1 })).toBe(true)
    expect(result).toEqual({ value: 2, rect: { x: 20, y: 10, width: 100, height: 100 } })
  })
})
