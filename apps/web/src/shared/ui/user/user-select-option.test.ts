import { describe, expect, it } from 'bun:test'

import { userSelectOptionMatches } from './user-select-option'

const xizhou = { id: 1, name: '西洲', username: 'west-zhou' }
const teo = { id: 2, name: '小龙', username: 'teo' }

describe('userSelectOptionMatches', () => {
  it('matches a Chinese display name by full pinyin', () => {
    expect(userSelectOptionMatches(xizhou, 'xizhou')).toBe(true)
    expect(userSelectOptionMatches(xizhou, 'XiZhou')).toBe(true)
    expect(userSelectOptionMatches(teo, 'xizhou')).toBe(false)
  })

  it('still matches username, Chinese text, and pinyin initials', () => {
    expect(userSelectOptionMatches(xizhou, '')).toBe(true)
    expect(userSelectOptionMatches(xizhou, '西')).toBe(true)
    expect(userSelectOptionMatches(xizhou, 'west-zhou')).toBe(true)
    expect(userSelectOptionMatches(xizhou, 'xz')).toBe(true)
    expect(userSelectOptionMatches(teo, 'teo')).toBe(true)
  })
})
