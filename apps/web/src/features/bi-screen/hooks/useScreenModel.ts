import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { isSafeApiError, isValidPMScopeSelection } from '@/shared/api'

import type { Environment } from '../components/environment'
import {
  dashboardState,
  drilldownPath,
  hasScope,
  parseRoute,
  resetScope,
  routePath,
  sameRouteTarget,
  routeWithTab,
  routeWithTeamDimension,
  routeWithTeamDimensionKey,
  selectedScopeCount,
  selectedScopeLabel,
} from '../model'
import { shouldEscalateRuntimeFailure } from '../api/client'
import type {
  Drilldown,
  RouteState,
  Scope,
  ScreenTab,
  TeamDimension,
	TeamDimensionOption,
} from '../types'
import type { DashboardState } from '../types/dashboard'
import {
  executeTeamRouteNavigation,
  resolveDashboardState,
  resolveInitialRuntimeFailure,
  resolveScopeState,
} from './useScreenModel-utils'
import {
	useDashboardQuery,
	useScopeOptionsQuery,
	useTeamScoreProfileMutation,
} from './useQueries'
import {
	useTeamMetricSettingsMutation,
	useTeamMetricSettingsQuery,
} from './useTeamMetricSettings'

export function useScreenModel(
  environment: Environment,
  onRuntimeFailure?: (error: unknown) => void,
) {
  const operationAccess = environment.operationAccess
  const canonicalRoute = useMemo(() => parseRoute(environment.search), [environment.search])
  const [pendingTeamRoute, setPendingTeamRoute] = useState<RouteState | undefined>(undefined)
  const [navigationStatus, setNavigationStatus] = useState<'idle' | 'loading' | 'error'>('idle')
  const pendingTeamRouteRef = useRef<RouteState | undefined>(undefined)
  const navigationSequenceRef = useRef(0)
  const navigationQueueRef = useRef<Promise<void>>(Promise.resolve())
  const optionCacheRef = useRef<Partial<Record<TeamDimension, readonly TeamDimensionOption[]>>>({})
  const route = pendingTeamRoute ?? canonicalRoute

  useEffect(() => {
    const pending = pendingTeamRouteRef.current
    if (!pending || !sameRouteTarget(pending, canonicalRoute)) return
    pendingTeamRouteRef.current = undefined
    setPendingTeamRoute(undefined)
    setNavigationStatus('idle')
  }, [canonicalRoute])

  const permissionReady = operationAccess.status === 'ready'
  const baseEnabled = environment.allowed
    && permissionReady
    && operationAccess.viewOverview
    && !route.invalidScope
  const scopeQuery = useScopeOptionsQuery(environment, baseEnabled)
  const options = scopeQuery.data
  const selected = hasScope(route.scope)
  const validSelectedScope = !selected
    || Boolean(options && isValidPMScopeSelection(route.scope, options))
  const dashboardEnabled = baseEnabled
    && operationAccess.viewOverview
    && validSelectedScope
    && (!selected || Boolean(options))
  const dashboardQuery = useDashboardQuery(
    environment,
    route,
    dashboardEnabled,
  )
	const dashboardData = dashboardQuery.data
	const currentTeamData = dashboardQuery.data?.team_data
	useEffect(() => {
		if (!currentTeamData) return
		optionCacheRef.current[currentTeamData.dimension] = currentTeamData.dimension_options
	}, [currentTeamData])
	const scoreProfileMutation = useTeamScoreProfileMutation(
		environment,
		route,
		operationAccess.updateScoring,
	)
	const metricSettingsQuery = useTeamMetricSettingsQuery(
		environment,
		baseEnabled && operationAccess.viewMetrics && route.tab === 'team',
	)
	const metricSettingsMutation = useTeamMetricSettingsMutation(
		environment,
		operationAccess.updateMetrics,
	)
  const initialRuntimeFailure = resolveInitialRuntimeFailure(scopeQuery, dashboardQuery)

  useEffect(() => {
    if (initialRuntimeFailure) onRuntimeFailure?.(initialRuntimeFailure)
  }, [initialRuntimeFailure, onRuntimeFailure])

  useEffect(() => {
    if (!dashboardEnabled || document.visibilityState !== 'visible') return
    const refresh = () => {
      if (document.visibilityState === 'visible') void dashboardQuery.refetch()
    }
    document.addEventListener('visibilitychange', refresh)
    return () => document.removeEventListener('visibilitychange', refresh)
  }, [dashboardEnabled, dashboardQuery.refetch])

  const reportNavigationFailure = useCallback((error: unknown) => {
    setNavigationStatus('error')
    if (isSafeApiError(error) && shouldEscalateRuntimeFailure(error, false)) {
      onRuntimeFailure?.(error)
    }
  }, [onRuntimeFailure])
  const navigatePagePath = useCallback((path: string, replace = false) => {
    const requestSequence = ++navigationSequenceRef.current
    pendingTeamRouteRef.current = undefined
    setPendingTeamRoute(undefined)
    setNavigationStatus('idle')
    const navigation = replace ? environment.navigate(path, true) : environment.navigate(path)
    void navigation.catch((error: unknown) => {
      if (navigationSequenceRef.current !== requestSequence) return
      reportNavigationFailure(error)
    })
  }, [environment, reportNavigationFailure])
  const navigateRoute = useCallback((scope: Scope, replace = false) => {
    navigatePagePath(routePath({ ...route, scope, invalidScope: false }), replace)
  }, [navigatePagePath, route])
  const setScope = useCallback((scope: Scope) => navigateRoute(scope), [navigateRoute])
  const setTab = useCallback((tab: ScreenTab) => {
    navigatePagePath(routePath(routeWithTab(route, tab)))
  }, [navigatePagePath, route])

  const navigateTeamRoute = useCallback((nextRoute: RouteState) => {
    executeTeamRouteNavigation(
      nextRoute,
      canonicalRoute,
      { pendingTeamRouteRef, navigationSequenceRef, navigationQueueRef },
      { setPendingTeamRoute, setNavigationStatus },
      environment,
      reportNavigationFailure,
      sameRouteTarget,
      routePath,
    )
  }, [canonicalRoute, environment, reportNavigationFailure])
  const setTeamDimension = useCallback((dimension: TeamDimension) => {
    const currentRoute = pendingTeamRouteRef.current ?? canonicalRoute
    navigateTeamRoute(routeWithTeamDimension(currentRoute, dimension))
  }, [canonicalRoute, navigateTeamRoute])
  const setTeamDimensionKey = useCallback((key?: string) => {
    const currentRoute = pendingTeamRouteRef.current ?? canonicalRoute
    navigateTeamRoute(routeWithTeamDimensionKey(currentRoute, key))
  }, [canonicalRoute, navigateTeamRoute])
  const clearInvalidScope = useCallback(() => {
    navigatePagePath(routePath({
      ...route,
      scope: resetScope(),
      invalidScope: false,
    }), true)
  }, [navigatePagePath, route])
  const drilldown = useCallback((target: Drilldown) => {
    navigatePagePath(drilldownPath(route, target))
  }, [navigatePagePath, route])

  const scope = resolveScopeState(
    route, selected, options, validSelectedScope, scopeQuery, clearInvalidScope,
  )
  const dashboard = resolveDashboardState(
    selected, scopeQuery, options, validSelectedScope, clearInvalidScope,
    dashboardData, dashboardQuery, dashboardState as (input: { data: unknown; query: 'pending' | 'failed' | 'settled'; refresh: 'idle' | 'refreshing' | 'failed'; retry: () => void }) => DashboardState,
  )
	const teamOptions = currentTeamData?.dimension === route.teamDimension
		? currentTeamData.dimension_options
		: optionCacheRef.current[route.teamDimension] ?? []
	const teamNavigationLoading = navigationStatus === 'loading'

  return {
    allowed: environment.allowed && operationAccess.viewOverview,
    workspaceId: environment.workspaceId,
    operationAccess,
    route,
    scope,
    dashboard,
    refresh: dashboard.status === 'ready' ? dashboard.refresh : 'idle' as const,
    scopeCount: selectedScopeCount(route.scope),
    scopeLabel: options ? selectedScopeLabel(route.scope, options) : undefined,
    setScope,
    setTab,
		teamDimension: {
			value: route.teamDimension,
			loading: teamNavigationLoading,
			onChange: setTeamDimension,
		},
		teamSlice: {
			value: route.teamDimensionKey,
			options: teamOptions,
			loading: teamNavigationLoading || dashboardQuery.isPending,
			onChange: setTeamDimensionKey,
		},
		scoreProfileMutation,
		metricSettingsQuery,
		metricSettingsMutation,
    navigation: navigationStatus === 'error'
      ? { status: 'error' as const, dismiss: () => setNavigationStatus('idle') }
      : navigationStatus === 'loading'
        ? { status: 'loading' as const }
        : { status: 'idle' as const },
    clearInvalidScope,
    drilldown,
  }
}
