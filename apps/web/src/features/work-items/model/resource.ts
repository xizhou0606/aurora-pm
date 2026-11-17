import type { ReadyRefresh } from './collection'
import type { WorkItem } from './item'
import type { MutationState, WorkItemDetailPatch } from './mutation'

export type WorkItemsDownload = {
  blob: Blob
  contentType?: string
  filename?: string
}

export type ResourceState<T> =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: T; refresh: ReadyRefresh }

export type DetailViewState<T> =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'not_found' }
  | { status: 'access_required' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: T; refresh: ReadyRefresh }

export type WorkItemActivity = {
  id: number
  summary: string
  createdAt?: string
  actorId?: number
  actorName?: string
  action?: string
  fieldKey?: string
  oldValue?: string
  newValue?: string
}
export type WorkItemActivityPage = { items: WorkItemActivity[]; total: number }
export type WorkItemStatusTransition = {
  id: number
  fromKey: string
  toKey: string
  actorId?: number
  actorName?: string
  createdAt?: string
  automatic: boolean
}
export type WorkItemComment = {
  id: number
  body: string
  createdAt?: string
  authorId?: number
  authorName?: string
  canDelete: boolean
}
export type WorkItemAttachment = {
  id: number
  filename: string
  url?: string
  createdAt?: string
  contentType?: string
  size: number
  storageBackend?: string
}
export type WorkItemGitLink = {
  id: number
  createdAt?: string
  repoURL?: string
  branch?: string
  ref?: string
  commitSHA: string
  commitMessage?: string
}
export type WorkItemRelated = WorkItem & {
  relation: string
}

export type WorkItemChildDraft = {
  clientId: string
  title: string
  reporterUserId?: number
  assigneeId?: number
  priority: string
  startsAt: string
  dueAt: string
  estimatedHours: string
  status: string
}

export type WorkItemChildCreatePayload = {
  clientId: string
  title: string
  reporterUserId?: number
  assigneeId?: number
  priority?: string
  startsAt?: string
  dueAt?: string
  estimatedMinutes?: number
  status?: string
}

export type WorkItemChildResource = {
  state: ResourceState<WorkItemRelated[]>
  mutation: MutationState
  open: (id: number, mode?: 'view' | 'edit') => void
  update: (row: WorkItemRelated, patch: WorkItemDetailPatch) => Promise<boolean>
  remove: (id: number) => Promise<boolean>
}
export type WorkItemDetailResources = {
  activities: WorkItemActivity[]
  comments: WorkItemComment[]
  attachments: WorkItemAttachment[]
  children: WorkItemRelated[]
}
export type DetailResourceId = keyof WorkItemDetailResources
export type DetailResourceRow = {
  activity?: WorkItemActivity
  id: string
  primary: string
  secondary?: string
  meta?: string
  userId?: number
  userName?: string
  timestamp?: string
  dateTime?: string
  href?: string
  canDelete?: boolean
}
export type DetailResourceEntry = {
  id: DetailResourceId
  title: string
  emptyLabel: string
  state: ResourceState<DetailResourceRow[]>
  warning?: { label: string; retry: () => void }
  pagination?: { label: string; pending: boolean; loadMore: () => void }
  onOpenRow?: (id: string) => void
  onDeleteRow?: (id: string) => Promise<boolean>
}
export type DetailTabId = 'comments' | 'activity' | 'transitions'
export type DetailNavigation = {
  canGoBack: boolean
  openTask: (taskId: number, mode?: 'view' | 'edit') => void
  openRequirement: (item: {
    projectSetId?: number
    projectId?: number
    requirementId?: number
  }) => void
  goBack: () => void
}
