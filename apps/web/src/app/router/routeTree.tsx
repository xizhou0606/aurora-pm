import type { Route } from '@tanstack/react-router'

import type { AppRouterContext } from './guards'
import { buildAuthRoutes } from './routeTree.auth'
import { buildProjectRoutes } from './routeTree.projects'
import { buildNativeRoutes } from './routeTree.native'
import { buildAiPmRoutes } from './routeTree.ai-pm'
import { buildMiscRoutes } from './routeTree.misc'
import { buildDesignManagementRoutes } from './routeTree.design-management'
import { buildDocumentRoutes } from './routeTree.documents'

export function buildRouteTree(
  rootRoute: Route<AppRouterContext>,
) {
  return rootRoute.addChildren([
    ...buildAuthRoutes(rootRoute),
    ...buildProjectRoutes(rootRoute),
    ...buildNativeRoutes(rootRoute),
    ...buildAiPmRoutes(rootRoute),
    ...buildDesignManagementRoutes(rootRoute),
    ...buildDocumentRoutes(rootRoute),
    ...buildMiscRoutes(rootRoute),
  ])
}
