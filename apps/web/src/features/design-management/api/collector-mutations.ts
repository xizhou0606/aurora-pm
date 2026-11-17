import { useMutation, useQueryClient } from '@tanstack/react-query'

import { collectorClient } from './collectors-client'
import { designReviewKeys } from './query-keys'
import type { CreateMobileCaptureInput } from '../model'

export function useCollectorMutations(workspaceId: number) {
  const queryClient = useQueryClient()
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: designReviewKeys.collectors(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: designReviewKeys.captures(workspaceId) }),
    ])
  }
  return {
    pair: useMutation({ mutationFn: () => collectorClient.createPairingCode() }),
    createCapture: useMutation({ mutationFn: (input: CreateMobileCaptureInput) => collectorClient.createCapture(input), onSuccess: refresh }),
    captureAsset: useMutation({ mutationFn: (id: string) => collectorClient.captureAsset(id) }),
    consumeCapture: useMutation({ mutationFn: (id: string) => collectorClient.consumeCapture(id), onSuccess: refresh }),
    revoke: useMutation({ mutationFn: (id: string) => collectorClient.revoke(id), onSuccess: refresh }),
    removeCapture: useMutation({ mutationFn: (id: string) => collectorClient.removeCapture(id), onSuccess: refresh }),
  }
}
