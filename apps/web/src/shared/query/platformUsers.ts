import { queryOptions, useQuery } from '@tanstack/react-query'

import {
  listPlatformUsers,
  type PlatformUser,
} from '@/shared/api'
import { pmKeys } from './keys'

export type PlatformUsersLoader = (signal?: AbortSignal) => Promise<PlatformUser[]>

export function platformUsersQueryOptions(
  workspaceId: number,
  loader: PlatformUsersLoader = listPlatformUsers,
) {
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'workspace', 'users'),
    queryFn: ({ signal }) => loader(signal),
    staleTime: 120_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
    // Session-snapshot semantics: the loader drains the directory page by
    // page, so an implicit refetch on every window focus would re-run that
    // multi-request burst for little gain — workspace membership rarely
    // changes mid-session, and a reload starts a fresh snapshot anyway.
    refetchOnWindowFocus: false,
  })
}

export function usePlatformUsersQuery(
  workspaceId: number,
  enabled = true,
  loader: PlatformUsersLoader = listPlatformUsers,
) {
  const valid = Number.isInteger(workspaceId) && workspaceId > 0
  return useQuery({
    ...platformUsersQueryOptions(valid ? workspaceId : 1, loader),
    enabled: enabled && valid,
  })
}
