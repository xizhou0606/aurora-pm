import { queryOptions, useQuery } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useSessionStore } from '@/shared/state'

import { fetchAccount } from './api'

export function accountQueryOptions() {
  return queryOptions({
    queryKey: pmKeys.account(),
    queryFn: ({ signal }) => fetchAccount(signal),
    retry: false,
  })
}

export function useAccountQuery() {
  const mode = useSessionStore((state) => state.mode)
  const token = useSessionStore((state) => state.token)
  return useQuery({
    ...accountQueryOptions(),
    enabled: mode === 'standalone' && Boolean(token),
  })
}
