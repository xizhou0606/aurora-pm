import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy, QUALITY_COPY } from '@/shared/copy'

import { AssignmentDialog } from './AssignmentDialog'

describe('AssignmentDialog', () => {
  it('keeps empty assignee and CC values unselected instead of presenting a filter-wide all state', async () => {
    render(
      <AssignmentDialog
        open
        users={{
          status: 'ready',
          rows: [{ id: 7, name: '小龙', username: 'teo', avatarUrl: '/teo.png' }],
        }}
        canUpload={false}
        state={{ status: 'idle', action: 'assign' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        onOpenChange={vi.fn()}
        onUpload={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    expect(await screen.findAllByPlaceholderText(QUALITY_COPY.none)).toHaveLength(2)
    expect((screen.getByRole('button', {
      name: QUALITY_COPY.lifecycle.assign,
    }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('keeps the assignment payload while selecting CC users by username', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <AssignmentDialog
        open
        initialAssigneeId={7}
        users={{
          status: 'ready',
          rows: [
            { id: 7, name: '小龙', username: 'teo', avatarUrl: '/teo.png' },
            { id: 8, name: 'Codex', username: 'agent' },
          ],
        }}
        canUpload={false}
        state={{ status: 'idle', action: 'assign' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        onOpenChange={vi.fn()}
        onUpload={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const ccInput = await screen.findByRole('combobox', { name: QUALITY_COPY.lifecycle.cc })
    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.userSelect.openOptions(QUALITY_COPY.lifecycle.cc),
    }))
    fireEvent.change(ccInput, { target: { value: 'agent' } })
    const option = await screen.findByRole('option', { name: /Codex/ })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.lifecycle.assign }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      assignee_user_id: 7,
      defect_owner_user_id: 7,
      cc_user_ids: [8],
      copy_user_ids: [8],
      assignment_materials: [],
    })))
  })
})
