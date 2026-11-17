import {
  createRootRouteWithContext,
  createRouter,
  stringifySearchWith,
  type RouterHistory,
} from '@tanstack/react-router'
import type {
  FetchQueryOptions,
  QueryClient,
  QueryKey,
} from '@tanstack/react-query'

import { queryClient } from '@/app/query'
import { RouteBoundary, type RouteBoundaryId } from './Boundary'
import { AppRouteLayout } from './Layout'
import {
  EmbeddedSessionUnavailableError,
  routeAccess,
  type AppRouterContext,
  type RouteAccess,
} from './guards'
import { buildRouteTree } from './routeTree'

function boundary(id: RouteBoundaryId) {
  return function Boundary() {
    return <RouteBoundary id={id} />
  }
}

export function prefetchBusinessQuery<
  TQueryFnData,
  TError,
  TData,
  TQueryKey extends QueryKey,
>(
  client: QueryClient,
  options: FetchQueryOptions<TQueryFnData, TError, TData, TQueryKey>,
): Promise<void> {
  if (client.getQueryState(options.queryKey)?.status === 'error') {
    return Promise.resolve()
  }
  return client.prefetchQuery(options)
}

export function startBusinessPrefetch(requests: Promise<void>[]): void {
  // Business screens own their loading and error states. Starting these reads
  // here still warms React Query, while allowing the route to commit so the
  // destination screen can replace stale content with its skeleton immediately.
  void Promise.all(requests).catch(() => undefined)
}

function RootErrorBoundary({ error }: { error: unknown }) {
  return (
    <RouteBoundary
      id="error"
      embeddedSessionUnavailable={
        error instanceof EmbeddedSessionUnavailableError
      }
    />
  )
}

export const rootRoute = createRootRouteWithContext<AppRouterContext>()({
  component: AppRouteLayout,
  pendingComponent: boundary('pending'),
  errorComponent: RootErrorBoundary,
  notFoundComponent: boundary('not-found'),
})

const routeTree = buildRouteTree(rootRoute as unknown as Parameters<typeof buildRouteTree>[0])

export function createAppRouter(options: {
  history?: RouterHistory
  access?: RouteAccess
} = {}) {
  return createRouter({
    routeTree,
    history: options.history,
    context: {
      queryClient,
      access: options.access ?? routeAccess,
    },
    stringifySearch: stringifySearchWith(JSON.stringify),
    defaultPreload: 'intent',
    scrollRestoration: true,
    // Cross-fade the named content area on menu switches only. Search-param
    // updates (filters, paging, tab state) keep pathChanged false and skip
    // the transition; the CSS lives under the `page-fade` type in
    // shared/styles.css.
    defaultViewTransition: {
      types: ({ pathChanged }) => (pathChanged ? ['page-fade'] : false),
    },
  })
}

export const router = createAppRouter()

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}