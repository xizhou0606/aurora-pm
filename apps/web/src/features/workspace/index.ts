export { fetchWorkspaces } from './api/api'
export { useWorkspacesQuery, type WorkspacesQueryState } from './api/query'
export { resolveWorkspaceSelection } from './hooks/selection'
export {
  seedWorkspacesForCurrentSession,
  useWorkspacesStore,
  workspacesStore,
  type WorkspacesStatus,
  type WorkspacesStoreState,
} from './model/workspaces-store'
export type {
  WorkspaceOption,
  WorkspaceRouteResolution,
  WorkspaceSelection,
} from './model/types'
export { WorkspaceSelectPage } from './routes/WorkspaceSelectPage'
export {
  resolveWorkspaceAfterLogin,
  resolveWorkspaceForRoute,
  switchWorkspaceWorkflow,
  useWorkspaceSwitcher,
  type WorkspaceSwitcher,
  type WorkspaceSwitchOptions,
} from './utils/workflows'
