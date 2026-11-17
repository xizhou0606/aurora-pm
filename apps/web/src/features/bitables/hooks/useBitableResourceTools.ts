import { useEffect, useMemo, useState } from 'react'
import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { createResourceAccessEnvironment, useResourceAccessWorkflow } from '@/features/resource-access'

export function useBitableResourceTools(resourceId: string, bitableId: string) {
  const workspaceId = useWorkspaceStore(state => state.currentWorkspaceId)
  const sessionRevision = useSessionStore(state => state.revision)
  const [shareOpen, setShareOpen] = useState(false)
  const environment = useMemo(() => createResourceAccessEnvironment({ workspaceId, sessionRevision, resourceId }),
    [workspaceId, sessionRevision, resourceId])
  useEffect(() => setShareOpen(false), [environment, bitableId])
  const sharing = useResourceAccessWorkflow({ environment, enabled: shareOpen,
    sharePath: `/workspace/bitables/${encodeURIComponent(bitableId)}` })
  return { shareOpen, setShareOpen, sharing }
}
