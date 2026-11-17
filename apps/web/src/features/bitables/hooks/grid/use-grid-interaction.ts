import { useEffect, useRef, useState } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { isEditableField, parseFieldInput } from '../../model/fields'
import type { CellAddress, CellValue, Field, Row } from '../../model/types'
import type { GridCommands, GridSelection } from '../../model/grid-port'

export type CellEditorState =
  | { phase: 'view' }
  | { phase: 'editing' | 'saving' | 'save_error'; address: CellAddress; baseRevision: number; value: CellValue; original: CellValue; message?: string }

export function sameCell(left: CellAddress | null | undefined, right: CellAddress | null | undefined) {
  return Boolean(left && right && left.recordId === right.recordId && left.fieldId === right.fieldId)
}

export function useGridInteraction({ instanceKey, rows, fields, writableFieldIds, commands }: {
  instanceKey: string; rows: readonly Row[]; fields: readonly Field[]; writableFieldIds: readonly string[]; commands: GridCommands
}) {
  const [selection, setSelection] = useState<GridSelection | null>(null)
  const [editor, setEditor] = useState<CellEditorState>({ phase: 'view' })
  const editorRef = useRef(editor)
  const submitting = useRef(false)
  const acquiring = useRef(false)
  const generation = useRef(0)
  const composing = useRef(false)
  const drag = useRef(false)
  const setCurrentEditor = (next: CellEditorState) => { editorRef.current = next; setEditor(next) }

  useEffect(() => {
    generation.current += 1
    submitting.current = false
    acquiring.current = false
    drag.current = false
    composing.current = false
    setSelection(null)
    setCurrentEditor({ phase: 'view' })
    return () => { generation.current += 1 }
  }, [instanceKey])

  useEffect(() => {
    const end = () => { drag.current = false }
    window.addEventListener('pointerup', end)
    window.addEventListener('pointercancel', end)
    return () => { window.removeEventListener('pointerup', end); window.removeEventListener('pointercancel', end) }
  }, [])

  const cancel = () => {
    if (submitting.current) return
    if (editorRef.current.phase !== 'view') commands.onCancelEdit?.()
    setCurrentEditor({ phase: 'view' })
  }
  const select = (address: CellAddress, extend = false) => {
    if (editorRef.current.phase !== 'view') return
    setSelection((current) => ({ anchor: extend && current ? current.anchor : address, focus: address }))
  }
  const begin = (address: CellAddress, initial?: string) => {
    if (submitting.current || acquiring.current || commands.pending || editorRef.current.phase !== 'view') return
    const row = rows.find((entry) => entry.id === address.recordId)
    const field = fields.find((entry) => entry.id === address.fieldId)
    if (!row || !field || !isEditableField(field) || !writableFieldIds.includes(field.id) || field.type === 'attachment') return
    const baseRevision = row.fieldRevisions[field.id]
    if (!Number.isSafeInteger(baseRevision)) return
    const original = row.values[field.id] ?? null
    setSelection({ anchor: address, focus: address })
    const next = { phase: 'editing' as const, address, baseRevision, original, value: initial ?? original }
    if (!commands.onBeginEdit) { setCurrentEditor(next); return }
    acquiring.current = true
    const ticket = generation.current
    void commands.onBeginEdit({ address, baseRevision, original, value: next.value }).then((acquired) => {
      if (ticket === generation.current && editorRef.current.phase === 'view' && acquired) setCurrentEditor(next)
    }).finally(() => {
      if (ticket === generation.current) acquiring.current = false
    })
  }
  const change = (value: CellValue) => {
    const current = editorRef.current
    if (current.phase !== 'view' && current.phase !== 'saving') setCurrentEditor({ ...current, phase: 'editing', value, message: undefined })
  }
  const commit = async (): Promise<boolean> => {
    const current = editorRef.current
    if (current.phase === 'view') return true
    if (submitting.current || commands.pending || composing.current) return false
    const field = fields.find((entry) => entry.id === current.address.fieldId)
    if (!field || !isEditableField(field) || !writableFieldIds.includes(field.id)) return false
    const parsed = parseFieldInput(field, current.value)
    if (!parsed.ok) { setCurrentEditor({ ...current, phase: 'save_error', message: parsed.message }); return false }
    if (JSON.stringify(parsed.value) === JSON.stringify(current.original)) { cancel(); return true }
    submitting.current = true
    setCurrentEditor({ ...current, phase: 'saving' })
    const ticket = generation.current
    let saved = false
    try { saved = await commands.onEdit({ address: current.address, baseRevision: current.baseRevision, value: parsed.value }) } catch { /* Safe feedback remains at this boundary. */ }
    if (ticket !== generation.current) return false
    submitting.current = false
    setCurrentEditor(saved ? { phase: 'view' } : { ...current, phase: 'save_error', message: BITABLE_COPY.saveError })
    return saved
  }
  const move = (rowDelta: number, columnDelta: number, extend = false): CellAddress | null => {
    if (!rows.length || !fields.length) return null
    const base = selection?.focus
    const rowIndex = Math.max(0, rows.findIndex((row) => row.id === base?.recordId))
    const fieldIndex = Math.max(0, fields.findIndex((field) => field.id === base?.fieldId))
    const next = {
      recordId: rows[Math.min(rows.length - 1, Math.max(0, rowIndex + rowDelta))].id,
      fieldId: fields[Math.min(fields.length - 1, Math.max(0, fieldIndex + columnDelta))].id,
    }
    select(next, extend)
    return next
  }
  const isSelected = (address: CellAddress) => {
    if (!selection) return false
    const row = rows.findIndex((entry) => entry.id === address.recordId)
    const column = fields.findIndex((entry) => entry.id === address.fieldId)
    const firstRow = rows.findIndex((entry) => entry.id === selection.anchor.recordId)
    const lastRow = rows.findIndex((entry) => entry.id === selection.focus.recordId)
    const firstColumn = fields.findIndex((entry) => entry.id === selection.anchor.fieldId)
    const lastColumn = fields.findIndex((entry) => entry.id === selection.focus.fieldId)
    return row >= Math.min(firstRow, lastRow) && row <= Math.max(firstRow, lastRow)
      && column >= Math.min(firstColumn, lastColumn) && column <= Math.max(firstColumn, lastColumn)
  }
  return {
    selection, editor, composing, cancel, select, begin, change, commit, move, isSelected,
    isEditing: () => editorRef.current.phase !== 'view',
    isRowSelected: (recordId: string) => Boolean(fields.length && isSelected({ recordId, fieldId: fields[0].id }) && isSelected({ recordId, fieldId: fields[fields.length - 1].id })),
    clearSelection: () => { if (editorRef.current.phase === 'view') setSelection(null) },
    selectRow: (recordId: string) => { if (fields.length && editorRef.current.phase === 'view') setSelection({ anchor: { recordId, fieldId: fields[0].id }, focus: { recordId, fieldId: fields[fields.length - 1].id } }) },
    pointerDown: (address: CellAddress, extend: boolean) => { drag.current = true; select(address, extend) },
    pointerEnter: (address: CellAddress) => { if (drag.current) select(address, true) },
  }
}
