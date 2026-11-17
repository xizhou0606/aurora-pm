import { describe, expect, it } from 'bun:test'
import { bitableRecordPath } from './record-link'
describe('record share references', () => {
  it('contains only public resource coordinates', () => {
    expect(bitableRecordPath({ resourceId: 'resource', tableId: 'table' }, 'view', 'row')).toBe('/workspace/bitables/resource?table=table&view=view&record=row')
  })
  it('rejects path and query injection', () => {
    expect(() => bitableRecordPath({ resourceId: '../login', tableId: 'table' }, 'view', 'row')).toThrow()
    expect(() => bitableRecordPath({ resourceId: 'resource', tableId: 'table' }, 'view', 'row&token=secret')).toThrow()
  })
})
