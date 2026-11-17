import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import {
  BiScreenPage,
  EnvironmentProvider,
  type Environment,
} from '@/features/bi-screen'
import { LandingWorkbench } from '../screens/workbench-live'
import { createLandingCommandClient } from './command-client'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'

export function CommandLive() {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo<Environment>(() => ({
    allowed: true,
    client: createLandingCommandClient(),
    navigate: ignoreLandingNavigate,
    pathname: '/workspace/bi-screen',
    search: {},
    sessionRevision: 1,
    workspaceId: LANDING_WORKSPACE_ID,
    operationAccess: {
      status: 'ready',
      retry: () => undefined,
      viewOverview: true,
      viewTeamDetails: true,
      updateScoring: true,
      viewMetrics: true,
      updateMetrics: true,
    },
  }), [])

  return (
    <LandingWorkbench scene="command">
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <EnvironmentProvider value={environment}>
            <BiScreenPage />
          </EnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}
