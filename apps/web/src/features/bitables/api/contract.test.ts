import { describe, expect, it } from 'bun:test'
import { field, id, resource, row, snapshot } from './contract'

describe('choice option contract', () => {
  it('retains option identity, colors, order and default values', () => {
    const options = { choices: [{ id: 'a', name: 'A', color: 'pink' }, { id: 'b', name: 'B', color: 'blue' }], defaultValue: ['b', 'a'] }
    expect(field({ id: 'f', name: 'F', type: 'multiple_select', readOnly: false, options }).options).toEqual(options as never)
  })
  it('rejects unsupported colors instead of rendering an arbitrary style', () => {
    expect(() => field({ id: 'f', name: 'F', type: 'single_select', readOnly: false, options: { choices: [{ id: 'a', name: 'A', color: 'url(invalid)' }] } })).toThrow()
  })
})

describe('export snapshot contract', () => {
  it('accepts an opaque table/schema/data revision snapshot identifier', () => {
    const snapshotId = '01991f18-80e0-7000-8000-000000000001:2:13'
    expect(snapshot({ snapshotId, schemaRevision: 2, fields: [], records: [] }).snapshotId).toBe(snapshotId)
    expect(() => id(snapshotId)).toThrow()
  })

  it.each(['', 'x'.repeat(201), '../table?token=secret', '<script>'])('rejects an unsafe snapshot identifier %s', (snapshotId) => {
    expect(() => snapshot({ snapshotId, schemaRevision: 2, fields: [], records: [] })).toThrow()
  })
})

describe('unified resource projection', () => {
  const base = { id: 'table-book', name: 'Table', workspaceId: 7, tables: [], allowedActions: [] }

  it('keeps resourceId absent instead of guessing from the bitable UUID', () => {
    expect(resource(base).resourceId).toBeUndefined()
    expect(resource({ ...base, resourceId: 'resource-table-1' }).resourceId).toBe('resource-table-1')
  })
})

describe('field revision projection', () => {
  it('retains authoritative per-field bases and does not substitute row revision', () => {
    expect(row({ id: 'row-1', revision: 20, fieldRevisions: { title: 7, status: 9 }, values: { title: 'A', status: 'open' } }))
      .toEqual({ id: 'row-1', revision: 20, fieldRevisions: { title: 7, status: 9 }, values: { title: 'A', status: 'open' } })
  })

  it('rejects legacy records without fieldRevisions', () => {
    expect(() => row({ id: 'row-1', revision: 20, values: { title: 'A' } })).toThrow()
  })
})
