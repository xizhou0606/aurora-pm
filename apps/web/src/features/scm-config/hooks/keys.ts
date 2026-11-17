import { nativeModuleKeys } from '@/shared/query'

export function scmConfigKeys(workspaceId: number) {
  return {
    root: nativeModuleKeys.root(workspaceId, 'scm-config'),
    connections: () => nativeModuleKeys.resource(workspaceId, 'scm-config', 'connections'),
    repositories: () => nativeModuleKeys.resource(workspaceId, 'scm-config', 'repositories'),
    bindings: (status?: string) =>
      nativeModuleKeys.resource(workspaceId, 'scm-config', 'bindings', status ?? 'all'),
    settings: () => nativeModuleKeys.resource(workspaceId, 'scm-config', 'settings'),
    members: () => nativeModuleKeys.resource(workspaceId, 'scm-config', 'members'),
  }
}
