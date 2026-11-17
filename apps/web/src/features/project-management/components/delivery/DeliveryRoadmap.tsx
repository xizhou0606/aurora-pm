import { copy } from '@/shared/copy'
import { EmptyState } from '@/shared/ui'

const deliveryCopy = copy.projectManagement.delivery

import type { DeliveryRoadmapState } from '../../model'
import { ReadyRoadmap } from './DeliveryRoadmapParts'
import type { DeliveryRoadmapActions } from './DeliveryRoadmapTypes'

export type { DeliveryRoadmapActions }

export function DeliveryRoadmap({
  state,
  actions = {},
}: {
  state: DeliveryRoadmapState
  actions?: DeliveryRoadmapActions
}) {
  if (state.status === 'empty') {
    return (
      <EmptyState
        title={deliveryCopy.emptyTitle}
        className="border border-dashed border-border"
      />
    )
  }
  return <ReadyRoadmap state={state} actions={actions} />
}