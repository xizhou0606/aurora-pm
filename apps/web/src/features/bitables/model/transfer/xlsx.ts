import { BITABLE_LIMITS as limits } from '../types'
import { readArchive } from './archive'
import { elements, firstText, parseXml } from './xml'
import { assertActive, TransferError, type Diagnostic, type TransferSheet, type TransferValue, type TransferWorkbook } from './types'

function sheetPath(target: string): string {
  const path = target.startsWith('/xl/') ? target.slice(1) : `xl/${target}`
  if (!/^xl\/[\w./ -]+\.xml$/.test(path) || path.split('/').some(p => p === '..' || p === '.')) throw new TransferError('unsafe_archive')
  return path
}
function dateStyles(files: Map<string, Uint8Array>): Set<number> {
  const result = new Set<number>(), bytes = files.get('xl/styles.xml')
  if (!bytes) return result
  const doc = parseXml(bytes), formats = new Map(elements(doc, 'numFmt').map(el => [Number(el.getAttribute('numFmtId')), el.getAttribute('formatCode') ?? '']))
  const xfs = elements(doc, 'cellXfs')[0]
  if (!xfs) return result
  elements(xfs, 'xf').forEach((xf, index) => {
    const id = Number(xf.getAttribute('numFmtId')), custom = (formats.get(id) ?? '').replace(/"[^"]*"|\\.|\[[^\]]*\]/g, '')
    if ((id >= 14 && id <= 22) || (id >= 45 && id <= 47) || /[ydhs]/i.test(custom)) result.add(index)
  })
  return result
}
function excelDate(serial: number, system1904: boolean): string | null {
  if (!Number.isFinite(serial) || serial < 0 || (!system1904 && Math.floor(serial) === 60)) return null
  const days = system1904 ? serial : serial - (serial >= 60 ? 1 : 0)
  const ms = Date.UTC(system1904 ? 1904 : 1899, system1904 ? 0 : 11, system1904 ? 1 : 31) + Math.round(days * 86400000)
  const date = new Date(ms)
  return Number.isFinite(date.getTime()) ? date.toISOString() : null
}

function readSheet(input: { bytes: Uint8Array | undefined; id: string; name: string; strings: string[]; dates: Set<number>; system1904: boolean; signal?: AbortSignal }): TransferSheet {
  const doc = parseXml(input.bytes), rows: TransferValue[][] = [], diagnostics: Diagnostic[] = []
  for (const row of elements(doc, 'row')) {
    const index = Number(row.getAttribute('r'))
    // Missing row references are tolerated when each cell supplies its address.
    if (!row.hasAttribute('r')) continue
    if (!Number.isSafeInteger(index) || index < 1) throw new TransferError('invalid_cell')
    if (index > limits.snapshotRows + 1) throw new TransferError('limit')
    while (rows.length < index) rows.push([])
  }
  let cells = 0
  const seen = new Set<string>()
  for (const cell of elements(doc, 'c')) {
    assertActive(input.signal)
    const address = cell.getAttribute('r') ?? '', match = /^([A-Z]+)([1-9]\d*)$/.exec(address)
    if (!match || seen.has(address)) throw new TransferError('invalid_cell')
    seen.add(address)
    const row = Number(match[2]), column = [...match[1]].reduce((n, c) => n * 26 + c.charCodeAt(0) - 64, 0)
    if (row > limits.snapshotRows + 1 || column > limits.columns || ++cells > limits.snapshotCells) throw new TransferError('limit')
    const report = (code: Diagnostic['code'], severity: Diagnostic['severity'] = 'error') => diagnostics.push({ code, severity, row, column, sheet: input.id })
    const type = cell.getAttribute('t'), raw = firstText(cell, 'v'), formula = elements(cell, 'f').length > 0
    let value: TransferValue = null
    if (formula) report(raw === null ? 'formula_missing' : 'formula_cached', raw === null ? 'error' : 'warning')
    if (type === 'inlineStr') value = elements(cell, 't').map(t => t.textContent ?? '').join('')
    else if (type === 's') {
      const index = raw === null ? -1 : Number(raw)
      if (!Number.isInteger(index) || index < 0 || index >= input.strings.length) report('invalid_cell')
      else value = input.strings[index]
    } else if (type === 'str') value = raw
    else if (type === 'b') { if (raw !== '0' && raw !== '1') report('invalid_cell'); else value = raw === '1' }
    else if (type === 'e') report('invalid_cell')
    else if (type === 'd') {
      if (!raw || !/^\d{4}-\d{2}-\d{2}T/.test(raw) || !Number.isFinite(Date.parse(raw))) report('invalid_cell')
      else value = new Date(raw).toISOString()
    } else if (type && type !== 'n') report('invalid_cell')
    else if (raw !== null) {
      const number = Number(raw)
      if (!raw.trim() || !Number.isFinite(number)) report('invalid_cell')
      else if (input.dates.has(Number(cell.getAttribute('s') ?? -1))) { value = excelDate(number, input.system1904); if (!value) report('invalid_cell') }
      else if ((Number.isInteger(number) && !Number.isSafeInteger(number)) || raw.replace(/[+\-.eE]/g, '').replace(/^0+/, '').length > 15) { value = raw; report('precision') }
      else value = number
    }
    if (typeof value === 'string' && new TextEncoder().encode(value).length > limits.cellBytes) throw new TransferError('limit')
    while (rows.length < row) rows.push([])
    rows[row - 1][column - 1] = value
  }
  const width = Math.max(0, ...rows.map(row => row.length))
  if (rows.length * width > limits.snapshotCells) throw new TransferError('limit')
  for (let i = 0; i < rows.length; i++) rows[i] = Array.from({ length: width }, (_, col) => rows[i][col] ?? null)
  if (elements(doc, 'mergeCell').length) diagnostics.push({ code: 'merged_cells', severity: 'warning', sheet: input.id })
  if (elements(doc, 'hyperlink').length || elements(doc, 'drawing').length || elements(doc, 'conditionalFormatting').length) diagnostics.push({ code: 'unsupported_content', severity: 'warning', sheet: input.id })
  return { id: input.id, name: input.name, rows, diagnostics }
}

