import { describe, expect, it } from 'bun:test'
import { unwrapApiData, unwrapApiRows, unwrapApiTotal } from './response'

describe('PM API response resolution', () => {
  it('unwraps Axios response and API envelope boundaries', () => {
    const payload = { id: 9, name: 'PM' }

    expect(unwrapApiData<typeof payload>({ data: { data: payload } })).toEqual(payload)
    expect(unwrapApiData<typeof payload>({ data: payload })).toEqual(payload)
    expect(unwrapApiData<typeof payload>(payload)).toEqual(payload)
  })

  it('resolves direct, enveloped, and paged rows', () => {
    const rows = [{ id: 1 }, { id: 2 }]

    expect(unwrapApiRows(rows)).toEqual(rows)
    expect(unwrapApiRows({ data: rows })).toEqual(rows)
    expect(unwrapApiRows({ data: { data: { items: rows } } })).toEqual(rows)
    expect(unwrapApiRows({ data: { total: 0 } })).toEqual([])
    expect(unwrapApiRows(null)).toEqual([])
  })

  it('resolves pagination totals without coupling adapters to the envelope', () => {
    expect(unwrapApiTotal({ data: { data: { items: [{ id: 1 }], total: 23 } } })).toBe(23)
    expect(unwrapApiTotal({ data: { data: [], total: 17 } })).toBe(17)
    expect(unwrapApiTotal({ data: [], total: '8' })).toBe(8)
    expect(unwrapApiTotal({ data: [] })).toBe(0)
    expect(unwrapApiTotal({ total: 'invalid' })).toBe(0)
  })
})
