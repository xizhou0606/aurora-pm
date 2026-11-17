import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { pmKeys } from '@/shared/query'
import { bitableClient, type BitableClient, type TableScope } from '../api/client'

export type BitableEnvironment = { workspaceId: number; sessionRevision: number; client: BitableClient }
export const createBitableEnvironment = (workspaceId: number, sessionRevision: number, client = bitableClient): BitableEnvironment => ({ workspaceId, sessionRevision, client })
export function currentEnvironment(env: BitableEnvironment): boolean {
  return runtimeWorkspaceStore.getState().currentWorkspaceId === env.workspaceId && runtimeSessionStore.getState().revision === env.sessionRevision
}
export function bitableKey(env: BitableEnvironment, scope?: TableScope, ...suffix: readonly unknown[]) {
  return env.workspaceId > 0 ? pmKeys.resource(env.workspaceId, 'bitables', env.sessionRevision, scope?.resourceId ?? '', scope?.tableId ?? '', ...suffix)
    : [...pmKeys.session(), 'bitables-disabled', ...suffix]
}
