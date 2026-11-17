import { useNavigate, useRouterState } from '@tanstack/react-router'
import { createContext, type ReactNode, useContext, useMemo } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { createNativeInternalNavigator } from '@/shared/navigation'
import { qualityClient, type QualityClient } from '../api'

export type QualityEnvironment = {
  client: QualityClient
  workspaceId: number
  userId: number
  sessionRevision: number
  pathname: string
  search: Readonly<Record<string, unknown>>
  allowed: boolean
  navigate(path: string, replace?: boolean): Promise<void>
}

const EnvironmentContext = createContext<QualityEnvironment | null>(null)

export function QualityEnvironmentProvider({
  value,
  children,
}: {
  value: QualityEnvironment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function usePMQualityEnvironment(): QualityEnvironment {
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const internalNavigate = useMemo(() => createNativeInternalNavigator(navigate, {
    deniedMessage: 'Quality navigation target is not allowed',
  }), [navigate])
  return useMemo(() => ({
    client: qualityClient,
    workspaceId,
    userId: session.user?.id ?? 0,
    sessionRevision: session.revision,
    pathname: location.pathname,
    search: location.search as Readonly<Record<string, unknown>>,
    allowed: workspaceId > 0,
    navigate: internalNavigate,
  }), [internalNavigate, location.pathname, location.search, session, workspaceId])
}

export function useInjectedQualityEnvironment(): QualityEnvironment | null {
  return useContext(EnvironmentContext)
}
