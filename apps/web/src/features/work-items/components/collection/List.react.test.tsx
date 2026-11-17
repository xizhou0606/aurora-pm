import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { cleanup } from '@testing-library/react'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemTreeRow } from '../../model/types'
import { List } from './List'
import type { TableColumnPreference } from '@/shared/model/table-column-preferences'
import type { WorkItemsListColumnId } from './List'

afterEach(cleanup)

const row: WorkItemTreeRow = {
  id: 31,
  rowId: 'task:31',
  taskKey: 'PM-31',
  title: '列表任务',
  kind: 'task',
  status: 'draft',
  priority: 'normal',
  assigneeId: 7,
  assigneeName: 'Teo',
  dueAt: '2026-08-12',
  watched: false,
  depth: 0,
  hasChildren: false,
  treeGroupId: 'work-items-tree-all',
  capabilities: {
    canUpdate: true,
    canDelete: true,
    canComment: true,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: ['draft', 'in_dev'],
  },
}

function renderList(input: {
  onOpen?: ReturnType<typeof vi.fn>
  onUpdate?: ReturnType<typeof vi.fn>
  item?: WorkItemTreeRow
  groupLabel?: string
  groupId?: string
  users?: Array<{ id: number; label: string; username?: string; avatarUrl?: string }>
  columnPreference?: TableColumnPreference<WorkItemsListColumnId>
} = {}) {
  const onOpen = input.onOpen || vi.fn()
  const onUpdate = input.onUpdate || vi.fn().mockResolvedValue(true)
  const onToggleSelected = vi.fn()
  const onToggleExpanded = vi.fn()
  const item = input.item || row
  render(
    <List
      state={{ status: 'ready', rows: [item], total: 1, refresh: 'idle', truncated: false }}
      groups={[{ id: input.groupId || (input.groupLabel ? 'requirement_id:9' : 'all'), label: input.groupLabel || '', rows: [item] }]}
      selectedRowIds={[]}
      expandedRowIds={new Set()}
      users={{ status: 'ready', data: input.users || [{ id: 7, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' }, { id: 8, label: 'Ada' }] }}
      workflow={{
        strict: false,
        statuses: [
          { key: 'draft', label: '草稿', sortOrder: 10 },
          { key: 'in_dev', label: '开发中', sortOrder: 20 },
        ],
        transitions: [{ from: 'draft', to: 'in_dev' }],
      }}
      updateState={{ status: 'idle' }}
      deleteState={{ status: 'idle' }}
      onToggleSelected={onToggleSelected}
      onToggleExpanded={onToggleExpanded}
      onOpen={onOpen}
      onUpdate={onUpdate}
      onDelete={vi.fn().mockResolvedValue(true)}
      columnPreference={input.columnPreference}
    />,
  )
  return { onOpen, onUpdate, onToggleSelected, onToggleExpanded }
}

describe('Work-items list parity', () => {
  it('preserves selection, child expansion and detail callback modes through cell helpers', () => {
    const item = { ...row, hasChildren: true }
    const { onOpen, onToggleSelected, onToggleExpanded } = renderList({ item })
    fireEvent.click(screen.getByRole('checkbox', { name: WORK_ITEMS_COPY.list.selectRow(item.title) }))
    expect(onToggleSelected).toHaveBeenCalledWith(item)
    expect(onToggleSelected).toHaveBeenCalledTimes(1)
    expect(onOpen).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.list.expandChildren(item.title) }))
    expect(onToggleExpanded).toHaveBeenCalledWith(item, [item])
    const detail = screen.getByRole('button', { name: WORK_ITEMS_COPY.list.openDetail(item.title) })
    fireEvent.click(detail)
    expect(onOpen).toHaveBeenCalledWith(item, detail, 'view')
    const edit = screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.edit })
    fireEvent.click(edit)
    expect(onOpen).toHaveBeenCalledWith(item, edit, 'edit')
  })

  it('uses a fixed column layout so inline editors cannot resize the table', () => {
    renderList()

    const table = screen.getByRole('table', { name: WORK_ITEMS_COPY.list.label })
    expect(table.className).toContain('table-fixed')
    expect(table.className).toContain('min-w-max')
    expect(screen.getByRole('columnheader', { name: WORK_ITEMS_COPY.list.taskNumber }).style.getPropertyValue('--data-table-column-size')).toBe('240px')
    expect(screen.getByRole('columnheader', { name: WORK_ITEMS_COPY.list.taskName }).style.getPropertyValue('--data-table-column-size')).toBe('320px')
    expect(table.closest('[data-slot="data-table"]')?.className).toContain('overflow-x-auto')
  })

  it('applies the locally configured field visibility and order', () => {
    renderList({
      columnPreference: {
        order: ['selection', 'task_name', 'status', 'task_number', 'assignee', 'priority', 'estimated_minutes', 'starts_at', 'due_at', 'actions'],
        hidden: ['priority', 'estimated_minutes'],
      },
    })

    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)
    expect(headers[1]).toContain(WORK_ITEMS_COPY.list.taskName)
    expect(headers[2]).toContain(WORK_ITEMS_COPY.list.status)
    expect(headers).not.toContain(WORK_ITEMS_COPY.list.priority)
    expect(headers).not.toContain(WORK_ITEMS_COPY.list.estimatedTime)
  })

  it('uses the canonical status label and tone for legacy response aliases', () => {
    renderList({ item: { ...row, status: 'review' } })

    const status = screen.getByText(WORK_ITEMS_COPY.enums.statuses.pending_test)
    expect(status.getAttribute('data-slot')).toBe('status-pill')
    expect(status.getAttribute('data-tone')).toBe('warning')
  })

  it('keeps the inline status trigger and option panel wide enough for complete labels', async () => {
    renderList()

    const statusName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.status, row.title)
    fireEvent.click(screen.getByRole('button', { name: statusName }))
    const status = screen.getByRole('combobox', { name: statusName })
    fireEvent.click(status)

    const content = await waitFor(() => {
      const value = document.querySelector('[data-slot="select-content"]')
      expect(value).not.toBeNull()
      return value!
    })
    expect(content.className).toContain('min-w-36')
    expect(screen.getByRole('option', { name: '草稿' }).textContent).toBe('草稿')
    expect(screen.getByRole('option', { name: '开发中' }).textContent).toBe('开发中')
  })

  it('keeps detail navigation on the task key and edits the title inline', async () => {
    const { onOpen, onUpdate } = renderList()
    const titleName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.taskName, row.title)
    const titleTrigger = screen.getByRole('button', { name: titleName })
    expect(titleTrigger.querySelector('[data-base-ui-tooltip-trigger]')?.textContent).toBe(row.title)
    fireEvent.click(titleTrigger)
    const title = screen.getByRole('textbox', { name: titleName })
    fireEvent.change(title, { target: { value: '更新后的列表任务' } })
    fireEvent.keyDown(title, { key: 'Enter' })

    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(row, { title: '更新后的列表任务' }))

    const edit = screen.getByRole('button', { name: WORK_ITEMS_COPY.mutation.edit })
    fireEvent.click(edit)

    expect(onOpen).toHaveBeenCalledWith(row, edit, 'edit')
  })

  it('renders the task number as the emphasized detail link', () => {
    renderList()

    const taskNumber = screen.getByRole('button', {
      name: WORK_ITEMS_COPY.list.openDetail(row.title),
    })
    expect(taskNumber.className).toContain('text-primary')
    expect(taskNumber.className).toContain('hover:underline')
    expect(taskNumber.querySelector('span')?.className).toContain('font-semibold')
  })

  it('exposes priority, assignee and due date as permission-controlled inline editors', async () => {
    const { onUpdate } = renderList()

    const priorityName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.priority, row.title)
    fireEvent.click(screen.getByRole('button', { name: priorityName }))
    const priority = screen.getByRole('combobox', { name: priorityName })
    fireEvent.click(priority)
    const priorityOption = await screen.findByRole('option', { name: WORK_ITEMS_COPY.enums.priorities.high })
    fireEvent.pointerDown(priorityOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(priorityOption)
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(row, { priority: 'high' }))

    const assigneeName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.assignee, row.title)
    fireEvent.click(screen.getByRole('button', { name: assigneeName }))
    const assignee = screen.getByRole('combobox', { name: assigneeName })
    fireEvent.click(assignee)
    const assigneeOption = await screen.findByRole('option', { name: 'Ada' })
    fireEvent.pointerDown(assigneeOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(assigneeOption)
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(row, { assigneeId: 8 }))

    const dueName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.due, row.title)
    fireEvent.click(screen.getByRole('button', { name: dueName }))
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月20日' }))
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(row, { dueAt: '2026-08-20' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: '选择 2026年8月20日' })).toBeNull())
  })

  it('prefers the canonical user directory identity over a legacy task snapshot', () => {
    renderList({
      item: { ...row, assigneeId: 73, assigneeName: 'Legacy User 73' },
      users: [{ id: 73, label: 'Koko', username: 'koko' }],
    })

    const assigneeName = WORK_ITEMS_COPY.format.fieldLabel(WORK_ITEMS_COPY.list.assignee, row.title)
    expect(screen.getByRole('button', { name: assigneeName }).textContent).toContain('Koko')
    expect(screen.queryByText('Legacy User 73')).toBeNull()
  })

  it('keeps requirement groups inside one table and collapses their task rows', () => {
    renderList({ groupLabel: 'UGC社区需求' })

    expect(screen.getAllByRole('table')).toHaveLength(1)
    expect(screen.getByRole('columnheader', { name: WORK_ITEMS_COPY.list.estimatedTime })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: WORK_ITEMS_COPY.list.startsAt })).toBeTruthy()
    expect(screen.queryByRole('columnheader', { name: WORK_ITEMS_COPY.list.requirement })).toBeNull()
    const collapse = screen.getByRole('button', {
      name: WORK_ITEMS_COPY.list.collapseGroup('UGC社区需求'),
    })
    expect(collapse.getAttribute('aria-expanded')).toBe('true')
    expect(collapse.closest('tr')?.classList.contains('bg-muted/60')).toBe(true)
    fireEvent.click(collapse)

    expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.list.openDetail(row.title) })).toBeNull()
    expect(screen.getByRole('button', {
      name: WORK_ITEMS_COPY.list.expandGroup('UGC社区需求'),
    }).getAttribute('aria-expanded')).toBe('false')
  })

  it('renders an injected parent context as read-only and excludes it from selection actions', () => {
    const { onToggleSelected } = renderList({
      item: {
        ...row,
        contextOnly: true,
        capabilities: {
          ...row.capabilities,
          canUpdate: false,
          canDelete: false,
          allowedTransitions: [],
        },
      },
    })

    expect(screen.getByText(WORK_ITEMS_COPY.list.filterContext)).toBeTruthy()
    const selection = screen.getByRole('checkbox', {
      name: WORK_ITEMS_COPY.list.selectRow(row.title),
    })
    expect(selection.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(selection)
    expect(onToggleSelected).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.mutation.edit })).toBeNull()
    expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.mutation.delete })).toBeNull()
  })

  it('renders assignee group headers through the shared user preview', () => {
    renderList({ groupLabel: 'Teo', groupId: 'assignee_id:7' })

    const group = screen.getByRole('button', {
      name: WORK_ITEMS_COPY.list.collapseGroup('Teo'),
    })
    expect(group.querySelector('[data-slot="user-preview"]')).toBeTruthy()
    expect(group.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
  })

  it('marks unfinished rows whose due time is in the past', () => {
    renderList({ item: { ...row, dueAt: '2020-01-01', status: 'in_dev' } })

    expect(screen.getByText(WORK_ITEMS_COPY.list.overdue).getAttribute('title'))
      .toBe(WORK_ITEMS_COPY.list.overdueTitle)
  })
})
