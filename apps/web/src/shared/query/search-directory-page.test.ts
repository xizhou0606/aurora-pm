import { describe, expect, it } from 'bun:test'
import { searchDirectoryPage } from './search-directory-page'

const pages = [
  { items: [{ name: 'Alice' }], more: true },
  { items: [{ name: '铁奥' }], more: true },
  { items: [{ name: '铁奥二' }], more: false },
]
const fields = (item: { name: string }) => [item.name]
const next = (page: typeof pages[number], cursor: number) => page.more ? cursor + 1 : undefined

describe('paged pinyin search', () => {
  it('finds later-page matches and resumes from the source cursor', async () => {
    const queries: string[] = []
    const load = async (cursor: number, query: string) => { queries.push(query); return pages[cursor] }
    const first = await searchDirectoryPage({ cursor: 0, query: 'ta', search: { pinyin: true }, pageSize: 1, load, next, fields })
    expect(first.items).toEqual([{ name: '铁奥' }])
    expect(first.nextCursor).toBe(2)
    const last = await searchDirectoryPage({ cursor: first.nextCursor!, query: 'ta', search: { pinyin: true }, pageSize: 1, load, next, fields })
    expect(last.items).toEqual([{ name: '铁奥二' }])
    expect(last.nextCursor).toBeUndefined()
    expect(queries).toEqual(['', '', ''])
  })
  it('keeps remote literal filtering when disabled', async () => {
    let received = ''
    await searchDirectoryPage({ cursor: 0, query: 'ta', search: { pinyin: false }, pageSize: 1,
      load: async (_, query) => { received = query; return pages[0] }, next, fields })
    expect(received).toBe('ta')
  })
  it('stops at exhaustion and propagates permission failures', async () => {
    const empty = await searchDirectoryPage({ cursor: 0, query: 'nobody', search: { pinyin: true }, pageSize: 1, load: async cursor => pages[cursor], next, fields })
    expect(empty.items).toEqual([])
    expect(empty.nextCursor).toBeUndefined()
    await expect(searchDirectoryPage({ cursor: 0, query: 'ta', search: { pinyin: true }, pageSize: 1,
      load: async (): Promise<typeof pages[number]> => { throw new Error('FORBIDDEN') }, next, fields })).rejects.toThrow('FORBIDDEN')
  })
  it('stops scanning after cancellation', async () => {
    const controller = new AbortController()
    let calls = 0
    await expect(searchDirectoryPage({ cursor: 0, query: 'ta', search: { pinyin: true }, pageSize: 1, signal: controller.signal,
      load: async cursor => { calls++; controller.abort(); return pages[cursor] }, next, fields })).rejects.toThrow()
    expect(calls).toBe(1)
  })
})
