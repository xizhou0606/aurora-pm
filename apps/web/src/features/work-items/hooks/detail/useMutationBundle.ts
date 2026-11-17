import type { WorkItemsEnvironment } from '../../model/environment'
import {
  useWorkItemAttachmentDeleteMutation,
  useWorkItemAttachmentUploadMutation,
  useWorkItemChildCreateMutation,
  useWorkItemChildDeleteMutation,
  useWorkItemChildPatchMutation,
  useWorkItemCommentDeleteMutation,
  useWorkItemDetailPatchMutation,
  useWorkItemWatchMutation,
} from './useMutations'
import { useWorkItemCommentMutation } from '../collection/useMutations'

export function useWorkItemDetailMutationBundle(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  collectionKey: readonly unknown[],
) {
  const commentMutation = useWorkItemCommentMutation(environment, id)
  const commentDelete = useWorkItemCommentDeleteMutation(environment, id)
  const attachmentDelete = useWorkItemAttachmentDeleteMutation(environment, id)
  const attachmentUpload = useWorkItemAttachmentUploadMutation(environment, id)
  const watchMutation = useWorkItemWatchMutation(environment, id)
  const childCreate = useWorkItemChildCreateMutation(environment, id, collectionKey)
  const childDelete = useWorkItemChildDeleteMutation(environment, id, collectionKey)
  const childUpdate = useWorkItemChildPatchMutation(environment, id, collectionKey)
  const titleMutation = useWorkItemDetailPatchMutation(environment, id, collectionKey)
  const descriptionMutation = useWorkItemDetailPatchMutation(environment, id, collectionKey)
  const fieldMutation = useWorkItemDetailPatchMutation(environment, id, collectionKey)

  return {
    commentMutation,
    commentDelete,
    attachmentDelete,
    attachmentUpload,
    watchMutation,
    childCreate,
    childDelete,
    childUpdate,
    titleMutation,
    descriptionMutation,
    fieldMutation,
  }
}

export type WorkItemDetailMutationBundle = ReturnType<typeof useWorkItemDetailMutationBundle>
