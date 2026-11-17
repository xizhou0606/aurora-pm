import { queryOptions, useQuery } from '@tanstack/react-query'
import { collectorClient } from './collectors-client'
import { designReviewKeys } from './query-keys'
import { reviewClient } from './reviews-client'
import type { ReviewListInput } from '../model'

export function reviewCapabilitiesQueryOptions(workspaceId: number) {
  return queryOptions({
    queryKey: designReviewKeys.capabilities(workspaceId),
    queryFn: ({ signal }) => reviewClient.capabilities(signal),
    staleTime: 60_000,
  })
}

export function reviewRequirementsQueryOptions(workspaceId: number, enabled = true) {
  return queryOptions({
    queryKey: designReviewKeys.requirements(workspaceId),
    queryFn: ({ signal }) => reviewClient.requirements(signal),
    staleTime: 60_000,
    enabled,
  })
}

export function reviewListQueryOptions(workspaceId: number, input: ReviewListInput, enabled = true) {
  return queryOptions({
    queryKey: designReviewKeys.list(workspaceId, input),
    queryFn: ({ signal }) => reviewClient.list(input, signal),
    enabled,
  })
}

export function reviewDetailQueryOptions(workspaceId: number, id: string, enabled = true) {
  return queryOptions({
    queryKey: designReviewKeys.detail(workspaceId, id),
    queryFn: ({ signal }) => reviewClient.detail(id, signal),
    enabled: Boolean(id) && enabled,
    refetchInterval: ({ state }) => (
      ['queued', 'analyzing'].includes(state.data?.status ?? '')
      || state.data?.deep_review_jobs.some((job) => job.status === 'pending' || job.status === 'running')
    ) ? 3_000 : false,
  })
}

export const useReviewCapabilities = (workspaceId: number) => useQuery(reviewCapabilitiesQueryOptions(workspaceId))
export const useReviewRequirements = (workspaceId: number, enabled = true) => useQuery(reviewRequirementsQueryOptions(workspaceId, enabled))
export const useReviewList = (workspaceId: number, input: ReviewListInput, enabled = true) => useQuery(reviewListQueryOptions(workspaceId, input, enabled))
export const useReviewDetail = (workspaceId: number, id: string, enabled = true) => useQuery(reviewDetailQueryOptions(workspaceId, id, enabled))
export const useReviewAsset = (workspaceId: number, id: string, assetId?: string) => useQuery({
  queryKey: designReviewKeys.asset(workspaceId, id, assetId ?? 'missing'),
  queryFn: ({ signal }) => reviewClient.asset(id, assetId!, signal),
  enabled: Boolean(id && assetId),
  staleTime: Number.POSITIVE_INFINITY,
})
export const useCollectors = (workspaceId: number, enabled = true) => useQuery({
  queryKey: designReviewKeys.collectors(workspaceId),
  queryFn: ({ signal }) => collectorClient.list(signal),
  enabled,
  refetchInterval: 10_000,
})
export const useCaptureDrafts = (workspaceId: number, enabled = true) => useQuery({
  queryKey: designReviewKeys.captures(workspaceId),
  queryFn: ({ signal }) => collectorClient.captures(signal),
  enabled,
  refetchInterval: ({ state }) => state.data?.some((item) => item.status === 'queued' || item.status === 'claimed') ? 2_000 : false,
})
