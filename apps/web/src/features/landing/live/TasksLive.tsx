import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import { WorkItemsEnvironmentProvider, WorkItemsPage } from '@/features/work-items'
import { LandingWorkbench } from '../screens/workbench-live'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'
import { createLandingTasksClient } from './tasks-client'

export function TasksLive() {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo(() => ({
    allowed: true,
    operationAccess: {
      status: 'ready' as const,
      retry: () => undefined,
      viewReferenceData: true,
      viewTaskTable: true,
      viewTaskDetail: true,
      createTask: true,
      updateTask: true,
      transitionTask: true,
      deleteTask: true,
      viewAttachments: true,
      uploadAttachment: true,
      downloadAttachment: true,
      deleteAttachment: true,
      viewComments: true,
      createComment: true,
      deleteComment: true,
      updateWatching: true,
    },
    client: createLandingTasksClient(),
    createAllowed: true,
    navigate: ignoreLandingNavigate,
    pathname: '/workspace/work-items',
    search: { view: 'list' },
    sessionRevision: 1,
    userId: 47,
    workspaceId: LANDING_WORKSPACE_ID,
  }), [])

  return (
    <LandingWorkbench scene="tasks">
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <WorkItemsEnvironmentProvider value={environment}>
            <WorkItemsPage />
          </WorkItemsEnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}
