import { useMemo, type ReactNode } from 'react'

import { WorkItemCommentSection, useCollaborationAnchor } from '@/features/work-items'
import { QUALITY_COPY } from '@/shared/copy'
import {
  DetailSection,
  ViewTabs,
} from '@/shared/ui'

import type { QualityCommentsState } from '../../hooks'
import type { QualityComment, QualityMutationState, QualityUsersState } from '../../model/types'

type CollaborationTab = 'comments' | 'activity'

function mutationState(state: QualityMutationState) {
  if (state.status === 'pending') return { status: 'pending' as const, action: state.action }
  if (state.status === 'error') {
    return { status: 'error' as const, action: state.action, message: QUALITY_COPY.mutation.commentFailed }
  }
  if (state.status === 'success') return { status: 'success' as const, action: state.action }
  return { status: 'idle' as const }
}

export function Collaboration({
  bugId,
  currentUserId,
  activity,
  comments,
  users,
  commentMutation,
  canViewComments,
  canCreateComment,
  canDeleteComment,
  canUpload,
  onComment,
  onDeleteComment,
  onUploadImage,
}: {
  bugId: number
  currentUserId: number
  activity: ReactNode
  comments: QualityCommentsState
  users: QualityUsersState
  commentMutation: QualityMutationState
  canViewComments: boolean
  canCreateComment: boolean
  canDeleteComment: (comment: QualityComment) => boolean
  canUpload: boolean
  onComment: (body: string) => Promise<boolean>
  onDeleteComment: (id: number) => Promise<boolean>
  onUploadImage: (file: File) => Promise<string | null>
}) {
  const { tab, setTab, rootRef } = useCollaborationAnchor({resourceId: bugId, defaultTab: 'activity', canViewComments, revision: comments})
  const commentUsers = useMemo(() => {
    if (users.status === 'loading') return { status: 'loading' as const }
    if (users.status === 'error' && !users.rows.length) {
      return { status: 'error' as const, retry: () => void users.retry() }
    }
    return {
      status: 'ready' as const,
      data: users.rows.map((user) => ({
        id: user.id, label: user.name, username: user.username, avatarUrl: user.avatarUrl,
      })),
    }
  }, [users])
  const entry = useMemo(() => ({
    id: 'comments' as const,
    title: QUALITY_COPY.detail.comments,
    emptyLabel: QUALITY_COPY.detail.commentEmpty,
    state: !canViewComments || comments.status === 'idle'
      ? { status: 'idle' as const }
      : comments.status === 'loading'
        ? { status: 'loading' as const }
        : comments.status === 'error'
          ? { status: 'error' as const, retry: () => void comments.retry() }
          : {
              status: 'ready' as const,
              refresh: comments.refresh,
              data: comments.rows.map((row) => ({
                id: String(row.id), primary: row.body, secondary: row.userName,
                userId: row.userId,
                meta: row.createdAt ? QUALITY_COPY.format.dateTime(row.createdAt) : undefined,
                canDelete: canDeleteComment(row),
              })),
            },
    onDeleteRow: async (rowId: string) => onDeleteComment(Number(rowId)),
  }), [canDeleteComment, canViewComments, comments, onDeleteComment])
  const tabs = [
    ...(canViewComments ? [{ id: 'comments' as const, label: QUALITY_COPY.detail.comments }] : []),
    { id: 'activity', label: QUALITY_COPY.detail.activity },
  ] as const

  return (
    <DetailSection title={QUALITY_COPY.detail.collaboration}>
      <div ref={rootRef} className="grid min-w-0 gap-4">
        <ViewTabs
          value={tab}
          tabs={tabs}
          label={QUALITY_COPY.detail.collaboration}
          onValueChange={(value) => setTab(value as CollaborationTab)}
        />
        {tab === 'comments' && canViewComments ? (
          <WorkItemCommentSection
            taskId={bugId}
            entry={entry}
            canComment={canCreateComment}
            canAttach={canUpload}
            canUploadBinary={canUpload}
            users={commentUsers}
            currentUserId={currentUserId}
            mutation={mutationState(commentMutation)}
            onComment={onComment}
            onUploadImage={onUploadImage}
            onDownload={async () => null}
          />
        ) : activity}
      </div>
    </DetailSection>
  )
}
