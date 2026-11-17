import {
  createRoute,
  lazyRouteComponent,
  redirect,
  type Route,
} from '@tanstack/react-router'

import type { AppRouterContext } from './guards'
import { requireWorkspace } from './guards'
import { parsePositiveRouteId } from '@/shared/router'

export function buildDocumentRoutes(rootRoute: Route<AppRouterContext>) {
  const documentsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/documents',
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(
      () => import('./DocumentTableWorkspace'),
      'DocumentTableWorkspace',
    ),
  })

  const documentDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/documents/$documentId',
    beforeLoad: async (input) => {
      if (!parsePositiveRouteId(input.params.documentId)) {
        throw redirect({ to: '/workspace/documents', replace: true })
      }
      await requireWorkspace(input)
    },
    component: lazyRouteComponent(
      () => import('./DocumentTableWorkspace'),
      'DocumentTableWorkspace',
    ),
  })

  const bitableListRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/bitables',
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(() => import('./DocumentTableWorkspace'), 'DocumentTableWorkspace'),
  })
  const bitableDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/bitables/$resourceRef',
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(() => import('./DocumentTableWorkspace'), 'DocumentTableWorkspace'),
  })
  return [documentsRoute, documentDetailRoute, bitableListRoute, bitableDetailRoute]
}
