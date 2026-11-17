import { BITABLE_LIMITS as limits } from '../types'
import { assertActive, assertMatrixBudget, TransferError, type TransferSheet, type TransferValue } from './types'

export function parseCsv(bytes: Uint8Array, signal?: AbortSignal): TransferSheet {
  if (bytes.length > limits.fileBytes) throw new TransferError('limit')
  let text: string
  try { text = new TextDecoder('utf-8', { fatal: true }).decode(bytes).replace(/^\uFEFF/, '') }
  catch { throw new TransferError('invalid_encoding') }
  return parseDelimited(text, ',', signal)
}

export function parseDelimited(text: string, delimiter: ',' | '\t', signal?: AbortSignal): TransferSheet {
  if (new TextEncoder().encode(text).length > limits.fileBytes) throw new TransferError('limit')
  const rows: string[][] = []
  let row: string[] = [], cell = '', state: 'plain' | 'quoted' | 'closed' = 'plain', cells = 0
  const finishCell = () => {
    if (new TextEncoder().encode(cell).length > limits.cellBytes || row.length >= limits.columns || ++cells > limits.snapshotCells) throw new TransferError('limit')
    row.push(cell); cell = ''; state = 'plain'
  }
  const finishRow = () => {
    finishCell()
    if (rows.length >= limits.snapshotRows + 1) throw new TransferError('limit')
    rows.push(row); row = []
  }
  for (let i = 0; i < text.length; i++) {
    if (i % 4096 === 0) assertActive(signal)
    const ch = text[i]
    if (ch === '\0') throw new TransferError('invalid_csv')
    if (state === 'quoted') {
      if (ch === '"' && text[i + 1] === '"') { cell += '"'; i++ }
      else if (ch === '"') state = 'closed'
      else cell += ch
    } else if (ch === delimiter) finishCell()
    else if (ch === '\r' || ch === '\n') { finishRow(); if (ch === '\r' && text[i + 1] === '\n') i++ }
    else if (state === 'closed') throw new TransferError('invalid_csv')
    else if (ch === '"') { if (cell) throw new TransferError('invalid_csv'); state = 'quoted' }
    else cell += ch
    if (cell.length > limits.cellBytes) throw new TransferError('limit')
  }
  if (state === 'quoted') throw new TransferError('invalid_csv')
  if (cell || row.length || state === 'closed' || (text.length > 0 && !/[\r\n]$/.test(text))) finishRow()
  return { id: 'csv', name: '', rows, diagnostics: [] }
}

// Escape text, never reinterpret legitimate typed negative numbers as formulas.
export function csvCell(value: TransferValue): { text: string; escaped: boolean } {
  let text = value == null ? '' : String(value)
  const escaped = typeof value === 'string' && /^[\s\u0000-\u001f]*[=+\-@]/.test(value)
  if (escaped) text = `'${text}`
  return { text: `"${text.replaceAll('"', '""')}"`, escaped }
}

export function encodeCsv(rows: readonly (readonly TransferValue[])[]): { bytes: Uint8Array; escaped: boolean } {
  assertMatrixBudget(rows)
  let escaped = false
  const output = rows.map(row => row.map(value => {
    const result = csvCell(value); escaped ||= result.escaped; return result.text
  }).join(',')).join('\r\n')
  return { bytes: new TextEncoder().encode(`\uFEFF${output}\r\n`), escaped }
}

export function stringifyDelimited(rows: readonly (readonly TransferValue[])[], delimiter: ',' | '\t'): string {
  assertMatrixBudget(rows)
  return rows.map(row => row.map(value => csvCell(value).text).join(delimiter)).join('\r\n')
}
