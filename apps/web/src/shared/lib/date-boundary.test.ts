import { describe, expect, it } from 'bun:test'

import {
  dateInputValue,
  endOfLocalDay,
  formatDateOnly,
  startOfLocalDay,
} from './date-boundary'

describe('date-only UI boundary', () => {
  it('keeps controls date-only and maps writes to local day boundaries', () => {
    expect(dateInputValue('2026-08-06')).toBe('2026-08-06')
    expect(new Date(startOfLocalDay('2026-08-06')).getHours()).toBe(0)
    expect(new Date(startOfLocalDay('2026-08-06')).getMinutes()).toBe(0)
    expect(new Date(endOfLocalDay('2026-08-06')).getHours()).toBe(23)
    expect(new Date(endOfLocalDay('2026-08-06')).getMinutes()).toBe(59)
    expect(new Date(endOfLocalDay('2026-08-06')).getSeconds()).toBe(59)
    expect(formatDateOnly('2026-08-06')).toBe('2026/08/06')
  })

  it('preserves an existing timestamp at the API boundary', () => {
    const timestamp = '2026-08-06T08:30:00Z'
    expect(startOfLocalDay(timestamp)).toBe(timestamp)
    expect(endOfLocalDay(timestamp)).toBe(timestamp)
  })
})
