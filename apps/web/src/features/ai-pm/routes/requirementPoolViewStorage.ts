import type { RequirementPoolSearchState } from '../utils/requirement-pool-filter-fields'

type PoolView = 'cards' | 'list'
const poolViewStorageKey = 'pm.requirement-pool.view'

export function readPoolView(): PoolView {
  try {
    return globalThis.localStorage?.getItem(poolViewStorageKey) === 'cards' ? 'cards' : 'list'
  } catch {
    return 'list'
  }
}

export function persistPoolView(view: PoolView) {
  try {
    globalThis.localStorage?.setItem(poolViewStorageKey, view)
  } catch {
    return
  }
}

export function poolViewFromSearch(search: RequirementPoolSearchState): PoolView {
  if (search.view === 'cards' || search.view === 'list') return search.view
  return readPoolView()
}