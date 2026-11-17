import { describe, expect, it } from 'bun:test'

import { PM_API_ERROR_CODES } from './errorCodes'

describe('PM API error-code registry', () => {
  it('contains unique machine-readable wire values', () => {
    const values = Object.values(PM_API_ERROR_CODES)

    expect(new Set(values).size).toBe(values.length)
    expect(values.every((code) => /^[A-Z][A-Z0-9_]*$/.test(code))).toBe(true)
  })

  it('pins workspace and session contracts used by global routing', () => {
    expect(PM_API_ERROR_CODES.WORKSPACE_FORBIDDEN).toBe('WORKSPACE_FORBIDDEN')
    expect(PM_API_ERROR_CODES.SESSION_EXPIRED).toBe('SESSION_EXPIRED')
  })
})
