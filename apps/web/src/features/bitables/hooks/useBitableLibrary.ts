import { useEffect, useMemo, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import type { Resource } from '../model/types'
import { bitableKey, createBitableEnvironment, currentEnvironment } from './environment'
import { useResourceList, visibleQueryData } from './useBitableQueries'
import { useBitableCapabilities } from './useBitableCapabilities'

export function useBitableLibrary() {
  const workspaceId = useWorkspaceStore(state => state.currentWorkspaceId), sessionRevision = useSessionStore(state => state.revision)
  const env = useMemo(() => createBitableEnvironment(workspaceId, sessionRevision), [workspaceId, sessionRevision])
  const access = useBitableCapabilities(undefined, false), resources = useResourceList(env, access.canList)
  const [pending, setPending] = useState(false), [error, setError] = useState('')
  const operation = useRef<{ id: string; name: string } | undefined>(undefined), busy = useRef(false), queryClient = useQueryClient()
  const owner = useRef(env); owner.current = env
  useEffect(() => { operation.current = undefined; busy.current = false; setPending(false); setError('') }, [env])
  return { resources: visibleQueryData(resources.data, resources.error) ?? [], loading: resources.isPending && access.canList,
    canRead: access.canList, canCreate: access.canCreate, pending, error: error || (resources.error ? BITABLE_COPY.error : ''),
    retry: () => void resources.refetch(),
    create: async (name: string): Promise<Resource | null> => {
      if (!access.canCreate || busy.current || !name.trim() || !currentEnvironment(env)) return null
      if (operation.current && operation.current.name !== name.trim()) { setError(BITABLE_COPY.unknown); return null }
      operation.current ??= { id: crypto.randomUUID(), name: name.trim() }
      busy.current = true; setPending(true); setError('')
      try {
        const resource = await env.client.create(operation.current.name, operation.current.id)
        if (owner.current !== env || !currentEnvironment(env)) return null
        operation.current = undefined
        await queryClient.invalidateQueries({ queryKey: bitableKey(env, undefined, 'list') })
        return resource
      } catch { if (owner.current === env) setError(BITABLE_COPY.saveError); return null }
      finally { if (owner.current === env) { busy.current = false; setPending(false) } }
    },
  }
}
