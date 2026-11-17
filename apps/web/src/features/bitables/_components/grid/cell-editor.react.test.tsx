import { act, fireEvent, render, screen } from '@testing-library/react'
import { useEffect } from 'react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_COLLABORATION_COPY } from '@/shared/copy/bitables-collaboration'
import { copy } from '@/shared/copy'
import type { CellEdit, CellValue, Field, UserRef } from '../../model/types'
import type { GridCommands } from '../../model/grid-port'
import { useGridInteraction } from '../../hooks/grid/use-grid-interaction'
import { CellEditor } from './cell-editor'


const __stubPrev_navigator = (globalThis as any).navigator as unknown
const people: UserRef[] = [{ id: 1, name: 'Alice' }, { id: 2, name: 'Bob' }]
function mount(field: Field, value: CellValue = null) {
  const edit = vi.fn(async (_edit: CellEdit) => true)
  function Harness() {
    const interaction = useGridInteraction({ instanceKey: 'scope', rows: [{ id: 'r', revision: 7, fieldRevisions: { f: 13 }, values: { f: value } }], fields: [field], writableFieldIds: ['f'], commands: { pending: false, onEdit: edit } as unknown as GridCommands })
    useEffect(() => { interaction.begin({ recordId: 'r', fieldId: 'f' }) }, [])
    return interaction.editor.phase === 'view' ? null : <CellEditor field={field} state={interaction.editor} people={people} height={24} onChange={interaction.change} onCommit={interaction.commit} onCancel={interaction.cancel} onComposition={() => undefined} />
  }
  const result = render(<Harness />)
  return { ...result, edit }
}
describe('in-cell automatic picker commits', () => {
  it('copies a failed draft before exiting instead of retrying its stale version', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(globalThis, 'navigator', { configurable: true, writable: true, value: { ...navigator, clipboard: { writeText } } })
    const onCancel = vi.fn(), onCommit = vi.fn()
    try {
      render(<CellEditor field={{ id: 'f', name: 'Title', type: 'text', readOnly: false }} state={{ phase: 'save_error', address: { recordId: 'r', fieldId: 'f' }, original: 'old', value: 'my draft', baseRevision: 1, message: BITABLE_COPY.saveError }} people={[]} onChange={vi.fn()} onCommit={onCommit} onCancel={onCancel} onComposition={vi.fn()} />)
      await act(async () => { fireEvent.click(screen.getByRole('button', { name: BITABLE_COLLABORATION_COPY.copyDraftAndExit })) })
      expect(writeText).toHaveBeenCalledWith('my draft')
      expect(onCancel).toHaveBeenCalledTimes(1)
      expect(onCommit).not.toHaveBeenCalled()
    } finally { Object.defineProperty(globalThis, 'navigator', { configurable: true, writable: true, value: __stubPrev_navigator }) }
  })
  it('focuses the compact checkbox editor and Escape leaves without a write', () => {
    const { edit, container } = mount({ id: 'f', name: 'Done', type: 'checkbox', readOnly: false }, false)
    const checkbox = screen.getByRole('checkbox', { name: 'Done' })
    expect(document.activeElement).toBe(checkbox)
    expect(checkbox.className).toContain('focus-visible:ring-0')
    fireEvent.keyDown(checkbox, { key: 'Escape' })
    expect(container.querySelector('[data-bitable-editor]')).toBeNull()
    expect(edit).not.toHaveBeenCalled()
  })
  it('uses a transparent borderless number input within the unchanged cell', () => {
    mount({ id: 'f', name: 'Count', type: 'number', readOnly: false }, 12)
    const input = screen.getByRole('textbox', { name: 'Count' })
    expect(input.className).toContain('bg-transparent')
    expect(input.className).toContain('border-0')
    expect(input.className).toContain('focus-visible:ring-0')
  })
  it('single choice immediately saves its latest ID without a save/cancel footer', async () => {
    const { edit, container } = mount({ id: 'f', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'done', name: 'Done' }] } })
    expect((container.querySelector('[data-bitable-editor]') as HTMLElement).style.getPropertyValue('--editor-h')).toBe('24px')
    expect(screen.queryByRole('button', { name: BITABLE_COPY.save })).toBeNull()
    expect(screen.queryByRole('button', { name: BITABLE_COPY.cancel })).toBeNull()
    await act(async () => { fireEvent.click(await screen.findByRole('option', { name: 'Done' })) })
    expect(edit).toHaveBeenCalledWith({ address: { recordId: 'r', fieldId: 'f' }, baseRevision: 13, value: 'done' })
  })
  it('single person opens automatically and selection saves the authoritative person immediately', async () => {
    const { edit } = mount({ id: 'f', name: 'Owner', type: 'user', readOnly: false })
    const bob = await screen.findByRole('option', { name: 'Bob' })
    await act(async () => { fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' }); fireEvent.click(bob) })
    expect(edit).toHaveBeenCalledTimes(1)
    expect(edit.mock.calls[0][0]).toEqual({ address: { recordId: 'r', fieldId: 'f' }, baseRevision: 13, value: [{ id: 2, name: 'Bob' }] })
  })
  it('multiple people accumulate locally and commit once when the picker closes', async () => {
    const { edit } = mount({ id: 'f', name: 'Owners', type: 'user', readOnly: false, options: { multiple: true } }, [people[0]])
    const bob = await screen.findByRole('option', { name: 'Bob' })
    fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' }); fireEvent.click(bob)
    expect(edit).not.toHaveBeenCalled()
    await act(async () => { fireEvent.pointerDown(document.body, { button: 0, pointerType: 'mouse' }); fireEvent.mouseDown(document.body, { button: 0 }); fireEvent.click(document.body) })
    expect(edit).toHaveBeenCalledTimes(1)
    expect(edit.mock.calls[0][0]).toEqual({ address: { recordId: 'r', fieldId: 'f' }, baseRevision: 13, value: people })
  })
  it('Escape discards a multiple-person draft without committing', async () => {
    const { edit } = mount({ id: 'f', name: 'Owners', type: 'user', readOnly: false, options: { multiple: true } }, [people[0]])
    const bob = await screen.findByRole('option', { name: 'Bob' })
    fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' }); fireEvent.click(bob)
    fireEvent.keyDown(screen.getByRole('combobox', { name: 'Owners' }), { key: 'Escape' })
    expect(edit).not.toHaveBeenCalled()
  })
  it('date picker opens automatically and choosing today commits without resizing the cell', async () => {
    const { edit, container } = mount({ id: 'f', name: 'Due', type: 'date', readOnly: false })
    const today = await screen.findByRole('button', { name: copy.ui.datePicker.today })
    expect((container.querySelector('[data-bitable-editor]') as HTMLElement).style.getPropertyValue('--editor-h')).toBe('24px')
    await act(async () => { fireEvent.click(today) })
    expect(edit).toHaveBeenCalledTimes(1)
    expect(edit.mock.calls[0][0].value).toMatch(/^\d{4}-\d{2}-\d{2}T00:00:00.000Z$/)
  })
  it('closing an unchanged date picker outside releases editing without a mutation', async () => {
    const { edit, container } = mount({ id: 'f', name: 'Due', type: 'date', readOnly: false }, '2026-09-07T00:00:00.000Z')
    await screen.findByRole('button', { name: copy.ui.datePicker.today })
    await act(async () => { fireEvent.pointerDown(document.body, { button: 0, pointerType: 'mouse' }); fireEvent.mouseDown(document.body, { button: 0 }); fireEvent.click(document.body) })
    expect(container.querySelector('[data-bitable-editor]')).toBeNull()
    expect(edit).not.toHaveBeenCalled()
  })
  it('Escape closes a date editor without a mutation or an outside-close follow-up', async () => {
    const { edit, container } = mount({ id: 'f', name: 'Due', type: 'date', readOnly: false })
    const today = await screen.findByRole('button', { name: copy.ui.datePicker.today })
    await act(async () => { fireEvent.keyDown(today, { key: 'Escape' }) })
    expect(container.querySelector('[data-bitable-editor]')).toBeNull()
    expect(edit).not.toHaveBeenCalled()
  })
  it('keeps a failed draft and displays its error outside the fixed cell height', async () => {
    const { edit, container } = mount({ id: 'f', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'done', name: 'Done' }] } })
    edit.mockResolvedValue(false)
    const option = await screen.findByRole('option', { name: 'Done' })
    await act(async () => { fireEvent.click(option) })
    expect(screen.getByText(BITABLE_COPY.saveError)).toBeTruthy()
    expect((container.querySelector('[data-bitable-editor]') as HTMLElement).style.getPropertyValue('--editor-h')).toBe('24px')
    expect(container.querySelector('aside')?.className).toContain('absolute')
  })
})
