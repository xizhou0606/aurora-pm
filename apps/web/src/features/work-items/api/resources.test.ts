import { describe, expect, it, vi } from 'bun:test'

import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

import type { PMTransportRequest } from './client'
import type { WorkItemsClient } from './client'
import {
  requestWorkItemActivities,
  requestWorkItemAttachments,
  requestWorkItemComments,
  requestWorkItemDetail,
  requestWorkItemRelated,
  patchWorkItemDetail,
} from './resources'
import {
  batchPatchWorkItems,
  patchWorkItem,
} from './write'
import {
  deleteWorkItemAttachment,
  downloadWorkItemAttachment,
} from './write-batch'

function task(id = 31) {
  return {
    id,
    workspace_id: 7,
    task_key: `PM-${id}`,
    title: 'PM task',
    kind: 'task',
    status: 'draft',
    priority: 'normal',
    can_update: true,
    can_delete: false,
    can_comment: true,
    allowed_status_transitions: ['draft', 'in_dev'],
    watched: true,
    starts_at: '2026-08-01T00:00:00Z',
    due_at: '2026-08-02T00:00:00Z',
    estimated_minutes: 0,
    created_by_user_id: 47,
  }
}

function client(input: Partial<WorkItemsClient> = {}): WorkItemsClient {
  return {
    request: vi.fn(),
    requestPage: vi.fn(),
    requestPMScopeOptions: vi.fn(),
    requestPMUsers: vi.fn(),
    ...input,
  } as WorkItemsClient
}

