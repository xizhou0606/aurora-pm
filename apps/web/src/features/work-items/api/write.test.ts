import { describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemsClient } from './client'
import {
  createWorkItem,
  deleteWorkItem,
} from './write'
import {
  batchDeleteWorkItems,
  createWorkItemBatch,
  createWorkItemChild,
  createWorkItemChildren,
  uploadWorkItemAttachment,
  uploadWorkItemRichTextImage,
} from './write-batch'

describe('Work-items create PM contract', () => {
  it('requires an explicit cascade query before deleting a task tree', async () => {
    const request = vi.fn().mockResolvedValue(undefined)
    const client = { request } as unknown as WorkItemsClient

    await deleteWorkItem(client, 51, true)

    expect(request).toHaveBeenCalledWith({
      method: 'DELETE', path: '/tasks/51', query: { cascade: true },
    })
  })

  it('requests cascade deletion for selected task trees', async () => {
    const request = vi.fn().mockResolvedValue({ deleted: 3, failed: 0 })
    const client = { request } as unknown as WorkItemsClient

    await batchDeleteWorkItems(client, [51])

    expect(request).toHaveBeenCalledWith({
      method: 'POST', path: '/tasks/batch-delete', body: { ids: [51], cascade: true },
    })
  })

  it('issues one PM POST /tasks and normalizes the created row', async () => {
    const request = vi.fn().mockResolvedValue({
      id: 51,
      workspace_id: 7,
      project_set_id: 8,
      project_id: 13,
      requirement_id: 21,
      kind: 'task',
      task_key: 'PM-51',
      title: 'Created task',
      status: 'draft',
      priority: 'normal',
      reporter_user_id: 101,
      can_update: true,
      can_delete: false,
      can_comment: true,
      allowed_status_transitions: ['draft', 'in_dev'],
    })
    const client = { request } as unknown as WorkItemsClient
    const body = {
      requirement_id: 21,
      kind: 'task' as const,
      title: 'Created task',
      description: '',
      status: 'draft',
      priority: 'normal',
      reporter_user_id: 101,
    }

    await expect(createWorkItem(client, body)).resolves.toMatchObject({
      id: 51,
      rowId: 'task:51',
      requirementId: 21,
      title: 'Created task',
    })
    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith({ method: 'POST', path: '/tasks', body })
  })

  it('submits the atomic batch-create contract and keeps client id mappings', async () => {
    const request = vi.fn().mockResolvedValue({
      created: [{
        client_id: 'parent-1', id: 71, task_key: 'PM-71',
        children: [{ client_id: 'child-1', id: 72, task_key: 'PM-72' }],
      }],
      total: 2,
    })
    const client = { request } as unknown as WorkItemsClient
    const body = {
      tasks: [{
        client_id: 'parent-1', title: 'Parent', requirement_id: 21,
        priority: 'normal', assignee_id: 101, children: [{
          client_id: 'child-1', title: 'Child', priority: 'normal', assignee_id: 101,
        }],
      }],
    }

    await expect(createWorkItemBatch(client, body)).resolves.toEqual({
      created: [{
        clientId: 'parent-1', id: 71, taskKey: 'PM-71',
        children: [{ clientId: 'child-1', id: 72, taskKey: 'PM-72' }],
      }],
      total: 2,
    })
    expect(request).toHaveBeenCalledWith({ method: 'POST', path: '/tasks/batch-create', body })
  })

  it('uploads a staged file only after a task id exists', async () => {
    const upload = vi.fn().mockResolvedValue({ id: 91 })
    const client = { upload } as unknown as WorkItemsClient
    const file = new File(['evidence'], 'evidence.txt', { type: 'text/plain' })

    await expect(uploadWorkItemAttachment(client, 51, file)).resolves.toBe(91)
    expect(upload).toHaveBeenCalledTimes(1)
    const request = upload.mock.calls[0]?.[0]
    expect(request.path).toBe('/tasks/51/attachments')
    expect(request.formData.get('file')).toBe(file)
  })

  it('uploads a rich-text image without requiring a selected requirement', async () => {
    const upload = vi.fn().mockResolvedValue({ url: 'https://cdn.example.test/task-proof.png' })
    const client = { upload } as unknown as WorkItemsClient
    const file = new File(['proof'], 'task-proof.png', { type: 'image/png' })

    await expect(uploadWorkItemRichTextImage(client, undefined, file)).resolves.toBe(
      'https://cdn.example.test/task-proof.png',
    )
    const request = upload.mock.calls[0]?.[0]
    expect(request.path).toBe('/tasks/materials/upload')
    expect(request.formData.get('requirement_id')).toBeNull()
    expect(request.formData.get('file')).toBe(file)
  })

  it('submits only PM main-supported child task fields', async () => {
    const request = vi.fn().mockResolvedValue({
      id: 52, title: 'Child', kind: 'subtask', status: 'draft', priority: 'high',
    })
    const client = { request } as unknown as WorkItemsClient

    await createWorkItemChild(client, 51, {
      clientId: 'draft-1',
      title: 'Child',
      reporterUserId: 9,
      assigneeId: 10,
      priority: 'high',
      startsAt: '2026-08-06',
      dueAt: '2026-08-08',
      estimatedMinutes: 90,
      status: 'draft',
    })

    expect(request).toHaveBeenCalledWith({
      method: 'POST', path: '/tasks', body: {
        title: 'Child', parent_id: 51, kind: 'subtask', reporter_user_id: 9,
        assignee_id: 10, priority: 'high', starts_at: new Date(2026, 7, 6, 0, 0, 0).toISOString(),
        due_at: new Date(2026, 7, 8, 23, 59, 59).toISOString(), estimated_minutes: 90, status: 'draft',
      },
    })
    expect(request.mock.calls[0]?.[0].body).not.toHaveProperty('owner_user_id')
    expect(request.mock.calls[0]?.[0].body).not.toHaveProperty('clientId')
  })

  it('creates children sequentially and returns stable partial failures', async () => {
    const order: string[] = []
    const request = vi.fn(async ({ body }: { body: Record<string, unknown> }) => {
      order.push(String(body.title))
      if (body.title === 'Second') throw new Error('in-memory failure')
      return { id: order.length + 60, title: body.title, kind: 'subtask' }
    })
    const client = { request } as unknown as WorkItemsClient

    await expect(createWorkItemChildren(client, 51, [
      { clientId: 'first', title: 'First' },
      { clientId: 'second', title: 'Second' },
      { clientId: 'third', title: 'Third' },
    ])).resolves.toEqual({
      succeeded: 2,
      failures: [{
        id: 'second', code: 'child_create_failed',
        message: WORK_ITEMS_COPY.detail.childCreateFailed,
      }],
    })
    expect(order).toEqual(['First', 'Second', 'Third'])
  })
})
