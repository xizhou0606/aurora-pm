import { nativeModuleKeys } from '@/shared/query'

export function biScreenKey(
  workspaceId: number,
  ...scope: readonly unknown[]
): readonly unknown[] {
  return nativeModuleKeys.resource(workspaceId, 'biScreen', ...scope)
}
