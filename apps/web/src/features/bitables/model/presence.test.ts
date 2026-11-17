import { describe, expect, it } from 'bun:test'

import { presentBitablePresence } from './presence'

describe('presentBitablePresence', () => {
  it('counts one user once across connections and merges editing state per cell', () => {
    const member = { actorUserId: 7, actorName: 'User', state: { tableId: 'table-1', activeCell: { recordId: 'r1', fieldId: 'f1' } }, expiresAt: 'later' }
    const view = presentBitablePresence([{ ...member, connectionId: 'old' }, { ...member, connectionId: 'new' }], 'table-1')
    expect(view.people).toEqual([{ id: '7', name: 'User' }])
    expect(view.occupiedCells[0].people).toMatchObject([{ id: '7', name: 'User', editing: true }])
  })
  it('filters another table and groups server-projected occupants by active cell', () => {
    const view = presentBitablePresence([
      { connectionId: 'one', actorUserId: 1, actorName: '甲', state: { tableId: 'table-1', activeCell: { recordId: 'r1', fieldId: 'f1' } }, expiresAt: 'later' },
      { connectionId: 'two', actorUserId: 2, actorName: '乙', state: { tableId: 'table-1', selection: { anchor: { recordId: 'r1', fieldId: 'f1' }, focus: { recordId: 'r1', fieldId: 'f1' } } }, expiresAt: 'later' },
      { connectionId: 'three', actorUserId: 3, actorName: '丙', state: { tableId: 'table-2', activeCell: { recordId: 'r1', fieldId: 'f1' } }, expiresAt: 'later' },
    ], 'table-1')
    expect(view.people.map((person) => person.name)).toEqual(['甲', '乙'])
    expect(view.occupiedCells).toMatchObject([{
      address: { recordId: 'r1', fieldId: 'f1' },
      people: [{ id: '1', name: '甲', editing: true }, { id: '2', name: '乙', editing: false }],
    }])
  })
})
