import { describe, expect, it } from 'bun:test'

import { dailyQuote, dailyQuoteIndex } from './dailyQuote'

describe('Inbox daily quote rotation', () => {
  it('uses every quote exactly once across any complete 66-day cycle for a user', () => {
    const start = new Date(2026, 0, 1, 12)
    const indexes = Array.from({ length: 66 }, (_, offset) => {
      const date = new Date(start)
      date.setDate(start.getDate() + offset)
      return dailyQuoteIndex(date, 47)
    })

    expect(new Set(indexes).size).toBe(66)
    expect(indexes.every((index) => index >= 0 && index < 66)).toBe(true)
  })

  it('keeps the quote stable during a local day and starts a new cycle after 66 days', () => {
    const morning = new Date(2026, 7, 6, 0, 1)
    const evening = new Date(2026, 7, 6, 23, 59)
    const nextCycle = new Date(2026, 7, 6, 12)
    nextCycle.setDate(nextCycle.getDate() + 66)

    expect(dailyQuoteIndex(morning, 47)).toBe(dailyQuoteIndex(evening, 47))
    expect(dailyQuoteIndex(nextCycle, 47)).toBe(dailyQuoteIndex(morning, 47))
  })

  it('uses the current user id as the schedule salt', () => {
    const date = new Date(2026, 7, 6, 12)
    expect(dailyQuoteIndex(date, 47)).not.toBe(dailyQuoteIndex(date, 73))
  })

  it('requires the complete catalog so the non-repetition guarantee cannot silently weaken', () => {
    expect(() => dailyQuote(['道可道'], 47, new Date(2026, 7, 6))).toThrow(
      'Daily quote catalog must contain exactly 66 entries',
    )
  })
})
