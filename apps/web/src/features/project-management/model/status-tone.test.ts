import { describe, expect, it } from 'bun:test'

import {
  planUnitStatusTone,
  projectStatusTone,
} from './status-tone'

describe('project domain status tones', () => {
  it('keeps project-set enums separate from their display labels', () => {
    expect(projectStatusTone('active')).toBe('success')
    expect(projectStatusTone('archived')).toBe('neutral')
    expect(projectStatusTone('进行中')).toBe('neutral')
  })

  it.each([
    ['planning', 'warning'],
    ['grooming', 'warning'],
    ['development', 'info'],
    ['acceptance', 'warning'],
    ['released', 'success'],
    ['archived', 'neutral'],
  ] as const)('maps plan-unit enum %s independently', (status, tone) => {
    expect(planUnitStatusTone(status)).toBe(tone)
  })
})
