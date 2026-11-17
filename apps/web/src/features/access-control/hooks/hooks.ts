import { subscribePermissionRefresh } from '../model/permission-refresh'
import { useCallback, useEffect, useMemo } from 'react'

import {
  PM_PAGE_BY_ROUTE,
  type PMActionCode,
  type PMPageCode,
} from '@/generated/access-catalog'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import {
  permissionsStore,
  usePermissionsStore,
} from '../model/permissions-store'

export type ActionPermissionState = {
  status: 'loading' | 'error' | 'ready'
  canAction: (code: PMActionCode) => boolean
  canPage: (code: PMPageCode) => boolean
  retry: () => void
}

const pageRoutes = Object.entries(PM_PAGE_BY_ROUTE)
  .sort(([left], [right]) => right.length - left.length)

export function pageCodeForPath(pathname: string): PMPageCode | undefined {
  const pathSegments = pathname.replace(/\/$/, '').split('/')
  const match = pageRoutes.find(([route]) => {
    const routeSegments = route.split('/')
    return routeSegments.length === pathSegments.length
      && routeSegments.every((segment, index) => (
        segment.startsWith('$') ? Boolean(pathSegments[index]) : segment === pathSegments[index]
      ))
  })
  return match?.[1]
}

export function useActionPermissions(enabled = true): ActionPermissionState {
  const revision = useSessionStore((state) => state.revision)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const scopeKey = `${revision}:${workspaceId}`
  const loadedScopeKey = usePermissionsStore((state) => state.scopeKey)
  const storeStatus = usePermissionsStore((state) => state.status)
  const actionCodes = usePermissionsStore((state) => state.actionCodes)

  useEffect(() => {
    if (!enabled) return
    void permissionsStore.getState().ensure(scopeKey)
    return subscribePermissionRefresh()
  }, [enabled, scopeKey])

  const scoped = loadedScopeKey === scopeKey
  const granted = scoped && storeStatus === 'ready'
  const allowed = useMemo(() => new Set(
    granted ? actionCodes : [],
  ), [actionCodes, granted])
  const canAction = useCallback((code: PMActionCode) => (
    granted && allowed.has(code)
  ), [allowed, granted])
  const canPage = useCallback((code: PMPageCode) => (
    granted && [...allowed].some((action) => action.startsWith(`${code}.`))
  ), [allowed, granted])
  const retry = useCallback(() => {
    void permissionsStore.getState().reload(scopeKey)
  }, [scopeKey])
  return {
    status: granted
      ? 'ready'
      : scoped && storeStatus === 'error'
        ? 'error'
        : 'loading',
    canAction,
    canPage,
    retry,
  }
}

export function useCanAction(code: PMActionCode): boolean {
  return useActionPermissions().canAction(code)
}
