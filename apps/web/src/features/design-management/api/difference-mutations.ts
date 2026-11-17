import { useMutation, useQueryClient } from '@tanstack/react-query'

import { differenceClient } from './differences-client'
import { designReviewKeys } from './query-keys'
import type { DifferenceInput, HistoricalRecheckStatus, Region } from '../model'

export function useDifferenceMutations(workspaceId: number, reviewId: string) {
  const queryClient = useQueryClient()
  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: designReviewKeys.detail(workspaceId, reviewId) })
  }
  return {
    create: useMutation({ mutationFn: (input: DifferenceInput) => differenceClient.create(reviewId, input), onSuccess: refresh }),
    update: useMutation({ mutationFn: ({ id, input }: { id: string; input: DifferenceInput }) => differenceClient.update(reviewId, id, input), onSuccess: refresh }),
    remove: useMutation({ mutationFn: (id: string) => differenceClient.remove(reviewId, id), onSuccess: refresh }),
    verify: useMutation({ mutationFn: ({ id, status }: { id: string; status: 'confirmed' | 'false_positive' }) => differenceClient.verify(reviewId, id, status), onSuccess: refresh }),
    resolve: useMutation({ mutationFn: (id: string) => differenceClient.resolve(reviewId, id), onSuccess: refresh }),
    reopen: useMutation({ mutationFn: (id: string) => differenceClient.reopen(reviewId, id), onSuccess: refresh }),
    deepReview: useMutation({
      mutationFn: (input: { designRegion: Region; implementationRegion: Region; focus: string }) => differenceClient.createDeepReview({
        review_id: reviewId,
        design_region: input.designRegion,
        implementation_region: input.implementationRegion,
        focus: input.focus,
      }),
      onSuccess: refresh,
    }),
    retryDeepReview: useMutation({ mutationFn: (id: string) => differenceClient.retryDeepReview(reviewId, id), onSuccess: refresh }),
    cancelDeepReview: useMutation({ mutationFn: (id: string) => differenceClient.cancelDeepReview(reviewId, id), onSuccess: refresh }),
    resolveRecheckMatch: useMutation({ mutationFn: ({ id, resolution }: { id: string; resolution: 'inherit' | 'new' }) => differenceClient.resolveRecheckMatch(reviewId, id, resolution), onSuccess: refresh }),
    overrideRecheckVerification: useMutation({ mutationFn: ({ id, status }: { id: string; status: HistoricalRecheckStatus | null }) => differenceClient.overrideRecheckVerification(reviewId, id, status), onSuccess: refresh }),
  }
}
