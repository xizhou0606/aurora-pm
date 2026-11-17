import { useNavigate, useRouterState } from '@tanstack/react-router'
import { createContext, type ReactNode, useContext, useMemo } from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { reportingClient, type ReportingClient } from '../api'

export type ReportingEnvironment = {
  client: ReportingClient
  workspaceId: number
  userId: number
  sessionRevision: number
  pathname: string
  allowed: boolean
  navigate(path: string, replace?: boolean): Promise<void>
}

const EnvironmentContext = createContext<ReportingEnvironment | null>(null)

export function ReportingEnvironmentProvider({
  value,
  children,
}: {
  value: ReportingEnvironment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function usePMReportingEnvironment(): ReportingEnvironment {
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  return useMemo(() => ({
    client: reportingClient,
    workspaceId,
    userId: session.user?.id ?? 0,
    sessionRevision: session.revision,
    pathname: location.pathname,
    allowed: workspaceId > 0,
    navigate: async (path, replace) => {
      await navigate({ to: path, replace: replace === true })
    },
  }), [location.pathname, navigate, session, workspaceId])
}

export function useInjectedReportingEnvironment(): ReportingEnvironment | null {
  return useContext(EnvironmentContext)
}
