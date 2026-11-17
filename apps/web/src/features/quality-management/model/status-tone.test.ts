import { describe, expect, it } from 'bun:test'

import { qualitySeverityTone, qualityStatusTone } from './status-tone'

describe('quality status tones', () => {
  it.each([
    ['triage', 'warning'],
    ['todo', 'success'],
    ['in_progress', 'info'],
    ['reopened', 'danger'],
    ['review', 'accent'],
    ['done', 'neutral'],
  ] as const)('maps the canonical %s enum without using its display label', (status, tone) => {
    expect(qualityStatusTone(status)).toBe(tone)
  })

  it('keeps unknown enum values neutral', () => {
    expect(qualityStatusTone('future_status')).toBe('neutral')
  })
})

describe('quality severity tones', () => {
  it.each([
    ['p0', 'danger'],
    ['p1', 'attention'],
    ['p2', 'info'],
    ['p3', 'neutral'],
  ] as const)('maps %s to %s', (severity, tone) => {
    expect(qualitySeverityTone(severity)).toBe(tone)
  })

  it('keeps an empty or unknown severity neutral', () => {
    expect(qualitySeverityTone('')).toBe('neutral')
    expect(qualitySeverityTone('future_severity')).toBe('neutral')
  })
})
