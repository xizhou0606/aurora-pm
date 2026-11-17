import { QualityActivityChange, qualityActivityIcon } from './ActivityChange'
import type { QualityUsersState } from '../../model/types'
import { qualityActivitySummary, qualityActivityUser } from '../../model/presenters'
import { QUALITY_COPY } from '@/shared/copy'
import { UserPreview, type ActivityTimelineState } from '@/shared/ui'
import type { PMScopeOptions } from '@/shared/api'
import { activityActionLabel } from './ActivitySummary'
import type { QualityActivitiesState } from '../../hooks'

export function activityTimelineState(
  state: QualityActivitiesState,
  users: QualityUsersState,
  scopeOptions: PMScopeOptions | null,
  onPreview: (url: string, trigger: HTMLElement) => void,
): ActivityTimelineState {
  if (state.status === 'idle' || state.status === 'loading') {
    return { status: 'loading', loadingLabel: QUALITY_COPY.detail.activityLoading }
  }
  if (state.status === 'error') {
    return {
      status: 'error',
      errorLabel: QUALITY_COPY.detail.activityError,
      retryLabel: QUALITY_COPY.detail.retry,
      onRetry: () => void state.retry(),
    }
  }
  if (!state.rows.length) {
    return { status: 'empty', emptyLabel: QUALITY_COPY.detail.activityEmpty }
  }
  return {
    status: 'ready',
    items: state.rows.map((activity) => {
      const user = qualityActivityUser(activity, users.rows)
      return {
        id: activity.id,
        icon: qualityActivityIcon(activity),
        summary: (
          <QualityActivityChange
            activity={activity}
            users={users.rows}
            fallback={qualityActivitySummary(activity, activityActionLabel(activity.action), users.rows, scopeOptions)}
            onPreview={onPreview}
          />
        ),
        meta: (
          <UserPreview
            name={user?.name || activity.userName || QUALITY_COPY.list.unknown}
            secondary={user?.username}
            avatarUrl={user?.avatarUrl}
            size="sm"
          />
        ),
        timestamp: activity.createdAt
          ? QUALITY_COPY.format.dateTime(activity.createdAt)
          : undefined,
        dateTime: activity.createdAt,
      }
    }),
    emptyLabel: QUALITY_COPY.detail.activityEmpty,
    warning: state.refresh === 'stale_error' ? QUALITY_COPY.detail.activityError : undefined,
    hasMore: state.hasMore,
    loadingMore: state.loadingMore,
    loadMoreLabel: QUALITY_COPY.detail.loadMore,
    loadingMoreLabel: QUALITY_COPY.detail.activityLoading,
    onLoadMore: () => void state.loadMore(),
  }
}