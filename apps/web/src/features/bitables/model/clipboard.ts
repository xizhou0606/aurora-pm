import { BITABLE_LIMITS, type CellEdit, type Field, type Row } from './types'
import type { GridSelection } from './grid-port'
import { displayCellValue, parseFieldInput } from './fields'
import { parseDelimited, stringifyDelimited } from './transfer/csv'

export function copySelection(selection: GridSelection, fields: readonly Field[], rows: readonly Row[]): string {
  const r1 = rows.findIndex(row => row.id === selection.anchor.recordId), r2 = rows.findIndex(row => row.id === selection.focus.recordId)
  const c1 = fields.findIndex(field => field.id === selection.anchor.fieldId), c2 = fields.findIndex(field => field.id === selection.focus.fieldId)
  if ([r1, r2, c1, c2].some(index => index < 0)) return ''
  return stringifyDelimited(rows.slice(Math.min(r1, r2), Math.max(r1, r2) + 1).map(row => fields.slice(Math.min(c1, c2), Math.max(c1, c2) + 1).map(field => displayCellValue(field, row.values[field.id] ?? null))), '\t')
}
export function pasteEdits(text: string, anchor: GridSelection['anchor'], fields: readonly Field[], rows: readonly Row[], writable: readonly string[]): readonly CellEdit[] {
  const matrix = parseDelimited(text, '\t').rows
  const startRow = rows.findIndex(row => row.id === anchor.recordId), startColumn = fields.findIndex(field => field.id === anchor.fieldId)
  if (startRow < 0 || startColumn < 0 || !matrix.length || matrix.length > BITABLE_LIMITS.batchRows || matrix.reduce((n, row) => n + row.length, 0) > BITABLE_LIMITS.batchCells || startRow + matrix.length > rows.length) throw new Error('BITABLE_PASTE_RANGE_INVALID')
  return matrix.flatMap((values, index) => {
    const row = rows[startRow + index]
    return values.map((value, offset) => {
      const field = fields[startColumn + offset]
      if (!field || !writable.includes(field.id)) throw new Error('BITABLE_PASTE_RANGE_INVALID')
      const parsed = parseFieldInput(field, value)
      if (!parsed.ok) throw new Error('BITABLE_VALUE_INVALID')
      const baseRevision = row.fieldRevisions[field.id]
      if (!Number.isSafeInteger(baseRevision)) throw new Error('BITABLE_PASTE_RANGE_INVALID')
      return { address: { recordId: row.id, fieldId: field.id }, baseRevision, value: parsed.value }
    })
  })
}
