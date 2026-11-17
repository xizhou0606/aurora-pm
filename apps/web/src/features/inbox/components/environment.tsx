import { useNavigate, useRouterState } from '@tanstack/react-router'
import { createContext, type ReactNode, useContext, useMemo } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { createNativeInternalNavigator } from '@/shared/navigation'
import {
  inboxClient,
  withInboxRuntimeFailure,
  type InboxClient,
} from '../api'
import { useInboxOperationAccess, type InboxOperationAccess } from '../hooks/useOperationAccess'

export type InboxEnvironment = {
  client: InboxClient
  workspaceId: number
  sessionRevision: number
  userId: number
  pathname: string
  search: Readonly<Record<string, unknown>>
  allowed: boolean
  canReadBugs: boolean
  operationAccess: InboxOperationAccess
  navigate(path: string, replace?: boolean): Promise<void>
}

const EnvironmentContext = createContext<InboxEnvironment | null>(null)

export function InboxEnvironmentProvider({
  value,
  children,
}: {
  value: InboxEnvironment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function usePMInboxEnvironment(): InboxEnvironment {
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const operationAccess = useInboxOperationAccess()
  return useMemo(() => {
    const allowed = workspaceId > 0
    return {
      client: inboxClient,
      workspaceId,
      sessionRevision: session.revision,
      userId: session.user?.id ?? 0,
      pathname: location.pathname,
      search: location.search as Readonly<Record<string, unknown>>,
      allowed,
      canReadBugs: allowed,
      operationAccess,
      navigate: createNativeInternalNavigator(navigate, {
        deniedMessage: 'Inbox navigation target is not allowed',
      }),
    }
  }, [location.pathname, location.search, navigate, operationAccess, session, workspaceId])
}

export function useInjectedInboxEnvironment(): InboxEnvironment | null {
  return useContext(EnvironmentContext)
}

export function useInboxRuntimeEnvironment(
  baseEnvironment: InboxEnvironment,
  onRuntimeFailure?: (error: unknown) => void,
): InboxEnvironment {
  const client = useMemo(
    () => withInboxRuntimeFailure(baseEnvironment.client, onRuntimeFailure),
    [baseEnvironment.client, onRuntimeFailure],
  )
  return useMemo(
    () => ({ ...baseEnvironment, client }),
    [baseEnvironment, client],
  )
}
