import type { CommandResult, RecordCommand, Row } from './types'

// The provider returns one versioned row per command, including delete tombstones.
// Inverse commands carry those exact versions: history never overwrites another writer.
export function inverseCommands(commands: readonly RecordCommand[], _before: readonly Row[], result: CommandResult): readonly RecordCommand[] {
  if (commands.length !== result.records.length) throw new Error('BITABLE_COMMAND_RECEIPT_INVALID')
  return commands.map((command, index): RecordCommand => {
    const row = result.records[index]
    if (command.kind !== 'create' && row.id !== command.recordId) throw new Error('BITABLE_COMMAND_RECEIPT_INVALID')
    if (command.kind === 'create' || command.kind === 'restore') return { kind: 'delete', recordId: row.id, expectedRevision: row.revision }
    if (command.kind === 'delete') return { kind: 'restore', recordId: row.id, expectedRevision: row.revision }
    if (command.kind === 'move') {
      const positions = result.moves?.filter(move => move.recordId === row.id)
      if (positions?.length !== 1 || positions[0].before.recordId === row.id) throw new Error('BITABLE_COMMAND_RECEIPT_INVALID')
      return { kind: 'move', recordId: row.id, expectedRevision: row.revision, position: positions[0].before }
    }
    // Cell undo must acquire fresh leases and field bases; a committed lease proof
    // must never be replayed as history through the structural command owner.
    throw new Error('BITABLE_HISTORY_CELL_LEASE_REQUIRED')
  })
}

export type HistoryEntry = { commands: readonly RecordCommand[]; before: readonly Row[] }
export type HistoryState = { undo: readonly HistoryEntry[]; redo: readonly HistoryEntry[] }
export const emptyHistory: HistoryState = { undo: [], redo: [] }
export function advanceHistory(state: HistoryState, direction: 'execute' | 'undo' | 'redo', entry: HistoryEntry): HistoryState {
  if (direction === 'execute') return { undo: [...state.undo.slice(-49), entry], redo: [] }
  if (direction === 'undo') return { undo: state.undo.slice(0, -1), redo: [...state.redo, entry] }
  return { undo: [...state.undo, entry], redo: state.redo.slice(0, -1) }
}