// Bounded main-thread XML parsing. No worker or hard wall-clock timeout is claimed.
export function parseXlsx(bytes: Uint8Array, signal?: AbortSignal): TransferWorkbook {
  const files = readArchive(bytes, signal)
  const types = parseXml(files.get('[Content_Types].xml'))
  if (!elements(types, 'Override').some(el => el.getAttribute('ContentType') === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml')) throw new TransferError('unsupported_file')
  const diagnostics: Diagnostic[] = []
  for (const name of files.keys()) {
    if (/vbaProject|activeX|embeddings/i.test(name)) throw new TransferError('unsupported_file')
    if (/externalLinks|drawings|charts|pivotTables|media/i.test(name)) diagnostics.push({ code: 'unsupported_content', severity: 'warning' })
  }
  const workbook = parseXml(files.get('xl/workbook.xml')), rels = parseXml(files.get('xl/_rels/workbook.xml.rels'))
  const relationships = new Map<string, string>()
  for (const rel of elements(rels, 'Relationship')) {
    if (rel.getAttribute('TargetMode') === 'External') { diagnostics.push({ code: 'unsupported_content', severity: 'warning' }); continue }
    if (!rel.getAttribute('Type')?.endsWith('/worksheet')) continue
    const id = rel.getAttribute('Id') ?? ''
    if (!id || relationships.has(id)) throw new TransferError('invalid_file')
    relationships.set(id, sheetPath(rel.getAttribute('Target') ?? ''))
  }
  const shared = files.get('xl/sharedStrings.xml')
  const strings = shared ? elements(parseXml(shared), 'si').map(si => elements(si, 't').map(t => t.textContent ?? '').join('')) : []
  if (strings.length > limits.snapshotCells || strings.some(str => new TextEncoder().encode(str).length > limits.cellBytes)) throw new TransferError('limit')
  const dates = dateStyles(files), system1904 = ['1', 'true'].includes(elements(workbook, 'workbookPr')[0]?.getAttribute('date1904') ?? '')
  const sheets = elements(workbook, 'sheet').map(sheet => {
    const id = sheet.getAttribute('r:id') ?? '', name = sheet.getAttribute('name') ?? ''
    const path = relationships.get(id)
    if (!path) throw new TransferError('invalid_file')
    return readSheet({ bytes: files.get(path), id, name, strings, dates, system1904, signal })
  })
  if (!sheets.length || new Set(sheets.map(s => s.id)).size !== sheets.length) throw new TransferError('invalid_file')
  if (sheets.reduce((n, s) => n + s.rows.reduce((m, r) => m + r.length, 0), 0) > limits.snapshotCells) throw new TransferError('limit')
  return { sheets, diagnostics }
}
