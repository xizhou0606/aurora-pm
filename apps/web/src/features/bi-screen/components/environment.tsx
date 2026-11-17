import { useNavigate, useRouterState } from '@tanstack/react-router'
import { createContext, type ReactNode, useContext, useMemo } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { createNativeInternalNavigator } from '@/shared/navigation'

import { client, type Client } from '../api/client'
import {
  useBiScreenOperationAccess,
  type BiScreenOperationAccess,
} from '../hooks/useOperationAccess'

export type Environment = {
  client: Client
  workspaceId: number
  sessionRevision: number
  pathname: string
  search: Readonly<Record<string, unknown>>
  allowed: boolean
  operationAccess: BiScreenOperationAccess
  navigate(path: string, replace?: boolean): Promise<void>
}

const EnvironmentContext = createContext<Environment | null>(null)

export function EnvironmentProvider({
  value,
  children,
}: {
  value: Environment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function usePMEnvironment(): Environment {
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const operationAccess = useBiScreenOperationAccess()
  return useMemo(() => ({
    client,
    workspaceId,
    sessionRevision: session.revision,
    pathname: location.pathname,
    search: location.search as Readonly<Record<string, unknown>>,
    allowed: workspaceId > 0,
    operationAccess,
    navigate: createNativeInternalNavigator(navigate, {
      deniedMessage: 'BI screen navigation target is not allowed',
    }),
  }), [location.pathname, location.search, navigate, operationAccess, session, workspaceId])
}

export function useInjectedEnvironment(): Environment | null {
  return useContext(EnvironmentContext)
}
