import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy, WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItem } from '../../model/types'
import { Detail } from './Detail'

const item: WorkItem = {
  id: 32,
  rowId: 'task:32',
  title: 'Child task',
  kind: 'subtask',
  status: 'draft',
  priority: 'normal',
  watched: false,
  capabilities: {
    canUpdate: false,
    canDelete: false,
    canComment: false,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: [],
  },
}

describe('Work-item detail child navigation', () => {
  it('owns the return action in the ResponsivePanel header', async () => {
    const goBack = vi.fn()
    render(
      <Detail
        view={{
          state: { status: 'ready', data: item, refresh: 'idle' },
          resources: [
            {
              id: 'activities', title: 'Activities', emptyLabel: 'Empty',
              state: { status: 'ready', data: [], refresh: 'idle' },
            },
            {
              id: 'comments', title: 'Comments', emptyLabel: 'Empty',
              state: { status: 'ready', data: [], refresh: 'idle' },
            },
          ],
          children: {
            state: { status: 'ready', data: [], refresh: 'idle' },
            mutation: { status: 'idle' },
            open: vi.fn(),
            update: vi.fn().mockResolvedValue(false),
            remove: vi.fn().mockResolvedValue(false),
          },
          attachments: { status: 'ready', data: [], refresh: 'idle' },
          commentUsers: { status: 'hidden' },
          navigation: {
            canGoBack: true,
            openTask: vi.fn(),
            openRequirement: vi.fn(),
            goBack,
          },
          currentUserId: 9,
        }}
        actionState={{
          update: { status: 'idle' }, delete: { status: 'idle' },
          comment: { status: 'unavailable', reason: 'unsupported' },
          attachment: { status: 'unavailable', reason: 'unsupported' },
          watch: { status: 'unavailable', reason: 'unsupported' },
          child: { status: 'idle' },
        }}
        capabilities={{ uploadAttachments: false, downloadAttachments: false, deleteAttachments: false, createChild: false }}
        actions={{
          update: vi.fn().mockResolvedValue(false),
          delete: vi.fn().mockResolvedValue(false),
          comment: vi.fn().mockResolvedValue(false),
          uploadAttachment: vi.fn().mockResolvedValue(false),
          uploadCommentImage: vi.fn().mockResolvedValue(null),
          uploadRichTextImage: vi.fn().mockResolvedValue(null),
          deleteAttachment: vi.fn().mockResolvedValue(false),
          downloadAttachment: vi.fn().mockResolvedValue(null),
          toggleWatch: vi.fn().mockResolvedValue(false),
          createChildren: vi.fn().mockResolvedValue({ succeeded: 0, failures: [] }),
          saveField: vi.fn().mockResolvedValue(true),
        }}
        panel={{ open: true, close: vi.fn(), sharePath: '/workspace/work-items/32?view=list' }}
      />,
    )

    const back = await screen.findByRole('button', { name: WORK_ITEMS_COPY.detail.backToParentTask })
    expect(screen.getByRole('dialog').className).toContain('h-[90dvh]')
    expect(back.closest('[data-slot="responsive-panel-header"]')).not.toBeNull()
    expect(screen.queryByText('返回上一任务')).toBeNull()
    fireEvent.click(back)
    expect(goBack).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: copy.ui.copyLink })
      .closest('[data-slot="responsive-panel-header"]')).not.toBeNull()
  })

  it('opens read-only, enters edit from the footer and saves all local changes once', async () => {
    const close = vi.fn()
    const saveField = vi.fn().mockResolvedValue(true)
    const editableItem: WorkItem = {
      ...item,
      startsAt: '2026-08-05',
      capabilities: { ...item.capabilities, canUpdate: true },
    }
    render(
      <Detail
        view={{
          state: { status: 'ready', data: editableItem, refresh: 'idle' },
          resources: [
            { id: 'activities', title: 'Activities', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
            { id: 'comments', title: 'Comments', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
          ],
          children: {
            state: { status: 'ready', data: [], refresh: 'idle' },
            mutation: { status: 'idle' }, open: vi.fn(),
            update: vi.fn().mockResolvedValue(false), remove: vi.fn().mockResolvedValue(false),
          },
          attachments: { status: 'ready', data: [], refresh: 'idle' },
          commentUsers: { status: 'hidden' },
          workflow: { status: 'ready', data: [{ value: 'draft', label: '草稿' }], refresh: 'idle' },
          currentUserId: 9,
        }}
        actionState={{
          update: { status: 'idle' }, delete: { status: 'idle' },
          comment: { status: 'unavailable', reason: 'resource-unavailable' },
          attachment: { status: 'unavailable', reason: 'resource-unavailable' },
          watch: { status: 'unavailable', reason: 'resource-unavailable' },
          child: { status: 'idle' }, field: { status: 'idle' },
        }}
        capabilities={{ uploadAttachments: false, downloadAttachments: false, deleteAttachments: false, createChild: false }}
        actions={{
          update: vi.fn().mockResolvedValue(false), delete: vi.fn().mockResolvedValue(false),
          comment: vi.fn().mockResolvedValue(false), uploadAttachment: vi.fn().mockResolvedValue(false),
          uploadCommentImage: vi.fn().mockResolvedValue(null), uploadRichTextImage: vi.fn().mockResolvedValue(null),
          deleteAttachment: vi.fn().mockResolvedValue(false),
          downloadAttachment: vi.fn().mockResolvedValue(null), toggleWatch: vi.fn().mockResolvedValue(false),
          createChildren: vi.fn().mockResolvedValue({ succeeded: 0, failures: [] }), saveField,
        }}
        panel={{ open: true, close }}
      />,
    )

    expect(await screen.findByRole('button', { name: WORK_ITEMS_COPY.mutation.edit })).toBeTruthy()
    expect(screen.queryByRole('textbox', { name: WORK_ITEMS_COPY.detail.title })).toBeNull()
    expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.detail.save })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.edit }))
    fireEvent.change(screen.getByRole('textbox', { name: WORK_ITEMS_COPY.detail.title }), {
      target: { value: 'Updated task' },
    })
    fireEvent.click(screen.getByLabelText(WORK_ITEMS_COPY.gantt.startDate))
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月6日' }))
    expect(saveField).not.toHaveBeenCalled()

    const closeButton = screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.cancel })
    expect(closeButton.closest('[data-slot="responsive-panel-header"]')).not.toBeNull()
    fireEvent.click(closeButton)
    expect(close).not.toHaveBeenCalled()
    expect(screen.getByRole('alertdialog', { name: WORK_ITEMS_COPY.detail.unsavedTitle })).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.continueEditing }))

    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.save }))
    await waitFor(() => expect(saveField).toHaveBeenCalledWith({
      title: 'Updated task',
      startsAt: '2026-08-06',
    }))
    await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
  })

  it('publishes edit mode through the query-owned panel contract', async () => {
    const setMode = vi.fn()
    const close = vi.fn()
    render(
      <Detail
        view={{
          state: { status: 'ready', data: { ...item, capabilities: { ...item.capabilities, canUpdate: true } }, refresh: 'idle' },
          resources: [
            { id: 'activities', title: 'Activities', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
            { id: 'comments', title: 'Comments', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
          ],
          children: {
            state: { status: 'ready', data: [], refresh: 'idle' },
            mutation: { status: 'idle' }, open: vi.fn(),
            update: vi.fn().mockResolvedValue(false), remove: vi.fn().mockResolvedValue(false),
          },
          attachments: { status: 'ready', data: [], refresh: 'idle' },
          commentUsers: { status: 'hidden' },
          currentUserId: 9,
        }}
        actionState={{
          update: { status: 'idle' }, delete: { status: 'idle' },
          comment: { status: 'unavailable', reason: 'unsupported' },
          attachment: { status: 'unavailable', reason: 'unsupported' },
          watch: { status: 'unavailable', reason: 'unsupported' },
          child: { status: 'idle' }, field: { status: 'idle' },
        }}
        capabilities={{ uploadAttachments: false, downloadAttachments: false, deleteAttachments: false, createChild: false }}
        actions={{
          update: vi.fn().mockResolvedValue(false), delete: vi.fn().mockResolvedValue(false),
          comment: vi.fn().mockResolvedValue(false), uploadAttachment: vi.fn().mockResolvedValue(false),
          uploadCommentImage: vi.fn().mockResolvedValue(null), uploadRichTextImage: vi.fn().mockResolvedValue(null),
          deleteAttachment: vi.fn().mockResolvedValue(false),
          downloadAttachment: vi.fn().mockResolvedValue(null), toggleWatch: vi.fn().mockResolvedValue(false),
          createChildren: vi.fn().mockResolvedValue({ succeeded: 0, failures: [] }),
          saveField: vi.fn().mockResolvedValue(true),
        }}
        panel={{ open: true, mode: 'view', setMode, close }}
      />,
    )

    fireEvent.click(await screen.findByRole('button', { name: WORK_ITEMS_COPY.mutation.edit }))
    expect(setMode).toHaveBeenCalledWith('edit')
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.cancel }))
    expect(close).toHaveBeenCalledTimes(1)
  })

  it('shows a status-only edit entry for a transition-only actor', async () => {
    const transitionItem: WorkItem = {
      ...item,
      capabilities: { ...item.capabilities, allowedTransitions: ['in_dev'] },
    }
    render(
      <Detail
        view={{
          state: { status: 'ready', data: transitionItem, refresh: 'idle' },
          resources: [
            { id: 'activities', title: 'Activities', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
            { id: 'comments', title: 'Comments', emptyLabel: 'Empty', state: { status: 'ready', data: [], refresh: 'idle' } },
          ],
          children: {
            state: { status: 'ready', data: [], refresh: 'idle' },
            mutation: { status: 'idle' }, open: vi.fn(),
            update: vi.fn().mockResolvedValue(false), remove: vi.fn().mockResolvedValue(false),
          },
          attachments: { status: 'ready', data: [], refresh: 'idle' },
          commentUsers: { status: 'hidden' },
          workflow: {
            status: 'ready',
            data: [{ value: 'draft', label: '草稿' }, { value: 'in_dev', label: '开发中' }],
            refresh: 'idle',
          },
          currentUserId: 9,
        }}
        actionState={{
          update: { status: 'idle' }, delete: { status: 'idle' },
          comment: { status: 'unavailable', reason: 'unsupported' },
          attachment: { status: 'unavailable', reason: 'unsupported' },
          watch: { status: 'unavailable', reason: 'unsupported' },
          child: { status: 'idle' }, field: { status: 'idle' },
        }}
        capabilities={{ uploadAttachments: false, downloadAttachments: false, deleteAttachments: false, createChild: false }}
        actions={{
          update: vi.fn().mockResolvedValue(false), delete: vi.fn().mockResolvedValue(false),
          comment: vi.fn().mockResolvedValue(false), uploadAttachment: vi.fn().mockResolvedValue(false),
          uploadCommentImage: vi.fn().mockResolvedValue(null), uploadRichTextImage: vi.fn().mockResolvedValue(null),
          deleteAttachment: vi.fn().mockResolvedValue(false), downloadAttachment: vi.fn().mockResolvedValue(null),
          toggleWatch: vi.fn().mockResolvedValue(false),
          createChildren: vi.fn().mockResolvedValue({ succeeded: 0, failures: [] }),
          saveField: vi.fn().mockResolvedValue(true),
        }}
        panel={{ open: true, close: vi.fn() }}
      />,
    )

    fireEvent.click(await screen.findByRole('button', { name: WORK_ITEMS_COPY.mutation.edit }))
    expect((screen.getByRole('combobox', { name: WORK_ITEMS_COPY.detail.status }) as HTMLButtonElement).disabled).toBe(false)
    expect((screen.getByRole('combobox', { name: WORK_ITEMS_COPY.detail.priority }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.queryByRole('textbox', { name: WORK_ITEMS_COPY.detail.title })).toBeNull()
  })
})
