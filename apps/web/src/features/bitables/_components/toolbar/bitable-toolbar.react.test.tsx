import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import type { ViewToolbarProps } from '../../model/grid-port'
import { defaultViewConfig } from '../../model/types'
import { BitableToolbar } from './bitable-toolbar'
import { ViewSettingPopovers } from './view-setting-popovers'

function props(): ViewToolbarProps {
  return { fields: [{ id: 'title', name: 'Title', type: 'text', readOnly: false }], views: [
    { id: 'grid-stable', name: 'Grid', kind: 'grid', revision: 1, config: defaultViewConfig },
    { id: 'board-stable', name: 'Board', kind: 'kanban', revision: 3, config: defaultViewConfig },
  ], activeViewId: 'grid-stable', capabilities: { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title'] }, pending: false,
    search: { query: '', matches: [], current: 0, total: 0, pending: false, onQueryChange: vi.fn(), onNavigate: vi.fn() }, onViewSelect: vi.fn(), onViewCreate: vi.fn().mockResolvedValue(true), onViewRename: vi.fn().mockResolvedValue(true), onViewConfigChange: vi.fn().mockResolvedValue(true), onCreateField: vi.fn().mockResolvedValue(true), onRenameField: vi.fn().mockResolvedValue(true) }
}
describe('controlled view toolbar', () => {
  it('clears grouping from its chip without changing other settings', async () => {
    const input = props()
    const config = { ...defaultViewConfig, groupFieldId: 'title', rowHeight: 56 }
    input.views = [{ ...input.views[0], config }]
    render(<BitableToolbar {...input} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: text.clearGroup })))
    expect(input.onViewConfigChange).toHaveBeenCalledWith('grid-stable', { ...config, groupFieldId: undefined })
  })
  it('exposes independent lightweight settings and does not show the old panel', async () => {
    const input = props(); render(<BitableToolbar {...input} />)
    expect(screen.queryByRole('button', { name: text.viewSettings })).toBeNull()
    for (const label of [text.rowHeight, text.group, text.filter, text.sort]) expect(screen.getByRole('button', { name: label })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: text.filter }))
    expect(await screen.findByRole('button', { name: text.addFilter })).toBeTruthy()
    expect(screen.queryByRole('button', { name: text.addSort })).toBeNull()
    expect(screen.queryByRole('spinbutton')).toBeNull()
    expect(document.querySelector('[data-slot="responsive-panel-header"]')).toBeNull()
  })
  it('readonly access hides schema mutations while retaining view navigation and search', () => {
    const input = props()
    input.capabilities = { ...input.capabilities, canCreateRecords: false, canUpdateRecords: false, canDeleteRecords: false, canManageFields: false, canManageViews: false, canImport: false, canUploadAttachments: false, writableFieldIds: [] }
    render(<BitableToolbar {...input} />)
    expect(screen.queryByRole('button', { name: text.addField })).toBeNull()
    expect(screen.queryByRole('button', { name: text.createView })).toBeNull()
    expect(screen.queryByRole('button', { name: text.viewSettings })).toBeNull()
    expect(screen.getByRole('button', { name: text.rowHeight })).toBeTruthy()
    expect(screen.queryByRole('button', { name: text.renameView })).toBeNull()
    fireEvent.click(screen.getByRole('tab', { name: 'Board' }))
    fireEvent.change(screen.getByRole('textbox', { name: text.search }), { target: { value: 'read-only query' } })
    expect(input.onViewSelect).toHaveBeenCalledWith('board-stable')
    expect(input.search.onQueryChange).toHaveBeenCalledWith('read-only query')
    expect(input.onCreateField).not.toHaveBeenCalled()
  })
  it('switches by stable view ID and controlled selection follows the host', () => {
    const input = props(); const { rerender } = render(<BitableToolbar {...input} />)
    fireEvent.click(screen.getByRole('tab', { name: 'Board' }))
    expect(input.onViewSelect).toHaveBeenCalledWith('board-stable')
    expect(screen.getByRole('tab', { name: 'Grid' }).getAttribute('aria-selected')).toBe('true')
    rerender(<BitableToolbar {...input} activeViewId="board-stable" />)
    expect(screen.getByRole('tab', { name: 'Board' }).getAttribute('aria-selected')).toBe('true')
  })
  it('search emits only query intent, and navigation neither changes view nor filters', () => {
    const input = props(); input.search = { ...input.search, query: 'Alpha', total: 3, current: 1 }
    render(<BitableToolbar {...input} />)
    fireEvent.change(screen.getByRole('textbox', { name: text.search }), { target: { value: 'Beta' } })
    fireEvent.click(screen.getByRole('button', { name: text.nextMatch }))
    expect(input.search.onQueryChange).toHaveBeenCalledWith('Beta'); expect(input.search.onNavigate).toHaveBeenCalledWith(1)
    expect(input.onViewConfigChange).not.toHaveBeenCalled(); expect(input.onViewSelect).not.toHaveBeenCalled()
  })
  it('blocks view switching while a write or failed editor is guarded by the host', () => {
    const input = props(); render(<BitableToolbar {...input} pending />)
    fireEvent.click(screen.getByRole('tab', { name: 'Board' }))
    expect(input.onViewSelect).not.toHaveBeenCalled()
  })
  it('renders resolved filter chips and removes one while preserving sort and column configuration', async () => {
    const input = props()
    input.fields = [...input.fields, { id: 'status', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'done', name: 'Done' }] } }]
    const config = { ...defaultViewConfig, filters: [{ fieldId: 'status', operator: 'eq' as const, value: 'done' }], sorts: [{ fieldId: 'title', direction: 'asc' as const }], columnWidths: { title: 240 } }
    input.views = [{ ...input.views[0], config }, ...input.views.slice(1)]
    render(<BitableToolbar {...input} />)
    expect(screen.getByText('Done', { exact: false })).toBeTruthy()
    await act(async () => fireEvent.click(screen.getByRole('button', { name: text.removeFilter })))
    expect(input.onViewConfigChange).toHaveBeenCalledWith('grid-stable', { ...config, filters: [] })
    expect(input.search.onQueryChange).not.toHaveBeenCalled()
  })
  it('opens read-only view configuration without mutation controls', async () => {
    const input = props(); input.capabilities.canManageViews = false
    render(<BitableToolbar {...input} />)
    fireEvent.click(screen.getByRole('button', { name: text.rowHeight }))
    expect((await screen.findByRole('button', { name: text.rowHeightPresets.high }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.queryByRole('spinbutton')).toBeNull()
    expect(screen.queryByRole('button', { name: text.apply })).toBeNull()
    expect(input.onViewConfigChange).not.toHaveBeenCalled()
  })
  it('opens the existing field form on a new authorized header request', async () => {
    const input = props(); const { rerender } = render(<BitableToolbar {...input} createFieldRequest={0} />)
    expect(screen.queryByRole('dialog')).toBeNull()
    rerender(<BitableToolbar {...input} createFieldRequest={1} />)
    expect(await screen.findByRole('dialog', { name: text.addField })).toBeTruthy()
    expect(input.onCreateField).not.toHaveBeenCalled()
  })
  it('selecting a row-height preset saves once and preserves unrelated settings', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    const initial = { ...defaultViewConfig, columnWidths: { title: 220 }, sorts: [{ fieldId: 'title', direction: 'desc' as const }] }
    render(<ViewSettingPopovers fields={props().fields} view={{ ...props().views[0], config: initial }} pending={false} readOnly={false} onSave={onSave} />)
    fireEvent.click(screen.getByRole('button', { name: text.rowHeight }))
    expect(onSave).not.toHaveBeenCalled()
    const high = await screen.findByRole('button', { name: text.rowHeightPresets.high })
    expect(screen.queryByRole('button', { name: text.apply })).toBeNull()
    await act(async () => { fireEvent.click(high); fireEvent.click(high) })
    expect(onSave).toHaveBeenCalledTimes(1)
    expect(onSave).toHaveBeenCalledWith('grid-stable', { ...initial, rowHeight: 88 })
  })
})
