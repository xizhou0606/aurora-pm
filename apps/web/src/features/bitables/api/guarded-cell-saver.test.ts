import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import type { BitableCellLeaseProof } from '../model/collaboration'
import type { BitableClient } from './client'
import { createBitableGuardedCellSaver } from './guarded-cell-saver'

const scope = { resourceId: 'native-bitable', tableId: 'table-1' }
const proof: BitableCellLeaseProof = {
  key: {
    workspaceId: 7,
    resourceId: 'unified-resource',
    tableId: 'table-1',
    recordId: 'record-1',
    fieldId: 'field-1',
  },
  leaseId: 'lease-1',
  fencingToken: 4,
  connectionId: 'a1111111-1111-4111-8111-111111111111',
  editorInstanceId: 'editor-1',
}

function setup() {
  const execute = vi.fn(async (_scope, batch) => ({
    operationId: batch.operationId,
    state: 'committed' as const,
    records: [],
    deletedRecordIds: [],
  }))
  const saver = createBitableGuardedCellSaver(
    { execute } as unknown as BitableClient,
    scope,
    () => 9,
    () => ({ workspaceId: 7, resourceId: 'unified-resource' }),
  )
  return { execute, saver }
}

describe('production bitable guarded cell saver', () => {
  it('sends the frozen field-base contract without row expectedRevision', async () => {
    const { execute, saver } = setup()
    await expect(saver({
      proof,
      baseRevision: 6,
      baseValue: 'before',
      value: 'after',
      operationId: 'operation-1',
    })).resolves.toEqual({ status: 'committed', operationId: 'operation-1' })
    expect(execute).toHaveBeenCalledWith(scope, {
      operationId: 'operation-1',
      expectedSchemaRevision: 9,
      atomic: true,
      commands: [{
        kind: 'update',
        recordId: 'record-1',
        values: { 'field-1': 'after' },
        cellContracts: [{
          recordId: 'record-1',
          fieldId: 'field-1',
          baseRevision: 6,
          leaseId: 'lease-1',
          fencingToken: 4,
          connectionId: 'a1111111-1111-4111-8111-111111111111',
          editorInstanceId: 'editor-1',
        }],
      }],
    })
    expect(execute.mock.calls[0]?.[1].commands[0]).not.toHaveProperty('expectedRevision')
  })

  it.each([
    [new PMSafeApiError('BITABLE_CELL_LEASE_LOST', 409), 'lease_lost'],
    [new PMSafeApiError('BITABLE_CONFLICT', 409), 'conflict'],
    [new PMSafeApiError('BITABLE_FORBIDDEN', 403), 'lease_lost'],
    [new PMSafeApiError('BITABLE_UNKNOWN_OUTCOME', 503), 'unknown'],
  ] as const)('maps %s to %s without retrying the mutation', async (failure, status) => {
    const { execute, saver } = setup()
    execute.mockRejectedValueOnce(failure)
    await expect(saver({ proof, baseRevision: 6, baseValue: null, value: 'after', operationId: 'operation-1' }))
      .resolves.toEqual({ status })
    expect(execute).toHaveBeenCalledTimes(1)
  })

  it('fails closed before transport when schema or table scope is stale', async () => {
    const execute = vi.fn()
    const saver = createBitableGuardedCellSaver(
      { execute } as unknown as BitableClient,
      scope,
      () => undefined,
      () => ({ workspaceId: 7, resourceId: 'unified-resource' }),
    )
    await expect(saver({ proof: { ...proof, key: { ...proof.key, tableId: 'other' } }, baseRevision: 6, baseValue: null, value: 'after', operationId: 'operation-1' }))
      .resolves.toEqual({ status: 'conflict' })
    expect(execute).not.toHaveBeenCalled()
  })

  it('accepts only the backend-projected workspace and unified resource scope', async () => {
    const execute = vi.fn()
    const saver = createBitableGuardedCellSaver(
      { execute } as unknown as BitableClient,
      scope,
      () => 9,
      () => ({ workspaceId: 7, resourceId: 'unified-resource' }),
    )
    await expect(saver({
      proof: { ...proof, key: { ...proof.key, resourceId: 'native-bitable' } },
      baseRevision: 6,
      baseValue: null,
      value: 'after',
      operationId: 'operation-1',
    })).resolves.toEqual({ status: 'conflict' })
    expect(execute).not.toHaveBeenCalled()
  })
})
