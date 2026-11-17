import { useCallback, useEffect, useMemo } from 'react'

import { isValidPMScopeSelection } from '@/shared/api'
import type { QualityEnvironment } from '../components/environment'
import { parseQualityRoute, qualityCreatePath, qualityDetailPath } from '../model/route'
import type { QualityRouteState } from '../model/types'
import { useQualityScopeQuery } from './useQueries'

export function useQualityScopeWorkflow(environment: QualityEnvironment) {
  const parsedRoute = useMemo(
    () => parseQualityRoute(environment.pathname, environment.search),
    [environment.pathname, environment.search],
  )
  const routeScopeSelected = Boolean(
    parsedRoute.scope.projectSetIds.length
    || parsedRoute.scope.projectIds.length
    || parsedRoute.scope.requirementIds.length,
  )
  const scopeEnvironment = useMemo(
    () => parsedRoute.invalidScope ? { ...environment, allowed: false } : environment,
    [environment, parsedRoute.invalidScope],
  )
  const scope = useQualityScopeQuery(scopeEnvironment)
  const hierarchyInvalid = routeScopeSelected
    && scope.status === 'ready'
    && !isValidPMScopeSelection(parsedRoute.scope, scope.data)
  const scopeProven = scope.status === 'ready'
  const queryEnvironment = useMemo(
    () => parsedRoute.invalidScope || hierarchyInvalid || !scopeProven
      ? { ...environment, allowed: false }
      : environment,
    [environment, hierarchyInvalid, parsedRoute.invalidScope, scopeProven],
  )
  const clearInvalidScope = useCallback(
    () => environment.navigate('/workspace/quality-management', true),
    [environment],
  )
  const route: QualityRouteState = hierarchyInvalid
    ? { ...parsedRoute, invalidScope: true }
    : parsedRoute

  const normalizationTarget = parsedRoute.kind === 'create'
    ? qualityCreatePath({ ...parsedRoute, needsRouteNormalization: false })
    : parsedRoute.kind === 'detail' && parsedRoute.bugId
      ? qualityDetailPath(
        { ...parsedRoute, needsRouteNormalization: false },
        parsedRoute.bugId,
        parsedRoute.mode,
      )
      : '/workspace/quality-management'

  useEffect(() => {
    if (!parsedRoute.needsRouteNormalization || parsedRoute.invalidScope) return
    void environment.navigate(normalizationTarget, true)
  }, [environment, normalizationTarget, parsedRoute.invalidScope, parsedRoute.needsRouteNormalization])

  return { route, scope, queryEnvironment, clearInvalidScope }
}
