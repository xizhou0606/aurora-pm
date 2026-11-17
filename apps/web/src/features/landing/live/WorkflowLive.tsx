import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import {
  AIPMEnvironmentProvider,
  WorkflowEditorPage,
  type AIPMEnvironment,
} from '@/features/ai-pm'
import { LandingWorkbench } from '../screens/workbench-live'
import {
  createLandingAIPMClient,
  LANDING_TEMPLATE_ID,
} from './ai-pm-client'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'

export function WorkflowLive() {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo<AIPMEnvironment>(() => ({
    allowed: true,
    client: createLandingAIPMClient(),
    navigate: ignoreLandingNavigate,
    pathname: `/workspace/pm/workflows/${LANDING_TEMPLATE_ID}`,
    search: {},
    sessionRevision: 1,
    templateId: LANDING_TEMPLATE_ID,
    userId: 47,
    workspaceId: LANDING_WORKSPACE_ID,
  }), [])

  return (
    <LandingWorkbench scene="workflow">
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <AIPMEnvironmentProvider value={environment}>
            <WorkflowEditorPage />
          </AIPMEnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}
