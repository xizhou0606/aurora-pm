import type { QueryKey } from '@tanstack/react-query'

export type PMQueryResource =
  | 'auth'
  | 'workspace'
  | 'projectSets'
  | 'planUnits'
  | 'workflowTemplates'
  | 'requirements'
  | 'tasks'
  | 'delivery'
  | 'audit'
  | 'inbox'
  | 'globalWorkItems'
  | 'biScreen'
  | 'quality'
  | 'designReviews'
  | 'documents'

export const nativeModuleQueryResources = [
  'inbox',
  'globalWorkItems',
  'biScreen',
  'quality',
] as const satisfies readonly PMQueryResource[]

export type NativeModuleQueryResource = (typeof nativeModuleQueryResources)[number]

function positiveWorkspaceId(value: unknown): number {
  const workspaceId = Number(value || 0)
  if (!Number.isInteger(workspaceId) || workspaceId <= 0) {
    throw new Error('PM query keys require a positive workspace ID')
  }
  return workspaceId
}

export const pmKeys = {
  all: ['pm'] as const,
  session: () => ['pm', 'session'] as const,
  me: () => ['pm', 'session', 'auth', 'me'] as const,
  account: () => ['pm', 'session', 'auth', 'account'] as const,
  workspaces: () => ['pm', 'session', 'workspaces'] as const,
  workspaceRoot: () => ['pm', 'workspace'] as const,
  workspace: (workspaceId: number) =>
    ['pm', 'workspace', positiveWorkspaceId(workspaceId)] as const,
  resource: (
    workspaceId: number,
    resource: PMQueryResource,
    ...scope: QueryKey
  ) =>
    [
      'pm',
      'workspace',
      positiveWorkspaceId(workspaceId),
      resource,
      ...scope,
    ] as const,
}

export const nativeModuleKeys = {
  root: (workspaceId: number, resource: NativeModuleQueryResource) =>
    pmKeys.resource(workspaceId, resource),
  resource: (
    workspaceId: number,
    resource: NativeModuleQueryResource,
    ...scope: QueryKey
  ) => pmKeys.resource(workspaceId, resource, ...scope),
}
