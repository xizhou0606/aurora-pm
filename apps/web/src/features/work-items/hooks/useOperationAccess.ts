import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError } from '@/shared/api'
import type { WorkItemsOperationAccess } from '../model/environment'

export function requireWorkItemsAction(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('WORK_ITEMS_ACTION_FORBIDDEN', 403)
}

export function useWorkItemsOperationAccess(): WorkItemsOperationAccess {
  const operations = useOperationCapabilities('work-items')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    viewReferenceData: operations.allows(PM_ACTION.WorkItemsReferenceDataView),
    viewTaskTable: operations.allows(PM_ACTION.WorkItemsTaskTableView),
    viewTaskDetail: operations.allows(PM_ACTION.WorkItemsTaskDetailView),
    createTask: operations.allows(PM_ACTION.WorkItemsTasksCreate),
    updateTask: operations.allows(PM_ACTION.WorkItemsTasksUpdate),
    transitionTask: operations.allows(PM_ACTION.WorkItemsTasksTransitionStatus),
    deleteTask: operations.allows(PM_ACTION.WorkItemsTasksDelete),
    viewAttachments: operations.allows(PM_ACTION.WorkItemsAttachmentsView),
    uploadAttachment: operations.allows(PM_ACTION.WorkItemsAttachmentsUpload),
    downloadAttachment: operations.allows(PM_ACTION.WorkItemsAttachmentsDownload),
    deleteAttachment: operations.allows(PM_ACTION.WorkItemsAttachmentsDelete),
    viewComments: operations.allows(PM_ACTION.WorkItemsCommentsView),
    createComment: operations.allows(PM_ACTION.WorkItemsCommentsCreate),
    deleteComment: operations.allows(PM_ACTION.WorkItemsCommentsDelete),
    updateWatching: operations.allows(PM_ACTION.WorkItemsWatchingUpdate),
    viewClientOptions: operations.allows(PM_ACTION.WorkItemsClientOptionsView),
    createClientOption: operations.allows(PM_ACTION.WorkItemsClientOptionsCreate),
    updateClientOption: operations.allows(PM_ACTION.WorkItemsClientOptionsUpdate),
    deleteClientOption: operations.allows(PM_ACTION.WorkItemsClientOptionsDelete),
  }), [operations.allows, operations.retry, operations.status])
}

// AI task breakdown reuses the existing ai-runs action codes; no new catalog
// entries. The 'ai-runs' domain is evaluated separately from work-items.
export function useWorkItemsAIAccess() {
  const operations = useOperationCapabilities('ai-runs')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canCreateRun: operations.allows(PM_ACTION.AiRunsRunsCreate),
    canApplyRun: operations.allows(PM_ACTION.AiRunsRunsApply),
  }), [operations.allows, operations.retry, operations.status])
}
