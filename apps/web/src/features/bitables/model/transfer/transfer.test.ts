import { describe, expect, it } from 'bun:test'
import { strToU8, unzipSync, zipSync } from 'fflate'
import { BITABLE_LIMITS, type ExportSnapshot, type Field } from '../types'
import { parseCsv, encodeCsv, parseDelimited, stringifyDelimited } from './csv'
import { createExport, encodeXlsx, exportRows } from './export'
import { initialMapping, prepareImport } from './import'
import { parseXlsx } from './xlsx'
import { readArchive } from './archive'
import { parseXml } from './xml'

const field: Field = { id: 'text', name: '标题', type: 'text', readOnly: false }
const snapshot: ExportSnapshot = { snapshotId: 'test', schemaRevision: 1, fields: [field], records: [{ id: 'r1', revision: 1, fieldRevisions: { text: 1 }, values: { text: '中文' } }] }
const bytes = (text: string) => new TextEncoder().encode(text)
function fixture(xml: string, extra: Record<string, Uint8Array> = {}) {
  return zipSync({ ...unzipSync(encodeXlsx([['a']])), 'xl/worksheets/sheet1.xml': strToU8(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>${xml}</sheetData></worksheet>`), ...extra })
}

describe('bounded CSV exchange', () => {
  it('preserves BOM, Chinese, leading zeros, quoted commas/newlines and empty cells', () => {
    expect(parseCsv(bytes('\uFEFF标题,内容,空\r\n001,"中,文\n第二行",\r\n"a""b",,')).rows).toEqual([['标题', '内容', '空'], ['001', '中,文\n第二行', ''], ['a"b', '', '']])
  })
  it.each(['"unterminated', 'a"b', '"a"b', 'a\0b'])('rejects malformed CSV %s', text => expect(() => parseCsv(bytes(text))).toThrow())
  it('rejects non-UTF8, file/cell/column limits and cancellation', () => {
    expect(() => parseCsv(new Uint8Array([0xff]))).toThrow('invalid_encoding')
    expect(() => parseCsv(new Uint8Array(BITABLE_LIMITS.fileBytes + 1))).toThrow('limit')
    expect(() => parseCsv(bytes('x'.repeat(BITABLE_LIMITS.cellBytes + 1)))).toThrow('limit')
    expect(() => parseCsv(bytes(Array(101).fill('a').join(',')))).toThrow('limit')
    const abort = new AbortController(); abort.abort()
    expect(() => parseCsv(bytes('a'), abort.signal)).toThrow('cancelled')
  })
  it('escapes formula text but preserves typed negative numbers', () => {
    const result = encodeCsv([['=1+1', '  @evil', '-123', -123, 'ordinary']])
    expect(result.escaped).toBe(true)
    expect(parseCsv(result.bytes).rows[0]).toEqual(["'=1+1", "'  @evil", "'-123", '-123', 'ordinary'])
  })
  it('shares quoted TSV parser with clipboard', () => {
    const data = [['a\tb', '多\n行'], ['001', 'x"y']]
    expect(parseDelimited(stringifyDelimited(data, '\t'), '\t').rows).toEqual(data)
  })
})

describe('one mapping and validation path', () => {
  it('does not guess duplicate headers or duplicate field names', () => {
    expect(initialMapping(parseCsv(bytes('标题,标题\na,b')), [field], true)).toEqual(['', ''])
    expect(initialMapping(parseCsv(bytes('标题\na')), [field, { ...field, id: 'other' }], true)).toEqual([''])
  })
  it('only prepares canonical commands and catches duplicate mappings', () => {
    const sheet = parseCsv(bytes('标题\n001'))
    expect(prepareImport(sheet, [field], ['text'], true)).toEqual({ commands: [{ kind: 'create', values: { text: '001' } }], diagnostics: [] })
    expect(prepareImport(sheet, [field], ['text', 'text'], true).diagnostics.some(d => d.code === 'mapping')).toBe(true)
  })
  it('delegates number validation and rejects audit/attachment input', () => {
    const sheet = parseCsv(bytes('bad'))
    for (const type of ['number', 'created_time', 'attachment'] as const) expect(prepareImport(sheet, [{ ...field, type }], ['text'], false).diagnostics.some(d => d.severity === 'error')).toBe(true)
  })
  it('reports atomic batch overflow rather than split or truncate', () => {
    const result = prepareImport(parseCsv(bytes(Array(101).fill('row').join('\n'))), [field], ['text'], false)
    expect(result.commands).toHaveLength(101)
    expect(result.diagnostics).toContainEqual({ code: 'batch_limit', severity: 'error' })
  })
})

describe('basic XLSX exchange', () => {
  it('roundtrips strings, booleans, numbers, dates, blanks and formula-looking text without executing it', () => {
    const data = [['text', 'flag', 'count', 'date'], ['=1+1', true, 1.25, '2026-09-07T12:34:56.000Z'], ['001', false, null, null]]
    expect(parseXlsx(encodeXlsx(data, new Set([3]))).sheets[0].rows).toEqual(data)
  })
  it('reads shared strings and cached formulas; diagnoses missing cache and errors', () => {
    const parsed = parseXlsx(fixture('<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1"><f>1+1</f><v>2</v></c><c r="C1"><f>3+4</f></c><c r="D1" t="e"><v>#DIV/0!</v></c></row>', {
      'xl/sharedStrings.xml': strToU8('<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>中文</t></si></sst>'),
    }))
    expect(parsed.sheets[0].rows[0]).toEqual(['中文', 2, null, null])
    expect(parsed.sheets[0].diagnostics.map(d => d.code)).toEqual(['formula_cached', 'formula_missing', 'invalid_cell'])
  })
  it('rejects hostile XML, unsafe ZIP and duplicate cell addresses', () => {
    expect(() => parseXml(bytes('<!DOCTYPE a [<!ENTITY a "x">]><a/>'))).toThrow('invalid_xml')
    expect(() => readArchive(zipSync({ '../escape': bytes('x') }))).toThrow('unsafe_archive')
    expect(() => parseXlsx(fixture('<row><c r="A1"><v>1</v></c><c r="A1"><v>2</v></c></row>'))).toThrow('invalid_cell')
  })
  it('rejects ZIP bombs before inflate and post-expansion metadata mismatch', () => {
    const archive = zipSync({ 'large.xml': bytes('a'.repeat(10000)) }), copy = archive.slice()
    const view = new DataView(copy.buffer)
    for (let i = 0; i < copy.length - 46; i++) if (view.getUint32(i, true) === 0x02014b50) { view.setUint32(i + 24, BITABLE_LIMITS.expandedBytes + 1, true); break }
    expect(() => readArchive(copy)).toThrow('limit')
    const mismatch = archive.slice(), modified = new DataView(mismatch.buffer)
    for (let i = 0; i < mismatch.length - 46; i++) if (modified.getUint32(i, true) === 0x02014b50) { modified.setUint32(i + 24, 1, true); break }
    expect(() => readArchive(mismatch)).toThrow()
  })
  it('reports unsafe numeric precision and fictitious 1900 leap day', () => {
    const parsed = parseXlsx(fixture('<row><c r="A1"><v>12345678901234567</v></c><c r="B1" s="1"><v>60</v></c></row>'))
    expect(parsed.sheets[0].diagnostics.map(d => d.code)).toEqual(['precision', 'invalid_cell'])
  })
  it('rejects macro archives and corrupt formats', () => {
    expect(() => parseXlsx(fixture('', { 'xl/vbaProject.bin': bytes('macro') }))).toThrow('unsupported_file')
    expect(() => parseXlsx(bytes('not zip'))).toThrow('invalid_file')
  })
  it('supports multiple sheets and the 1904 date system', () => {
    const files = unzipSync(encodeXlsx([['title'], ['2026-09-07T00:00:00.000Z']], new Set([0])))
    files['xl/workbook.xml'] = strToU8('<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><workbookPr date1904="1"/><sheets><sheet name="第一张" sheetId="1" r:id="rId1"/><sheet name="第二张" sheetId="2" r:id="rId2"/></sheets></workbook>')
    files['xl/_rels/workbook.xml.rels'] = strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>')
    files['xl/worksheets/sheet2.xml'] = strToU8('<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" s="1"><v>0</v></c></row><row r="2"/></sheetData><mergeCells><mergeCell ref="A1:B1"/></mergeCells></worksheet>')
    const workbook = parseXlsx(zipSync(files))
    expect(workbook.sheets.map(sheet => sheet.name)).toEqual(['第一张', '第二张'])
    expect(workbook.sheets[1].rows).toEqual([['1904-01-01T00:00:00.000Z'], [null]])
    expect(workbook.sheets[1].diagnostics[0].code).toBe('merged_cells')
  })
  it('verifies CRC and rejects corrupted stored bytes', () => {
    const archive = zipSync({ 'a.xml': bytes('<a/>') }, { level: 0 })
    archive[30 + 'a.xml'.length] ^= 1
    expect(() => readArchive(archive)).toThrow('invalid_file')
  })
  it('preserves early-1900 dates in export', () => {
    const data = [['date'], ['1900-01-01T00:00:00.000Z'], ['1900-03-01T00:00:00.000Z']]
    expect(parseXlsx(encodeXlsx(data, new Set([0]))).sheets[0].rows).toEqual(data)
  })
})

describe('authorized snapshot projection', () => {
  it('exports provided complete snapshot and not live page state', () => {
    const result = exportRows({ ...snapshot, records: Array.from({ length: 250 }, (_, n) => ({ id: String(n), revision: 1, fieldRevisions: { text: 1 }, values: { text: n } })) })
    expect(result).toHaveLength(251)
  })
  it('strips private attachment links and uses explicit PM IDs for people', () => {
    const result = exportRows({ ...snapshot, fields: [{ ...field, type: 'attachment' }, { ...field, id: 'user', type: 'user' }], records: [{ id: 'r', revision: 1, fieldRevisions: { text: 1, user: 1 }, values: {
      text: [{ id: 'a', name: 'safe.txt', size: 1, mimeType: 'text/plain', downloadPath: '/secret?token=secret' }], user: [{ id: 109, name: 'Admin' }],
    } }] })
    expect(result[1]).toEqual(['safe.txt', '109'])
    expect(JSON.stringify(result)).not.toContain('secret')
  })
  it('returns a CSV safety diagnostic', () => {
    expect(createExport({ ...snapshot, records: [{ id: 'r', revision: 1, fieldRevisions: { text: 1 }, values: { text: '=1+1' } }] }, 'csv').diagnostics[0].code).toBe('csv_escaped')
  })
})
