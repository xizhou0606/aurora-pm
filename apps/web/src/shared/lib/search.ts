import pinyinMatchModule from 'pinyin-match'

export type SearchOptions = { pinyin?: boolean }

// People controls share this preset; generic text search stays literal by default.
export const USER_SEARCH: Readonly<SearchOptions> = { pinyin: true }

type PinyinMatchFn = (text: string, keys: string) => [number, number] | false

function resolvePinyinMatch(module: unknown): PinyinMatchFn {
  if (!module || typeof module !== 'object') return () => false
  const record = module as { match?: unknown; default?: { match?: unknown } }
  const fn = record.match ?? record.default?.match
  return typeof fn === 'function' ? (fn as PinyinMatchFn) : () => false
}

const matchPinyin = resolvePinyinMatch(pinyinMatchModule)

export function textMatchesQuery(text: string, query: string, options: SearchOptions = {}): boolean {
  const key = query.trim().toLocaleLowerCase()
  if (!key) return true
  if (!text) return false
  if (text.toLocaleLowerCase().includes(key)) return true
  if (!options.pinyin) return false
  try { return matchPinyin(text, key) !== false } catch { return false }
}

export function searchFieldsMatch(fields: readonly (string | undefined)[], query: string, options: SearchOptions = {}): boolean {
  return !query.trim() || fields.some(value => value !== undefined && textMatchesQuery(value, query, options))
}
