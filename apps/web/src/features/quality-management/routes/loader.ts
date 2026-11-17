import type { FetchQueryOptions, QueryClient, QueryKey } from '@tanstack/react-query'

import { parseQualityRoute } from '../model/route'
import type { QualityRouteState } from '../model/types-route'
import {
  listQuery,
  qualityListQueryOptions,
  qualityScopeQueryOptions,
} from '../hooks/useQueries-list'

// Query option factories live in the hooks layer (Query ownership + payload
// mapping stay out of routes). The loader derives the exact query keys the
// page reads on mount so a route switch renders warm data instead of a fresh
// request waterfall; consumers reach the factories through
// prefetchQualityListData rather than importing them directly.

async function prefetchBusinessQuery<TQueryFnData, TError, TData, TQueryKey extends QueryKey>(
  client: QueryClient,
  options: FetchQueryOptions<TQueryFnData, TError, TData, TQueryKey>,
): Promise<void> {
  if (client.getQueryState(options.queryKey)?.status === 'error') {
    return
  }
  await client.prefetchQuery(options)
}

/**
 * Warms the scope and list cache entries the quality list page reads on
 * mount for the given URL search state. No-op for create/detail/dashboard
 * routes, which own their fetches.
 */
export async function prefetchQualityListData(
  client: QueryClient,
  workspaceId: number,
  search: Readonly<Record<string, unknown>>,
): Promise<void> {
  const route = parseQualityRoute('/workspace/quality-management', search)
  if (route.kind !== 'list' || route.view !== 'list' || workspaceId <= 0) return
  await Promise.all([
    prefetchBusinessQuery(client, qualityScopeQueryOptions(workspaceId)),
    prefetchBusinessQuery(client, qualityListQueryOptions(workspaceId, route)),
  ])
}
