import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import {
  QualityEnvironmentProvider,
  QualityManagementPage,
  type QualityEnvironment,
} from '@/features/quality-management'
import { LandingWorkbench } from '../screens/workbench-live'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQualityClient } from './quality-client'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'

export function QualityLive() {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo<QualityEnvironment>(() => ({
    allowed: true,
    client: createLandingQualityClient(),
    navigate: ignoreLandingNavigate,
    pathname: '/workspace/quality-management',
    search: {},
    sessionRevision: 1,
    userId: 47,
    workspaceId: LANDING_WORKSPACE_ID,
  }), [])

  return (
    <LandingWorkbench scene="quality">
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <QualityEnvironmentProvider value={environment}>
            <QualityManagementPage />
          </QualityEnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}
