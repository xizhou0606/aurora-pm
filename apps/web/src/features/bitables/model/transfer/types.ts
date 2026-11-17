import { BITABLE_LIMITS as limits } from '../types'

export type TransferValue = string | number | boolean | null
export type DiagnosticCode = 'invalid_file' | 'unsupported_file' | 'limit' | 'invalid_encoding' | 'invalid_csv'
  | 'invalid_xml' | 'unsafe_archive' | 'formula_cached' | 'formula_missing' | 'merged_cells' | 'unsupported_content'
  | 'invalid_cell' | 'precision' | 'mapping' | 'read_only' | 'batch_limit' | 'empty' | 'cancelled' | 'csv_escaped'
export type Diagnostic = { code: DiagnosticCode; severity: 'error' | 'warning'; sheet?: string; row?: number; column?: number; detail?: string }
export type TransferSheet = { id: string; name: string; rows: TransferValue[][]; diagnostics: Diagnostic[] }
export type TransferWorkbook = { sheets: TransferSheet[]; diagnostics: Diagnostic[] }
export class TransferError extends Error {
  constructor(readonly code: DiagnosticCode) { super(code); this.name = 'TransferError' }
}
export function assertActive(signal?: AbortSignal) { if (signal?.aborted) throw new TransferError('cancelled') }

export function assertMatrixBudget(rows: readonly (readonly TransferValue[])[]) {
  if (rows.length > limits.snapshotRows + 1) throw new TransferError('limit')
  let cells = 0, bytes = 0
  for (const row of rows) {
    if (row.length > limits.columns || (cells += row.length) > limits.snapshotCells) throw new TransferError('limit')
    for (const value of row) {
      const size = new TextEncoder().encode(String(value ?? '')).length
      if (size > limits.cellBytes || (bytes += size) > limits.expandedBytes) throw new TransferError('limit')
    }
  }
}
