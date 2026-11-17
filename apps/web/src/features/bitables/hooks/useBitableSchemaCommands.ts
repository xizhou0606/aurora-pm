import { useEffect, useRef, useState } from 'react'
import { toSafeApiError } from '@/shared/api'
import { BITABLE_COPY } from '@/shared/copy/bitables'

type Kind = 'field' | 'view'
type Attempt = { kind: Kind; run: () => Promise<unknown>; complete?: (value: unknown) => void }
export function useBitableSchemaCommands(identity: string, permissions: { field: boolean; view: boolean }, validEnvironment: () => boolean, invalidate: () => Promise<unknown>) {
  const latest = useRef({ identity, permissions }); latest.current = { identity, permissions }
  const generation = useRef(0), lock = useRef(false), saved = useRef<Attempt | null>(null)
  const [phase, setPhase] = useState<'idle' | 'pending' | 'unknown'>('idle')
  const [error, setError] = useState('')
  useEffect(() => {
    generation.current++; lock.current = false; saved.current = null; setPhase('idle'); setError('')
    return () => { generation.current++ }
  }, [identity, permissions.field, permissions.view])
  const run = async (attempt: Attempt) => {
    const ticket = generation.current
    const current = () => generation.current === ticket && latest.current.identity === identity && latest.current.permissions[attempt.kind] && validEnvironment()
    if (lock.current || !current()) return false
    lock.current = true; setPhase('pending'); setError('')
    try {
      const value = await attempt.run()
      if (!current()) return false
      saved.current = null; setPhase('idle'); attempt.complete?.(value)
      // A failed refresh does not turn a confirmed successful mutation into an unknown write.
      await invalidate().catch(() => { if (current()) setError(BITABLE_COPY.saveError) })
      return true
    } catch (caught) {
      if (!current()) return false
      const failure = toSafeApiError(caught)
      const unknown = !failure.status || failure.status === 408 || failure.status >= 500 || failure.code === 'BITABLE_OPERATION_UNKNOWN'
      saved.current = unknown ? attempt : null; setPhase(unknown ? 'unknown' : 'idle')
      setError(unknown ? BITABLE_COPY.unknown : failure.status === 403 ? BITABLE_COPY.forbidden : BITABLE_COPY.saveError)
      return false
    } finally { if (current()) lock.current = false }
  }
  return { pending: phase !== 'idle', unknown: phase === 'unknown', error,
    execute: <P, R>(kind: Kind, payload: P, action: (payload: P, operationId: string) => Promise<R>, complete?: (value: R) => void) => {
      if (saved.current || lock.current || !latest.current.permissions[kind] || latest.current.identity !== identity || !validEnvironment()) return Promise.resolve(false)
      // Copy every input once: generated choice IDs, schema/view revision and full config survive retries.
      const snapshot = structuredClone(payload), operationId = crypto.randomUUID()
      const attempt: Attempt = { kind, run: () => action(snapshot, operationId), complete: complete ? value => complete(value as R) : undefined }
      saved.current = attempt
      return run(attempt)
    },
    resolve: () => saved.current ? run(saved.current) : Promise.resolve(false) }
}
