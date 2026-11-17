import { useEffect, useMemo, useState } from 'react'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { usePlatformUsersQuery } from '@/shared/query'
import type {
  DocumentCommentAnchorView,
  DocumentCommentAuthor,
  DocumentCommentCapabilities,
  DocumentCommentMutationIntent,
  DocumentCommentMutationState,
  DocumentCommentsComposer,
  DocumentCommentsControl,
  DocumentCommentsViewState,
  DocumentCommentThreadRecord,
  DocumentCommentThreadView,
} from '../model/comment'
import type { DocumentCapabilities } from '../model/types'
import {
  useCreateDocumentCommentThreadMutation,
  useDeleteDocumentCommentMutation,
  useDocumentCommentThreadsQuery,
  useEditDocumentCommentMutation,
  useReopenDocumentCommentThreadMutation,
  useReplyDocumentCommentMutation,
  useResolveDocumentCommentThreadMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'

export type DocumentCommentsActor = {
  id: number
  name: string
  username?: string
  avatarUrl?: string
}

export type DocumentCommentsWorkflow = {
  state: DocumentCommentsViewState
  mutation: DocumentCommentMutationState
  capabilities: DocumentCommentCapabilities
  composer: DocumentCommentsComposer
  control: DocumentCommentsControl
}

const EMPTY_COMMENT_THREADS: readonly DocumentCommentThreadRecord[] = []

function dateLabel(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function useDocumentCommentsWorkflow(input: {
  environment: DocumentsEnvironment
  documentId: number
  documentCapabilities?: DocumentCapabilities
  actor?: DocumentCommentsActor
}): DocumentCommentsWorkflow {
  const { environment, documentId, documentCapabilities, actor } = input
  const canView = Boolean(documentCapabilities?.canViewComments)
  const query = useDocumentCommentThreadsQuery(environment, documentId, canView)
  const records = query.status === 'ready' ? query.data.items : EMPTY_COMMENT_THREADS
  const users = usePlatformUsersQuery(
    environment.workspaceId,
    canView,
  )
  const create = useCreateDocumentCommentThreadMutation(environment)
  const reply = useReplyDocumentCommentMutation(environment)
  const edit = useEditDocumentCommentMutation(environment)
  const remove = useDeleteDocumentCommentMutation(environment)
  const resolve = useResolveDocumentCommentThreadMutation(environment)
  const reopen = useReopenDocumentCommentThreadMutation(environment)
  const [mutation, setMutation] = useState<DocumentCommentMutationState>({ status: 'idle' })
  const [anchor, setAnchor] = useState<DocumentCommentAnchorView>()
  const [optimisticThreadStatuses, setOptimisticThreadStatuses] = useState<ReadonlyMap<number, 'open' | 'resolved'>>(
    () => new Map(),
  )

  useEffect(() => {
    setAnchor(undefined)
    setMutation({ status: 'idle' })
    setOptimisticThreadStatuses(new Map())
  }, [documentId])

  useEffect(() => {
    setOptimisticThreadStatuses((current) => {
      let changed = false
      const next = new Map(current)
      for (const record of records) {
        if (current.get(record.id) !== record.status) continue
        next.delete(record.id)
        changed = true
      }
      return changed ? next : current
    })
  }, [records])

  const directory = useMemo(() => {
    const byID = new Map<number, DocumentCommentAuthor>((users.data ?? []).map((user) => [user.id, {
      id: user.id,
      name: user.display_name?.trim() || user.username,
      username: user.username,
      avatarUrl: user.avatar_url,
    }]))
    if (actor?.id) byID.set(actor.id, actor)
    return byID
  }, [actor, users.data])

  const threads = useMemo<readonly DocumentCommentThreadView[]>(() => records.map((thread) => ({
    id: thread.id,
    status: optimisticThreadStatuses.get(thread.id) ?? thread.status,
    anchor: thread.anchor,
    canResolve: thread.canResolve,
    canReopen: thread.canReopen,
    comments: thread.comments.map((comment) => ({
      id: comment.id,
      ...(comment.parentCommentId ? { parentCommentId: comment.parentCommentId } : {}),
      author: comment.author ?? (actor?.id === comment.authorUserId ? actor : undefined) ?? {
        id: comment.authorUserId,
        name: DOCUMENTS_COPY.editor.memberName(comment.authorUserId),
      },
      body: comment.body,
      deleted: Boolean(comment.deletedAt),
      createdAtLabel: dateLabel(comment.createdAt),
      ...(comment.updatedAt !== comment.createdAt ? { updatedAtLabel: dateLabel(comment.updatedAt) } : {}),
      canEdit: comment.canEdit,
      canDelete: comment.canDelete,
    })),
  })), [actor, optimisticThreadStatuses, records])
  const state: DocumentCommentsViewState = !canView || documentId <= 0
    ? { status: 'hidden' }
    : query.status === 'disabled' || query.status === 'loading'
      ? { status: 'loading' }
      : query.status === 'error'
        ? { status: 'error' }
        : threads.length
          ? { status: 'ready', threads }
          : { status: 'empty' }

  const run = async (
    intent: DocumentCommentMutationIntent,
    targetId: number | undefined,
    action: () => Promise<unknown>,
  ): Promise<boolean> => {
    setMutation({ status: 'pending', intent, ...(targetId ? { targetId } : {}) })
    try {
      await action()
      setMutation({ status: 'idle' })
      return true
    } catch {
      setMutation({ status: 'error', intent, ...(targetId ? { targetId } : {}) })
      return false
    }
  }

  const control: DocumentCommentsControl = {
    retry: () => { if (query.status === 'error') void query.retry() },
    selectAnchor: setAnchor,
    createThread: async ({ anchor: nextAnchor, body }) => {
      const created = await run('create', undefined, () => create.run({
        documentId,
        input: { anchor: nextAnchor, body },
      }))
      if (created) setAnchor(undefined)
      return created
    },
    reply: ({ threadId, parentCommentId, body }) => run('reply', threadId, () => reply.run({
      documentId,
      threadId,
      input: { parentCommentId, body },
    })),
    edit: ({ commentId, body }) => run('edit', commentId, () => edit.run({ documentId, commentId, body })),
    remove: (commentId) => run('delete', commentId, () => remove.run({ documentId, commentId })),
    resolve: async (threadId) => {
      setOptimisticThreadStatuses((current) => new Map(current).set(threadId, 'resolved'))
      const succeeded = await run('resolve', threadId, () => resolve.run({ documentId, threadId }))
      if (!succeeded) {
        setOptimisticThreadStatuses((current) => {
          const next = new Map(current)
          next.delete(threadId)
          return next
        })
      }
      return succeeded
    },
    reopen: async (threadId) => {
      setOptimisticThreadStatuses((current) => new Map(current).set(threadId, 'open'))
      const succeeded = await run('reopen', threadId, () => reopen.run({ documentId, threadId }))
      if (!succeeded) {
        setOptimisticThreadStatuses((current) => {
          const next = new Map(current)
          next.delete(threadId)
          return next
        })
      }
      return succeeded
    },
  }

  return {
    state,
    mutation,
    capabilities: {
      canCreate: Boolean(documentCapabilities?.canCreateComments),
      canReply: Boolean(documentCapabilities?.canReplyComments),
    },
    composer: {
      author: actor ?? { id: 0, name: DOCUMENTS_COPY.comments.currentUser },
      mentions: [...directory.values()].map((user) => ({
        id: user.id,
        label: user.name,
        username: user.username,
        avatarUrl: user.avatarUrl,
      })),
      anchor,
    },
    control,
  }
}
