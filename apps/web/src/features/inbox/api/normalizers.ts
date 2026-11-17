import { normalizePMScopeOptions } from '@/shared/api'
import { normalizeWorkItemStatus } from '@/shared/model'
import type {
  InboxBug,
  InboxFeed,
  InboxFeedId,
  InboxScopeOptions,
  InboxSummary,
  InboxTask,
  InboxWorkflowTodo,
} from '../model'
import type { InboxClient } from './client'
import { failInboxContract, failInboxScopeContract } from './client'
import {
  inboxRecord,
  isInboxRecord,
  optionalInboxNumber,
  optionalInboxString,
  requiredInboxCount,
} from './contracts'

function normalizeTask(client: InboxClient, value: unknown): InboxTask {
  const source = inboxRecord(value)
  const id = optionalInboxNumber(source.id)
  const title = optionalInboxString(source.title)
  if (!id || !title) failInboxContract(client)
  return {
    id,
    title,
    taskKey: optionalInboxString(source.task_key),
    status: normalizeWorkItemStatus(optionalInboxString(source.status)) || undefined,
    priority: optionalInboxString(source.priority),
    dueAt: optionalInboxString(source.due_at),
    projectSetId: optionalInboxNumber(source.project_set_id),
    projectId: optionalInboxNumber(source.project_id),
    requirementId: optionalInboxNumber(source.requirement_id),
    requirementTitle: optionalInboxString(source.requirement_title),
    mentionAt: optionalInboxString(source.mention_at),
  }
}

function normalizeBug(client: InboxClient, value: unknown): InboxBug {
  const source = inboxRecord(value)
  const id = optionalInboxNumber(source.id)
  const title = optionalInboxString(source.title)
  if (!id || !title) failInboxContract(client)
  return {
    id,
    title,
    bugKey: optionalInboxString(source.bug_key),
    status: optionalInboxString(source.status),
    priority: optionalInboxString(source.priority_key) || optionalInboxString(source.priority),
    severity: optionalInboxString(source.severity_key),
    dueDate: optionalInboxString(source.due_date),
    projectSetName: optionalInboxString(source.project_set_name),
    projectName: optionalInboxString(source.project_name),
    requirementTitle: optionalInboxString(source.requirement_title),
  }
}

function normalizeWorkflowTodo(client: InboxClient, value: unknown): InboxWorkflowTodo {
  const source = inboxRecord(value)
  const id = optionalInboxNumber(source.id)
  const todoId = optionalInboxString(source.todo_id)
  const sourceType = optionalInboxString(source.source_type)
  const sourceRevision = optionalInboxString(source.source_revision)
  const title = optionalInboxString(source.title)
  const status = optionalInboxString(source.status)
  const actionKind = optionalInboxString(source.action_kind)
  const detailPath = optionalInboxString(source.detail_path)
  const workflowInstanceId = optionalInboxNumber(source.workflow_instance_id)
  const workflowNodeId = optionalInboxString(source.workflow_node_id)
  const nodeType = optionalInboxString(source.node_type)
  const subjectTitle = optionalInboxString(source.subject_title)
  const allowedActions = Array.isArray(source.allowed_actions)
    ? source.allowed_actions.flatMap((action) => {
        const normalized = optionalInboxString(action)
        return normalized ? [normalized] : []
      })
    : []
  if (
    !id
    || !todoId
    || sourceType !== 'workflow_node'
    || !sourceRevision
    || !title
    || !status
    || !actionKind
    || !detailPath?.startsWith('/workspace/')
    || !detailPath.includes('workflow_node=')
    || !workflowInstanceId
    || !workflowNodeId
    || !nodeType
    || !subjectTitle
    || !allowedActions.length
  ) {
    failInboxContract(client)
  }
  return {
    id,
    todoId,
    sourceType,
    sourceRevision,
    title,
    status,
    actionKind,
    allowedActions,
    detailPath,
    workflowInstanceId,
    workflowNodeId,
    nodeType,
    subjectTitle,
    templateName: optionalInboxString(source.template_name),
    aiStatus: optionalInboxString(source.ai_status),
    dueAt: optionalInboxString(source.due_at),
    createdAt: optionalInboxString(source.created_at),
    projectSetName: optionalInboxString(source.project_set_name),
    projectName: optionalInboxString(source.project_name),
    requirementTitle: optionalInboxString(source.requirement_title),
  }
}

export function normalizeInboxScopePayload(
  _client: InboxClient,
  payload: unknown,
): InboxScopeOptions {
  if (!isInboxRecord(payload)) failInboxScopeContract()
  const collections = ['project_sets', 'projects', 'requirements'] as const
  const normalizedPayload = Object.fromEntries(collections.map((key) => [
    key,
    payload[key] === null ? [] : payload[key],
  ]))
  if (collections.some((key) => !Array.isArray(normalizedPayload[key]))) {
    failInboxScopeContract()
  }
  const normalized = normalizePMScopeOptions(normalizedPayload)
  if (
    normalized.projectSets.length !== (normalizedPayload.project_sets as unknown[]).length
    || normalized.projects.length !== (normalizedPayload.projects as unknown[]).length
    || normalized.requirements.length !== (normalizedPayload.requirements as unknown[]).length
    || normalized.projects.some((option) => !option.parentId)
    || normalized.requirements.some((option) => !option.parentId)
  ) {
    failInboxScopeContract()
  }
  return normalized
}

export function normalizeInboxSummary(
  client: InboxClient,
  payload: unknown,
  canReadBugs: boolean,
): InboxSummary {
  if (!isInboxRecord(payload) || !isInboxRecord(payload.summary)) {
    failInboxContract(client)
  }
  const source = payload.summary
  const optionalCount = (key: string) => {
    const value = source[key]
    return value === undefined ? 0 : requiredInboxCount(client, source, key)
  }
  return {
    total: requiredInboxCount(client, source, 'total'),
    tasks: requiredInboxCount(client, source, 'tasks'),
    bugs: canReadBugs ? requiredInboxCount(client, source, 'bugs') : 0,
    workflows: optionalCount('workflows'),
    overdue: requiredInboxCount(client, source, 'overdue'),
    dueToday: requiredInboxCount(client, source, 'due_today'),
    blockedTasks: requiredInboxCount(client, source, 'blocked_tasks'),
    reviewBugs: canReadBugs ? requiredInboxCount(client, source, 'review_bugs') : 0,
    workflowAIFailed: optionalCount('workflow_ai_failed'),
    mentions: requiredInboxCount(client, source, 'mentions'),
  }
}

export function normalizeInboxFeed(
  client: InboxClient,
  payload: { items: unknown[]; total: number },
  feedId: InboxFeedId,
): InboxFeed<InboxTask | InboxBug | InboxWorkflowTodo> {
  if (
    !Array.isArray(payload.items)
    || !Number.isSafeInteger(payload.total)
    || payload.total < 0
    || payload.total < payload.items.length
  ) {
    failInboxContract(client)
  }
  return {
    items: payload.items.map((item) => (
      feedId === 'bug'
        ? normalizeBug(client, item)
        : feedId === 'workflow'
          ? normalizeWorkflowTodo(client, item)
          : normalizeTask(client, item)
    )),
    total: payload.total,
  }
}
