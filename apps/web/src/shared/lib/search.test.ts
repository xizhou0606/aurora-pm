import { describe, expect, it } from 'bun:test'
import { searchFieldsMatch, textMatchesQuery, USER_SEARCH } from './search'
import { filterMentions } from '@/shared/ui/file-media/mention-helpers'

describe('configured people search', () => {
  it('requires opt-in for pinyin and preserves literal matches', () => {
    expect(textMatchesQuery('铁奥', 'tieao')).toBe(false)
    expect(textMatchesQuery('铁奥', 'tieao', USER_SEARCH)).toBe(true)
    expect(textMatchesQuery('铁奥', 'TA', USER_SEARCH)).toBe(true)
    expect(textMatchesQuery('铁奥', '铁', { pinyin: false })).toBe(true)
    expect(searchFieldsMatch(['铁奥', 'FS_123', 'Teo@example.com'], 'teo@EXAMPLE', USER_SEARCH)).toBe(true)
    expect(searchFieldsMatch(['铁奥'], 'nobody', USER_SEARCH)).toBe(false)
  })
  it('uses the same switch for mentions', () => {
    const users = [{ id: 1, label: '铁奥', username: 'fs_123' }]
    expect(filterMentions(users, 'ta', USER_SEARCH)).toEqual(users)
    expect(filterMentions(users, 'ta', { pinyin: false })).toEqual([])
    expect(filterMentions(users, 'FS_123', { pinyin: false })).toEqual(users)
  })
})
