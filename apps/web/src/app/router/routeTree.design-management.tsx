import {
  createRoute,
  lazyRouteComponent,
  redirect,
  type Route,
} from '@tanstack/react-router'

import type { AppRouterContext } from './guards'
import { requireWorkspace } from './guards'
import {
  isUUID,
  parseDesignReviewListSearch,
  parseNewDesignReviewSearch,
} from '@/shared/router'

export function buildDesignManagementRoutes(rootRoute: Route<AppRouterContext>) {
  const list = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/design-management',
    validateSearch: parseDesignReviewListSearch,
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(() => import('@/features/design-management'), 'ReviewListPage'),
  })
  const create = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/design-management/reviews/new',
    validateSearch: parseNewDesignReviewSearch,
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(() => import('@/features/design-management'), 'NewReviewPage'),
  })
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/design-management/reviews/$reviewId',
    beforeLoad: async (input) => {
      if (!isUUID(input.params.reviewId)) {
        throw redirect({ to: '/workspace/design-management', replace: true })
      }
      await requireWorkspace(input)
    },
    component: lazyRouteComponent(() => import('@/features/design-management'), 'ReviewDetailPage'),
  })
  const collectors = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/design-management/mobile-collectors',
    beforeLoad: requireWorkspace,
    component: lazyRouteComponent(() => import('@/features/design-management'), 'MobileCollectorsPage'),
  })
  return [list, create, detail, collectors]
}
