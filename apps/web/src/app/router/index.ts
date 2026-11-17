export { RouteBoundary, type RouteBoundaryId } from './Boundary'
export {
  EmbeddedSessionUnavailableError,
  redirectAuthenticatedFromLogin,
  requireAuthentication,
  requireStandalone,
  requireStandaloneSelection,
  requireWorkspace,
  routeAccess,
  type AppRouterContext,
  type RouteAccess,
  type RouteAccessSnapshot,
} from './guards'
export { createAppRouter, router } from './router'
export { installHostRouteSync, type HostRouteSync } from './host'
export {
  installSessionRouteLifecycle,
  type SessionRouteLifecycle,
} from './lifecycle'
export {
  hasLegacyProjectItemsSearch,
  parseLoginSearch,
  parseProfileSearch,
  parseBiScreenSearch,
  parseInboxSearch,
  parsePlanUnitSearch,
  parsePositiveRouteId,
  parseProjectsSearch,
  parseProjectSearch,
  parseQualityCreateSearch,
  parseQualityDetailSearch,
  parseQualitySearch,
  parseWorkItemsSearch,
  parseWorkspaceSearch,
  projectDetailTabs,
  projectSearchTabs,
  type ProjectDetailTab,
  type ProjectSearchTab,
} from '@/shared/router'
