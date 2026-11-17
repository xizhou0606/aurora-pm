import { useMemo } from 'react'

import { withWorkItemsRuntimeFailure } from '../api/client'
import type { WorkItemsEnvironment } from '../model/environment'

export function useWorkItemsRuntimeEnvironment(
  baseEnvironment: WorkItemsEnvironment,
  onRuntimeFailure?: (error: unknown) => void,
): WorkItemsEnvironment {
  const client = useMemo(
    () => withWorkItemsRuntimeFailure(baseEnvironment.client, onRuntimeFailure),
    [baseEnvironment.client, onRuntimeFailure],
  )
  return useMemo(
    () => ({ ...baseEnvironment, client }),
    [baseEnvironment, client],
  )
}
