import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import type { GridCommands } from '../../model/grid-port'
import type { Field, Row } from '../../model/types'
import { useGridInteraction } from './use-grid-interaction'

const fields: Field[] = [{ id: 'title', name: 'Title', type: 'text', readOnly: false }, { id: 'amount', name: 'Amount', type: 'number', readOnly: false }]
const rows: Row[] = [
  { id: 'record-b', revision: 7, fieldRevisions: { title: 11, amount: 12 }, values: { title: 'Original', amount: 2 } },
  { id: 'record-a', revision: 3, fieldRevisions: { title: 5, amount: 6 }, values: { title: 'Second', amount: 5 } },
]
const address = { recordId: 'record-b', fieldId: 'title' }
function commands(): GridCommands {
  return { onEdit: vi.fn().mockResolvedValue(true), onCreateEmpty: vi.fn(), onDelete: vi.fn(), onPaste: vi.fn(), onCopy: vi.fn(), onUndo: vi.fn(), onRedo: vi.fn(), canUndo: false, canRedo: false, pending: false }
}

describe('instance-owned grid interaction', () => {
  it('captures the original revision, does not autosave on open, and submits once during pending', async () => {
    const command = commands()
    let resolve!: (value: boolean) => void
    command.onEdit = vi.fn(() => new Promise<boolean>((done) => { resolve = done }))
    const { result } = renderHook(() => useGridInteraction({ instanceKey: 'a', rows, fields, writableFieldIds: ['title', 'amount'], commands: command }))
    act(() => result.current.begin(address))
    expect(command.onEdit).not.toHaveBeenCalled()
    act(() => result.current.change('Changed'))
    let first!: Promise<boolean>
    act(() => { first = result.current.commit(); void result.current.commit() })
    expect(command.onEdit).toHaveBeenCalledTimes(1)
    expect(command.onEdit).toHaveBeenCalledWith({ address, value: 'Changed', baseRevision: 11 })
    await act(async () => { resolve(true); await first })
    expect(result.current.editor.phase).toBe('view')
  })
  it('does not expose an editor before the collaboration lease is acquired', async () => {
    const command = commands()
    let acquire!: (value: boolean) => void
    command.onBeginEdit = vi.fn(() => new Promise<boolean>((resolve) => { acquire = resolve }))
    command.onCancelEdit = vi.fn()
    const { result } = renderHook(() => useGridInteraction({ instanceKey: 'a', rows, fields, writableFieldIds: ['title'], commands: command }))
    act(() => result.current.begin(address))
    expect(result.current.editor.phase).toBe('view')
    expect(command.onBeginEdit).toHaveBeenCalledWith({
      address,
      baseRevision: 11,
      original: 'Original',
      value: 'Original',
    })
    await act(async () => { acquire(true); await Promise.resolve() })
    expect(result.current.editor.phase).toBe('editing')
    act(() => result.current.cancel())
    expect(command.onCancelEdit).toHaveBeenCalledTimes(1)
  })
  it('guards IME, preserves failed drafts, and Escape cancels without a write', async () => {
    const command = commands(); command.onEdit = vi.fn().mockResolvedValue(false)
    const { result } = renderHook(() => useGridInteraction({ instanceKey: 'a', rows, fields, writableFieldIds: ['title'], commands: command }))
    act(() => { result.current.begin(address); result.current.change('中文'); result.current.composing.current = true })
    await act(async () => { expect(await result.current.commit()).toBe(false) })
    expect(command.onEdit).not.toHaveBeenCalled()
    result.current.composing.current = false
    await act(async () => { await result.current.commit() })
    expect(result.current.editor).toMatchObject({ phase: 'save_error', value: '中文', baseRevision: 11 })
    act(() => result.current.cancel())
    expect(result.current.editor.phase).toBe('view')
    expect(command.onEdit).toHaveBeenCalledTimes(1)
  })
  it('leaves invalid number drafts editable and unchanged edits are zero-write', async () => {
    const command = commands()
    const { result } = renderHook(() => useGridInteraction({ instanceKey: 'a', rows, fields, writableFieldIds: ['title', 'amount'], commands: command }))
    act(() => result.current.begin(address))
    await act(async () => { expect(await result.current.commit()).toBe(true) })
    act(() => { result.current.begin({ ...address, fieldId: 'amount' }); result.current.change('NaN') })
    await act(async () => { expect(await result.current.commit()).toBe(false) })
    expect(result.current.editor.phase).toBe('save_error')
    expect(command.onEdit).not.toHaveBeenCalled()
  })
  it('rejects read-only editing and stable IDs survive row order changes', () => {
    const command = commands()
    const { result, rerender } = renderHook(({ items }) => useGridInteraction({ instanceKey: 'a', rows: items, fields, writableFieldIds: [], commands: command }), { initialProps: { items: rows } })
    act(() => { result.current.select(address); result.current.begin(address) })
    expect(result.current.editor.phase).toBe('view')
    rerender({ items: [...rows].reverse() })
    expect(result.current.selection?.focus).toEqual(address)
    expect(command.onEdit).not.toHaveBeenCalled()
  })
  it('supports short rectangle selection, full row selection, and context reset', () => {
    const command = commands()
    const { result, rerender } = renderHook(({ instanceKey }) => useGridInteraction({ instanceKey, rows, fields, writableFieldIds: ['title'], commands: command }), { initialProps: { instanceKey: 'first' } })
    act(() => { result.current.pointerDown(address, false); result.current.pointerEnter({ recordId: 'record-a', fieldId: 'amount' }) })
    expect(result.current.isSelected({ recordId: 'record-b', fieldId: 'amount' })).toBe(true)
    act(() => result.current.selectRow('record-a'))
    expect(result.current.selection).toEqual({ anchor: { recordId: 'record-a', fieldId: 'title' }, focus: { recordId: 'record-a', fieldId: 'amount' } })
    rerender({ instanceKey: 'second' })
    expect(result.current.selection).toBeNull()
  })
  it('ignores an old save completion after instance/workspace change', async () => {
    const command = commands(); let resolve!: (value: boolean) => void
    command.onEdit = vi.fn(() => new Promise<boolean>((done) => { resolve = done }))
    const { result, rerender } = renderHook(({ instanceKey }) => useGridInteraction({ instanceKey, rows, fields, writableFieldIds: ['title'], commands: command }), { initialProps: { instanceKey: 'first' } })
    act(() => { result.current.begin(address); result.current.change('old') })
    let promise!: Promise<boolean>
    act(() => { promise = result.current.commit() })
    rerender({ instanceKey: 'second' })
    act(() => { result.current.begin(address); result.current.change('new') })
    await act(async () => { resolve(true); await promise })
    expect(result.current.editor).toMatchObject({ phase: 'editing', value: 'new' })
  })
})
