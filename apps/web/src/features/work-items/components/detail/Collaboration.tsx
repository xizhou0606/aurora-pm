import {
  ActivityTimeline,
  DetailSection,
  UserPreview,
  ViewTabs,
  type ActivityTimelineState,
} from '@/shared/ui'
import { useCollaborationAnchor } from '../../hooks/useCollaborationAnchor'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useMemo } from 'react'

import type { WorkItemsDownload } from '../../model/types'
import type {
  DetailResourceEntry,
  DetailTabId,
  MutationState,
  ResourceState,
  UserOptionsState,
  WorkItemStatusTransition,
} from '../../model/types'
import { ActivitySummary, activityIcon } from './ActivitySummary'
import { CommentSection } from './CommentSection'
import { Transitions } from './Transitions'
import { workItemCollaborationUser } from '../../model/detail-presenter'

function Activity({
  entry,
  users,
}: {
  entry: DetailResourceEntry
  users: UserOptionsState
}) {
  const state = useMemo((): ActivityTimelineState => {
    if (entry.state.status === 'idle' || entry.state.status === 'loading') {
      return { status: 'loading', loadingLabel: entry.title }
    }
    if (entry.state.status === 'error') {
      return {
        status: 'error',
        errorLabel: WORK_ITEMS_COPY.detail.resourceError,
        retryLabel: WORK_ITEMS_COPY.actions.retry,
        onRetry: entry.state.retry,
      }
    }
    if (!entry.state.data.length) return { status: 'empty', emptyLabel: entry.emptyLabel }
    return {
      status: 'ready',
      emptyLabel: entry.emptyLabel,
      items: entry.state.data.map((row) => {
        const actor = users.status === 'ready'
          ? workItemCollaborationUser(row, users.data)
          : undefined
        return {
          id: row.id,
          summary: <ActivitySummary row={row} users={users.status === 'ready' ? users.data : []} />,
          icon: activityIcon(row),
          meta: row.userName ? (
            <UserPreview
              name={actor?.label || row.userName}
              secondary={actor?.username}
              avatarUrl={actor?.avatarUrl}
              size="sm"
            />
          ) : row.secondary,
          timestamp: row.timestamp,
          dateTime: row.dateTime,
        }
      }),
      warning: entry.warning?.label,
      hasMore: Boolean(entry.pagination),
      loadingMore: entry.pagination?.pending,
      loadMoreLabel: entry.pagination?.label,
      loadingMoreLabel: entry.pagination?.label,
      onLoadMore: entry.pagination?.loadMore,
    }
  }, [entry, users])
  return <ActivityTimeline state={state} />
}

export function Collaboration({
  taskId,
  activities,
  comments,
  transitions,
  commentState,
  commentUsers,
  currentUserId,
  canComment,
  canAttach,
  canUploadBinary,
  onComment,
  onUploadCommentImage,
  onDownloadAttachment,
}: {
  taskId: number
  activities: DetailResourceEntry
  comments: DetailResourceEntry
  transitions: ResourceState<WorkItemStatusTransition[]>
  commentState: MutationState
  commentUsers: UserOptionsState
  currentUserId: number
  canComment: boolean
  canAttach: boolean
  canUploadBinary: boolean
  onComment: (body: string) => Promise<boolean>
  onUploadCommentImage: (file: File) => Promise<string | null>
  onDownloadAttachment: (id: number) => Promise<WorkItemsDownload | null>
}) {
  const { tab, setTab, rootRef } = useCollaborationAnchor({resourceId: taskId, defaultTab: 'comments', revision: comments.state})
  const tabs = [
    { id: 'comments', label: WORK_ITEMS_COPY.detail.tabs.comments },
    { id: 'activity', label: WORK_ITEMS_COPY.detail.tabs.activity },
    { id: 'transitions', label: WORK_ITEMS_COPY.detail.tabs.transitions },
  ] as const

  return (
    <DetailSection id="work-items-detail-collaboration" title={WORK_ITEMS_COPY.detail.collaboration}>
      <div ref={rootRef} className="grid min-w-0 gap-4">
        <ViewTabs
          value={tab}
          tabs={tabs}
          label={WORK_ITEMS_COPY.detail.collaboration}
          onValueChange={(value) => setTab(value as DetailTabId)}
        />
        {tab === 'comments' ? (
          <CommentSection
            taskId={taskId}
            entry={comments}
            canComment={canComment}
            canAttach={canAttach}
            canUploadBinary={canUploadBinary}
            users={commentUsers}
            currentUserId={currentUserId}
            mutation={commentState}
            onComment={onComment}
            onUploadImage={onUploadCommentImage}
            onDownload={onDownloadAttachment}
          />
        ) : tab === 'activity' ? (
          <Activity entry={activities} users={commentUsers} />
        ) : (
          <Transitions state={transitions} users={commentUsers} />
        )}
      </div>
    </DetailSection>
  )
}
