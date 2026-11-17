import {
  createRoute,
  lazyRouteComponent,
  redirect,
  type Route,
} from '@tanstack/react-router'

import { copy } from '@/shared/copy'

import type { AppRouterContext } from './guards'
import { requireWorkspace } from './guards'
import { RoutePendingScreen } from './PendingScreen'
import { prefetchBusinessQuery, startBusinessPrefetch } from './router'
import {
  parseRequirementDetailSearch,
  parseRequirementPoolSearch,
  parsePositiveRouteId,
} from '@/shared/router'
import {
  requirementPoolFiltersFromSearch,
  requirementPoolQueryFilters,
  requirementPoolQueryOptions,
  requirementScopeQueryOptions,
} from '@/features/ai-pm/routes/loader'

const POOL_PAGE_SIZES = [20, 50, 100] as const

function normalizedPoolPaging(search: {
  view?: 'cards'
  page?: number
  page_size?: number
}) {
  return {
    view: search.view ?? 'list' as const,
    page: search.page && search.page > 0 ? search.page : 1,
    pageSize: search.page_size && (POOL_PAGE_SIZES as readonly number[]).includes(search.page_size)
      ? search.page_size
      : 20,
  }
}

export function buildAiPmRoutes(rootRoute: Route<AppRouterContext>) {
  const requirementsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/requirements',
    validateSearch: parseRequirementPoolSearch,
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.requirementPool.title} variant="table" />
    ),
    loaderDeps: ({ search }) => ({ search }),
    loader: ({ context, deps }) => {
      const workspaceId = context.access.snapshot().workspaceId
      const filters = requirementPoolFiltersFromSearch(deps.search)
      const paging = normalizedPoolPaging(deps.search)
      const scopeKey = requirementScopeQueryOptions(workspaceId).queryKey
      // The pool request filters depend on resolved scope options, so warm the
      // scope cache first, then reuse it to warm the exact pool entry the page
      // will read. The pool response carries the people directory, so one
      // request warms rows and people together. Everything stays non-blocking;
      // the page renders its own skeleton while these settle.
      async function prefetchWarmup() {
        await prefetchBusinessQuery(context.queryClient, requirementScopeQueryOptions(workspaceId))
        const projects = context.queryClient.getQueryData(scopeKey)?.projects ?? []
        await prefetchBusinessQuery(context.queryClient, requirementPoolQueryOptions(
          workspaceId,
          requirementPoolQueryFilters({
            filters,
            projects,
            view: paging.view,
            page: paging.page,
            pageSize: paging.pageSize,
          }),
        ))
      }
      startBusinessPrefetch([prefetchWarmup()])
    },
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/RequirementPoolPage'), 'RequirementPoolPage'),
  })

  const requirementDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/requirements/$requirementId',
    validateSearch: parseRequirementDetailSearch,
    beforeLoad: async (input) => {
      if (!parsePositiveRouteId(input.params.requirementId)) throw redirect({ to: '/workspace/requirements', replace: true })
      await requireWorkspace(input)
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.requirementDetail.title} variant="project-board" />
    ),
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/RequirementDetailPage'), 'RequirementDetailPage'),
  })

  const testingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/testing',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.testingLibrary.title} variant="table" />
    ),
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/TestingLibraryPage'), 'TestingLibraryPage'),
  })

  const workflowsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/workflows',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.workflow.title} variant="table" />
    ),
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/WorkflowListPage'), 'WorkflowListPage'),
  })

  const workflowEditorRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/workflows/$templateId',
    beforeLoad: async (input) => {
      if (!parsePositiveRouteId(input.params.templateId)) throw redirect({ to: '/workspace/pm/workflows', replace: true })
      await requireWorkspace(input)
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.workflow.title} variant="table" />
    ),
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/WorkflowEditorPage'), 'WorkflowEditorPage'),
  })

  const aiRunsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/ai-runs',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.aiPM.runs.title} variant="table" />
    ),
    component: lazyRouteComponent(() => import('@/features/ai-pm/routes/AIRunsPage'), 'AIRunsPage'),
  })

  return [requirementsRoute, requirementDetailRoute, testingRoute, workflowsRoute, workflowEditorRoute, aiRunsRoute]
}
