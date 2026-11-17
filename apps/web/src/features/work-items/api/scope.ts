import { normalizePMScopeOptions, type PMScopeOptions } from '@/shared/api'
import type { WorkItemsClient } from './client'

/**
 * The project-set -> project -> requirement hierarchy is the only task-page
 * data-scope contract. Source selection never swaps it for classic team scope.
 */
export async function requestScopeOptions(
  client: WorkItemsClient,
  signal?: AbortSignal,
): Promise<PMScopeOptions> {
  return normalizePMScopeOptions(await client.requestPMScopeOptions<unknown>(signal))
}
