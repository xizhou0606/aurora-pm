import {
  createRoute,
  redirect,
  type Route,
} from '@tanstack/react-router'

import { QUALITY_COPY, WORK_ITEMS_COPY, INBOX_COPY, BI_SCREEN_COPY, REPORTING_COPY } from '@/shared/copy'

import type { AppRouterContext } from './guards'
import { requireWorkspace } from './guards'
import { RoutePendingScreen } from './PendingScreen'
import { startBusinessPrefetch } from './router'
import {
  parseInboxSearch,
  parseWorkItemsSearch,
  parseBiScreenSearch,
  parseQualitySearch,
  parseQualityCreateSearch,
  parseQualityDetailSearch,
  parsePositiveRouteId,
} from '@/shared/router'
import { NativeOnlyModuleBoundary } from '@/app/native-modules'
import { preloadLocalNativeModule } from '@/app/native-modules'
import type { PMBusinessModuleId } from '@/shared/config'

function preloadNativeRoute(moduleId: PMBusinessModuleId): Promise<void> {
  return preloadLocalNativeModule(moduleId).catch(() => undefined)
}

export function buildNativeRoutes(rootRoute: Route<AppRouterContext>) {
  const inboxRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/inbox',
    validateSearch: parseInboxSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('inbox')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={INBOX_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="inbox" />,
  })

  const workItemsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/work-items',
    validateSearch: parseWorkItemsSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('workItems')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={WORK_ITEMS_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="workItems" />,
  })

  const workItemDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/work-items/$taskId',
    validateSearch: parseWorkItemsSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('workItems')
      await Promise.all([preload, requireWorkspace(input)])
      if (!parsePositiveRouteId(input.params.taskId)) {
        throw redirect({
          to: '/workspace/work-items',
          search: { view: 'list' },
          replace: true,
        })
      }
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={WORK_ITEMS_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="workItems" />,
  })

  const biScreenRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/bi-screen',
    validateSearch: parseBiScreenSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('biScreen')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={BI_SCREEN_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="biScreen" />,
  })

  const qualityRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/quality-management',
    validateSearch: parseQualitySearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('quality')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={QUALITY_COPY.page.title} variant="table" />
    ),
    loaderDeps: ({ search }) => ({ search }),
    loader: ({ context, deps }) => {
      const workspaceId = context.access.snapshot().workspaceId
      // The prefetch factories live inside the encapsulated feature and ship
      // with the quality chunk; beforeLoad has already warmed that chunk, so
      // this dynamic import resolves immediately and warms the scope + list
      // cache entries the page reads on mount. Non-blocking: the pending
      // screen covers the wait.
      startBusinessPrefetch([
        import('@/features/quality-management/routes/loader')
          .then(({ prefetchQualityListData }) => prefetchQualityListData(
            context.queryClient,
            workspaceId,
            deps.search as Readonly<Record<string, unknown>>,
          ))
          .catch(() => undefined),
      ])
    },
    component: () => <NativeOnlyModuleBoundary moduleId="quality" />,
  })

  const qualityCreateRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/quality-management/bugs/new',
    validateSearch: parseQualityCreateSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('quality')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={QUALITY_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="quality" />,
  })

  const qualityDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/quality-management/bugs/$bugId',
    validateSearch: parseQualityDetailSearch,
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('quality')
      await Promise.all([preload, requireWorkspace(input)])
      if (!parsePositiveRouteId(input.params.bugId)) {
        throw redirect({ to: '/workspace/quality-management', replace: true })
      }
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={QUALITY_COPY.page.title} variant="table" />
    ),
    component: () => <NativeOnlyModuleBoundary moduleId="quality" />,
  })

  const reportingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/reporting',
    beforeLoad: async (input) => {
      const preload = preloadNativeRoute('reporting')
      await Promise.all([preload, requireWorkspace(input)])
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={REPORTING_COPY.page.title} variant="table" />
    ),
    loader: ({ context }) => {
      const workspaceId = context.access.snapshot().workspaceId
      startBusinessPrefetch([
        import('@/features/reporting/routes/loader')
          .then(({ prefetchReportingData }) => prefetchReportingData(context.queryClient, workspaceId))
          .catch(() => undefined),
      ])
    },
    component: () => <NativeOnlyModuleBoundary moduleId="reporting" />,
  })

  return [
    inboxRoute,
    workItemsRoute,
    workItemDetailRoute,
    biScreenRoute,
    reportingRoute,
    qualityRoute,
    qualityCreateRoute,
    qualityDetailRoute,
  ]
}