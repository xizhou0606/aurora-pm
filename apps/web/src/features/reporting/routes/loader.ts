import type { QueryClient } from '@tanstack/react-query'

import { reportingKeys } from '../api/queryKeys'
import { reportingClient } from '../api'
import { normalizeDirectory } from '../api/normalizers'

/**
 * Warms the reporting directory cache entry the page reads on mount. The
 * router loader calls this non-blockingly; the pending screen covers the wait.
 */
export async function prefetchReportingData(
  queryClient: QueryClient,
  workspaceId: number,
): Promise<void> {
  if (!Number.isInteger(workspaceId) || workspaceId <= 0) return
  await queryClient.ensureQueryData({
    queryKey: reportingKeys(workspaceId).directory(),
    queryFn: () => reportingClient.directory().then(normalizeDirectory),
    staleTime: 30_000,
  })
}
