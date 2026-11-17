import type { WorkItemsClient } from './client'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  normalizeWorkItem,
} from './contracts'
import type {
  BatchMutationResult,
  WorkItem,
  WorkItemCreatePayload,
  WorkItemPatch,
} from '../model/types'
import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

export function normalizeDetail(value: unknown): WorkItem {
  const item = normalizeWorkItem(value)
  if (!item) throw new Error('invalid_work_item_detail')
  return item
}

export async function patchWorkItem(
  client: WorkItemsClient,
  id: number,
  patch: WorkItemPatch,
): Promise<WorkItem> {
	const presentFields = Object.entries(patch).filter(([, value]) => value !== undefined)
	const statusOnly = presentFields.length === 1 && presentFields[0]?.[0] === 'status'
  const payload = await client.request<unknown>({
    method: 'PATCH',
    path: statusOnly ? `/tasks/${id}/status` : `/tasks/${id}`,
    body: {
      ...(patch.title !== undefined ? { title: patch.title } : {}),
      ...(patch.status !== undefined ? { status: patch.status } : {}),
      ...(patch.priority !== undefined ? { priority: patch.priority } : {}),
      ...(patch.startsAt !== undefined ? { starts_at: startOfLocalDay(patch.startsAt) } : {}),
      ...(patch.dueAt !== undefined ? { due_at: endOfLocalDay(patch.dueAt) } : {}),
    },
  })
  return normalizeDetail(payload)
}

export async function deleteWorkItem(
  client: WorkItemsClient,
  id: number,
  cascade = false,
): Promise<void> {
  await client.request<void>({
    method: 'DELETE',
    path: `/tasks/${id}`,
    query: cascade ? { cascade: true } : undefined,
  })
}

export async function createWorkItem(
  client: WorkItemsClient,
  body: WorkItemCreatePayload,
): Promise<WorkItem> {
  const payload = await client.request<unknown>({
    method: 'POST',
    path: '/tasks',
    body,
  })
  return normalizeDetail(payload)
}

export async function batchPatchWorkItems(
  client: WorkItemsClient,
  ids: readonly number[],
  patch: WorkItemPatch,
): Promise<BatchMutationResult> {
	const presentFields = Object.entries(patch).filter(([, value]) => value !== undefined)
	if (presentFields.length === 1 && presentFields[0]?.[0] === 'status') {
		await client.request<unknown>({
			method: 'POST',
			path: '/tasks/batch-status-update',
			body: { ids, updates: { status: patch.status }, atomic: true },
		})
		return { succeeded: ids.length, failures: [] }
	}
  const failures: BatchMutationResult['failures'][number][] = []
  let succeeded = 0
  for (const id of ids) {
    try {
      await patchWorkItem(client, id, patch)
      succeeded += 1
    } catch {
      failures.push({ id, code: 'operation_failed', message: WORK_ITEMS_COPY.mutation.failed })
    }
  }
  return { succeeded, failures }
}

export async function setWorkItemWatch(
  client: WorkItemsClient,
  taskId: number,
  watch: boolean,
): Promise<void> {
  await client.request<void>({
    method: 'PUT', path: `/tasks/${taskId}/watch`, body: { watch },
  })
}
