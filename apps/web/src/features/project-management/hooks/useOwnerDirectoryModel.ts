import { useCallback } from 'react'

import {
  usePlatformUsersQuery,
  ownerDirectoryItems,
  type OwnerDirectoryState,
} from '../model'

/**
 * Owner pickers render the full active workspace directory and filter it
 * locally, so no search/pagination handlers are wired into the state.
 */
export function useOwnerDirectoryModel(
  teamId: number | null | undefined,
  enabled: boolean,
): OwnerDirectoryState {
  const validTeamId = Number.isInteger(teamId) && Number(teamId) > 0
  const query = usePlatformUsersQuery(enabled && validTeamId)
  const retry = useCallback(() => {
    void query.refetch()
  }, [query.refetch])

  if (!enabled || !validTeamId) return { status: 'empty' }
  if (query.isPending) return { status: 'loading' }
  if (query.isError) return { status: 'error', retry }
  const items = ownerDirectoryItems(query.data ?? [])
  return items.length > 0
    ? { status: 'ready', items }
    : { status: 'empty' }
}
