import { useNavigate, useRouterState } from '@tanstack/react-router'
import { createContext, type ReactNode, useContext, useMemo } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { createNativeInternalNavigator } from '@/shared/navigation'
import { workItemsClient } from '../api/client'
import type { WorkItemsEnvironment } from '../model/environment'
import { useWorkItemsOperationAccess } from './useOperationAccess'

const EnvironmentContext = createContext<WorkItemsEnvironment | null>(null)

export function WorkItemsEnvironmentProvider({
  value,
  children,
}: {
  value: WorkItemsEnvironment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function usePMWorkItemsEnvironment(): WorkItemsEnvironment {
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const operationAccess = useWorkItemsOperationAccess()
  return useMemo(() => ({
    client: workItemsClient,
    workspaceId,
    userId: session.user?.id ?? 0,
    sessionRevision: session.revision,
    pathname: location.pathname,
    hash: location.hash,
    search: location.search as Readonly<Record<string, unknown>>,
    allowed: workspaceId > 0,
    createAllowed: workspaceId > 0 && operationAccess.status === 'ready' && operationAccess.createTask,
    operationAccess,
    navigate: createNativeInternalNavigator(navigate, {
      deniedMessage: 'Work-items navigation target is not allowed',
    }),
  }), [location.pathname, location.search, location.hash, navigate, operationAccess, session, workspaceId])
}

export function useInjectedWorkItemsEnvironment(): WorkItemsEnvironment | null {
  return useContext(EnvironmentContext)
}
