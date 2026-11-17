import {
  createRoute,
  lazyRouteComponent,
  type Route,
} from '@tanstack/react-router'

import type { AppRouterContext } from './guards'
import { requireWorkspace } from './guards'
import { RoutePendingScreen } from './PendingScreen'
import { ACCESS_MANAGEMENT_COPY, SCM_CONFIG_COPY, copy } from '@/shared/copy'
import {
  parseAuthorizeSearch,
  parseDeviceAuthorizeSearch,
} from '@/features/mcp-cli'

export function buildMiscRoutes(rootRoute: Route<AppRouterContext>) {
  // Public standalone page (no workspace guard): the shareable release notes
  // link, e.g. opened from the version info dialog.
  const releaseNotesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/release-notes',
    component: lazyRouteComponent(
      () => import('@/features/release-notes'),
      'ReleaseNotesPage',
    ),
  })

  const mcpCliRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/mcp-cli',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.mcpCli.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/mcp-cli'),
      'McpCliPage',
    ),
  })

  const mcpCliAuthorizeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/mcp-cli/authorize',
    validateSearch: parseAuthorizeSearch,
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.mcpCli.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/mcp-cli'),
      'McpCliAuthorizePage',
    ),
  })

  const mcpCliDeviceAuthorizeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/mcp-cli/device',
    validateSearch: parseDeviceAuthorizeSearch,
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.mcpCli.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/mcp-cli'),
      'McpCliDeviceAuthorizePage',
    ),
  })

  const hostNavigationRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/host-navigation',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={copy.hostNavigation.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/host-navigation'),
      'HostNavigationPage',
    ),
  })

  const accessManagementRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/access-management',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={ACCESS_MANAGEMENT_COPY.page.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/access-management'),
      'AccessManagementPage',
    ),
  })

  const workspaceManagementRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/workspace-management',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => (
      <RoutePendingScreen title={ACCESS_MANAGEMENT_COPY.page.title} variant="table" />
    ),
    component: lazyRouteComponent(
      () => import('@/features/access-management'),
      'WorkspaceManagementPage',
    ),
  })

  const scmIntegrationRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/workspace/pm/scm-integration',
    beforeLoad: requireWorkspace,
    pendingMs: 120,
    pendingComponent: () => <RoutePendingScreen title={SCM_CONFIG_COPY.page.title} variant="table" />,
    component: lazyRouteComponent(() => import('@/features/scm-config'), 'ScmConfigPage'),
  })

  return [
    releaseNotesRoute,
    mcpCliRoute,
    mcpCliAuthorizeRoute,
    mcpCliDeviceAuthorizeRoute,
    hostNavigationRoute,
    accessManagementRoute,
    workspaceManagementRoute,
    scmIntegrationRoute,
  ]
}
