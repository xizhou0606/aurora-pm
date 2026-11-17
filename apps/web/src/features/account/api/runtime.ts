import { useAccountQuery } from './query'
import { useAccountActions } from '../utils/workflows'
import type { AccountDetails } from './api'

export type AccountScreenState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; account: AccountDetails }

export function useAccountScreen() {
  const query = useAccountQuery()
  const actions = useAccountActions()
  let state: AccountScreenState
  if (query.isPending || !query.data) {
    state = query.isError
      ? { status: 'error', retry: () => void query.refetch() }
      : { status: 'loading' }
  } else {
    state = { status: 'ready', account: query.data }
  }
  return { actions, state }
}
