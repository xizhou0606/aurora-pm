import { shouldEscalateRuntimeFailure } from '../api/client'
import type { Environment } from '../components/environment'
import type {
  DashboardState,
  Drilldown,
  RouteState,
  ScopeState,
  ScreenTab,
  TeamDimension,
} from '../types'

/**
 * Resolve the initial runtime failure from query error states.
 * Returns the first failure that should escalate, or undefined if none.
 */
export function resolveInitialRuntimeFailure(
  scopeQuery: { isError: boolean; error: unknown; data: unknown },
  dashboardQuery: { isError: boolean; error: unknown; data: unknown; isPlaceholderData: boolean },
): unknown | undefined {
  if (scopeQuery.isError && shouldEscalateRuntimeFailure(scopeQuery.error, scopeQuery.data !== undefined)) {
    return scopeQuery.error
  }
  if (
    dashboardQuery.isError
    && shouldEscalateRuntimeFailure(
      dashboardQuery.error,
      dashboardQuery.data !== undefined && !dashboardQuery.isPlaceholderData,
    )
  ) {
    return dashboardQuery.error
  }
  return undefined
}

interface QueryLike {
  data: unknown
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
}

/**
 * Derive the scope state from route and query state.
 */
export function resolveScopeState(
  route: { invalidScope: boolean; scope: unknown },
  selected: boolean,
  options: unknown,
  validSelectedScope: boolean,
  scopeQuery: QueryLike,
  clearInvalidScope: () => void,
): ScopeState {
  if (route.invalidScope || (selected && options && !validSelectedScope)) {
    return { status: 'invalid', reset: clearInvalidScope }
  }
  if (scopeQuery.data) {
    return { status: 'ready', data: scopeQuery.data as any }
  }
  if (scopeQuery.isPending) {
    return { status: 'loading' }
  }
  return { status: 'error', retry: () => void scopeQuery.refetch() }
}

/**
 * Derive the dashboard state from route, query, and scope state.
 */
export function resolveDashboardState(
  selected: boolean,
  scopeQuery: { isError: boolean; refetch: () => void },
  options: unknown,
  validSelectedScope: boolean,
  clearInvalidScope: () => void,
  dashboardData: unknown,
  dashboardQuery: QueryLike,
  dashboardStateFn: (input: {
    data: unknown
    query: 'pending' | 'failed' | 'settled'
    refresh: 'idle' | 'refreshing' | 'failed'
    retry: () => void
  }) => DashboardState,
): DashboardState {
  if (selected && scopeQuery.isError) {
    return { status: 'error', retry: () => void scopeQuery.refetch() }
  }
  if (selected && !options) {
    return { status: 'loading' }
  }
  if (selected && !validSelectedScope) {
    return { status: 'error', retry: clearInvalidScope }
  }
  return dashboardStateFn({
    data: dashboardData,
    query: dashboardQuery.isPending
      ? 'pending'
      : dashboardQuery.isError
        ? 'failed'
        : 'settled',
    refresh: dashboardQuery.isError
      ? 'failed'
      : dashboardQuery.isFetching
        ? 'refreshing'
        : 'idle',
    retry: () => void dashboardQuery.refetch(),
  })
}

// --- Navigation helpers ---------------------------------------------------

type NavigationRefs = {
  pendingTeamRouteRef: React.MutableRefObject<RouteState | undefined>
  navigationSequenceRef: React.MutableRefObject<number>
  navigationQueueRef: React.MutableRefObject<Promise<void>>
}

type NavigationSetters = {
  setPendingTeamRoute: React.Dispatch<React.SetStateAction<RouteState | undefined>>
  setNavigationStatus: React.Dispatch<React.SetStateAction<'idle' | 'loading' | 'error'>>
}

/**
 * Navigate to a team route, deduplicating with the current route and
 * serializing navigation through a queue to prevent races.
 */
export function executeTeamRouteNavigation(
  nextRoute: RouteState,
  canonicalRoute: RouteState,
  refs: NavigationRefs,
  setters: NavigationSetters,
  environment: Environment,
  reportNavigationFailure: (error: unknown) => void,
  sameRouteTarget: (a: RouteState, b: RouteState) => boolean,
  routePath: (route: RouteState) => string,
): void {
  const currentRoute = refs.pendingTeamRouteRef.current ?? canonicalRoute
  if (sameRouteTarget(currentRoute, nextRoute)) return
  const requestSequence = ++refs.navigationSequenceRef.current
  refs.pendingTeamRouteRef.current = nextRoute
  setters.setPendingTeamRoute(nextRoute)
  setters.setNavigationStatus('loading')

  const navigation = refs.navigationQueueRef.current.then(() => (
    refs.navigationSequenceRef.current === requestSequence
      ? environment.navigate(routePath(nextRoute))
      : undefined
  ))
  refs.navigationQueueRef.current = navigation.catch(() => undefined)
  void navigation.then(() => {
    if (refs.navigationSequenceRef.current !== requestSequence) return
    setters.setNavigationStatus('idle')
    if (!sameRouteTarget(nextRoute, canonicalRoute)) return
    refs.pendingTeamRouteRef.current = undefined
    setters.setPendingTeamRoute(undefined)
  }, () => undefined)
  void navigation.catch((error: unknown) => {
    if (refs.navigationSequenceRef.current !== requestSequence) return
    refs.pendingTeamRouteRef.current = undefined
    setters.setPendingTeamRoute(undefined)
    reportNavigationFailure(error)
  })
}