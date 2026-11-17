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
import {
  parseProjectSearch,
  parseProjectsSearch,
  parsePlanUnitSearch,
  hasLegacyProjectItemsSearch,
  parsePositiveRouteId,
} from '@/shared/router'
import {
  projectAuditQueryOptions,
  projectBoardQueryOptions,
  projectListQueryOptions,
  platformUsersQueryOptions,
  requirementListQueryOptions,
  taskDeepLinkQueryOptions,
  taskListQueryOptions,
} from '@/features/project-management/routes/loader'
import { prefetchBusinessQuery, startBusinessPrefetch } from './router'

export function buildProjectRoutes(rootRoute: Route<AppRouterContext>) {
  const projectsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/projects',
    validateSearch: parseProjectsSearch,
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen
        title={copy.projectManagement.title}
        variant="project-split"
      />
    ),
    loaderDeps: ({ search }) => ({ projectId: search.project_id }),
    loader: ({ context, deps }) => {
      const workspaceId = context.access.snapshot().workspaceId
      const requests = [
        prefetchBusinessQuery(context.queryClient, projectListQueryOptions(workspaceId)),
        prefetchBusinessQuery(context.queryClient, platformUsersQueryOptions(workspaceId)),
      ]
      if (deps.projectId) {
        requests.push(prefetchBusinessQuery(
          context.queryClient,
          projectBoardQueryOptions(workspaceId, deps.projectId),
        ))
      }
      startBusinessPrefetch(requests)
    },
    component: lazyRouteComponent(
      () => import('@/features/project-management'),
      'ProjectsPage',
    ),
  })

  const projectDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/projects/$projectId',
    validateSearch: parseProjectSearch,
    beforeLoad: async (input) => {
      if (!parsePositiveRouteId(input.params.projectId)) {
        throw redirect({ to: '/workspace/projects', replace: true })
      }
      if (hasLegacyProjectItemsSearch(input.location.searchStr)) {
        throw redirect({
          to: '/workspace/projects/$projectId',
          params: { projectId: input.params.projectId },
          search: input.search,
          replace: true,
        })
      }
      await requireWorkspace(input)
    },
    loaderDeps: ({ search }) => ({ tab: search.tab }),
    loader: ({ context, deps, params }) => {
      const workspaceId = context.access.snapshot().workspaceId
      const projectId = parsePositiveRouteId(params.projectId)
      if (!projectId) return
      const requests = [
        prefetchBusinessQuery(context.queryClient, projectBoardQueryOptions(workspaceId, projectId)),
        prefetchBusinessQuery(context.queryClient, projectListQueryOptions(workspaceId)),
        prefetchBusinessQuery(context.queryClient, platformUsersQueryOptions(workspaceId)),
      ]
      if (deps.tab === 'activity') {
        requests.push(prefetchBusinessQuery(context.queryClient,
          projectAuditQueryOptions(workspaceId, projectId),
        ))
      }
      startBusinessPrefetch(requests)
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen
        title={copy.projectManagement.title}
        variant="project-board"
      />
    ),
    component: lazyRouteComponent(
      () => import('@/features/project-management'),
      'ProjectDetailPage',
    ),
  })

  const projectItemDetailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/projects/$projectId/items/$projectItemId',
    validateSearch: parsePlanUnitSearch,
    beforeLoad: async (input) => {
      if (
        !parsePositiveRouteId(input.params.projectId) ||
        !parsePositiveRouteId(input.params.projectItemId)
      ) {
        throw redirect({ to: '/workspace/projects', replace: true })
      }
      await requireWorkspace(input)
    },
    loaderDeps: ({ search }) => ({
      tab: search.tab,
      req_q: search.req_q,
      req_status: search.req_status,
      req_priority: search.req_priority,
      req_page: search.req_page,
      req_page_size: search.req_page_size,
      task_requirement_id: search.task_requirement_id,
      task_id: search.task_id,
      task_page: search.task_page,
      task_page_size: search.task_page_size,
    }),
    loader: ({ context, deps, params }) => {
      const access = context.access.snapshot()
      const workspaceId = access.workspaceId
      const projectId = parsePositiveRouteId(params.projectId)
      const planUnitId = parsePositiveRouteId(params.projectItemId)
      if (!projectId || !planUnitId) return
      const requests = [
        prefetchBusinessQuery(context.queryClient, projectBoardQueryOptions(workspaceId, projectId)),
        prefetchBusinessQuery(context.queryClient, projectListQueryOptions(workspaceId)),
        prefetchBusinessQuery(context.queryClient, platformUsersQueryOptions(workspaceId)),
      ]
      if (deps.tab === 'activity') {
        requests.push(prefetchBusinessQuery(context.queryClient,
          projectAuditQueryOptions(workspaceId, projectId),
        ))
      }
      if (deps.tab === 'requirements') {
        const pageSize = deps.req_page_size ?? 50
        const page = deps.req_page ?? 1
        requests.push(prefetchBusinessQuery(context.queryClient,
          requirementListQueryOptions(workspaceId, {
            projectNewId: projectId,
            versionTaskNewId: planUnitId,
            q: deps.req_q,
            status: deps.req_status,
            priority: deps.req_priority,
            limit: pageSize,
            offset: (page - 1) * pageSize,
          }),
        ))
      }
      if (deps.tab === 'tasks' && !access.embedded) {
        const pageSize = deps.task_page_size ?? 50
        const page = deps.task_page ?? 1
        requests.push(
          prefetchBusinessQuery(context.queryClient,
            requirementListQueryOptions(workspaceId, {
              projectNewId: projectId,
              versionTaskNewId: planUnitId,
              limit: 200,
              offset: 0,
            }),
          ),
          prefetchBusinessQuery(context.queryClient,
            taskListQueryOptions(workspaceId, {
              projectNewId: projectId,
              versionTaskNewId: planUnitId,
              requirementNewId: deps.task_requirement_id,
              limit: pageSize,
              offset: (page - 1) * pageSize,
            }),
          ),
        )
        if (deps.task_id && deps.task_requirement_id) {
          requests.push(prefetchBusinessQuery(context.queryClient,
            taskDeepLinkQueryOptions(workspaceId, {
              projectNewId: projectId,
              versionTaskNewId: planUnitId,
              requirementNewId: deps.task_requirement_id,
              taskId: deps.task_id,
            }),
          ))
        }
      }
      startBusinessPrefetch(requests)
    },
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen
        title={copy.projectManagement.title}
        variant="project-board"
      />
    ),
    component: lazyRouteComponent(
      () => import('@/features/project-management'),
      'ProjectItemDetailPage',
    ),
  })

  return [projectsRoute, projectDetailRoute, projectItemDetailRoute]
}