import { useCallback, useEffect, useState } from 'react'

import { ROUTE_SEARCH_DEBOUNCE_MS } from '@/shared/model'

import { resetFeedPages, type InboxRouteState } from '../model'
import type { InboxRouteNavigator } from './useRoute'

export function useInboxSearch(
  route: InboxRouteState,
  navigateRoute: InboxRouteNavigator,
) {
  const [draft, setDraft] = useState(route.search)

  useEffect(() => {
    setDraft(route.search)
  }, [route.search])

  useEffect(() => {
    const normalized = draft.trim().slice(0, 120)
    if (normalized === route.search) return
    const timer = window.setTimeout(() => {
      navigateRoute(resetFeedPages({ ...route, search: normalized }), true)
    }, ROUTE_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [draft, navigateRoute, route])

  const submit = useCallback(() => {
    const normalized = draft.trim().slice(0, 120)
    if (normalized === route.search) return
    navigateRoute(resetFeedPages({ ...route, search: normalized }), true)
  }, [draft, navigateRoute, route])

  return { draft, setDraft, submit }
}
