import { zipSync, strToU8 } from 'fflate'
import { BITABLE_LIMITS as limits, type ExportSnapshot } from '../types'
import { encodeCsv } from './csv'
import { escapeXml } from './xml'
import { assertMatrixBudget, TransferError, type Diagnostic, type TransferValue } from './types'

export function exportRows(snapshot: ExportSnapshot): TransferValue[][] {
  if (snapshot.fields.length > limits.columns || snapshot.records.length > limits.snapshotRows || snapshot.records.length * snapshot.fields.length > limits.snapshotCells) throw new TransferError('limit')
  return [snapshot.fields.map(field => field.name), ...snapshot.records.map(record => snapshot.fields.map(field => {
    const value = record.values[field.id] ?? null
    if (!Array.isArray(value)) return value as TransferValue
    // Deliberately export no URLs, signed tokens or provider identity information.
    if (field.type === 'user' || field.type === 'created_by' || field.type === 'last_modified_by') return value.map(item => typeof item === 'object' && 'id' in item ? String(item.id) : '').join(',')
    if (field.type === 'attachment') return value.map(item => typeof item === 'object' && 'name' in item ? item.name : '').join(', ')
    return value.filter((item): item is string => typeof item === 'string').join(',')
  }))]
}

function columnName(index: number): string {
  let result = '', value = index + 1
  while (value) { result = String.fromCharCode(65 + (value - 1) % 26) + result; value = Math.floor((value - 1) / 26) }
  return result
}
export function encodeXlsx(rows: readonly (readonly TransferValue[])[], dateColumns: ReadonlySet<number> = new Set()): Uint8Array {
  assertMatrixBudget(rows)
  const ns = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'
  const data = rows.map((row, r) => `<row r="${r + 1}">${row.map((value, c) => {
    if (value === null) return ''
    const ref = `${columnName(c)}${r + 1}`
    if (typeof value === 'boolean') return `<c r="${ref}" t="b"><v>${Number(value)}</v></c>`
    if (typeof value === 'number') { if (!Number.isFinite(value)) throw new TransferError('invalid_cell'); return `<c r="${ref}"><v>${value}</v></c>` }
    if (r > 0 && dateColumns.has(c) && /^\d{4}-\d{2}-\d{2}T/.test(value) && Number.isFinite(Date.parse(value))) {
      const ms = Date.parse(value), serial = (ms - Date.UTC(1899, 11, 30)) / 86400000 - (ms < Date.UTC(1900, 2, 1) ? 1 : 0)
      if (serial >= 0) return `<c r="${ref}" s="1"><v>${serial}</v></c>`
    }
    return `<c r="${ref}" t="inlineStr"><is><t xml:space="preserve">${escapeXml(value)}</t></is></c>`
  }).join('')}</row>`).join('')
  return zipSync({
    '[Content_Types].xml': strToU8('<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>'),
    '_rels/.rels': strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>'),
    'xl/workbook.xml': strToU8(`<workbook xmlns="${ns}" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`),
    'xl/_rels/workbook.xml.rels': strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>'),
    'xl/styles.xml': strToU8(`<styleSheet xmlns="${ns}"><numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy-mm-dd hh:mm:ss"/></numFmts><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="2"><xf numFmtId="0" xfId="0"/><xf numFmtId="164" xfId="0" applyNumberFormat="1"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`),
    'xl/worksheets/sheet1.xml': strToU8(`<worksheet xmlns="${ns}"><sheetData>${data}</sheetData></worksheet>`),
  }, { level: 6 })
}

export function createExport(snapshot: ExportSnapshot, format: 'csv' | 'xlsx'): { blob: Blob; diagnostics: Diagnostic[] } {
  const rows = exportRows(snapshot), diagnostics: Diagnostic[] = []
  if (rows.some(row => row.some(value => new TextEncoder().encode(String(value ?? '')).length > limits.cellBytes))) throw new TransferError('limit')
  if (snapshot.fields.some(field => field.type === 'attachment')) diagnostics.push({ code: 'unsupported_content', severity: 'warning' })
  if (format === 'csv') {
    const { bytes, escaped } = encodeCsv(rows)
    if (escaped) diagnostics.push({ code: 'csv_escaped', severity: 'warning' })
    return { blob: new Blob([bytes as BlobPart], { type: 'text/csv;charset=utf-8' }), diagnostics }
  }
  const dates = new Set(snapshot.fields.flatMap((field, i) => ['date', 'created_time', 'last_modified_time'].includes(field.type) ? [i] : []))
  return { blob: new Blob([encodeXlsx(rows, dates) as BlobPart], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), diagnostics }
}
