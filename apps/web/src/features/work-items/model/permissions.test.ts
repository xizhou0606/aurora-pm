import { describe, expect, it, vi } from 'bun:test'

import type { WorkItemsOperationAccess } from './environment'
import type { WorkItem } from './item'
import { applyWorkItemsOperationAccess } from './permissions'

const access = (patch: Partial<WorkItemsOperationAccess> = {}): WorkItemsOperationAccess => ({
  status: 'ready',
  retry: vi.fn(),
  viewReferenceData: true,
  viewTaskTable: true,
  viewTaskDetail: true,
  createTask: true,
  updateTask: true,
  transitionTask: true,
  deleteTask: true,
  viewAttachments: true,
  uploadAttachment: true,
  downloadAttachment: true,
  deleteAttachment: true,
  viewComments: true,
  createComment: true,
  deleteComment: true,
  updateWatching: true,
  ...patch,
})

const item = (): WorkItem => ({
  id: 1,
  rowId: 'task:1',
  title: '任务',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: true,
    canComment: true,
    canAttach: true,
    canWatch: true,
    canClone: true,
    allowedTransitions: ['in_dev'],
  },
})

describe('work-items operation access', () => {
  it('combines global actions with object capabilities for both UI and submit owners', () => {
    const result = applyWorkItemsOperationAccess(item(), access({
      updateTask: false,
      deleteTask: false,
      createComment: false,
      uploadAttachment: false,
      updateWatching: false,
    }), 7)

    expect(result.capabilities).toMatchObject({
      canUpdate: false,
      canDelete: false,
      canComment: false,
      canAttach: false,
      canWatch: false,
      canClone: false,
      allowedTransitions: ['in_dev'],
    })
  })

  it('fails closed while the global permission state is unresolved', () => {
    const result = applyWorkItemsOperationAccess(item(), access({ status: 'loading' }), 7)
    expect(result.capabilities.canUpdate).toBe(false)
    expect(result.capabilities.canDelete).toBe(false)
    expect(result.capabilities.canComment).toBe(false)
  })

  it('keeps update, delete and status transition available to the immutable task creator', () => {
    const result = applyWorkItemsOperationAccess({ ...item(), createdByUserId: 7 }, access({
      updateTask: false,
      transitionTask: false,
      deleteTask: false,
    }), 7)

    expect(result.capabilities.canUpdate).toBe(true)
    expect(result.capabilities.canDelete).toBe(true)
    expect(result.capabilities.allowedTransitions).toEqual(['in_dev'])
  })
})
