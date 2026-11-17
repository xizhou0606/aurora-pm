import { describe, expect, it } from 'bun:test'
import { MAX_ELEMENTS, STYLE_PROPERTIES, validCaptureURL, validViewport } from './capture-policy'

describe('capture policy', () => {
  it('only accepts bounded HTTP targets and viewports', () => {
    expect(validCaptureURL('https://pm.example.test/page')).toBe(true)
    expect(validCaptureURL('javascript:alert(1)')).toBe(false)
    expect(validViewport({ width: 1440, height: 900, devicePixelRatio: 2 })).toBe(true)
    expect(validViewport({ width: 20, height: 900, devicePixelRatio: 2 })).toBe(false)
  })

  it('keeps the structural evidence whitelist bounded', () => {
    expect(MAX_ELEMENTS).toBe(1500)
    expect(STYLE_PROPERTIES).not.toContain('content')
    expect(STYLE_PROPERTIES.length).toBeLessThan(50)
  })
})
