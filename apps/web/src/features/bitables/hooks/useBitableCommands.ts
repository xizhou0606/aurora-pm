import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { PMSafeApiError, toSafeApiError } from '@/shared/api'
import type { TableScope } from '../api/client'
import { advanceHistory, emptyHistory, inverseCommands, type HistoryState } from '../model/history'
import type { Bootstrap, CommandBatch, CommandResult, RecordCommand, Row } from '../model/types'
import { bitableKey, currentEnvironment, type BitableEnvironment } from './environment'

type PendingCommand = { batch: CommandBatch; before: readonly Row[]; direction: 'execute' | 'undo' | 'redo'; importing: boolean }
export function useBitableCommands(env: BitableEnvironment, scope: TableScope, bootstrap: Bootstrap | undefined) {
  const queryClient = useQueryClient()
  const identity = JSON.stringify(bitableKey(env, scope))
  const active = useRef(identity); active.current = identity
  const latestBootstrap = useRef(bootstrap); latestBootstrap.current = bootstrap
  const lock = useRef(false)
  const denied = useRef(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<PMSafeApiError>()
  const [unknown, setUnknown] = useState<PendingCommand>()
  const [history, setHistory] = useState<HistoryState>(emptyHistory)
  useEffect(() => { setHistory(emptyHistory); setUnknown(undefined); setError(undefined); setPending(false); lock.current = false; denied.current = false }, [identity])
  const valid = () => active.current === identity && currentEnvironment(env) && latestBootstrap.current?.capabilities.canRead === true
  const invalidate = async () => {
    if (!valid()) return
    await queryClient.invalidateQueries({ queryKey: bitableKey(env, scope) })
  }
  const complete = async (operation: PendingCommand, result: CommandResult) => {
    if (!valid()) return null
    if (result.operationId !== operation.batch.operationId) throw new PMSafeApiError('BITABLE_COMMAND_RECEIPT_INVALID')
    const commands = inverseCommands(operation.batch.commands, operation.before, result)
    setHistory(previous => advanceHistory(previous, operation.direction, { commands, before: result.records }))
    setUnknown(undefined); setError(undefined)
    await invalidate()
    return result
  }
  const run = async (operation: PendingCommand, resolve = false): Promise<CommandResult | null> => {
    if (lock.current || denied.current || !valid()) return null
    lock.current = true; setPending(true); setError(undefined)
    try {
      const result = resolve ? await env.client.operation(scope, operation.batch.operationId)
        : await env.client.execute(scope, operation.batch, operation.importing)
      return await complete(operation, result)
    } catch (caught) {
      if (!valid()) return null
      const failure = toSafeApiError(caught, 'BITABLE_REQUEST_FAILED'); setError(failure)
      if (failure.status === 401 || failure.status === 403) {
        denied.current = true; setHistory(emptyHistory); setUnknown(undefined)
        await queryClient.cancelQueries({ queryKey: bitableKey(env, scope) })
        queryClient.removeQueries({ queryKey: bitableKey(env, scope) })
        return null
      }
      // No response is not a rejection. Keep this exact operation until its receipt is known.
      if (resolve || !failure.status || failure.status >= 500 || failure.code === 'BITABLE_OPERATION_UNKNOWN') setUnknown(operation)
      await invalidate()
      return null
    } finally { if (valid()) { lock.current = false; setPending(false) } }
  }
  const execute = async (commands: readonly RecordCommand[], before: readonly Row[], importing = false, supplied?: CommandBatch) => {
    if (!bootstrap || denied.current || unknown || !commands.length || !valid()) return null
    // Existing-cell updates are exclusively owned by the collaboration saver,
    // which supplies a live lease and field base. Structural history must not
    // become a second update transport.
    if (commands.some(command => command.kind === 'update')) return null
    const capability = latestBootstrap.current?.capabilities
    if (!capability || importing && !capability.canImport || commands.some(command => command.kind === 'create' || command.kind === 'restore' ? !capability.canCreateRecords : command.kind === 'delete' ? !capability.canDeleteRecords : !capability.canUpdateRecords)) return null
    const batch = supplied ?? { operationId: crypto.randomUUID(), expectedSchemaRevision: bootstrap.schemaRevision, commands, atomic: true }
    return run({ batch, before, direction: 'execute', importing })
  }
  const replay = async (direction: 'undo' | 'redo') => {
    const entry = history[direction].at(-1)
    if (!entry || !bootstrap || unknown) return false
    return Boolean(await run({ batch: { operationId: crypto.randomUUID(), expectedSchemaRevision: bootstrap.schemaRevision, commands: entry.commands, atomic: true }, before: entry.before, direction, importing: false }))
  }
  return { pending, error, unknown: Boolean(unknown), execute, invalidate,
    clearError: () => setError(undefined), canUndo: history.undo.length > 0 && !pending && !unknown,
    canRedo: history.redo.length > 0 && !pending && !unknown,
    undo: () => replay('undo'), redo: () => replay('redo'),
    resolve: () => unknown ? run(unknown, true) : Promise.resolve(null) }
}
