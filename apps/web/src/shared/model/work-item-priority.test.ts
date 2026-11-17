import { describe, expect, it } from 'bun:test'

import { workItemPriorityTone } from './work-item-priority'

describe('work item priority tones', () => {
  it.each([
    ['low', 'neutral'],
    ['normal', 'info'],
    ['high', 'attention'],
    ['urgent', 'danger'],
  ] as const)('maps %s to %s', (priority, tone) => {
    expect(workItemPriorityTone(priority)).toBe(tone)
  })

  it('keeps unknown priority values neutral', () => {
    expect(workItemPriorityTone('future')).toBe('neutral')
  })
})
