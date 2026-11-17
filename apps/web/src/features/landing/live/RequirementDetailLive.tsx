import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import {
  AIPMEnvironmentProvider,
  RequirementDetailPage,
  type AIPMEnvironment,
} from '@/features/ai-pm'
import { LandingWorkbench } from '../screens/workbench-live'
import type { LandingWorkbenchId } from '../screens/workbench'
import {
  createLandingAIPMClient,
  LANDING_REQUIREMENT_ID,
} from './ai-pm-client'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'

function RequirementLive({
  scene,
  workflowNode,
}: {
  scene: Extract<LandingWorkbenchId, 'requirement-detail' | 'requirement-todo'>
  workflowNode?: string
}) {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo<AIPMEnvironment>(() => ({
    allowed: true,
    client: createLandingAIPMClient(),
    navigate: ignoreLandingNavigate,
    pathname: `/workspace/requirements/${LANDING_REQUIREMENT_ID}`,
    requirementId: LANDING_REQUIREMENT_ID,
    search: workflowNode ? { workflow_node: workflowNode } : {},
    sessionRevision: 1,
    userId: 47,
    workspaceId: LANDING_WORKSPACE_ID,
  }), [workflowNode])

  return (
    <LandingWorkbench scene={scene}>
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <AIPMEnvironmentProvider value={environment}>
            <RequirementDetailPage />
          </AIPMEnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}

export function RequirementDetailLive() {
  return <RequirementLive scene="requirement-detail" />
}

export function RequirementTodoPreview() {
  return <RequirementLive scene="requirement-todo" workflowNode="design" />
}
