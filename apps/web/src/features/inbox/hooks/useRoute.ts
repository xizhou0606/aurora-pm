import { useCallback, useMemo } from 'react'

import type { InboxEnvironment } from '../components/environment'
import {
  emptyScope,
  inboxPath,
  parseInboxRoute,
  resetFeedPages,
  type InboxFeedId,
  type InboxRouteState,
  type InboxScope,
} from '../model'

export type InboxRouteNavigator = (next: InboxRouteState, replace?: boolean) => void

export function useInboxRoute(environment: InboxEnvironment) {
  const route = useMemo(
    () => parseInboxRoute(environment.search),
    [environment.search],
  )
  const navigateRoute = useCallback<InboxRouteNavigator>((next, replace = false) => {
    void environment.navigate(inboxPath(next), replace)
  }, [environment.navigate])

  const setScope = useCallback((scope: InboxScope) => {
    navigateRoute(resetFeedPages({
      ...route,
      ...scope,
      invalidScope: false,
      invalidLegacyScope: false,
    }))
  }, [navigateRoute, route])

  const clearInvalidScope = useCallback(() => {
    navigateRoute(resetFeedPages({
      ...route,
      ...emptyScope(),
      invalidScope: false,
      invalidLegacyScope: false,
    }), true)
  }, [navigateRoute, route])

  const setFeedPage = useCallback((feedId: InboxFeedId, page: number) => {
    const nextPage = Number.isSafeInteger(page) && page > 0 ? page : 1
    navigateRoute({
      ...route,
      ...(feedId === 'workflow' ? { workflowPage: nextPage } : {}),
      ...(feedId === 'task' ? { taskPage: nextPage } : {}),
      ...(feedId === 'bug' ? { bugPage: nextPage } : {}),
      ...(feedId === 'mention' ? { mentionPage: nextPage } : {}),
    })
  }, [navigateRoute, route])

  return { route, navigateRoute, setScope, clearInvalidScope, setFeedPage }
}
