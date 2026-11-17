import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { defaultViewConfig } from '../../model/types'
import type { GridCommands, ViewToolbarProps } from '../../model/grid-port'
import { EmbeddedToolbar } from './embedded-toolbar'

function setup() {
  const toolbar: ViewToolbarProps = { activeViewId: 'view', fields: [{ id: 'title', name: 'Title', type: 'text', readOnly: false }], views: [{ id: 'view', name: 'Grid', kind: 'grid', revision: 1, config: defaultViewConfig }, { id: 'board', name: 'Board', kind: 'kanban', revision: 1, config: defaultViewConfig }], capabilities: { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title'] }, pending: false, search: { query: '', matches: [], total: 0, current: 0, pending: false, onQueryChange: vi.fn(), onNavigate: vi.fn() }, onViewSelect: vi.fn(), onViewCreate: vi.fn().mockResolvedValue(true), onViewRename: vi.fn().mockResolvedValue(true), onViewConfigChange: vi.fn().mockResolvedValue(true), onCreateField: vi.fn().mockResolvedValue(true), createFieldRequest: 0 }
  const commands: GridCommands = { onCreateEmpty: vi.fn().mockResolvedValue(null), onEdit: vi.fn(), onDelete: vi.fn(), onPaste: vi.fn(), onCopy: vi.fn(), onUndo: vi.fn(), onRedo: vi.fn(), canUndo: true, canRedo: false, pending: false }
  return { resourceName: 'Embedded source', tables: [{ id: 'table', name: 'Table' }], tableId: 'table', canSwitchTable: false, onTableSelect: vi.fn(), toolbar, commands, onTransfer: vi.fn() }
}
describe('embedded compact controls', () => {
  it('shows one compact header and keeps tool actions available on demand', () => {
    const props = setup(); render(<EmbeddedToolbar {...props} />)
    expect(screen.getByText('Embedded source')).toBeTruthy()
    expect(screen.getByRole('combobox', { name: text.viewType })).toBeTruthy()
    expect(screen.queryByRole('tablist')).toBeNull()
    expect(screen.queryByRole('button', { name: text.fieldsConfig })).toBeNull()
    expect(screen.queryByRole('button', { name: text.undo })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: text.expandTools }))
    expect(screen.getByRole('button', { name: text.fieldsConfig })).toBeTruthy()
    expect(screen.getByRole('textbox', { name: text.search })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: BITABLE_COPY.transfer })); expect(props.onTransfer).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: text.undo })).toBeNull()
    expect(screen.queryByRole('button', { name: text.redo })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: text.collapseTools }))
    expect(screen.queryByRole('button', { name: text.fieldsConfig })).toBeNull()
  })
  it('opens field creation when the grid header requests it from compact mode', async () => {
    const props = setup(); const { rerender } = render(<EmbeddedToolbar {...props} />)
    rerender(<EmbeddedToolbar {...props} toolbar={{ ...props.toolbar, createFieldRequest: 1 }} />)
    expect(await screen.findByRole('textbox', { name: BITABLE_COPY.name })).toBeTruthy()
    expect(screen.getByRole('button', { name: text.collapseTools })).toBeTruthy()
  })
})
