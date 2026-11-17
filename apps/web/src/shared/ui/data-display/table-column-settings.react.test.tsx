import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { TableColumnSettings } from './table-column-settings'

describe('TableColumnSettings', () => {
  it('opens from the requested link action and emits visibility, order, and reset intents', () => {
    const onVisibleChange = vi.fn()
    const onMove = vi.fn()
    const onReorder = vi.fn()
    const onReset = vi.fn()
    render(
      <TableColumnSettings
        columns={[
          { id: 'name', label: '名称', required: true },
          { id: 'status', label: '状态' },
        ]}
        preference={{ order: ['name', 'status'], hidden: [] }}
        onVisibleChange={onVisibleChange}
        onMove={onMove}
        onReorder={onReorder}
        onReset={onReset}
      />,
    )

    const trigger = screen.getByRole('button', { name: copy.ui.tableColumns.trigger })
    expect(trigger.className).toContain('text-primary')
    fireEvent.click(trigger)
    fireEvent.click(screen.getByRole('checkbox', { name: copy.ui.tableColumns.toggle('状态') }))
    expect(onVisibleChange).toHaveBeenCalledWith('status', false)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.tableColumns.moveUp('状态') }))
    expect(onMove).toHaveBeenCalledWith('status', -1)
    const dragHandle = screen.getByRole('button', { name: copy.ui.tableColumns.drag('状态') })
    const nameRow = screen.getByText('名称').closest('li')
    expect(nameRow).not.toBeNull()
    fireEvent.dragStart(dragHandle, { dataTransfer: {
      effectAllowed: '',
      dropEffect: '',
      setData: vi.fn(),
    } })
    fireEvent.dragOver(nameRow as HTMLElement, { dataTransfer: { dropEffect: '' } })
    fireEvent.drop(nameRow as HTMLElement, { dataTransfer: {} })
    expect(onReorder).toHaveBeenCalledWith('status', 'name')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.tableColumns.reset }))
    expect(onReset).toHaveBeenCalledTimes(1)
  })
})
