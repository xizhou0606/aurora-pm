import {
  queryOptions,
  useQuery,
  type UseQueryResult,
} from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useSessionStore } from '@/shared/state'
import type { PMHostUser } from '@/shared/runtime'
import { fetchCurrentUser } from './api'

export function currentUserQueryOptions() {
  return queryOptions({
    queryKey: pmKeys.me(),
    queryFn: ({ signal }) => fetchCurrentUser(signal),
    retry: false,
  })
}

export function useCurrentUserQuery(): UseQueryResult<PMHostUser> {
  const token = useSessionStore((state) => state.token)
  return useQuery({
    ...currentUserQueryOptions(),
    enabled: Boolean(token),
  })
}
