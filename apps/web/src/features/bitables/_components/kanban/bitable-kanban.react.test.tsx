import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import type { GridProps } from '../../model/grid-port'
import { defaultViewConfig } from '../../model/types'
import { BitableKanban } from './bitable-kanban'

function setup(): GridProps {
  return { instanceKey: 'board', fields: [{ id: 'title', name: 'Title', type: 'text', readOnly: false }, { id: 'status', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'todo', name: 'Todo' }, { id: 'done', name: 'Done' }] } }], config: { ...defaultViewConfig, groupFieldId: 'status' },
    state: { status: 'ready', rows: [{ id: 'record-id', revision: 9, fieldRevisions: { title: 9, status: 9 }, values: { title: 'Shared record', status: 'todo' } }], total: 1, offset: 0, refresh: 'idle' },
    capabilities: { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title', 'status'] },
    commands: { pending: false, canUndo: false, canRedo: false, onEdit: vi.fn().mockResolvedValue(true), onCreateEmpty: vi.fn(), onDelete: vi.fn(), onCopy: vi.fn(), onPaste: vi.fn(), onUndo: vi.fn(), onRedo: vi.fn() } }
}
describe('kanban uses the same record commands', () => {
  it('dragging a local record to another choice submits the original record ID and revision', () => {
    const props = setup(); const { container } = render(<BitableKanban {...props} />)
    const dataTransfer = { effectAllowed: '', setData: vi.fn() }
    fireEvent.dragStart(container.querySelector('article')!, { dataTransfer })
    fireEvent.drop(screen.getByRole('region', { name: 'Done' }), { dataTransfer })
    expect(props.commands.onEdit).toHaveBeenCalledWith({ address: { recordId: 'record-id', fieldId: 'status' }, baseRevision: 9, value: 'done' })
    expect(screen.getByText('Shared record')).toBeTruthy()
  })
  it('ignores external drops and exposes no draggable readonly record', () => {
    const props = setup(); props.capabilities.canUpdateRecords = false
    const { container } = render(<BitableKanban {...props} />)
    expect(container.querySelector('article')?.getAttribute('draggable')).toBe('false')
    fireEvent.drop(screen.getByRole('region', { name: 'Done' }), { dataTransfer: { getData: () => 'another-record' } })
    expect(props.commands.onEdit).not.toHaveBeenCalled()
  })
  it('keeps records with removed choice values visible without rewriting them', () => {
    const props = setup()
    props.state = { status: 'ready', rows: [{ id: 'legacy', revision: 2, fieldRevisions: { title: 2, status: 2 }, values: { title: 'Still visible', status: 'removed-choice' } }], total: 1, offset: 0, refresh: 'idle' }
    render(<BitableKanban {...props} />)
    expect(screen.getByText('Still visible')).toBeTruthy()
    expect(props.commands.onEdit).not.toHaveBeenCalled()
  })
})
