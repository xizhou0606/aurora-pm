import type {
  CollectionState,
  InboxSummary,
  ReadyRefresh,
  SummaryState,
} from './types'

export function readyRefresh(fetching: boolean, failed: boolean): ReadyRefresh {
  if (failed) return 'stale_error'
  if (fetching) return 'refreshing'
  return 'idle'
}

export function resolveCollectionState<T>(input: {
  data?: { items: T[]; total: number }
  pending: boolean
  fetching: boolean
  failed: boolean
  retry: () => void
}): CollectionState<T> {
  if (!input.data) {
    if (input.pending) return { status: 'loading' }
    return { status: 'error', retry: input.retry }
  }
  if (!input.data.items.length) {
    return {
      status: 'empty',
      total: input.data.total,
      refresh: readyRefresh(input.fetching, input.failed),
    }
  }
  return {
    status: 'ready',
    rows: input.data.items,
    total: input.data.total,
    refresh: readyRefresh(input.fetching, input.failed),
  }
}

export function resolveSummaryState(input: {
  data?: InboxSummary
  pending: boolean
  fetching: boolean
  failed: boolean
  retry: () => void
}): SummaryState {
  if (!input.data) {
    if (input.pending) return { status: 'loading' }
    return { status: 'error', retry: input.retry }
  }
  return {
    status: 'ready',
    data: input.data,
    refresh: readyRefresh(input.fetching, input.failed),
  }
}

export function aggregateRefresh(
  states: readonly (CollectionState<unknown> | SummaryState | null)[],
): ReadyRefresh {
  const refreshStates = states.flatMap((state) => (
    state?.status === 'ready' || state?.status === 'empty'
      ? [state.refresh]
      : []
  ))
  if (refreshStates.includes('stale_error')) {
    return 'stale_error'
  }
  if (refreshStates.includes('refreshing')) {
    return 'refreshing'
  }
  return 'idle'
}
