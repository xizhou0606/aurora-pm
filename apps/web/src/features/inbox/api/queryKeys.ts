import { nativeModuleKeys } from '@/shared/query'

export function inboxKey(
  workspaceId: number,
  ...scope: readonly unknown[]
): readonly unknown[] {
  return nativeModuleKeys.resource(workspaceId, 'inbox', ...scope)
}
