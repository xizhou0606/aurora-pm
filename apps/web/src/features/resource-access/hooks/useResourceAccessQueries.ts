import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { searchDirectoryPage } from '@/shared/query/search-directory-page'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'

import { canRetainStaleApiData } from '@/shared/api'

import type {
  ResourceAccessPolicy,
  ResourceAccessQueryState,
  ResourceAccessSubjectKind,
  ResourceAccessSubjectPage,
  ResourceShareLink,
} from '../model'
import type { ResourceAccessEnvironment } from './environment'
import { isValidResourceAccessEnvironment } from './environment'
import { resourceAccessKeys } from './keys'

const ACCESS_STALE_TIME = 15_000
const SUBJECT_PAGE_SIZE = 20

function queryState<T>(query: {
  data: T | undefined
  error: unknown
  isError: boolean
  isFetching: boolean
  refetch: () => Promise<{ data: T | undefined }>
}, empty: (data: T) => boolean): ResourceAccessQueryState<T> {
  const retry = async () => (await query.refetch()).data
  const mayRetain = !query.error || canRetainStaleApiData(query.error)
  if (!mayRetain || (!query.data && query.isError)) return { status: 'error', retry }
  if (!query.data) return { status: 'loading' }
  if (empty(query.data)) return { status: 'empty' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

export function useResourceAccessPolicyQuery(
  environment: ResourceAccessEnvironment,
  requested = true,
): ResourceAccessQueryState<ResourceAccessPolicy> {
  const enabled = requested && isValidResourceAccessEnvironment(environment)
  const keys = resourceAccessKeys(environment)
  const query = useQuery({
    queryKey: keys.policy,
    queryFn: ({ signal }) => environment.client.policy(environment.resourceId, signal),
    enabled,
    staleTime: ACCESS_STALE_TIME,
  })
  return queryState(query, () => false)
}

export function useResourceShareLinksQuery(
  environment: ResourceAccessEnvironment,
  enabled = true,
): ResourceAccessQueryState<readonly ResourceShareLink[]> {
  const active = enabled && isValidResourceAccessEnvironment(environment)
  const keys = resourceAccessKeys(environment)
  const query = useQuery({
    queryKey: keys.links,
    queryFn: ({ signal }) => environment.client.links(environment.resourceId, signal),
    enabled: active,
    staleTime: ACCESS_STALE_TIME,
  })
  return queryState(query, (links) => links.length === 0)
}

export function useResourceAccessSubjectsQuery(
  environment: ResourceAccessEnvironment,
  kind: ResourceAccessSubjectKind,
  queryText: string,
  enabled = true,
  search: SearchOptions = USER_SEARCH,
) {
  const normalizedQuery = queryText.trim()
  const active = enabled && isValidResourceAccessEnvironment(environment)
  const keys = resourceAccessKeys(environment)
  return useInfiniteQuery({
    queryKey: [...keys.subjects(kind, normalizedQuery, SUBJECT_PAGE_SIZE), search],
    queryFn: async ({ pageParam, signal }) => {
      const result = await searchDirectoryPage({
        cursor: pageParam, query: normalizedQuery, search: kind === 'user' ? search : {},
        pageSize: SUBJECT_PAGE_SIZE, signal,
        load: (offset, query) => environment.client.subjects({ kind, query, limit: SUBJECT_PAGE_SIZE, offset, signal }),
        next: (page, offset) => page.hasMore && page.items.length ? offset + page.items.length : undefined,
        fields: item => [item.name, item.secondary],
      })
      return { ...result.source, items: result.items, hasMore: result.nextCursor !== undefined, nextCursor: result.nextCursor }
    },
    initialPageParam: 0,
    getNextPageParam: lastPage => lastPage.nextCursor,
    enabled: active,
    select: (data): ResourceAccessSubjectPage => ({
      items: data.pages.flatMap((page) => page.items),
      total: data.pages[0]?.total ?? 0,
      hasMore: data.pages.at(-1)?.hasMore ?? false,
      capabilities: data.pages[0]?.capabilities ?? data.pages.at(-1)!.capabilities,
    }),
  })
}
