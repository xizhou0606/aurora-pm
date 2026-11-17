import { useCallback, useEffect, useRef, useState } from 'react'
import { toSafeApiError } from '@/shared/api'
import { BITABLE_LIBRARY_COPY as copy } from '@/shared/copy/bitables-library'
import type { ResourceMutationResult } from '../model/types'

type Attempt = { action: string; execute: () => Promise<ResourceMutationResult>; complete?: (value: ResourceMutationResult) => void }
export function useBitableResourceCommands(identity: string, allowed: (action: string) => boolean, current: () => boolean, refresh: () => Promise<unknown>) {
  const latest = useRef({ identity, allowed, current, refresh }); latest.current = { identity, allowed, current, refresh }
  const generation = useRef(0), lock = useRef(false), attempt = useRef<Attempt | null>(null)
  const [phase, setPhase] = useState<'idle' | 'pending' | 'unknown'>('idle'), [error, setError] = useState('')
  useEffect(() => { generation.current++; lock.current = false; attempt.current = null; setPhase('idle'); setError(''); return () => { generation.current++ } }, [identity])
  useEffect(() => {
    if (attempt.current && (!allowed(attempt.current.action) || !current())) {
      generation.current++; lock.current = false; attempt.current = null; setPhase('idle'); setError('')
    }
  }, [allowed, current])
  const run = useCallback(async (value: Attempt) => {
    const ticket = generation.current, scope = latest.current.identity
    const valid = () => ticket === generation.current && scope === latest.current.identity && latest.current.allowed(value.action) && latest.current.current()
    if (lock.current || !valid()) return false
    lock.current = true; setPhase('pending'); setError('')
    try {
      const result = await value.execute()
      if (!valid()) return false
      attempt.current = null; value.complete?.(result)
      await latest.current.refresh().catch(() => { if (valid()) setError(copy.error) })
      if (valid()) setPhase('idle')
      return true
    } catch (caught) {
      if (!valid()) return false
      const failure = toSafeApiError(caught), uncertain = !failure.status || failure.status === 408 || failure.status >= 500
      attempt.current = uncertain ? value : null; setPhase(uncertain ? 'unknown' : 'idle'); setError(uncertain ? copy.unknown : copy.error)
      return false
    } finally {
      if (ticket === generation.current && scope === latest.current.identity) {
        lock.current = false
        if (!valid()) { attempt.current = null; setPhase('idle'); setError('') }
      }
    }
  }, [])
  const execute = useCallback(<P,>(action: string, payload: P, mutation: (payload: P, operationId: string) => Promise<ResourceMutationResult>, complete?: (value: ResourceMutationResult) => void) => {
    if (attempt.current || lock.current || !latest.current.allowed(action) || !latest.current.current()) return Promise.resolve(false)
    const snapshot = structuredClone(payload), operationId = crypto.randomUUID()
    const value = { action, execute: () => mutation(snapshot, operationId), complete }
    attempt.current = value
    return run(value)
  }, [run])
  const resolve = useCallback(() => attempt.current ? run(attempt.current) : Promise.resolve(false), [run])
  return { execute, resolve, pending: phase !== 'idle', unknown: phase === 'unknown', error }
}
