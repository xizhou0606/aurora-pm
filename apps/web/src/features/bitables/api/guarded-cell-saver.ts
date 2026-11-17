import { toSafeApiError } from '@/shared/api'

import type { BitableCollaborationRoom } from '../model/collaboration'
import type { CellContract, CommandBatch } from '../model/types'
import type { BitableClient, TableScope } from './client'
import type { BitableGuardedCellSaver } from './collaboration-port'

export function createBitableGuardedCellSaver(
  client: BitableClient,
  scope: TableScope,
  schemaRevision: () => number | undefined,
  room: () => BitableCollaborationRoom | undefined,
): BitableGuardedCellSaver {
  return async (input) => {
    const revision = schemaRevision()
    const expectedRoom = room()
    const { key, ...lease } = input.proof
    if (!expectedRoom
      || key.workspaceId !== expectedRoom.workspaceId
      || key.resourceId !== expectedRoom.resourceId
      || key.tableId !== scope.tableId
      || !Number.isSafeInteger(input.baseRevision)
      || input.baseRevision < 0
      || !Number.isSafeInteger(revision)
      || !revision
      || input.operationId.length === 0) {
      return { status: 'conflict' }
    }
    const cellContract: CellContract = {
      recordId: key.recordId,
      fieldId: key.fieldId,
      baseRevision: input.baseRevision,
      ...lease,
    }
    const batch: CommandBatch = {
      operationId: input.operationId,
      expectedSchemaRevision: revision,
      atomic: true,
      commands: [{
        kind: 'update',
        recordId: key.recordId,
        values: { [key.fieldId]: input.value },
        cellContracts: [cellContract],
      }],
    }
    try {
      const result = await client.execute(scope, batch)
      return result.operationId === input.operationId
        ? { status: 'committed', operationId: result.operationId }
        : { status: 'unknown' }
    } catch (caught) {
      const failure = toSafeApiError(caught, 'BITABLE_REQUEST_FAILED')
      if (failure.code === 'BITABLE_CELL_LEASE_LOST' || failure.status === 401 || failure.status === 403) {
        return { status: 'lease_lost' }
      }
      if (failure.status === 409) return { status: 'conflict' }
      return { status: 'unknown' }
    }
  }
}
