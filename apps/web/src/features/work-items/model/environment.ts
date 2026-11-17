import type { WorkItemsClient } from '../api/client'

export type WorkItemsOperationAccess = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
  viewReferenceData: boolean
  viewTaskTable: boolean
  viewTaskDetail: boolean
  createTask: boolean
  updateTask: boolean
  transitionTask: boolean
  deleteTask: boolean
  viewAttachments: boolean
  uploadAttachment: boolean
  downloadAttachment: boolean
  deleteAttachment: boolean
  viewComments: boolean
  createComment: boolean
  deleteComment: boolean
  updateWatching: boolean
  viewClientOptions?: boolean
  createClientOption?: boolean
  updateClientOption?: boolean
  deleteClientOption?: boolean
}

export type WorkItemsEnvironment = {
  client: WorkItemsClient
  workspaceId: number
  userId: number
  sessionRevision: number
  pathname: string
  hash?: string
  search: Readonly<Record<string, unknown>>
  allowed: boolean
  createAllowed: boolean
  operationAccess: WorkItemsOperationAccess
  navigate(path: string, replace?: boolean): Promise<void>
}
