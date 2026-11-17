import { describe, expect, it } from 'bun:test'
import { advanceHistory, emptyHistory, inverseCommands } from './history'
import type { CommandResult, Row } from './types'
const row: Row = { id: 'r1', revision: 2, fieldRevisions: { a: 2, b: 2 }, values: { a: 'before', b: 1 } }
const receipt = (rows: readonly Row[]): CommandResult => ({ operationId: 'op', state: 'committed', records: rows, deletedRecordIds: [] })
describe('versioned table history', () => {
  it('does not replay a committed cell lease as undo', () => {
    const contract = { recordId: 'r1', fieldId: 'a', baseRevision: 2, leaseId: 'lease', fencingToken: 1, connectionId: 'connection', editorInstanceId: 'editor' }
    expect(() => inverseCommands([{ kind: 'update', recordId: 'r1', values: { a: 'after' }, cellContracts: [contract] }], [row], receipt([{ ...row, revision: 3, fieldRevisions: { a: 3, b: 2 } }]))).toThrow('BITABLE_HISTORY_CELL_LEASE_REQUIRED')
  })
  it('delete undo restores the same identity with tombstone CAS', () => {
    expect(inverseCommands([{ kind: 'delete', recordId: 'r1', expectedRevision: 2 }], [row], receipt([{ ...row, revision: 3 }]))).toEqual([{ kind: 'restore', recordId: 'r1', expectedRevision: 3 }])
  })
  it('rejects mismatched or incomplete receipts', () => {
    expect(() => inverseCommands([{ kind: 'delete', recordId: 'r1', expectedRevision: 2 }], [row], receipt([]))).toThrow()
  })
  it('move undo uses the server original anchor even outside the loaded page', () => {
    const position = { recordId: 'off-page-anchor', direction: 'before' as const, expectedRevision: 8 }
    const commands = [{ kind: 'move' as const, recordId: row.id, expectedRevision: row.revision, position }]
    const result = { ...receipt([{ ...row, revision: 3 }]), moves: [{ recordId: row.id, before: position }] }
    expect(inverseCommands(commands, [], result)).toEqual([{ kind: 'move', recordId: row.id, expectedRevision: 3, position }])
    expect(() => inverseCommands(commands, [], receipt(result.records))).toThrow('BITABLE_COMMAND_RECEIPT_INVALID')
  })
  it('fresh commands clear redo and history is bounded', () => {
    const e = { commands: [], before: [] }
    let state = emptyHistory
    for (let i = 0; i < 80; i++) state = advanceHistory(state, 'execute', e)
    expect(state.undo).toHaveLength(50)
    state = advanceHistory(state, 'undo', e)
    expect(state.redo).toHaveLength(1)
    expect(advanceHistory(state, 'execute', e).redo).toHaveLength(0)
  })
})
