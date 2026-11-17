import { describe, expect, it } from 'bun:test'
import { displayCellValue, parseFieldInput } from './fields'
import type { Field } from './types'

const field = (type: Field['type']): Field => ({ id: 'f', name: 'Field', type, readOnly: false })
describe('bitable field boundary', () => {
  it('rejects unsafe numbers, calendar rollover, audit and attachment writes', () => {
    expect(parseFieldInput(field('number'), '9007199254740992').ok).toBe(false)
    expect(parseFieldInput(field('number'), 'Infinity').ok).toBe(false)
    expect(parseFieldInput(field('date'), '2026-02-30').ok).toBe(false)
    expect(parseFieldInput(field('created_by'), null).ok).toBe(false)
    expect(parseFieldInput(field('attachment'), 'https://example.com').ok).toBe(false)
  })
  it('keeps zero, false, empty and leading zeros distinct', () => {
    expect(parseFieldInput(field('number'), '0')).toEqual({ ok: true, value: 0 })
    expect(parseFieldInput(field('checkbox'), 'false')).toEqual({ ok: true, value: false })
    expect(parseFieldInput(field('text'), '001')).toEqual({ ok: true, value: '001' })
    expect(parseFieldInput(field('number'), '')).toEqual({ ok: true, value: null })
  })
  it('resolves unique choice labels, rejects ambiguous labels and duplicate values', () => {
    const f = { ...field('multiple_select'), options: { choices: [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }] } }
    expect(parseFieldInput(f, 'A,B')).toEqual({ ok: true, value: ['a', 'b'] })
    expect(parseFieldInput(f, ['a', 'a']).ok).toBe(false)
    expect(displayCellValue(f, ['a', 'b'])).toBe('A, B')
  })
  it('does not accept foreign provider user IDs', () => {
    expect(parseFieldInput(field('user'), [{ id: 'usr123', name: 'Fake' }]).ok).toBe(false)
    expect(parseFieldInput(field('user'), [{ id: 2, name: 'PM user' }]).ok).toBe(true)
  })
})
