import { ActivityTimeline, UserPreview, type ActivityTimelineState } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { StatusChange } from './ActivitySummary'
import { PhArrowsClockwise } from '@/shared/ui/icons'
import { useMemo } from 'react'

import type {
  ResourceState,
  UserOptionsState,
  WorkItemStatusTransition,
} from '../../model/types'
import { formatDateTime } from '../../model/utils'
import { workItemCollaborationUser } from '../../model/detail-presenter'

export function Transitions({
  state,
  users,
}: {
  state: ResourceState<WorkItemStatusTransition[]>
  users: UserOptionsState
}) {
  const timelineState = useMemo((): ActivityTimelineState => {
    if (state.status === 'idle' || state.status === 'loading') {
      return { status: 'loading', loadingLabel: WORK_ITEMS_COPY.detail.transitionsLoading }
    }
    if (state.status === 'error') {
      return {
        status: 'error',
        errorLabel: WORK_ITEMS_COPY.detail.transitionsError,
        retryLabel: WORK_ITEMS_COPY.actions.retry,
        onRetry: state.retry,
      }
    }
    if (!state.data.length) return { status: 'empty', emptyLabel: WORK_ITEMS_COPY.detail.noTransitions }
    return {
      status: 'ready',
      emptyLabel: WORK_ITEMS_COPY.detail.noTransitions,
      items: state.data.map((row) => {
        const actor = users.status === 'ready'
          ? workItemCollaborationUser({ userId: row.actorId, userName: row.actorName }, users.data)
          : undefined
        return {
          id: row.id,
          summary: <StatusChange from={row.fromKey} to={row.toKey} />,
          icon: <PhArrowsClockwise />,
          meta: (
            <span className="inline-flex min-w-0 flex-wrap items-center gap-2">
              <UserPreview
                name={actor?.label || row.actorName || WORK_ITEMS_COPY.detail.systemActor}
                secondary={actor?.username}
                avatarUrl={actor?.avatarUrl}
                size="sm"
              />
              <span>{WORK_ITEMS_COPY.detail.transitionMeta('', '', row.automatic)}</span>
            </span>
          ),
          timestamp: formatDateTime(row.createdAt),
          dateTime: row.createdAt,
          tone: ['done', 'completed'].includes(row.toKey) ? 'positive' as const : 'default' as const,
        }
      }),
    }
  }, [state, users])
  return <ActivityTimeline state={timelineState} />
}
