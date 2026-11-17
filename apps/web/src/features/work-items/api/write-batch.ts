import { WORK_ITEMS_COPY } from '@/shared/copy'
import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

import type { WorkItemsClient } from './client'
import {
  array,
  normalizeWorkItem,
  optionalNumber,
  optionalString,
  record,
} from './contracts'
import type {
  BatchMutationResult,
  WorkItem,
  WorkItemChildCreatePayload,
  QuickTaskBatchPayload,
  QuickTaskBatchResult,
} from '../model/types'

// Shared normalization helper (duplicated from write.ts to avoid circular deps)
function normalizeDetail(value: unknown): WorkItem {
  const item = normalizeWorkItem(value)
  if (!item) throw new Error('invalid_work_item_detail')
  return item
}

// --- Batch creation ---

function normalizeCreatedTask(value: unknown) {
  const source = record(value)
  const clientId = optionalString(source.client_id)
  const id = optionalNumber(source.id)
  if (!clientId || !id) throw new Error('invalid_batch_created_task')
  return {
    clientId,
    id,
    ...(optionalString(source.task_key) ? { taskKey: optionalString(source.task_key) } : {}),
    children: array(source.children).map((child) => {
      const childSource = record(child)
      const childClientId = optionalString(childSource.client_id)
      const childId = optionalNumber(childSource.id)
      if (!childClientId || !childId) throw new Error('invalid_batch_created_child')
      return {
        clientId: childClientId,
        id: childId,
        ...(optionalString(childSource.task_key)
          ? { taskKey: optionalString(childSource.task_key) }
          : {}),
      }
    }),
  }
}

export async function createWorkItemBatch(
  client: WorkItemsClient,
  body: QuickTaskBatchPayload,
): Promise<QuickTaskBatchResult> {
  const payload = await client.request<unknown>({
    method: 'POST',
    path: '/tasks/batch-create',
    body,
  })
  const source = record(payload)
  const created = array(source.created).map(normalizeCreatedTask)
  const total = optionalNumber(source.total)
  if (total === undefined) throw new Error('invalid_batch_create_total')
  return { created, total }
}

function batchResult(value: unknown, requestedCount: number): BatchMutationResult {
  const source = record(value)
  const failures = array(source.failures).map((entry, index) => {
    const failure = record(entry)
    return {
      id: optionalNumber(failure.id) || optionalString(failure.id) || `unknown:${index}`,
      code: optionalString(failure.code) || 'operation_failed',
      message: optionalString(failure.message)
        || optionalString(failure.error)
        || WORK_ITEMS_COPY.mutation.failed,
    }
  })
  const explicitSucceeded = Number(source.succeeded ?? source.updated ?? source.deleted)
  const succeeded = Number.isInteger(explicitSucceeded) && explicitSucceeded >= 0
    ? explicitSucceeded
    : Math.max(0, requestedCount - failures.length)
  return { succeeded, failures }
}

export async function batchDeleteWorkItems(
  client: WorkItemsClient,
  ids: readonly number[],
): Promise<BatchMutationResult> {
  const payload = await client.request<unknown>({
    method: 'POST', path: '/tasks/batch-delete', body: { ids, cascade: true },
  })
  return batchResult(payload, ids.length)
}

export {
  createWorkItemComment,
  deleteWorkItemComment,
  deleteWorkItemAttachment,
  uploadWorkItemAttachment,
  uploadWorkItemRichTextImage,
  downloadWorkItemAttachment,
} from './write-batch-comments'

// --- Child operations ---

export async function createWorkItemChild(
  client: WorkItemsClient,
  taskId: number,
  draft: WorkItemChildCreatePayload,
): Promise<WorkItem> {
  const payload = await client.request<unknown>({
    method: 'POST',
    path: '/tasks',
    body: {
      title: draft.title,
      parent_id: taskId,
      kind: 'subtask',
      ...(draft.reporterUserId ? { reporter_user_id: draft.reporterUserId } : {}),
      ...(draft.assigneeId ? { assignee_id: draft.assigneeId } : {}),
      ...(draft.priority ? { priority: draft.priority } : {}),
      ...(draft.startsAt ? { starts_at: startOfLocalDay(draft.startsAt) } : {}),
      ...(draft.dueAt ? { due_at: endOfLocalDay(draft.dueAt) } : {}),
      ...(draft.estimatedMinutes !== undefined
        ? { estimated_minutes: draft.estimatedMinutes }
        : {}),
      ...(draft.status ? { status: draft.status } : {}),
    },
  })
  return normalizeDetail(payload)
}

export async function createWorkItemChildren(
  client: WorkItemsClient,
  taskId: number,
  drafts: readonly WorkItemChildCreatePayload[],
): Promise<BatchMutationResult> {
  const failures: BatchMutationResult['failures'][number][] = []
  let succeeded = 0
  for (const draft of drafts) {
    try {
      await createWorkItemChild(client, taskId, draft)
      succeeded += 1
    } catch {
      failures.push({
        id: draft.clientId,
        code: 'child_create_failed',
        message: WORK_ITEMS_COPY.detail.childCreateFailed,
      })
    }
  }
  return { succeeded, failures }
}
