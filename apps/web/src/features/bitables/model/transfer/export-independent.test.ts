import { spawnSync } from 'node:child_process'
import { describe, expect, it } from 'bun:test'
import { encodeXlsx } from './export'
import { encodeCsv } from './csv'

// Optional external-reader acceptance gate. CI can provide its own openpyxl Python.
// The fixture is emitted by the production encoder, never by the independent reader.
const python = process.env.PM_XLSX_READER_PYTHON
describe.skipIf(!python)('XLSX independent reader acceptance', () => {
  it('Python CSV reader preserves BOM, quoting and safe formula text', () => {
    const { bytes } = encodeCsv([['标题', '备注', '编号', '公式文本', '数字'], ['中文', '逗号,引号"与\n换行', '001', '=1+1', -12.5]])
    const result = spawnSync(python!, ['-c', 'import sys, io, csv, json; print(json.dumps(list(csv.reader(io.StringIO(sys.stdin.buffer.read().decode("utf-8-sig"), newline="")))))'], { input: Buffer.from(bytes), encoding: 'utf8', timeout: 20000 })
    expect(result.error).toBeUndefined()
    expect(result.status, result.stderr).toBe(0)
    expect(JSON.parse(result.stdout)).toEqual([['标题', '备注', '编号', '公式文本', '数字'], ['中文', '逗号,引号"与\n换行', '001', "'=1+1", '-12.5']])
  })
  it('openpyxl preserves cell values and types without our XML parser', () => {
    const bytes = encodeXlsx([
      ['标题', '编号', '数字', '勾选', '日期', '空值', '公式文本', '多行'],
      ['中文 & <测试>', '001', -123.25, true, '2026-09-07T12:34:56.000Z', null, '=SUM(1,2)', '第一行\n第二行'],
      ['emoji 🧪', '000', 0, false, '1900-02-28T00:00:00.000Z', null, '@name', 'a,"b"'],
    ], new Set([4]))
    const result = spawnSync(python!, ['-c', [
      'import sys, io, json, datetime, warnings',
      'from openpyxl import load_workbook',
      'with warnings.catch_warnings(record=True) as notices:',
      '    book = load_workbook(io.BytesIO(sys.stdin.buffer.read()), data_only=False)',
      'sheet = book["Data"]',
      'def value(cell):',
      '    v = cell.value',
      '    return v.isoformat() if isinstance(v, (datetime.date, datetime.datetime)) else v',
      'print(json.dumps({"sheets": book.sheetnames, "rows": [[value(c) for c in row] for row in sheet], "types": [[c.data_type for c in row] for row in sheet], "warnings": [str(n.message) for n in notices]}))',
    ].join('\n')], { input: Buffer.from(bytes), encoding: 'utf8', timeout: 20000 })
    expect(result.error).toBeUndefined()
    expect(result.status, result.stderr).toBe(0)
    const read = JSON.parse(result.stdout)
    expect(read.sheets).toEqual(['Data'])
    expect(read.rows).toEqual([
      ['标题', '编号', '数字', '勾选', '日期', '空值', '公式文本', '多行'],
      ['中文 & <测试>', '001', -123.25, true, '2026-09-07T12:34:56', null, '=SUM(1,2)', '第一行\n第二行'],
      ['emoji 🧪', '000', 0, false, '1900-02-28T00:00:00', null, '@name', 'a,"b"'],
    ])
    expect(read.types[1]).toEqual(['s', 's', 'n', 'b', 'd', 'n', 's', 's'])
    expect(read.warnings).toEqual([])
  })
})
