import { QueryClient } from '@tanstack/react-query'

export function createLandingQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        gcTime: Infinity,
        refetchOnMount: false,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
        retry: false,
        staleTime: Infinity,
      },
      mutations: { retry: false },
    },
  })
}

export async function ignoreLandingNavigate() {}
