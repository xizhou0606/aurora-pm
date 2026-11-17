import type { DifferenceClient } from './contracts'
import { designReviewRequest } from './request'

export const differenceClient = Object.freeze({
  create: (reviewId, input) => designReviewRequest('POST', '/design-review-differences', {
    review_id: reviewId, ...input,
  }),
  update: (reviewId, id, input) => designReviewRequest('PATCH', `/design-review-differences/${id}`, {
    review_id: reviewId, ...input,
  }),
  remove: async (reviewId, id) => {
    await designReviewRequest('DELETE', `/design-review-differences/${id}?review_id=${encodeURIComponent(reviewId)}`)
  },
  verify: (reviewId, id, status) => designReviewRequest('POST', `/design-review-differences/${id}/verify`, {
    review_id: reviewId, status,
  }),
  resolve: (reviewId, id) => designReviewRequest('POST', `/design-review-differences/${id}/resolve`, { review_id: reviewId }),
  reopen: (reviewId, id) => designReviewRequest('POST', `/design-review-differences/${id}/reopen`, { review_id: reviewId }),
  createDeepReview: (input) => designReviewRequest('POST', '/design-review-deep-reviews', input),
  retryDeepReview: async (reviewId, id) => {
    await designReviewRequest('POST', `/design-review-deep-reviews/${id}/retry`, { review_id: reviewId })
  },
  cancelDeepReview: async (reviewId, id) => {
    await designReviewRequest('POST', `/design-review-deep-reviews/${id}/cancel`, { review_id: reviewId })
  },
  resolveRecheckMatch: async (reviewId, id, resolution) => {
    await designReviewRequest('PATCH', `/design-review-differences/recheck-matches/${id}`, { review_id: reviewId, resolution })
  },
  overrideRecheckVerification: async (reviewId, id, status) => {
    await designReviewRequest('PATCH', `/design-review-differences/recheck-verifications/${id}`, { review_id: reviewId, status })
  },
} satisfies DifferenceClient)
