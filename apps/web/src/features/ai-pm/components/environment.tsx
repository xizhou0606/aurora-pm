import { createContext, type ReactNode, useContext } from 'react'

import { aipmClient, type AIPMClient } from '../api/client'

export type AIPMEnvironment = {
  client: AIPMClient
  workspaceId: number
  sessionRevision: number
  pathname: string
  search: Readonly<Record<string, unknown>>
  allowed: boolean
  navigate(path: string, replace?: boolean): Promise<void>
  templateId?: number
  requirementId?: number
  userId?: number
}

const EnvironmentContext = createContext<AIPMEnvironment | null>(null)

export function AIPMEnvironmentProvider({
  value,
  children,
}: {
  value: AIPMEnvironment
  children: ReactNode
}) {
  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>
}

export function useInjectedAIPMEnvironment(): AIPMEnvironment | null {
  return useContext(EnvironmentContext)
}

export function useAIPMClient(): AIPMClient {
  return useInjectedAIPMEnvironment()?.client ?? aipmClient
}
