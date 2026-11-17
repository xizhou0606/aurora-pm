import { parseFieldInput, isEditableField } from '../fields'
import { BITABLE_LIMITS as limits, type Field, type RecordCommand } from '../types'
import { parseCsv } from './csv'
import { assertActive, TransferError, type Diagnostic, type TransferSheet, type TransferWorkbook } from './types'

export async function parseTransferFile(file: File, signal?: AbortSignal): Promise<TransferWorkbook> {
  assertActive(signal)
  if (file.size > limits.fileBytes) throw new TransferError('limit')
  const extension = file.name.split('.').at(-1)?.toLowerCase()
  if (extension !== 'csv' && extension !== 'xlsx') throw new TransferError('unsupported_file')
  const bytes = new Uint8Array(await file.arrayBuffer()); assertActive(signal)
  if (extension === 'csv') return { sheets: [parseCsv(bytes, signal)], diagnostics: [] }
  const { parseXlsx } = await import('./xlsx'); assertActive(signal)
  return parseXlsx(bytes, signal)
}

export function initialMapping(sheet: TransferSheet, fields: readonly Field[], header: boolean): string[] {
  const width = Math.max(0, ...sheet.rows.map(row => row.length))
  return Array.from({ length: width }, (_, i) => {
    if (!header) return ''
    const name = String(sheet.rows[0]?.[i] ?? '')
    const matches = fields.filter(field => field.name === name && isEditableField(field) && field.type !== 'attachment')
    return matches.length === 1 && sheet.rows[0].filter(value => String(value ?? '') === name).length === 1 ? matches[0].id : ''
  })
}

export function prepareImport(sheet: TransferSheet, fields: readonly Field[], mapping: readonly string[], header: boolean): { commands: RecordCommand[]; diagnostics: Diagnostic[] } {
  const diagnostics = [...sheet.diagnostics], commands: RecordCommand[] = []
  const mapped = mapping.filter(Boolean), source = sheet.rows.slice(header ? 1 : 0)
  if (!mapped.length || new Set(mapped).size !== mapped.length || mapping.length > limits.columns) diagnostics.push({ code: 'mapping', severity: 'error' })
  if (!source.length) diagnostics.push({ code: 'empty', severity: 'error' })
  if (source.length > limits.batchRows || source.length * mapped.length > limits.batchCells) diagnostics.push({ code: 'batch_limit', severity: 'error' })
  for (let r = 0; r < source.length; r++) {
    const values: Record<string, import('../types').CellValue> = {}
    mapping.forEach((id, column) => {
      if (!id) return
      const field = fields.find(item => item.id === id), row = r + (header ? 2 : 1)
      if (!field || !isEditableField(field) || field.type === 'attachment') { diagnostics.push({ code: 'read_only', severity: 'error', row, column: column + 1 }); return }
      const result = parseFieldInput(field, source[r][column] ?? null)
      if (!result.ok) diagnostics.push({ code: 'invalid_cell', severity: 'error', row, column: column + 1, detail: result.message })
      else values[id] = result.value
    })
    commands.push({ kind: 'create', values })
  }
  return { commands, diagnostics }
}
