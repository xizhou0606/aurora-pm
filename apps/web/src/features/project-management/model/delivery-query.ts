import { queryOptions, useQuery } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import { listScrumProjectDeliveryNodeLabelsNew } from '../api'

function positiveId(value: unknown): number {
  const id = Number(value)
  if (!Number.isInteger(id) || id <= 0) {
    throw new Error('PM delivery labels query requires a positive plan-unit ID')
  }
  return id
}

export function deliveryLabelsQueryOptions(workspaceId: number, planUnitId: number) {
  const id = positiveId(planUnitId)
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'delivery', 'labels', 'plan-unit', id),
    queryFn: ({ signal }) => listScrumProjectDeliveryNodeLabelsNew(id, signal),
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function useDeliveryLabelsQuery(planUnitId: number, enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && workspaceId > 0 && Number(planUnitId) > 0
  return useQuery({
    ...deliveryLabelsQueryOptions(valid ? workspaceId : 1, valid ? planUnitId : 1),
    enabled: valid,
  })
}
