import { describe, expect, it } from 'bun:test'
import { copySelection, pasteEdits } from './clipboard'
import type { Field, Row } from './types'

const fields: Field[] = [{ id: 'title', name: 'Title', type: 'long_text', readOnly: false }, { id: 'count', name: 'Count', type: 'number', readOnly: false }]
const rows: Row[] = [
  { id: 'second', revision: 8, fieldRevisions: { title: 10, count: 11 }, values: { title: 'line1\nline2\ttab', count: 2 } },
  { id: 'first', revision: 3, fieldRevisions: { title: 4, count: 5 }, values: { title: '"quoted"', count: 1 } },
]
describe('bitable clipboard shares the exchange codec', () => {
  it('copies reverse rectangular selection in current visual order and roundtrips quoted multiline text', () => {
    const tsv = copySelection({ anchor: { recordId: 'first', fieldId: 'count' }, focus: { recordId: 'second', fieldId: 'title' } }, fields, rows)
    expect(pasteEdits(tsv, { recordId: 'second', fieldId: 'title' }, fields, rows, ['title', 'count'])).toEqual([
      { address: { recordId: 'second', fieldId: 'title' }, baseRevision: 10, value: rows[0].values.title },
      { address: { recordId: 'second', fieldId: 'count' }, baseRevision: 11, value: rows[0].values.count },
      { address: { recordId: 'first', fieldId: 'title' }, baseRevision: 4, value: rows[1].values.title },
      { address: { recordId: 'first', fieldId: 'count' }, baseRevision: 5, value: rows[1].values.count },
    ])
  })
  it('rejects the whole batch if any destination is readonly or any typed value is invalid', () => {
    expect(() => pasteEdits('good\t2\nbad\tnot-number', { recordId: 'second', fieldId: 'title' }, fields, rows, ['title', 'count'])).toThrow('BITABLE_VALUE_INVALID')
    expect(() => pasteEdits('good\t2', { recordId: 'second', fieldId: 'title' }, fields, rows, ['title'])).toThrow('BITABLE_PASTE_RANGE_INVALID')
  })
  it('does not silently truncate excess rows or columns or manufacture records', () => {
    expect(() => pasteEdits('a\nb\nc', { recordId: 'second', fieldId: 'title' }, fields, rows, ['title', 'count'])).toThrow('BITABLE_PASTE_RANGE_INVALID')
    expect(() => pasteEdits('a\t1\textra', { recordId: 'second', fieldId: 'title' }, fields, rows, ['title', 'count'])).toThrow('BITABLE_PASTE_RANGE_INVALID')
  })
  it('does not copy unrelated data after the selected identity disappears', () => {
    expect(copySelection({ anchor: { recordId: 'missing', fieldId: 'title' }, focus: { recordId: 'first', fieldId: 'count' } }, fields, rows)).toBe('')
  })
})
