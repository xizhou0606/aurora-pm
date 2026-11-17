import { pmKeys, nativeModuleKeys } from '@/shared/query'
import type { QueryClient } from '@tanstack/react-query'

export function workItemsKey(
  workspaceId: number,
  ...scope: readonly unknown[]
): readonly unknown[] {
  return nativeModuleKeys.resource(workspaceId, 'globalWorkItems', ...scope)
}

// Requirement/plan-unit task progress is aggregated server-side from the task
// table. Task mutations must invalidate those read caches so pages such as the
// requirement pool recompute progress instead of showing stale counts.
// The 'ai-pm' prefixes must stay in sync with features/ai-pm/api/pool.ts keys.
export function invalidateTaskProgressReads(client: QueryClient, workspaceId: number): void {
  void client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'requirements') })
  void client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'planUnits') })
  void client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirements'] })
  void client.invalidateQueries({ queryKey: ['ai-pm', workspaceId, 'requirement'] })
}
