import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemChildResource, WorkItemRelated } from '../../model/types'
import { ChildList } from './ChildList'

afterEach(cleanup)

const child: WorkItemRelated = {
  relation: 'child',
  id: 32,
  rowId: 'task:32',
  taskKey: 'PM-32',
  title: '子任务标题',
  kind: 'subtask',
  status: 'draft',
  priority: 'normal',
  assigneeId: 7,
  assigneeName: 'Teo',
  estimatedMinutes: 90,
  startsAt: '2026-08-12',
  dueAt: '2026-08-15',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: true,
    canComment: true,
    canAttach: true,
    canWatch: true,
    canClone: false,
    allowedTransitions: ['draft', 'in_dev'],
  },
}

function renderChildList(item: WorkItemRelated = child) {
  const update = vi.fn().mockResolvedValue(true)
  const open = vi.fn()
  const resource: WorkItemChildResource = {
    state: { status: 'ready', data: [item], refresh: 'idle' },
    mutation: { status: 'idle' },
    update,
    open,
    remove: vi.fn().mockResolvedValue(true),
  }
  render(
    <ChildList
      resource={resource}
      users={{ status: 'ready', data: [{ id: 7, label: 'Teo' }, { id: 8, label: 'Ada' }] }}
      workflow={[
        { value: 'draft', label: '待处理' },
        { value: 'in_dev', label: '进行中' },
      ]}
    />,
  )
  return { update, open }
}

describe('Work-item child table parity', () => {
  it('keeps children without mutation permissions out of bulk selection', () => {
    renderChildList({ ...child, capabilities: { ...child.capabilities, canUpdate: false, canDelete: false, allowedTransitions: [] } })
    const selection = screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.list.selectRow(child.title) })
    expect(selection.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(selection)
    expect(selection.getAttribute('aria-checked')).toBe('false')
    expect(screen.queryByTestId('work-items-bulk')).toBeNull()
  })

  it('uses the outer task table fields in one fixed, non-configurable child table', () => {
    renderChildList()

    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
      WORK_ITEMS_COPY.list.selection,
      WORK_ITEMS_COPY.list.taskName,
      WORK_ITEMS_COPY.list.taskNumber,
      WORK_ITEMS_COPY.list.status,
      WORK_ITEMS_COPY.list.assignee,
      WORK_ITEMS_COPY.list.priority,
      WORK_ITEMS_COPY.list.estimatedTime,
      WORK_ITEMS_COPY.list.startsAt,
      WORK_ITEMS_COPY.list.due,
      WORK_ITEMS_COPY.list.actions,
    ])
    expect(screen.getByRole('table', { name: WORK_ITEMS_COPY.detail.children }).className)
      .toContain('table-fixed')
    expect(screen.queryByText(WORK_ITEMS_COPY.filterSettings.trigger)).toBeNull()
  })

  it('keeps outer-table row selection and batch actions without exposing column settings', async () => {
    const { update } = renderChildList()
    fireEvent.click(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.list.selectRow(child.title) }))

    const bulk = screen.getByTestId('work-items-bulk')
    expect(bulk.textContent).toContain(WORK_ITEMS_COPY.bulk.selected(1))
    fireEvent.click(screen.getByRole('combobox', { name: WORK_ITEMS_COPY.bulk.status }))
    const status = await screen.findByRole('option', { name: WORK_ITEMS_COPY.enums.statuses.in_dev })
    fireEvent.pointerDown(status, { button: 0, pointerType: 'mouse' })
    fireEvent.click(status)
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.bulk.update }))

    await waitFor(() => expect(update).toHaveBeenCalledWith(child, { status: 'in_dev' }))
    await waitFor(() => expect(screen.queryByTestId('work-items-bulk')).toBeNull())
  })

  it('reuses inline title, priority and date editing and keeps view/edit navigation', async () => {
    const { update, open } = renderChildList()
    const titleLabel = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.taskName, child.title)
    fireEvent.click(screen.getByRole('button', { name: titleLabel }))
    const title = screen.getByRole('textbox', { name: titleLabel })
    fireEvent.change(title, { target: { value: '更新后的子任务' } })
    fireEvent.keyDown(title, { key: 'Enter' })
    await waitFor(() => expect(update).toHaveBeenCalledWith(child, { title: '更新后的子任务' }))

    const priorityLabel = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.priority, child.title)
    fireEvent.click(screen.getByRole('button', { name: priorityLabel }))
    fireEvent.click(screen.getByRole('combobox', { name: priorityLabel }))
    const priority = await screen.findByRole('option', { name: WORK_ITEMS_COPY.enums.priorities.high })
    fireEvent.pointerDown(priority, { button: 0, pointerType: 'mouse' })
    fireEvent.click(priority)
    await waitFor(() => expect(update).toHaveBeenCalledWith(child, { priority: 'high' }))

    const dueLabel = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.due, child.title)
    fireEvent.click(screen.getByRole('button', { name: dueLabel }))
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月20日' }))
    await waitFor(() => expect(update).toHaveBeenCalledWith(child, { dueAt: '2026-08-20' }))

    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.list.openDetail(child.title) }))
    expect(open).toHaveBeenCalledWith(child.id, 'view')
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.edit }))
    expect(open).toHaveBeenCalledWith(child.id, 'edit')
  })
})
