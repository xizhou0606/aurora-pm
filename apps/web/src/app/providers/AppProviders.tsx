import { useEffect, type PropsWithChildren } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'

import { queryClient } from '@/app/query'
import { bindAppearanceMedia, hydrateAppearance } from '@/shared/ui/theme/appearance'
import { OperationFeedbackProvider } from '@/shared/ui'

function AppearanceLifecycle() {
  useEffect(() => {
    hydrateAppearance()
    return bindAppearanceMedia()
  }, [])
  return null
}

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <AppearanceLifecycle />
      <OperationFeedbackProvider>{children}</OperationFeedbackProvider>
    </QueryClientProvider>
  )
}
