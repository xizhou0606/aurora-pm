import { searchFieldsMatch, type SearchOptions } from '@/shared/lib/search'

// Search authorized pages without truncating matches to the first loaded page.
// Keep the source cursor, not the number of matches, for the next request.
export async function searchDirectoryPage<P extends { items: readonly unknown[] }>(input: {
  cursor: number
  query: string
  search: SearchOptions
  pageSize: number
  signal?: AbortSignal
  load: (cursor: number, query: string) => Promise<P>
  next: (page: P, cursor: number) => number | undefined
  fields: (item: P['items'][number]) => readonly (string | undefined)[]
}): Promise<{ source: P; items: P['items'][number][]; nextCursor: number | undefined }> {
  const local = Boolean(input.search.pinyin && /[a-z]/i.test(input.query))
  const items: P['items'][number][] = []
  let cursor = input.cursor
  while (true) {
    input.signal?.throwIfAborted()
    const source = await input.load(cursor, local ? '' : input.query)
    input.signal?.throwIfAborted()
    items.push(...source.items.filter(item => !local || searchFieldsMatch(input.fields(item), input.query, input.search)))
    const nextCursor = input.next(source, cursor)
    if (nextCursor !== undefined && nextCursor <= cursor) throw new Error('Directory cursor did not advance')
    if (!local || items.length >= input.pageSize || nextCursor === undefined) return { source, items, nextCursor }
    cursor = nextCursor
  }
}
