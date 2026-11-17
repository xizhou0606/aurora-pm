import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'

export type InboxFeedId = 'workflow' | 'task' | 'bug' | 'mention'

export type InboxScope = PMScopeSelection
export type InboxScopeOptions = PMScopeOptions

export type InboxRouteState = InboxScope & {
  search: string
  workflowPage: number
  taskPage: number
  bugPage: number
  mentionPage: number
  invalidScope: boolean
  invalidLegacyScope: boolean
}

export type InboxSummary = {
  total: number
  tasks: number
  bugs: number
  workflows: number
  overdue: number
  dueToday: number
  blockedTasks: number
  reviewBugs: number
  workflowAIFailed: number
  mentions: number
}

export type InboxTask = {
  id: number
  taskKey?: string
  title: string
  status?: string
  priority?: string
  dueAt?: string
  projectSetId?: number
  projectId?: number
  requirementId?: number
  requirementTitle?: string
  mentionAt?: string
}

export type InboxBug = {
  id: number
  bugKey?: string
  title: string
  status?: string
  priority?: string
  severity?: string
  dueDate?: string
  projectSetName?: string
  projectName?: string
  requirementTitle?: string
}

export type InboxWorkflowTodo = {
  id: number
  todoId: string
  sourceType: 'workflow_node'
  sourceRevision: string
  title: string
  status: string
  actionKind: string
  allowedActions: string[]
  detailPath: string
  workflowInstanceId: number
  workflowNodeId: string
  nodeType: string
  templateName?: string
  aiStatus?: string
  dueAt?: string
  createdAt?: string
  projectSetName?: string
  projectName?: string
  requirementTitle?: string
  subjectTitle: string
}

export type InboxFeedRow = InboxTask | InboxBug | InboxWorkflowTodo

export type InboxFeed<T extends InboxFeedRow> = {
  items: T[]
  total: number
}

export type ReadyRefresh = 'idle' | 'refreshing' | 'stale_error'

export type CollectionState<T> =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty'; total: number; refresh: ReadyRefresh }
  | {
    status: 'ready'
    rows: T[]
    total: number
    refresh: ReadyRefresh
  }

export type SummaryState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | {
    status: 'ready'
    data: InboxSummary
    refresh: ReadyRefresh
  }

export type ScopeOptionsState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: InboxScopeOptions }
