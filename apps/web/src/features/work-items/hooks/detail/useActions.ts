import { useCallback } from 'react'

import type { WorkItemsEnvironment } from '../../model/environment'
import type {
  DetailViewState,
  BatchMutationResult,
  WorkItemChildCreatePayload,
  UserOptionsState,
  WorkItem,
  WorkItemDetailPatch,
  WorkItemsDownload,
} from '../../model/types'
import { isValidEstimatedMinutes } from '../../model/utils'
import { downloadWorkItemAttachment, uploadWorkItemRichTextImage } from '../../api/write-batch'
import { isWorkItemInlineImage } from '../../model/attachments'
import { toMutationState } from '../mutationState'
import { requireWorkItemsAction } from '../useOperationAccess'
import type { WorkItemDetailMutationBundle } from './useMutationBundle'

const MAX_ATTACHMENT_SIZE = 32 * 1024 * 1024
const MAX_COMMENT_IMAGE_SIZE = 8 * 1024 * 1024

function validDate(value: string | null | undefined): boolean {
  return value === null || value === undefined || /^\d{4}-\d{2}-\d{2}$/.test(value)
}

export function useWorkItemDetailActions(input: {
  environment: WorkItemsEnvironment
  detail: DetailViewState<WorkItem>
  commentUsers: UserOptionsState
  allowedStatusTransitions: readonly string[]
  mutations: WorkItemDetailMutationBundle
}) {
  const {
    environment,
    detail,
    commentUsers,
    allowedStatusTransitions,
    mutations,
  } = input
  const {
    attachmentDelete,
    attachmentUpload,
    childCreate,
    commentMutation,
    descriptionMutation,
    fieldMutation,
    titleMutation,
    watchMutation,
  } = mutations

  const canUpdate = detail.status === 'ready' && detail.data.capabilities.canUpdate
  const canTransition = detail.status === 'ready'
    && detail.data.capabilities.allowedTransitions.length > 0
  const canComment = detail.status === 'ready' && detail.data.capabilities.canComment
  const canAttach = detail.status === 'ready' && detail.data.capabilities.canAttach
  const canWatch = detail.status === 'ready' && detail.data.capabilities.canWatch
  const canCreateChild = Boolean(
    environment.allowed
    && environment.createAllowed
    && environment.operationAccess.createTask
    && environment.workspaceId > 0
    && detail.status === 'ready'
    && detail.data.kind?.trim().toLowerCase() === 'task'
    && detail.data.requirementId,
  )

  const addComment = useCallback(async (body: string) => {
    const normalized = body.trim()
    if (!normalized || !canComment) return false
    await commentMutation.mutateAsync(normalized)
    return true
  }, [canComment, commentMutation])
  const uploadAttachment = useCallback(async (file: File) => {
    if (!canAttach || !environment.client.upload || file.size <= 0 || file.size > MAX_ATTACHMENT_SIZE) return false
    attachmentDelete.reset()
    try {
      await attachmentUpload.mutateAsync(file)
      return true
    } catch {
      return false
    }
  }, [attachmentDelete, attachmentUpload, canAttach, environment.client.upload])
  const uploadCommentImage = useCallback(async (file: File) => {
    if (detail.status !== 'ready' || !environment.client.upload || !isWorkItemInlineImage(file)
      || file.size > MAX_COMMENT_IMAGE_SIZE) return null
    requireWorkItemsAction(canComment && canAttach)
    return uploadWorkItemRichTextImage(environment.client, detail.data.requirementId, file)
  }, [canAttach, canComment, detail, environment.client])
  const uploadRichTextImage = useCallback(async (file: File) => {
    if (detail.status !== 'ready' || !environment.client.upload || !isWorkItemInlineImage(file)) return null
    requireWorkItemsAction(canUpdate && canAttach)
    return uploadWorkItemRichTextImage(environment.client, detail.data.requirementId, file)
  }, [canAttach, canUpdate, detail, environment.client])
  const deleteAttachment = useCallback(async (attachmentId: number) => {
    if (!environment.operationAccess.deleteAttachment) return false
    attachmentUpload.reset()
    await attachmentDelete.mutateAsync(attachmentId)
    return true
  }, [attachmentDelete, attachmentUpload, environment.operationAccess.deleteAttachment])
  const downloadAttachment = useCallback(async (attachmentId: number): Promise<WorkItemsDownload | null> => {
    if (!environment.operationAccess.downloadAttachment
      || !environment.client.download || !Number.isInteger(attachmentId) || attachmentId <= 0) return null
    return downloadWorkItemAttachment(environment.client, attachmentId)
  }, [environment])
  const toggleWatch = useCallback(async () => {
    if (!canWatch || detail.status !== 'ready') return false
    await watchMutation.mutateAsync(!detail.data.watched)
    return true
  }, [canWatch, detail, watchMutation])
  const createChildren = useCallback(async (
    drafts: readonly WorkItemChildCreatePayload[],
  ): Promise<BatchMutationResult> => {
    if (!canCreateChild || !drafts.length) {
      return { succeeded: 0, failures: [] }
    }
    return childCreate.mutateAsync(drafts)
  }, [canCreateChild, childCreate])
  const saveTitle = useCallback(async (title: string) => {
    const normalized = title.trim().slice(0, 200)
    if (!canUpdate || !normalized) return false
    try {
      await titleMutation.mutateAsync({ title: normalized })
      return true
    } catch {
      return false
    }
  }, [canUpdate, titleMutation])
  const saveDescription = useCallback(async (description: string) => {
    if (!canUpdate) return false
    try {
      await descriptionMutation.mutateAsync({ description })
      return true
    } catch {
      return false
    }
  }, [canUpdate, descriptionMutation])
  const saveField = useCallback(async (patch: WorkItemDetailPatch) => {
    const fields = Object.keys(patch)
    const statusOnly = fields.length === 1 && fields[0] === 'status'
    if ((!canUpdate && !(canTransition && statusOnly)) || detail.status !== 'ready') return false
    if (patch.title !== undefined && !patch.title.trim()) return false
    if (patch.status !== undefined
      && !allowedStatusTransitions.includes(patch.status.trim().toLowerCase())) return false
    if (patch.priority !== undefined
      && !['low', 'normal', 'high', 'urgent'].includes(patch.priority)) return false
    if (patch.assigneeId !== undefined && patch.assigneeId !== null) {
      if (commentUsers.status !== 'ready'
        || !commentUsers.data.some((user) => user.id === patch.assigneeId)) return false
    }
    if (!validDate(patch.startsAt) || !validDate(patch.dueAt)) return false
    const startsAt = patch.startsAt === undefined ? detail.data.startsAt : patch.startsAt || undefined
    const dueAt = patch.dueAt === undefined ? detail.data.dueAt : patch.dueAt || undefined
    if (startsAt && dueAt && startsAt > dueAt) return false
    if (!isValidEstimatedMinutes(patch.estimatedMinutes)) return false
    try {
      await fieldMutation.mutateAsync(patch)
      return true
    } catch {
      return false
    }
  }, [allowedStatusTransitions, canTransition, canUpdate, commentUsers, detail, fieldMutation])

  return {
    addComment,
    uploadAttachment,
    uploadCommentImage,
    uploadRichTextImage,
    deleteAttachment,
    downloadAttachment,
    toggleWatch,
    createChildren,
    saveTitle,
    saveDescription,
    saveField,
    canCreateChild,
    canComment,
    canAttach,
    canWatch,
    canClone: detail.status === 'ready' && detail.data.capabilities.canClone,
    canUploadAttachments: Boolean(canAttach && environment.client.upload),
    canDownloadAttachments: Boolean(environment.operationAccess.downloadAttachment && environment.client.download),
    canDeleteAttachments: environment.operationAccess.deleteAttachment,
    commentState: canComment
      ? toMutationState({ status: commentMutation.status, action: 'comment' })
      : { status: 'unavailable' as const, reason: 'unsupported' as const },
    attachmentState: !canAttach && !canUpdate
      ? { status: 'unavailable' as const, reason: 'unsupported' as const }
      : attachmentUpload.status !== 'idle'
        ? toMutationState({ status: attachmentUpload.status, action: 'attachment-upload' })
        : toMutationState({ status: attachmentDelete.status, action: 'attachment-delete' }),
    watchState: canWatch
      ? toMutationState({ status: watchMutation.status, action: 'watch' })
      : { status: 'unavailable' as const, reason: 'unsupported' as const },
    childState: toMutationState({ status: childCreate.status, action: 'child-create' }),
    titleState: canUpdate
      ? toMutationState({ status: titleMutation.status, action: 'title-update' })
      : { status: 'unavailable' as const, reason: 'unsupported' as const },
    descriptionState: canUpdate
      ? toMutationState({ status: descriptionMutation.status, action: 'description-update' })
      : { status: 'unavailable' as const, reason: 'unsupported' as const },
    fieldState: canUpdate || canTransition
      ? toMutationState({ status: fieldMutation.status, action: 'field-update' })
      : { status: 'unavailable' as const, reason: 'unsupported' as const },
  }
}
