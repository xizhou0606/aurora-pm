import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

export type BiScreenOperationAccess = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
  viewOverview: boolean
  viewTeamDetails: boolean
  updateScoring: boolean
  viewMetrics: boolean
  updateMetrics: boolean
}

export function useBiScreenOperationAccess(): BiScreenOperationAccess {
  const operations = useOperationCapabilities('bi-screen')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    viewOverview: operations.allows(PM_ACTION.BiScreenOverviewView),
    viewTeamDetails: operations.allows(PM_ACTION.BiScreenTeamDetailsView),
    updateScoring: operations.allows(PM_ACTION.BiScreenScoringUpdate),
    viewMetrics: operations.allows(PM_ACTION.BiScreenMetricsView),
    updateMetrics: operations.allows(PM_ACTION.BiScreenMetricsUpdate),
  }), [operations.allows, operations.retry, operations.status])
}
