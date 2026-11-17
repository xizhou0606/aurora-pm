import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { WorkItemsEnvironment } from '../../model/environment'
import type { WorkItem } from '../../model/types'
import { useWorkItemDetailActions } from './useActions'
import type { WorkItemDetailMutationBundle } from './useMutationBundle'

function setup(input: { canUpdate: boolean; canAttach: boolean; allowedTransitions?: string[] }) {
  const upload = vi.fn().mockResolvedValue({ url: '/uploads/description.png' })
  const idleMutation = () => ({
    status: 'idle' as const,
    mutateAsync: vi.fn(),
    reset: vi.fn(),
  })
  const mutations = {
    attachmentDelete: idleMutation(),
    attachmentUpload: idleMutation(),
    childCreate: idleMutation(),
    commentMutation: idleMutation(),
    descriptionMutation: idleMutation(),
    fieldMutation: idleMutation(),
    titleMutation: idleMutation(),
    watchMutation: idleMutation(),
  } as unknown as WorkItemDetailMutationBundle
  const environment = {
    workspaceId: 7,
    userId: 101,
    sessionRevision: 1,
    pathname: '/workspace/work-items/31',
    search: {},
    allowed: true,
    createAllowed: true,
    operationAccess: {
      status: 'ready', retry: vi.fn(), viewReferenceData: true,
      viewTaskTable: true, viewTaskDetail: true, createTask: true,
      updateTask: true, transitionTask: true, deleteTask: true, viewAttachments: true,
      uploadAttachment: true, downloadAttachment: true, deleteAttachment: true,
      viewComments: true, createComment: true, deleteComment: true,
      updateWatching: true,
    },
    navigate: vi.fn().mockResolvedValue(undefined),
    client: {
      request: vi.fn(),
      requestPage: vi.fn(),
      requestPMScopeOptions: vi.fn(),
      requestPMUsers: vi.fn(),
      upload,
    },
  } satisfies WorkItemsEnvironment
  const item: WorkItem = {
    id: 31,
    rowId: 'task:31',
    title: 'Task',
    status: 'draft',
    requirementId: 21,
    watched: false,
    capabilities: {
      canUpdate: input.canUpdate,
      canDelete: true,
      canComment: true,
      canAttach: input.canAttach,
      canWatch: true,
      canClone: false,
      allowedTransitions: input.allowedTransitions || [],
    },
  }
  const hook = renderHook(() => useWorkItemDetailActions({
    environment,
    detail: { status: 'ready', data: item, refresh: 'idle' },
    commentUsers: { status: 'ready', data: [] },
    allowedStatusTransitions: input.allowedTransitions || [],
    mutations,
  }))
  return { ...hook, upload, attachmentUpload: mutations.attachmentUpload, fieldMutation: mutations.fieldMutation }
}

describe('useWorkItemDetailActions rich-text upload access', () => {
  it.each([
    { canUpdate: false, canAttach: true },
    { canUpdate: true, canAttach: false },
  ])('requires both task update and attachment upload capability: %o', async (capabilities) => {
    const { result, upload } = setup(capabilities)
    await act(async () => {
      await expect(result.current.uploadRichTextImage(
        new File(['image'], 'image.png', { type: 'image/png' }),
      )).rejects.toMatchObject({ code: 'WORK_ITEMS_ACTION_FORBIDDEN', status: 403 })
    })
    expect(upload).not.toHaveBeenCalled()
    expect(result.current.canUploadAttachments).toBe(capabilities.canAttach)
  })

  it('uploads the detail image only when both capabilities are available', async () => {
    const { result, upload } = setup({ canUpdate: true, canAttach: true })
    await act(async () => {
      await expect(result.current.uploadRichTextImage(
        new File(['image'], 'image.png', { type: 'image/png' }),
      )).resolves.toBe('http://localhost:3000/uploads/description.png')
    })
    expect(upload).toHaveBeenCalledWith(expect.objectContaining({
      path: '/tasks/materials/upload',
    }))
  })

  it('allows a transition-only actor to update status but not other detail fields', async () => {
    const { result, fieldMutation } = setup({
      canUpdate: false,
      canAttach: false,
      allowedTransitions: ['in_dev'],
    })

    await act(async () => {
      await expect(result.current.saveField({ status: 'in_dev' })).resolves.toBe(true)
      await expect(result.current.saveField({ priority: 'high' })).resolves.toBe(false)
    })
    expect(fieldMutation.mutateAsync).toHaveBeenCalledWith({ status: 'in_dev' })
    expect(fieldMutation.mutateAsync).toHaveBeenCalledTimes(1)
  })
})

it('contains a failed attachment request and allows a subsequent upload', async () => {
  const {result, attachmentUpload} = setup({canUpdate:true,canAttach:true})
  const mutate = attachmentUpload.mutateAsync as ReturnType<typeof vi.fn>
  mutate.mockRejectedValueOnce(new Error('object storage is required for task attachments')).mockResolvedValueOnce(9)
  const file = new File(['test'],'test.txt')
  await act(async()=>{expect(await result.current.uploadAttachment(file)).toBe(false)})
  await act(async()=>{expect(await result.current.uploadAttachment(file)).toBe(true)})
})
