import type { WorkItemsClient } from './client'
import {
  array,
  normalizeWorkItem,
  optionalNumber,
  optionalString,
  record,
} from './contracts'
import type {
  WorkItem,
  WorkItemActivityPage,
  WorkItemAttachment,
  WorkItemComment,
  WorkItemGitLink,
  WorkItemDetailPatch,
  WorkItemRelated,
} from '../model/types'
import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

export class WorkItemDetailContractError extends Error {
  constructor() {
    super('work_item_detail_contract')
    this.name = 'WorkItemDetailContractError'
  }
}

export async function requestWorkItemDetail(
  client: WorkItemsClient,
  id: number,
  signal?: AbortSignal,
): Promise<WorkItem> {
  const payload = await client.request<unknown>({
    path: `/tasks/${id}`,
    signal,
  })
  const item = normalizeWorkItem(payload)
  if (!item) throw new WorkItemDetailContractError()
  return item
}

export async function requestWorkItemActivities(
  client: WorkItemsClient,
  id: number,
  offset = 0,
  signal?: AbortSignal,
  limit = 50,
): Promise<WorkItemActivityPage> {
  const payload = await client.requestPage<unknown>({
    path: `/tasks/${id}/activities`,
    query: { limit, offset },
    signal,
  })
  const items = payload.items.map((entry) => {
    const source = record(entry)
    const activityId = optionalNumber(source.id)
    const action = optionalString(source.action)
    const fieldKey = optionalString(source.field_key)
    const summary = optionalString(source.summary)
      || optionalString(source.display_text)
      || optionalString(source.change_summary)
      || ''
    if (!activityId || (!summary && !action && !fieldKey)) {
      throw new Error('invalid_work_item_activity')
    }
    return {
      id: activityId,
      summary,
      createdAt: optionalString(source.created_at),
      actorId: optionalNumber(source.user_id)
        || optionalNumber(source.actor_user_id),
      actorName: optionalString(source.actor_display_name)
        || optionalString(source.actor_name)
        || optionalString(source.display_name)
        || optionalString(source.username),
      action,
      fieldKey,
      oldValue: optionalString(source.old_value),
      newValue: optionalString(source.new_value),
    }
  })
  return { items, total: Math.max(0, Number(payload.total) || 0) }
}

export async function requestWorkItemComments(
  client: WorkItemsClient,
  id: number,
  signal?: AbortSignal,
): Promise<WorkItemComment[]> {
  const payload = await client.requestPage<unknown>({
    path: `/tasks/${id}/comments`,
    signal,
  })
  return payload.items.map((entry) => {
    const source = record(entry)
    const commentId = optionalNumber(source.id)
    const body = optionalString(source.body)
    if (!commentId || !body) throw new Error('invalid_work_item_comment')
    return {
      id: commentId,
      body,
      createdAt: optionalString(source.created_at),
      authorId: optionalNumber(source.user_id)
        || optionalNumber(source.author_user_id),
      authorName: optionalString(source.author_display_name)
        || optionalString(source.author_name)
        || optionalString(source.display_name)
        || optionalString(source.username),
      canDelete: true,
    }
  })
}

export async function requestWorkItemAttachments(
  client: WorkItemsClient,
  id: number,
  stepId?: string,
  signal?: AbortSignal,
): Promise<WorkItemAttachment[]> {
  const payload = await client.requestPage<unknown>({
    path: `/tasks/${id}/attachments`,
    query: stepId ? { step_id: stepId } : undefined,
    signal,
  })
  return payload.items.map((entry) => {
    const source = record(entry)
    const attachmentId = optionalNumber(source.id)
    const filename = optionalString(source.filename) || optionalString(source.name)
    if (!attachmentId || !filename) throw new Error('invalid_work_item_attachment')
    return {
      id: attachmentId,
      filename,
      url: optionalString(source.url),
      createdAt: optionalString(source.created_at),
      contentType: optionalString(source.content_type),
      size: Math.max(0, Number(source.size) || 0),
      storageBackend: optionalString(source.storage_backend),
    }
  })
}

export async function requestWorkItemGitLinks(
  client: WorkItemsClient,
  id: number,
  signal?: AbortSignal,
): Promise<WorkItemGitLink[]> {
  const payload = await client.request<unknown>({
    path: `/tasks/${id}/git-links`, signal,
  })
  return array(payload).map((entry) => {
    const source = record(entry)
    const linkId = optionalNumber(source.id)
    const commitSHA = optionalString(source.commit_sha)
    if (!linkId || !commitSHA) throw new Error('invalid_work_item_git_link')
    return {
      id: linkId,
      createdAt: optionalString(source.created_at),
      repoURL: optionalString(source.repo_url),
      branch: optionalString(source.branch),
      ref: optionalString(source.ref),
      commitSHA,
      commitMessage: optionalString(source.commit_message),
    }
  })
}

export async function requestWorkItemRelated(
  client: WorkItemsClient,
  id: number,
  signal?: AbortSignal,
): Promise<WorkItemRelated[]> {
  const payload = record(await client.request<unknown>({
    path: `/tasks/${id}/related`, signal,
  }))
  return array(payload.items).map((entry) => {
    const source = record(entry)
    const relation = optionalString(source.relation)
    const task = normalizeWorkItem(source.task)
    if (!relation || !task) throw new Error('invalid_work_item_related')
    return { ...task, relation }
  })
}

export async function patchWorkItemDetail(
  client: WorkItemsClient,
  id: number,
  patch: WorkItemDetailPatch,
): Promise<WorkItem> {
  const payload = await client.request<unknown>({
    method: 'PATCH',
    path: `/tasks/${id}`,
    body: {
      ...(patch.title !== undefined ? { title: patch.title } : {}),
      ...(patch.description !== undefined ? { description: patch.description } : {}),
      ...(patch.status !== undefined ? { status: patch.status } : {}),
      ...(patch.priority !== undefined ? { priority: patch.priority } : {}),
      ...(patch.taskClientId !== undefined ? { task_client_id: patch.taskClientId } : {}),
      ...(patch.assigneeId !== undefined ? { assignee_id: patch.assigneeId } : {}),
      ...(patch.startsAt !== undefined ? {
        starts_at: patch.startsAt === null ? null : startOfLocalDay(patch.startsAt),
      } : {}),
      ...(patch.dueAt !== undefined ? {
        due_at: patch.dueAt === null ? null : endOfLocalDay(patch.dueAt),
      } : {}),
      ...(patch.estimatedMinutes !== undefined
        ? { estimated_minutes: patch.estimatedMinutes }
        : {}),
    },
  })
  const item = normalizeWorkItem(payload)
  if (!item || item.id !== id) throw new WorkItemDetailContractError()
  return item
}
