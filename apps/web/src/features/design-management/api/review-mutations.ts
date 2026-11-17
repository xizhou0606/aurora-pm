import { useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { designReviewKeys } from './query-keys'
import { collectorClient } from './collectors-client'
import { reviewClient } from './reviews-client'
import type { CreateReviewInput, DesignReviewDetail, NewReviewSubmission } from '../model'

type ReviewCreationResume = {
  fingerprint: string
  review: DesignReviewDetail
}

type ResumeReference = { current?: ReviewCreationResume }

function creationFingerprint(input: CreateReviewInput): string {
  return JSON.stringify(input)
}

export async function executeReviewCreation(
  input: NewReviewSubmission,
  resume: ResumeReference,
): Promise<DesignReviewDetail> {
  const { implementationFile, implementationOriginalFile, implementationStructureFile, implementationStructureOriginalFile, normalizationManifest, mobileCaptureId, ...body } = input
  const fingerprint = creationFingerprint(body)
  let review: DesignReviewDetail
  if (resume.current?.fingerprint === fingerprint) {
    review = await reviewClient.detail(resume.current.review.id, undefined)
    if (review.status !== 'draft') {
      resume.current = undefined
      return review
    }
    resume.current.review = review
  } else {
    resume.current = undefined
    if (mobileCaptureId) await collectorClient.consumeCapture(mobileCaptureId)
    review = body.source_review_id
      ? await reviewClient.createRecheck(body.source_review_id, body)
      : await reviewClient.create(body)
    resume.current = { fingerprint, review }
  }
  await reviewClient.uploadAsset(review.id, 'implementation_original', implementationOriginalFile ?? implementationFile)
  if (implementationOriginalFile) await reviewClient.uploadAsset(review.id, 'implementation_normalized', implementationFile)
  await reviewClient.uploadAsset(review.id, 'implementation_structure', implementationStructureFile)
  if (implementationStructureOriginalFile) await reviewClient.uploadAsset(review.id, 'implementation_structure_original', implementationStructureOriginalFile)
  if (normalizationManifest) await reviewClient.uploadAsset(review.id, 'dom_normalization_manifest', normalizationManifest)
  await reviewClient.queueAnalysis(review.id)
  resume.current = undefined
  return review
}

export function useCreateReview(workspaceId: number) {
  const queryClient = useQueryClient()
  const resume = useRef<ReviewCreationResume | undefined>(undefined)
  return useMutation({
    mutationFn: (input: NewReviewSubmission) => executeReviewCreation(input, resume),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: designReviewKeys.root(workspaceId) })
    },
  })
}

export function useFigmaPreview() {
  return useMutation({ mutationFn: (figmaUrl: string) => reviewClient.figmaPreview(figmaUrl) })
}

export function useDOMNormalization() {
  return useMutation({ mutationFn: ({ figmaUrl, inventory }: { figmaUrl: string; inventory: Record<string, unknown> }) => reviewClient.normalizeDOM(figmaUrl, inventory) })
}

export function useReviewLifecycleMutations(workspaceId: number, reviewId: string) {
  const queryClient = useQueryClient()
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: designReviewKeys.detail(workspaceId, reviewId) }),
      queryClient.invalidateQueries({ queryKey: designReviewKeys.root(workspaceId) }),
    ])
  }
  return {
    retry: useMutation({ mutationFn: () => reviewClient.retryAnalysis(reviewId), onSuccess: refresh }),
    abandon: useMutation({ mutationFn: () => reviewClient.abandonAnalysis(reviewId), onSuccess: refresh }),
    remove: useMutation({ mutationFn: () => reviewClient.remove(reviewId), onSuccess: refresh }),
    exportFile: useMutation({
      mutationFn: async (format: 'markdown' | 'json') => ({
        blob: await reviewClient.exportFile(reviewId, format), format,
      }),
      onSuccess: ({ blob, format }) => {
        const href = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = href
        link.download = `design-review-${reviewId}.${format === 'markdown' ? 'md' : 'json'}`
        link.click()
        URL.revokeObjectURL(href)
      },
    }),
  }
}
