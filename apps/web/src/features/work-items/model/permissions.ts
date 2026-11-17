import type { WorkItemsOperationAccess } from './environment'
import type { WorkItem } from './item'

export function applyWorkItemsOperationAccess<T extends WorkItem>(
  item: T,
  access: WorkItemsOperationAccess,
  currentUserId: number,
): T {
  const ready = access.status === 'ready'
  const creatorOverride = currentUserId > 0 && item.createdByUserId === currentUserId
  const canUpdate = ready && (access.updateTask || creatorOverride) && item.capabilities.canUpdate
  return {
    ...item,
    capabilities: {
      ...item.capabilities,
      canUpdate,
      canDelete: ready && (access.deleteTask || creatorOverride) && item.capabilities.canDelete,
      canComment: ready && access.createComment && item.capabilities.canComment,
      canAttach: ready && access.uploadAttachment && item.capabilities.canAttach,
      canWatch: ready && access.updateWatching && item.capabilities.canWatch,
      canClone: false,
      allowedTransitions: ready && (access.transitionTask || creatorOverride)
        ? item.capabilities.allowedTransitions
        : [],
    },
  }
}
