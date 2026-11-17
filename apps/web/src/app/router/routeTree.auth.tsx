import {
  createRoute,
  lazyRouteComponent,
  type Route,
} from '@tanstack/react-router'

import type { AppRouterContext } from './guards'
import {
  redirectAuthenticatedFromLogin,
  requireStandalone,
  requireRestrictedIdentity,
  requireStandaloneSelection,
} from './guards'
import {
  parseLoginSearch,
  parseProfileSearch,
  parseWorkspaceSearch,
} from '@/shared/router'

export function buildAuthRoutes(rootRoute: Route<AppRouterContext>) {
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: lazyRouteComponent(
      () => import('@/features/landing'),
      'LandingPage',
    ),
  })

  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    validateSearch: parseLoginSearch,
    beforeLoad: redirectAuthenticatedFromLogin,
    component: lazyRouteComponent(
      () => import('@/features/auth'),
      'LoginPage',
    ),
  })

  const passwordResetRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/reset-password',
    component: lazyRouteComponent(
      () => import('@/features/auth'),
      'PasswordResetPage',
    ),
  })

  const identityStatusRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/identity-status',
    beforeLoad: requireRestrictedIdentity,
    component: lazyRouteComponent(
      () => import('@/features/auth'),
      'IdentityStatusPage',
    ),
  })

  const workspaceSelectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/select-workspace',
    validateSearch: parseWorkspaceSearch,
    beforeLoad: requireStandaloneSelection,
    component: lazyRouteComponent(
      () => import('@/features/workspace'),
      'WorkspaceSelectPage',
    ),
  })

  const profileRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/profile',
    validateSearch: parseProfileSearch,
    beforeLoad: requireStandalone,
    component: lazyRouteComponent(
      () => import('@/features/account'),
      'ProfilePage',
    ),
  })

  return [indexRoute, loginRoute, passwordResetRoute, identityStatusRoute, workspaceSelectRoute, profileRoute]
}