describe('PM task detail contracts', () => {
  it('enables task actions without an authority discriminator', async () => {
    const request = vi.fn().mockResolvedValue(task())

    await expect(requestWorkItemDetail(client({ request }), 31)).resolves.toMatchObject({
      id: 31,
      rowId: 'task:31',
      watched: true,
      startsAt: '2026-08-01',
      dueAt: '2026-08-02',
      estimatedMinutes: 0,
      createdByUserId: 47,
      capabilities: {
        canUpdate: true,
        canDelete: true,
        canComment: true,
        canAttach: true,
        canWatch: true,
        canClone: false,
        canLogTime: false,
        allowedTransitions: ['draft', 'in_dev'],
      },
    })
    expect(request).toHaveBeenCalledWith({ path: '/tasks/31', signal: undefined })
  })

  it('restores API timestamps to the selected local calendar dates', async () => {
    const request = vi.fn().mockResolvedValue({
      ...task(),
      starts_at: startOfLocalDay('2026-08-12'),
      due_at: endOfLocalDay('2026-08-12'),
    })

    await expect(requestWorkItemDetail(client({ request }), 31)).resolves.toMatchObject({
      startsAt: '2026-08-12',
      dueAt: '2026-08-12',
    })
  })

  it('sends explicit nulls when clearing task dates', async () => {
    const request = vi.fn().mockResolvedValue({
      ...task(),
      starts_at: null,
      due_at: null,
    })

    await patchWorkItemDetail(client({ request }), 31, {
      startsAt: null,
      dueAt: null,
    })

    expect(request).toHaveBeenCalledWith({
      method: 'PATCH',
      path: '/tasks/31',
      body: { starts_at: null, due_at: null },
    })
  })

  it('sends the selected client or explicit null from detail editing', async () => {
    const request = vi.fn().mockResolvedValue({ ...task(), task_client_id: 9 })

    await patchWorkItemDetail(client({ request }), 31, { taskClientId: 9 })
    expect(request).toHaveBeenLastCalledWith({
      method: 'PATCH', path: '/tasks/31', body: { task_client_id: 9 },
    })

    request.mockResolvedValueOnce({ ...task(), task_client_id: null })
    await patchWorkItemDetail(client({ request }), 31, { taskClientId: null })
    expect(request).toHaveBeenLastCalledWith({
      method: 'PATCH', path: '/tasks/31', body: { task_client_id: null },
    })
  })

  it('uses PM activities, comments, attachments and related endpoints without owner query params', async () => {
    const requestPage = vi.fn(async <T,>(entry: PMTransportRequest) => {
      if (entry.path.endsWith('/activities')) {
        return { items: [{ id: 1, summary: 'created' }] as T[], total: 1 }
      }
      if (entry.path.endsWith('/comments')) {
        return { items: [{ id: 2, body: 'hello', can_delete: true }] as T[], total: 1 }
      }
      return { items: [{ id: 3, filename: 'file.txt', size: 4 }] as T[], total: 1 }
    })
    const request = vi.fn().mockResolvedValue({
      items: [{ relation: 'child', task: {
        id: 32, title: 'Child', priority: 'high', status: 'in_dev',
        assignee_id: 9, assignee_display_name: 'Teo', can_delete: true,
        estimated_minutes: 90,
        starts_at: '2026-08-12T00:00:00Z',
        due_at: endOfLocalDay('2026-08-15'),
        allowed_status_transitions: ['draft', 'in_dev', 'pending_test'],
      } }],
    })
    const api = client({
      request: request as WorkItemsClient['request'],
      requestPage: requestPage as WorkItemsClient['requestPage'],
    })

    const [, , , related] = await Promise.all([
      requestWorkItemActivities(api, 31),
      requestWorkItemComments(api, 31),
      requestWorkItemAttachments(api, 31),
      requestWorkItemRelated(api, 31),
    ])

    expect(requestPage.mock.calls.map(([entry]) => entry)).toEqual([
      { path: '/tasks/31/activities', query: { limit: 50, offset: 0 }, signal: undefined },
      { path: '/tasks/31/comments', signal: undefined },
      { path: '/tasks/31/attachments', query: undefined, signal: undefined },
    ])
    expect(request).toHaveBeenCalledWith({ path: '/tasks/31/related', signal: undefined })
    expect(related).toEqual([expect.objectContaining({
      id: 32, priority: 'high', status: 'in_dev', assigneeId: 9, assigneeName: 'Teo',
      estimatedMinutes: 90, startsAt: '2026-08-12', dueAt: '2026-08-15',
      capabilities: expect.objectContaining({ canUpdate: true, canDelete: true }),
    })])
  })

  it('keeps the MIGSINFRA202-4 activity page when comment rows omit a display summary', async () => {
    const requestPage = vi.fn(async <T,>() => ({
      items: [
        {
          id: 4952,
          action: 'field_change',
          field_key: 'priority',
          old_value: 'normal',
          new_value: 'high',
          summary: '优先级：normal → high',
        },
        { id: 4950, action: 'comment', field_key: '', summary: '' },
        { id: 6, action: 'created', field_key: '', summary: '创建了任务' },
      ] as T[],
      total: 3,
    }))

    await expect(requestWorkItemActivities(client({
      requestPage: requestPage as WorkItemsClient['requestPage'],
    }), 31)).resolves.toEqual({
      items: [
        expect.objectContaining({ id: 4952, summary: '优先级：normal → high' }),
        expect.objectContaining({ id: 4950, summary: '', action: 'comment' }),
        expect.objectContaining({ id: 6, summary: '创建了任务' }),
      ],
      total: 3,
    })
  })

  it('uses attachment-global delete and download routes from main', async () => {
    const request = vi.fn().mockResolvedValue(undefined)
    const download = vi.fn().mockResolvedValue({ blob: new Blob(['file']) })
    const api = client({ request, download })

    await deleteWorkItemAttachment(api, 7)
    await downloadWorkItemAttachment(api, 7)

    expect(request).toHaveBeenCalledWith({
      method: 'DELETE', path: '/tasks/attachments/7', query: undefined,
    })
    expect(download).toHaveBeenCalledWith({
      path: '/tasks/attachments/7/download', query: undefined,
    })
  })

  it('uses the dedicated atomic batch status transition route', async () => {
    const request = vi.fn(async (_entry: PMTransportRequest) => ({}))

    await expect(batchPatchWorkItems(client({
      request: request as WorkItemsClient['request'],
    }), [31, 32], { status: 'in_dev' }))
      .resolves.toEqual({ succeeded: 2, failures: [] })
    expect(request.mock.calls.map(([entry]) => entry)).toEqual([
      {
        method: 'POST',
        path: '/tasks/batch-status-update',
        body: { ids: [31, 32], updates: { status: 'in_dev' }, atomic: true },
      },
    ])
  })

  it('uses the dedicated single status transition route', async () => {
    const request = vi.fn().mockResolvedValue(task())

    await patchWorkItem(client({ request: request as WorkItemsClient['request'] }), 31, {
      status: 'in_dev',
    })

    expect(request).toHaveBeenCalledWith({
      method: 'PATCH', path: '/tasks/31/status', body: { status: 'in_dev' },
    })
  })

  it('maps supported detail fields to the PM PATCH contract', async () => {
    const request = vi.fn().mockResolvedValue(task())
    await patchWorkItem(client({ request: request as WorkItemsClient['request'] }), 31, {
      title: 'Changed', status: 'in_dev', priority: 'high',
      startsAt: '2026-08-01', dueAt: '2026-08-02',
    })
    expect(request).toHaveBeenCalledWith({
      method: 'PATCH', path: '/tasks/31',
      body: {
        title: 'Changed', status: 'in_dev', priority: 'high',
        starts_at: new Date(2026, 7, 1, 0, 0, 0).toISOString(),
        due_at: new Date(2026, 7, 2, 23, 59, 59).toISOString(),
      },
    })
  })
})
