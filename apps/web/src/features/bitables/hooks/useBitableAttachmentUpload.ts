import { useEffect, useRef, useState } from 'react'
import { toSafeApiError } from '@/shared/api'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import type { TableScope } from '../api/client'
import type { BitableCollaborationClientPort } from '../api/collaboration-port'
import { bitableCellKey, bitableCellLeaseProof, type BitableCellLease, type BitableCollaborationRoom } from '../model/collaboration'
import type { CellAddress } from '../model/types'
import { bitableKey, currentEnvironment, type BitableEnvironment } from './environment'

type Operation = { operationId: string; address: CellAddress; revision: number; schemaRevision: number; file: File }
type Collaboration = { client: BitableCollaborationClientPort; room: BitableCollaborationRoom; connectionId: string; editorInstanceId: string }
export function useBitableAttachmentUpload(env: BitableEnvironment, scope: TableScope, schemaRevision: number | undefined, writable: readonly string[], allowed: boolean, invalidate: () => Promise<unknown>, collaboration?: Collaboration) {
  const identity = JSON.stringify(bitableKey(env, scope))
  const writableIdentity = JSON.stringify(writable)
  const latest = useRef({ identity, allowed, writable }); latest.current = { identity, allowed, writable }
  const lock = useRef(false), operation = useRef<Operation | null>(null)
  const cancelLease = useRef<(() => void) | undefined>(undefined)
  const generation = useRef(0)
  const [phase, setPhase] = useState<'idle' | 'pending' | 'unknown'>('idle'), [error, setError] = useState('')
  const valid = () => latest.current.identity === identity && latest.current.allowed && currentEnvironment(env)
  useEffect(() => { generation.current += 1; lock.current = false; operation.current = null; setPhase('idle'); setError(''); return () => { generation.current += 1; cancelLease.current?.() } }, [identity, allowed, writableIdentity])
  const run = async (target: Operation, resolve: boolean) => {
    if (!valid() || !collaboration || lock.current || !latest.current.writable.includes(target.address.fieldId)) return false
    const ticket = generation.current
    const current = () => valid() && generation.current === ticket && latest.current.writable.includes(target.address.fieldId)
    lock.current = true; setPhase('pending'); setError('')
    let lease: BitableCellLease | undefined, timer: ReturnType<typeof setTimeout> | undefined
    let sent = false, finished = false, released = false
    const release = () => {
      finished = true; clearTimeout(timer)
      if (lease && !released) { released = true; void collaboration.client.releaseCell(bitableCellLeaseProof(lease)).catch(() => undefined) }
    }
    cancelLease.current = release
    const renew = () => { timer = setTimeout(() => {
      if (!lease || !current() || finished) return
      void collaboration.client.renewCell(bitableCellLeaseProof(lease)).then(next => { if (!finished && current()) { lease = next; renew() } }).catch(() => { /* Server fencing rejects a write whose lease expired. Resolve its receipt, never blindly retry. */ })
    }, 10_000) }
    try {
      if (!resolve) {
        const acquired = await collaboration.client.acquireCell({ key: bitableCellKey(collaboration.room, scope.tableId, target.address), connectionId: collaboration.connectionId, editorInstanceId: collaboration.editorInstanceId })
        if (acquired.status !== 'acquired') { if (current()) { setPhase('idle'); setError(BITABLE_COPY.conflict) }; return false }
        lease = acquired.lease
        if (!current()) return false
        renew()
      }
      const proof = lease && bitableCellLeaseProof(lease)
      sent = true
      const result = resolve ? await env.client.operation(scope, target.operationId) : await env.client.upload(scope, target.address, { ...target.address, baseRevision: target.revision, leaseId: proof!.leaseId, fencingToken: proof!.fencingToken, connectionId: proof!.connectionId, editorInstanceId: proof!.editorInstanceId }, target.schemaRevision, target.operationId, target.file)
      if (!current()) return false
      if (result.operationId !== target.operationId) throw new Error('BITABLE_COMMAND_RECEIPT_INVALID')
      operation.current = null; setPhase('idle'); await invalidate(); return true
    } catch (caught) {
      if (!current()) return false
      const failure = toSafeApiError(caught)
      const unknown = sent && (!failure.status || failure.status >= 500 || resolve) && failure.status !== 401 && failure.status !== 403
      if (unknown) { operation.current = target; setPhase('unknown'); setError(BITABLE_COPY.unknown) }
      else { operation.current = null; setPhase('idle'); setError(failure.status === 403 ? BITABLE_COPY.forbidden : BITABLE_COPY.saveError) }
      return false
    } finally {
      release()
      if (cancelLease.current === release) cancelLease.current = undefined
      if (current()) lock.current = false
    }
  }
  return { pending: phase !== 'idle', unknown: phase === 'unknown', error,
    upload: async (address: CellAddress, revision: number, file: File) => {
      if (!valid() || !schemaRevision || !Number.isSafeInteger(revision) || revision < 0 || lock.current) return false
      // Unknown writes are resolved by receipt, never re-uploaded with a new key.
      if (operation.current) {
        const previous = operation.current
        if (previous.file !== file || previous.address.recordId !== address.recordId || previous.address.fieldId !== address.fieldId || previous.revision !== revision || previous.schemaRevision !== schemaRevision) return false
        return run(previous, true)
      }
      return run({ operationId: crypto.randomUUID(), address, revision, schemaRevision, file }, false)
    },
    resolve: () => operation.current ? run(operation.current, true) : Promise.resolve(false) }
}
