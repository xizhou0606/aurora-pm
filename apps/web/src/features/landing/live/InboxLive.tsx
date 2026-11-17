import { useMemo, useState } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import { InboxEnvironmentProvider, InboxPage } from '@/features/inbox'
import { LandingWorkbench } from '../screens/workbench-live'
import { LANDING_WORKSPACE_ID } from './fixtures'
import { createLandingInboxClient } from './inbox-client'
import { LandingPreviewBoundary } from './PreviewBoundary'
import { createLandingQueryClient, ignoreLandingNavigate } from './query'

export function InboxLive() {
  const [queryClient] = useState(createLandingQueryClient)
  const environment = useMemo(() => ({
    allowed: true,
    canReadBugs: true,
    operationAccess: {
      status: 'ready' as const,
      retry: () => undefined,
      viewTodos: true,
    },
    client: createLandingInboxClient(),
    navigate: ignoreLandingNavigate,
    pathname: '/workspace/inbox',
    search: {},
    sessionRevision: 1,
    userId: 47,
    workspaceId: LANDING_WORKSPACE_ID,
  }), [])

  return (
    <LandingWorkbench scene="inbox">
      <LandingPreviewBoundary>
        <QueryClientProvider client={queryClient}>
          <InboxEnvironmentProvider value={environment}>
            <InboxPage />
          </InboxEnvironmentProvider>
        </QueryClientProvider>
      </LandingPreviewBoundary>
    </LandingWorkbench>
  )
}
