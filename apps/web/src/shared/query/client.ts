import { MutationCache, QueryClient } from '@tanstack/react-query'

import {
  notifyOperationError,
  notifyOperationSuccess,
} from '@/shared/ui'

export function createPMQueryClient(): QueryClient {
  return new QueryClient({
    mutationCache: new MutationCache({
      onSuccess: (_data, _variables, _context, mutation) => {
        notifyOperationSuccess(mutation.meta)
      },
      onError: (error, _variables, _context, mutation) => {
        notifyOperationError(mutation.meta, error)
      },
    }),
  })
}

export const queryClient = createPMQueryClient()
