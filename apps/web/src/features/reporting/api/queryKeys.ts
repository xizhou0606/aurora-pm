import { nativeModuleKeys } from '@/shared/query'

export function reportingKeys(workspaceId: number) {
  return {
    root: nativeModuleKeys.root(workspaceId, 'reporting'),
    directory: () => nativeModuleKeys.resource(workspaceId, 'reporting', 'directory'),
    preview: (memberId: number, type: string, anchorDate: string) =>
      nativeModuleKeys.resource(workspaceId, 'reporting', 'preview', memberId, type, anchorDate),
    run: (runId: number) => nativeModuleKeys.resource(workspaceId, 'reporting', 'run', runId),
  }
}
