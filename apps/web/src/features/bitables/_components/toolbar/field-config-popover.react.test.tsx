import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { copy } from '@/shared/copy'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import type { Capabilities, Field, View } from '../../model/types'
import { defaultViewConfig } from '../../model/types'
import { FieldConfigPopover } from './field-config-popover'

const fields: readonly Field[] = [{ id: 'title', name: 'Title', type: 'text', readOnly: false }, { id: 'status', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'done', name: 'Done', color: 'green' }], defaultValue: 'done' } }]
const capabilities: Capabilities = { canRead: true, canManageFields: true, canManageViews: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title', 'status'] }
function props() {
  return { fields, capabilities, pending: false, view: { id: 'view', name: 'Grid', kind: 'grid', revision: 1, config: defaultViewConfig } as View, onViewConfigChange: vi.fn().mockResolvedValue(true), onCreateField: vi.fn().mockResolvedValue(true), onUpdateField: vi.fn().mockResolvedValue(true) }
}
async function open() { fireEvent.click(screen.getByRole('button', { name: text.fieldsConfig })); await screen.findByRole('searchbox', { name: text.fieldSearch }) }
describe('compact field configuration', () => {
  it('commits the displayed gap even when dropping on the list rather than a transformed row', async () => {
    const p = props()
    p.fields = [...fields, { id: 'date', name: 'Date', type: 'date', readOnly: false }]
    render(<FieldConfigPopover {...p} />); await open()
    const rows = Array.from(document.querySelectorAll<HTMLElement>('[data-drag-field]'))
    rows.forEach((row, i) => {
      row.getBoundingClientRect = () => {
        const offset = parseFloat(row.style.transform.slice(11)) || 0
        return { top: i * 40 + offset, bottom: (i + 1) * 40 + offset, left: 0, right: 200, width: 200, height: 40, x: 0, y: i * 40 + offset, toJSON() {} }
      }
    })
    const dataTransfer = { setData: vi.fn(), setDragImage: vi.fn(), effectAllowed: '' }
    fireEvent.dragStart(screen.getByRole('button', { name: copy.ui.tableColumns.drag('Title') }), { clientX: 10, clientY: 10, dataTransfer })
    const list = rows[0].parentElement!
    fireEvent.dragOver(list, { clientX: 20, clientY: 115, dataTransfer })
    await act(async () => fireEvent.drop(list, { clientX: 20, clientY: 115, dataTransfer }))
    expect(p.onViewConfigChange).toHaveBeenCalledWith('view', { ...defaultViewConfig, columnOrder: ['status', 'date', 'title'] })
    expect(p.onViewConfigChange).toHaveBeenCalledTimes(1)
    expect(document.querySelector('[data-slot="drag-insertion-preview"]')).toBeNull()
  })
  it('moves fields from the same lightweight list and preserves visibility and other settings', async () => {
    const p = props()
    p.view.config = { ...defaultViewConfig, hiddenFieldIds: ['status'], rowHeight: 72 }
    const { rerender } = render(<FieldConfigPopover {...p} />); await open()
    await act(async () => fireEvent.keyDown(screen.getByRole('button', { name: copy.ui.tableColumns.drag('Status') }), { altKey: true, key: 'ArrowUp' }))
    const config = { ...p.view.config, columnOrder: ['status', 'title'] }
    expect(p.onViewConfigChange).toHaveBeenCalledWith('view', config)
    rerender(<FieldConfigPopover {...p} view={{ ...p.view, config }} />)
    const handles = screen.getAllByRole('button', { name: /拖拽调整/ })
    expect(handles[0].getAttribute('aria-label')).toBe(copy.ui.tableColumns.drag('Status'))
    expect(screen.getByRole('button', { name: text.showField('Status') })).toBeTruthy()
  })
  it('searches the field list without a modal overlay or changing table search', async () => {
    const p = props(); render(<FieldConfigPopover {...p} />); await open()
    fireEvent.change(screen.getByRole('searchbox', { name: text.fieldSearch }), { target: { value: 'stat' } })
    expect(screen.queryByText('Title')).toBeNull(); expect(screen.getByText('Status')).toBeTruthy()
    expect(document.querySelector('[data-slot="responsive-panel-header"]')).toBeNull()
    expect(document.body.hasAttribute('data-modal-scroll-lock')).toBe(false)
    expect(p.onViewConfigChange).not.toHaveBeenCalled()
  })
  it('toggles visibility through existing view config and never hides the last visible column', async () => {
    const p = props(); const { rerender } = render(<FieldConfigPopover {...p} />); await open()
    await act(async () => fireEvent.click(screen.getByRole('button', { name: text.hideField('Status') })))
    expect(p.onViewConfigChange).toHaveBeenCalledWith('view', { ...defaultViewConfig, hiddenFieldIds: ['status'] })
    rerender(<FieldConfigPopover {...p} view={{ ...p.view, config: { ...defaultViewConfig, hiddenFieldIds: ['status'] } }} />)
    expect((screen.getByRole('button', { name: text.hideField('Title') }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: text.showField('Status') }) as HTMLButtonElement).disabled).toBe(false)
  })
  it('edits existing choices in a second popover, preserving IDs/colors/default and disabling type conversion', async () => {
    const p = props(); render(<FieldConfigPopover {...p} />); await open()
    fireEvent.click(screen.getByRole('button', { name: text.fieldMore('Status') }))
    const name = await screen.findByRole('textbox', { name: BITABLE_COPY.name })
    expect((screen.getByRole('combobox', { name: text.fieldType }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('textbox', { name: text.choiceName(1) }) as HTMLInputElement).value).toBe('Done')
    fireEvent.change(name, { target: { value: 'Progress' } })
    await act(async () => fireEvent.click(screen.getByRole('button', { name: BITABLE_COPY.save })))
    expect(p.onUpdateField).toHaveBeenCalledWith('status', 'Progress', { choices: [{ id: 'done', name: 'Done', color: 'green' }], defaultValue: 'done' })
    await waitFor(() => expect(screen.queryByRole('textbox', { name: BITABLE_COPY.name })).toBeNull())
    expect(screen.getByRole('searchbox', { name: text.fieldSearch })).toBeTruthy()
  })
  it('shows readonly configuration while removing write controls', async () => {
    const p = props(); render(<FieldConfigPopover {...p} capabilities={{ ...capabilities, canManageFields: false, canManageViews: false }} />); await open()
    expect(screen.queryByRole('button', { name: text.addField })).toBeNull()
    expect(screen.queryByRole('button', { name: text.fieldMore('Status') })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: text.hideField('Status') })); expect(p.onViewConfigChange).not.toHaveBeenCalled()
  })
})
